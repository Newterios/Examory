// Command auth is the Examory authentication service. This file is the
// composition root only: configuration, wiring and lifecycle.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	httpadapter "github.com/Newterios/Examory/services/auth/internal/adapters/http"
	"github.com/Newterios/Examory/services/auth/internal/adapters/postgres"
	redisadapter "github.com/Newterios/Examory/services/auth/internal/adapters/redis"
	"github.com/Newterios/Examory/services/auth/internal/app"
	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/auth/internal/password"
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
)

type settings struct {
	httpAddr, dbURL, redisAddr, redisPassword string
	kafka                                     []string
	issuer, audience, kid                     string
	signingSeed                               string
	extraKeys                                 []string
	devMode, cookieSecure                     bool
	bootstrapEmail, bootstrapPassword         string
	trustedProxies, corsOrigins               []string
	logLevel                                  string
	accessTTL, refreshTTL, familyMaxAge       time.Duration
	hashConcurrency                           int
}

func load() (settings, error) {
	e := config.New(os.LookupEnv)
	s := settings{
		httpAddr:          e.String("AUTH_HTTP_ADDR", ":8080"),
		dbURL:             e.Secret("AUTH_DATABASE_URL"),
		redisAddr:         e.Required("AUTH_REDIS_ADDR"),
		redisPassword:     e.OptionalSecret("AUTH_REDIS_PASSWORD"),
		kafka:             e.List("AUTH_KAFKA_BROKERS", nil),
		issuer:            e.String("AUTH_JWT_ISSUER", "examory-auth"),
		audience:          e.String("AUTH_JWT_AUDIENCE", "examory"),
		kid:               e.String("AUTH_JWT_KID", ""),
		signingSeed:       e.OptionalSecret("AUTH_JWT_SIGNING_KEY"),
		extraKeys:         e.List("AUTH_JWT_EXTRA_PUBLIC_KEYS", nil),
		devMode:           e.Bool("AUTH_DEV_MODE", false),
		cookieSecure:      e.Bool("AUTH_COOKIE_SECURE", true),
		bootstrapEmail:    e.String("AUTH_BOOTSTRAP_ADMIN_EMAIL", ""),
		bootstrapPassword: e.OptionalSecret("AUTH_BOOTSTRAP_ADMIN_PASSWORD"),
		trustedProxies:    e.List("AUTH_TRUSTED_PROXIES", nil),
		corsOrigins:       e.List("AUTH_CORS_ORIGINS", nil),
		logLevel:          e.Enum("AUTH_LOG_LEVEL", "info", "debug", "info", "warn", "error"),
		accessTTL:         e.Duration("AUTH_ACCESS_TTL", 10*time.Minute, config.DurationRange{Min: time.Minute, Max: time.Hour}),
		refreshTTL:        e.Duration("AUTH_REFRESH_TTL", 7*24*time.Hour, config.DurationRange{Min: time.Hour, Max: 30 * 24 * time.Hour}),
		familyMaxAge:      e.Duration("AUTH_SESSION_MAX_AGE", 30*24*time.Hour, config.DurationRange{Min: time.Hour, Max: 90 * 24 * time.Hour}),
		hashConcurrency:   e.Int("AUTH_HASH_CONCURRENCY", 4, config.IntRange{Min: 1, Max: 64}),
	}
	e.Check(len(s.kafka) > 0, "AUTH_KAFKA_BROKERS", "is required")
	e.Check(s.signingSeed != "" || s.devMode, "AUTH_JWT_SIGNING_KEY", "is required unless AUTH_DEV_MODE=true")
	e.Check((s.bootstrapEmail == "") == (s.bootstrapPassword == ""), "AUTH_BOOTSTRAP_ADMIN_PASSWORD", "must be set together with AUTH_BOOTSTRAP_ADMIN_EMAIL")
	e.Check(s.familyMaxAge >= s.refreshTTL, "AUTH_SESSION_MAX_AGE", "must be >= AUTH_REFRESH_TTL")
	return s, e.Err()
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := httpx.Probe(probeURL(os.Getenv("AUTH_HTTP_ADDR"))); err != nil {
			fmt.Fprintln(os.Stderr, "unhealthy:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "auth:", err)
		os.Exit(1)
	}
}

func probeURL(addr string) string {
	port := "8080"
	if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
		port = p
	}
	return "http://127.0.0.1:" + port + "/healthz"
}

