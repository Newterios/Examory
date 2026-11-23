package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Newterios/Examory/services/notifications/internal/inbox"
	"github.com/Newterios/Examory/services/platform/authn"
	"github.com/Newterios/Examory/services/platform/config"
	"github.com/Newterios/Examory/services/platform/consumer"
	"github.com/Newterios/Examory/services/platform/courseaccess"
	"github.com/Newterios/Examory/services/platform/db"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/httpx"
	"github.com/Newterios/Examory/services/platform/kafkax"
	"github.com/Newterios/Examory/services/platform/logging"
	"github.com/Newterios/Examory/services/platform/migrate"
	"github.com/gin-gonic/gin"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var e error
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		e = httpx.Probe("http://127.0.0.1:8080/healthz")
	} else {
		e = run()
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "notifications:", e)
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg := config.New(os.LookupEnv)
	dsn := cfg.Secret("NOTIFICATIONS_DATABASE_URL")
	keysURL := cfg.URL("NOTIFICATIONS_JWKS_URL", "")
	brokers := cfg.List("NOTIFICATIONS_KAFKA_BROKERS", nil)
	cfg.Check(len(brokers) > 0, "NOTIFICATIONS_KAFKA_BROKERS", "required")
	if e := cfg.Err(); e != nil {
		return e
	}
	log := logging.New(os.Stdout, "notifications", "info")
	pool, e := db.Open(ctx, db.Config{URL: dsn, ApplicationName: "notifications"})
	if e != nil {
		return e
	}
	defer pool.Close()
	if e = migrate.Up(ctx, pool, log, inbox.Migrations()...); e != nil {
		return e
	}
	keys := authn.NewJWKSClient(keysURL, authn.JWKSOptions{})
	v, e := authn.NewVerifier(keys, authn.VerifierConfig{Issuer: "examory-auth", Audience: "examory"})
	if e != nil {
		return e
	}
	eng, e := httpx.NewEngine(httpx.EngineConfig{Logger: log, MaxBodyBytes: 1024})
	if e != nil {
		return e
	}
	store := inbox.Store{Pool: pool, Courses: courseaccess.FromEnv()}
	httpx.RegisterHealth(eng, log, map[string]httpx.CheckFunc{"postgres": pool.Ping, "jwks": keys.Ready})
	eng.GET("/v1/notifications", httpx.Authenticate(v), httpx.RequireRoles(authn.RoleStudent), func(c *gin.Context) {
		n, cursor, ok := httpx.Page(c, 20)
		if !ok {
			return
		}
		p, _ := httpx.Principal(c)
		page, next, e := store.List(c.Request.Context(), p, cursor, n)
		if errors.Is(e, courseaccess.ErrUnavailable) {
			httpx.Fail(c, 503, "unavailable", "cannot confirm current enrollment")
			return
		}
		if e != nil {
			httpx.Internal(c, log, e)
			return
		}
		c.JSON(200, gin.H{"items": page, "next_cursor": next})
	})
	group, e := consumer.New(consumer.Config{Kafka: kafkax.Config{Brokers: brokers, ClientID: "notifications"}, Group: "notifications-v1", Topics: []string{events.TopicGrades, events.TopicCourses, events.TopicQuizzes, events.TopicSubmissions}}, pool, inbox.Handle, log)
	if e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- group.Run(ctx); stop() }()
	e = httpx.Serve(ctx, httpx.ServerConfig{Addr: ":8080"}, eng, log)
	stop()
	ce := <-done
	if ce != nil && !errors.Is(ce, context.Canceled) {
		return ce
	}
	return e
}
