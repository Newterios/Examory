// Command grading is the Examory grading service: risk signals and the
// teacher review workflow. This file is the composition root only.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	httpadapter "github.com/Newterios/Examory/services/grading/internal/adapters/http"
	"github.com/Newterios/Examory/services/grading/internal/adapters/postgres"
	"github.com/Newterios/Examory/services/grading/internal/adapters/schemas"
	"github.com/Newterios/Examory/services/grading/internal/app"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/config"
	"github.com/Newterios/Examory/services/platform/consumer"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/Newterios/Examory/services/platform/redisx"
)

type settings struct {
	httpAddr, dbURL, redisAddr, redisPassword string
	kafka                                     []string
	jwksURL, issuer, audience, group          string
	trustedProxies, corsOrigins               []string
	logLevel                                  string
	devMode                                   bool
	perIP, perSubject                         int
}

func load() (settings, error) {
	e := config.New(os.LookupEnv)
	s := settings{
		httpAddr:       e.String("GRADING_HTTP_ADDR", ":8080"),
		dbURL:          e.Secret("GRADING_DATABASE_URL"),
		redisAddr:      e.Required("GRADING_REDIS_ADDR"),
		redisPassword:  e.OptionalSecret("GRADING_REDIS_PASSWORD"),
		kafka:          e.List("GRADING_KAFKA_BROKERS", nil),
		jwksURL:        e.URL("GRADING_JWKS_URL", ""),
		issuer:         e.String("GRADING_JWT_ISSUER", "examory-auth"),
		audience:       e.String("GRADING_JWT_AUDIENCE", "examory"),
		group:          e.String("GRADING_CONSUMER_GROUP", "grading"),
		trustedProxies: e.List("GRADING_TRUSTED_PROXIES", nil),
		corsOrigins:    e.List("GRADING_CORS_ORIGINS", nil),
		logLevel:       e.Enum("GRADING_LOG_LEVEL", "info", "debug", "info", "warn", "error"),
		devMode:        e.Bool("GRADING_DEV_MODE", false),
		perIP:          e.Int("GRADING_RATE_LIMIT_PER_IP", 600, config.IntRange{Min: 10, Max: 100000}),
		perSubject:     e.Int("GRADING_RATE_LIMIT_PER_SUBJECT", 300, config.IntRange{Min: 10, Max: 100000}),
	}
	e.Check(len(s.kafka) > 0, "GRADING_KAFKA_BROKERS", "is required")
	return s, e.Err()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		port := "8080"
		if _, p, err := net.SplitHostPort(os.Getenv("GRADING_HTTP_ADDR")); err == nil && p != "" {
			port = p
		}
		if err := httpx.Probe("http://127.0.0.1:" + port + "/healthz"); err != nil {
			fmt.Fprintln(os.Stderr, "unhealthy:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "grading:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	logger := logging.New(os.Stdout, "grading", cfg.logLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, db.Config{URL: cfg.dbURL, ApplicationName: "grading", MaxConns: 15})
	if err != nil {
		return err
	}
	defer pool.Close()
	mctx, mcancel := context.WithTimeout(ctx, 60*time.Second)
	err = migrate.Up(mctx, pool, logger, postgres.Migrations()...)
	mcancel()
	if err != nil {
		return err
	}
	validator, err := schemas.Validator()
	if err != nil {
		return err
	}
	rdb := redisx.New(redisx.Config{Addr: cfg.redisAddr, Password: cfg.redisPassword})
	defer rdb.Close()
	clk := clock.System{}
	limiter := &ratelimit.Fallback{Primary: ratelimit.NewRedis(rdb, "grading:rl:"), Secondary: ratelimit.NewMemory(clk), Logger: logger}
	jwks := authn.NewJWKSClient(cfg.jwksURL, authn.JWKSOptions{})
	verifier, err := authn.NewVerifier(jwks, authn.VerifierConfig{Issuer: cfg.issuer, Audience: cfg.audience})
	if err != nil {
		return err
	}
	svc := app.New(postgres.New(pool), validator, clk, logger, courseaccess.FromEnv())

	kcfg := kafkax.Config{Brokers: cfg.kafka, ClientID: "grading", AllowAutoTopicCreation: cfg.devMode, DeliveryTimeout: 8 * time.Second}
	producer, err := kafkax.NewProducer(kcfg)
	if err != nil {
		return err
	}
	defer producer.Close()
	relay := outbox.NewRelay(pool, producer, logger, outbox.RelayConfig{PublishTimeout: 8 * time.Second})
	cons, err := consumer.New(consumer.Config{
		Kafka: kcfg, Group: cfg.group, Topics: []string{events.TopicRisk, events.TopicQuizzes},
		Rejected: func(ctx context.Context, r consumer.RejectedRecord) {
			svc.RecordRejected(ctx, app.Rejected{Topic: r.Topic, Partition: r.Partition, Offset: r.Offset,
				EventID: r.EventID, EventType: r.EventType, Reason: r.Reason, Excerpt: string(r.Value)})
		},
	}, pool, func(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
		return svc.Ingest(ctx, postgres.Bind(tx), env, consumer.TopicFromContext(ctx))
	}, logger)
	if err != nil {
		return err
	}

	engine, err := httpx.NewEngine(httpx.EngineConfig{TrustedProxies: cfg.trustedProxies, MaxBodyBytes: 64 << 10, CORSOrigins: cfg.corsOrigins, Logger: logger})
	if err != nil {
		return err
	}
	httpx.RegisterHealth(engine, logger, map[string]httpx.CheckFunc{"postgres": pool.Ping, "redis": redisx.Ping(rdb), "jwks": jwks.Ready})
	httpadapter.Register(engine, httpadapter.Deps{Service: svc, Verifier: verifier, Limiter: limiter, Logger: logger,
		PerIP: cfg.perIP, PerSubject: cfg.perSubject})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = relay.Run(ctx) }()
	go func() {
		defer wg.Done()
		if err := cons.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("consumer stopped", slog.String("error", err.Error()))
		}
	}()
	err = httpx.Serve(ctx, httpx.ServerConfig{Addr: cfg.httpAddr}, engine, logger)
	stop()
	wg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	logger.Info("grading stopped")
	return nil
}