func signingKey(s settings, log *slog.Logger) (ed25519.PrivateKey, error) {
	if s.signingSeed == "" {
		log.Warn("!!! AUTH_JWT_SIGNING_KEY is not set: generated an EPHEMERAL signing key (AUTH_DEV_MODE=true). " +
			"All tokens become invalid on restart and replicas cannot share sessions. NEVER run like this in production. !!!")
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		return priv, err
	}
	seed, err := base64.StdEncoding.DecodeString(s.signingSeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("AUTH_JWT_SIGNING_KEY must be base64 of a 32-byte Ed25519 seed")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func parseExtraKeys(list []string) (map[string]ed25519.PublicKey, error) {
	out := map[string]ed25519.PublicKey{}
	for _, item := range list {
		kid, b64, ok := strings.Cut(item, "=")
		raw, err := base64.RawURLEncoding.DecodeString(b64)
		if !ok || kid == "" || err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, errors.New("AUTH_JWT_EXTRA_PUBLIC_KEYS must be a comma-separated list of kid=base64url(Ed25519 public key)")
		}
		out[kid] = ed25519.PublicKey(raw)
	}
	return out, nil
}

type tokenIssuer struct{ s *authn.Signer }

func (t tokenIssuer) Issue(userID string, role domain.Role, groupID string) (string, time.Time, error) {
	return t.s.Issue(userID, authn.Role(role), groupID)
}

func run() error {
	cfg, err := load()
	if err != nil {
		return err
	}
	log := logging.New(os.Stdout, "auth", cfg.logLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	clk := clock.System{}

	priv, err := signingKey(cfg, log)
	if err != nil {
		return err
	}
	pub := priv.Public().(ed25519.PublicKey)
	kid := cfg.kid
	if kid == "" {
		kid = authn.Thumbprint(pub)
	}
	extra, err := parseExtraKeys(cfg.extraKeys)
	if err != nil {
		return err
	}
	keys := authn.StaticKeys{kid: pub}
	jwks := authn.JWKSet{Keys: []authn.JWK{authn.PublicJWK(kid, pub)}}
	for k, p := range extra {
		if k == kid {
			continue
		}
		keys[k] = p
		jwks.Keys = append(jwks.Keys, authn.PublicJWK(k, p))
	}
	signer, err := authn.NewSigner(kid, priv, cfg.issuer, cfg.audience, cfg.accessTTL, clk)
	if err != nil {
		return err
	}
	verifier, err := authn.NewVerifier(keys, authn.VerifierConfig{Issuer: cfg.issuer, Audience: cfg.audience, Clock: clk})
	if err != nil {
		return err
	}

	startCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	pool, err := db.Open(startCtx, db.Config{URL: cfg.dbURL, ApplicationName: "auth"})
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := migrate.Up(startCtx, pool, log, postgres.Migrations(), outbox.Migrations()); err != nil {
		return err
	}
	rdb := redisx.New(redisx.Config{Addr: cfg.redisAddr, Password: cfg.redisPassword})
	defer rdb.Close()
	producer, err := kafkax.NewProducer(kafkax.Config{Brokers: cfg.kafka, ClientID: "auth", AllowAutoTopicCreation: cfg.devMode})
	if err != nil {
		return err
	}
	defer producer.Close()

	svc, err := app.New(startCtx, app.Deps{
		Store:    postgres.New(pool),
		Sessions: redisadapter.NewRefreshStore(rdb, "auth:", cfg.refreshTTL, cfg.familyMaxAge),
		Lockout:  redisadapter.NewLockout(rdb, "auth:"),
		Hasher:   password.New(password.OWASP, cfg.hashConcurrency),
		Tokens:   tokenIssuer{signer},
		Clock:    clk,
		Logger:   log,
		Policy:   domain.DefaultLockout,
	})
	if err != nil {
		return err
	}
	if cfg.bootstrapEmail != "" {
		created, err := svc.EnsureBootstrapAdmin(startCtx, cfg.bootstrapEmail, cfg.bootstrapPassword)
		if err != nil {
			return fmt.Errorf("bootstrap admin: %w", err)
		}
		if created {
			log.Warn("bootstrap admin account created; change its password and remove AUTH_BOOTSTRAP_ADMIN_PASSWORD from the environment")
		}
	}

	limiter := &ratelimit.Fallback{Primary: ratelimit.NewRedis(rdb, "auth:rl:"), Secondary: ratelimit.NewMemory(clk), Logger: log}
	router, err := httpadapter.NewRouter(svc, verifier, httpadapter.Config{
		Engine:       httpx.EngineConfig{TrustedProxies: cfg.trustedProxies, MaxBodyBytes: 64 << 10, Logger: log, HSTS: cfg.cookieSecure && !cfg.devMode},
		CookieSecure: cfg.cookieSecure,
		RefreshTTL:   cfg.refreshTTL,
		AllowOrigins: cfg.corsOrigins,
		JWKS:         jwks,
		Limiter:      limiter,
		Checks: map[string]httpx.CheckFunc{
			"postgres": pool.Ping,
			"redis":    redisx.Ping(rdb),
		},
	})
	if err != nil {
		return err
	}

	relay := outbox.NewRelay(pool, producer, log, outbox.RelayConfig{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = relay.Run(ctx) }()
	err = httpx.Serve(ctx, httpx.ServerConfig{Addr: cfg.httpAddr}, router, log)
	stop()
	wg.Wait()
	return err
}
