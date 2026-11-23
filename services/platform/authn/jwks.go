package authn

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Newterios/Examory/services/platform/clock"
)

// JWKSClient fetches and caches the issuer's JWKS.
//
//   - Keys are cached for TTL. A token with an unknown kid triggers a refresh,
//     but at most once per MinRefreshInterval, so forged kids cannot be used
//     to hammer the issuer.
//   - If a refresh fails, previously cached keys keep working (the issuer may
//     be briefly down); a kid never seen before is rejected.
//   - The response body is capped at 64 KiB and the request has a timeout.
type JWKSClient struct {
	url        string
	http       *http.Client
	ttl        time.Duration
	minRefresh time.Duration
	clock      clock.Clock

	mu          sync.Mutex
	keys        map[string]ed25519.PublicKey
	fetchedAt   time.Time
	lastAttempt time.Time
}

// JWKSOptions configures a JWKSClient. Zero values use safe defaults.
type JWKSOptions struct {
	TTL                time.Duration // default 5m
	MinRefreshInterval time.Duration // default 30s
	Timeout            time.Duration // default 5s
	Clock              clock.Clock
	HTTPClient         *http.Client
}

// NewJWKSClient returns a client for the JWKS at url.
func NewJWKSClient(url string, o JWKSOptions) *JWKSClient {
	if o.TTL <= 0 {
		o.TTL = 5 * time.Minute
	}
	if o.MinRefreshInterval <= 0 {
		o.MinRefreshInterval = 30 * time.Second
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Second
	}
	if o.Clock == nil {
		o.Clock = clock.System{}
	}
	hc := o.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: o.Timeout}
	}
	return &JWKSClient{url: url, http: hc, ttl: o.TTL, minRefresh: o.MinRefreshInterval, clock: o.Clock}
}

const maxJWKSBytes = 64 << 10

// Key implements KeySource.
func (c *JWKSClient) Key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock.Now()
	k, ok := c.keys[kid]
	fresh := now.Sub(c.fetchedAt) < c.ttl
	if ok && fresh {
		return k, nil
	}
	if c.lastAttempt.IsZero() || now.Sub(c.lastAttempt) >= c.minRefresh {
		c.lastAttempt = now
		if keys, err := c.fetch(ctx); err == nil {
			c.keys, c.fetchedAt = keys, now
			k, ok = keys[kid]
		}
	}
	if ok {
		return k, nil
	}
	return nil, ErrUnknownKey
}

// Ready fetches the JWKS if it was never fetched (readiness probe).
func (c *JWKSClient) Ready(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys != nil {
		return nil
	}
	keys, err := c.fetch(ctx)
	if err != nil {
		return err
	}
	now := c.clock.Now()
	c.keys, c.fetchedAt, c.lastAttempt = keys, now, now
	return nil
}

func (c *JWKSClient) fetch(ctx context.Context) (map[string]ed25519.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read jwks: %w", err)
	}
	if len(body) > maxJWKSBytes {
		return nil, fmt.Errorf("jwks too large")
	}
	return ParseJWKSet(body)
}
