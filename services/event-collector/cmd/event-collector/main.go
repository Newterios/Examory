package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/Newterios/Examory/services/event-collector/internal/collector"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/config"
	"github.com/Newterios/Examory/services/platform/eventschema"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/Newterios/Examory/services/platform/redisx"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed schemas/*.json
var schemaFiles embed.FS

func main() {
	var e error
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		e = httpx.Probe("http://127.0.0.1:8080/healthz")
	} else {
		e = run()
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "event-collector:", e)
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg := config.New(os.LookupEnv)
	brokers := cfg.List("COLLECTOR_KAFKA_BROKERS", nil)
	cfg.Check(len(brokers) > 0, "COLLECTOR_KAFKA_BROKERS", "required")
	jwksURL := cfg.URL("COLLECTOR_JWKS_URL", "")
	quizzes := cfg.URL("COLLECTOR_QUIZZES_URL", "")
	redisAddr := cfg.Required("COLLECTOR_REDIS_ADDR")
	redisPass := cfg.OptionalSecret("COLLECTOR_REDIS_PASSWORD")
	if e := cfg.Err(); e != nil {
		return e
	}
	keys := authn.NewJWKSClient(jwksURL, authn.JWKSOptions{})
	v, e := authn.NewVerifier(keys, authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory"})
	if e != nil {
		return e
	}
	log := logging.New(os.Stdout, "event-collector", "info")
	eng, e := httpx.NewEngine(httpx.EngineConfig{Logger: log, MaxBodyBytes: 2048})
	if e != nil {
		return e
	}
	sub, e := fs.Sub(schemaFiles, "schemas")
	if e != nil {
		return e
	}
	validator, e := eventschema.LoadFS(sub)
	if e != nil {
		return e
	}
	pub, e := kafkax.NewProducer(kafkax.Config{Brokers: brokers, ClientID: "event-collector"})
	if e != nil {
		return e
	}
	defer pub.Close()
	redis := redisx.New(redisx.Config{Addr: redisAddr, Password: redisPass})
	defer redis.Close()
	httpx.RegisterHealth(eng, log, map[string]httpx.CheckFunc{"jwks": keys.Ready, "redis": redisx.Ping(redis), "kafka": pub.Ping})
	authority := collector.Authority{Base: quizzes, HTTP: &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	collector.Service{Quizzes: authority, Producer: pub, Validator: validator, Log: log}.Register(eng, httpx.Authenticate(v), ratelimit.NewRedis(redis, "events:"), 60)
	return httpx.Serve(ctx, httpx.ServerConfig{Addr: ":8080"}, eng, log)
}
