// Package redisx builds Redis clients with explicit timeouts.
package redisx

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config configures New.
type Config struct {
	Addr     string
	Password string
	DB       int
	TLS      bool
	PoolSize int // default 10
}

// New returns a client with 2s dial and 1s read/write timeouts.
func New(cfg Config) *redis.Client {
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = 10
	}
	o := &redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		PoolSize:     cfg.PoolSize,
		MaxRetries:   1,
	}
	if cfg.TLS {
		o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return redis.NewClient(o)
}

// Ping returns a readiness check for c.
func Ping(c redis.UniversalClient) func(ctx context.Context) error {
	return func(ctx context.Context) error { return c.Ping(ctx).Err() }
}
