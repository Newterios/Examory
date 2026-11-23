// Command quizzes is the Examory quizzes service. This file is the
// composition root only: configuration, wiring and lifecycle.
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

	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/config"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/Newterios/Examory/services/platform/outbox"
	"github.com/Newterios/Examory/services/platform/ratelimit"
	"github.com/Newterios/Examory/services/platform/redisx"
	httpadapter "github.com/Newterios/Examory/services/quizzes/internal/adapters/http"
	"github.com/Newterios/Examory/services/quizzes/internal/adapters/postgres"
	"github.com/Newterios/Examory/services/quizzes/internal/app"
)

type settings struct {
	httpAddr, dbURL, redisAddr, redisPassword string
	kafka                                     []string
	jwksURL, issuer, audience                 string
	simClock, devMode                         bool
	trustedProxies, corsOrigins               []string
	logLevel                                  string
	grace, sweepInterval                      time.Duration
	perIP, perSubject                         int
}

func load() (settings, error) {
	e := config.New(os.LookupEnv)
	s := settings{
		httpAddr:       e.String("QUIZZES_HTTP_ADDR", ":8080"),
		dbURL:          e.Secret("QUIZZES_DATABASE_URL"),
		redisAddr:      e.Required("QUIZZES_REDIS_ADDR"),
		redisPassword:  e.OptionalSecret("QUIZZES_REDIS_PASSWORD"),
		kafka:          e.List("QUIZZES_KAFKA_BROKERS", nil),
		jwksURL:        e.URL("QUIZZES_JWKS_URL", ""),
		issuer:         e.String("QUIZZES_JWT_ISSUER", "examory-auth"),
		audience:       e.String("QUIZZES_JWT_AUDIENCE", "examory"),
		simClock:       e.Enum("QUIZZES_SIM_CLOCK", "disabled", "enabled", "disabled") == "enabled",
		devMode:        e.Bool("QUIZZES_DEV_MODE", false),
		trustedProxies: e.List("QUIZZES_TRUSTED_PROXIES", nil),
		corsOrigins:    e.List("QUIZZES_CORS_ORIGINS", nil),
		logLevel:       e.Enum("QUIZZES_LOG_LEVEL", "info", "debug", "info", "warn", "error"),
		grace:          e.Duration("QUIZZES_ANSWER_GRACE", 2*time.Second, config.DurationRange{Min: 0, Max: 30 * time.Second}),
		sweepInterval:  e.Duration("QUIZZES_SWEEP_INTERVAL", 5*time.Second, config.DurationRange{Min: time.Second, Max: 5 * time.Minute}),
		perIP:          e.Int("QUIZZES_RATE_LIMIT_PER_IP", 600, config.IntRange{Min: 10, Max: 100000}),
		perSubject:     e.Int("QUIZZES_RATE_LIMIT_PER_SUBJECT", 300, config.IntRange{Min: 10, Max: 100000}),
	}
	e.Check(len(s.kafka) > 0, "QUIZZES_KAFKA_BROKERS", "is required")
	return s, e.Err()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "quizzes:", err)
		os.Exit(1)
	}
}

func healthcheck() int {
	addr := os.Getenv("QUIZZES_HTTP_ADDR")
	_, port, err := net.SplitHostPort(addr)
	if addr == "" || err != nil || port == "" {
		port = "8080"
	}
	if err := httpx.Probe("http://127.0.0.1:" + port + "/healthz"); err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	return 0
}

func run() error {
	cfg, err := load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	logger := logging.New(os.Stdout, "quizzes", cfg.logLevel)
	if cfg.simClock {
		logger.Warn("SIMULATION CLOCK ENABLED: callers with the system role may set X-Sim-Time and act on behalf of students " +
			"(X-Act-As-User/X-Act-As-Group). This is a research-replay feature and must never be enabled in production.")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, db.Config{URL: cfg.dbURL, ApplicationName: "quizzes", MaxConns: 20})
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

	rdb := redisx.New(redisx.Config{Addr: cfg.redisAddr, Password: cfg.redisPassword})
	defer rdb.Close()
	clk := clock.System{}
	limiter := &ratelimit.Fallback{Primary: ratelimit.NewRedis(rdb, "quizzes:rl:"), Secondary: ratelimit.NewMemory(clk), Logger: logger}

	jwks := authn.NewJWKSClient(cfg.jwksURL, authn.JWKSOptions{})
	verifier, err := authn.NewVerifier(jwks, authn.VerifierConfig{Issuer: cfg.issuer, Audience: cfg.audience})
	if err != nil {
		return err
	}

	producer, err := kafkax.NewProducer(kafkax.Config{Brokers: cfg.kafka, ClientID: "quizzes", AllowAutoTopicCreation: cfg.devMode, DeliveryTimeout: 8 * time.Second})
	if err != nil {
		return err
	}
	defer producer.Close()
	relay := outbox.NewRelay(pool, producer, logger, outbox.RelayConfig{PublishTimeout: 8 * time.Second})

	svc := app.New(postgres.New(pool), clk, logger, app.Config{Access: courseaccess.FromEnv(), Grace: cfg.grace})

	engine, err := httpx.NewEngine(httpx.EngineConfig{TrustedProxies: cfg.trustedProxies, MaxBodyBytes: 512 << 10,
		CORSOrigins: cfg.corsOrigins, Logger: logger})
	if err != nil {
		return err
	}
	httpx.RegisterHealth(engine, logger, map[string]httpx.CheckFunc{
		"postgres": pool.Ping,
		"redis":    redisx.Ping(rdb),
		"jwks":     jwks.Ready,
	})
	httpadapter.Register(engine, httpadapter.Deps{
		Service: svc, Verifier: verifier, Limiter: limiter, Logger: logger, SimClock: cfg.simClock,
		Limits: httpadapter.RateLimits{PerIP: cfg.perIP, PerSubject: cfg.perSubject, Window: time.Minute},
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = relay.Run(ctx) }()
	go func() { defer wg.Done(); svc.RunSweeper(ctx, cfg.sweepInterval) }()
	err = httpx.Serve(ctx, httpx.ServerConfig{Addr: cfg.httpAddr}, engine, logger)
	stop()
	wg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	logger.Info("quizzes stopped", slog.String("reason", "signal"))
	return nil
}
