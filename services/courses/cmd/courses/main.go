package main

import (
	"context"
	"errors"
	"fmt"
	courseshttp "github.com/Newterios/Examory/services/courses/internal/adapters/http"
	"github.com/Newterios/Examory/services/courses/internal/adapters/postgres"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/config"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var err error
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		err = httpx.Probe("http://127.0.0.1:8080/healthz")
	} else {
		err = run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "courses:", err)
		os.Exit(1)
	}
}
func run() error {
	cfg := config.New(os.LookupEnv)
	url := cfg.Secret("COURSES_DATABASE_URL")
	token := cfg.Secret("COURSES_INTERNAL_TOKEN")
	brokers := cfg.List("COURSES_KAFKA_BROKERS", nil)
	cfg.Check(len(brokers) > 0, "COURSES_KAFKA_BROKERS", "is required")
	jwksURL := cfg.URL("COURSES_JWKS_URL", "")
	addr := cfg.String("COURSES_HTTP_ADDR", ":8080")
	if err := cfg.Err(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	log := logging.New(os.Stdout, "courses", "info")
	pool, err := db.Open(ctx, db.Config{URL: url, ApplicationName: "courses"})
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = migrate.Up(ctx, pool, log, postgres.Migrations()...); err != nil {
		return err
	}
	keys := authn.NewJWKSClient(jwksURL, authn.JWKSOptions{})
	v, err := authn.NewVerifier(keys, authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory"})
	if err != nil {
		return err
	}
	pub, err := kafkax.NewProducer(kafkax.Config{Brokers: brokers, ClientID: "courses", DeliveryTimeout: 8 * time.Second})
	if err != nil {
		return err
	}
	defer pub.Close()
	engine, err := httpx.NewEngine(httpx.EngineConfig{MaxBodyBytes: 512 << 10, Logger: log})
	if err != nil {
		return err
	}
	httpx.RegisterHealth(engine, log, map[string]httpx.CheckFunc{"postgres": pool.Ping, "jwks": keys.Ready})
	courseshttp.Handler{Store: postgres.New(pool), Logger: log}.Register(engine, httpx.Authenticate(v), token)
	relay := outbox.NewRelay(pool, pub, log, outbox.RelayConfig{})
	done := make(chan struct{})
	go func() { defer close(done); _ = relay.Run(ctx) }()
	err = httpx.Serve(ctx, httpx.ServerConfig{Addr: addr}, engine, log)
	stop()
	<-done
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
