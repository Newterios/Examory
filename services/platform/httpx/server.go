package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// EngineConfig configures NewEngine.
type EngineConfig struct {
	// TrustedProxies are the CIDRs/IPs whose X-Forwarded-For is honoured
	// (the gateway). Empty means no proxy is trusted and ClientIP is the
	// TCP peer address.
	TrustedProxies []string
	// MaxBodyBytes caps request bodies (default 1 MiB).
	MaxBodyBytes int64
	// CORSOrigins is the exact-match origin allow-list (empty: no CORS).
	CORSOrigins     []string
	CORSCredentials bool
	// HSTS adds Strict-Transport-Security (enable when served over TLS).
	HSTS   bool
	Logger *slog.Logger
}

// NewEngine returns a Gin engine in release mode with the standard
// middleware chain: request id, panic recovery, security headers, access
// log, CORS and body limit. Unknown routes and methods return problems.
func NewEngine(cfg EngineConfig) (*gin.Engine, error) {
	if cfg.Logger == nil {
		return nil, errors.New("httpx: logger is required")
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = DefaultMaxJSONBytes
	}
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	if len(cfg.TrustedProxies) == 0 {
		if err := e.SetTrustedProxies(nil); err != nil {
			return nil, err
		}
		e.ForwardedByClientIP = false
	} else {
		if err := e.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			return nil, fmt.Errorf("httpx: trusted proxies: %w", err)
		}
		e.ForwardedByClientIP = true
		e.RemoteIPHeaders = []string{"X-Forwarded-For"}
	}
	e.HandleMethodNotAllowed = true
	e.RedirectTrailingSlash = false
	e.RedirectFixedPath = false
	e.Use(RequestID(), Recovery(cfg.Logger), SecurityHeaders(cfg.HSTS), AccessLog(cfg.Logger),
		CORS(cfg.CORSOrigins, cfg.CORSCredentials), BodyLimit(cfg.MaxBodyBytes))
	e.NoRoute(func(c *gin.Context) { Fail(c, http.StatusNotFound, "not_found", "resource not found") })
	e.NoMethod(func(c *gin.Context) { Fail(c, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed") })
	return e, nil
}

// CheckFunc is a readiness dependency check.
type CheckFunc func(ctx context.Context) error

// RegisterHealth adds GET /healthz (liveness, no dependencies) and
// GET /readyz (runs every check with a 2s timeout). Check errors are logged,
// never returned to the caller.
func RegisterHealth(e *gin.Engine, logger *slog.Logger, checks map[string]CheckFunc) {
	e.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	e.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		status, results := http.StatusOK, map[string]string{}
		for name, check := range checks {
			if err := check(ctx); err != nil {
				logger.WarnContext(ctx, "readiness check failed", slog.String("check", name), slog.String("error", err.Error()))
				results[name], status = "unavailable", http.StatusServiceUnavailable
			} else {
				results[name] = "ok"
			}
		}
		word := "ok"
		if status != http.StatusOK {
			word = "unavailable"
		}
		c.JSON(status, gin.H{"status": word, "checks": results})
	})
}

// ServerConfig holds HTTP server limits. Zero values use safe defaults.
type ServerConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration // default 5s
	ReadTimeout       time.Duration // default 15s
	WriteTimeout      time.Duration // default 30s
	IdleTimeout       time.Duration // default 120s
	MaxHeaderBytes    int           // default 16 KiB
	ShutdownTimeout   time.Duration // default 20s
}

func (c *ServerConfig) defaults() {
	if c.ReadHeaderTimeout <= 0 {
		c.ReadHeaderTimeout = 5 * time.Second
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 15 * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = 120 * time.Second
	}
	if c.MaxHeaderBytes <= 0 {
		c.MaxHeaderBytes = 16 << 10
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 20 * time.Second
	}
}

// Serve runs an HTTP server until ctx is cancelled, then shuts it down
// gracefully within ShutdownTimeout.
func Serve(ctx context.Context, cfg ServerConfig, h http.Handler, logger *slog.Logger) error {
	cfg.defaults()
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
	}
	errc := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", cfg.Addr))
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	logger.Info("http server shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Probe performs a GET against url and returns an error unless it answers
// 200 within 3s. It backs the "healthcheck" sub-command of distroless images.
func Probe(url string) error {
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(url) //nolint:noctx // short-lived CLI probe with client timeout
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
