// Package ratelimit provides fixed-window rate limiting backed by Redis, an
// in-memory implementation (tests, single instance) and a fallback wrapper
// that degrades to the in-memory limiter when Redis is unavailable, so a
// Redis outage neither disables limiting nor takes the service down.
package ratelimit

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Newterios/Examory/services/platform/clock"
)

// Decision is the outcome of a rate-limit check.
type Decision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration // time until the window resets
}

// Limiter checks and counts one hit for key.
type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (Decision, error)
}

func decide(count int64, limit int, ttl time.Duration) Decision {
	d := Decision{Allowed: count <= int64(limit), Remaining: max(0, limit-int(count)), RetryAfter: ttl}
	if d.RetryAfter < 0 {
		d.RetryAfter = 0
	}
	return d
}

// Redis is a fixed-window limiter using INCR + PEXPIRE in one Lua script.
type Redis struct {
	client redis.UniversalClient
	prefix string
}

// NewRedis returns a Redis-backed limiter. prefix namespaces keys.
func NewRedis(c redis.UniversalClient, prefix string) *Redis {
	return &Redis{client: c, prefix: prefix}
}

var incrScript = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local ttl = redis.call('PTTL', KEYS[1])
if ttl < 0 then redis.call('PEXPIRE', KEYS[1], ARGV[1]); ttl = tonumber(ARGV[1]) end
return {c, ttl}
`)

// Allow implements Limiter.
func (r *Redis) Allow(ctx context.Context, key string, limit int, window time.Duration) (Decision, error) {
	res, err := incrScript.Run(ctx, r.client, []string{r.prefix + key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return Decision{}, err
	}
	if len(res) != 2 {
		return Decision{}, errors.New("ratelimit: unexpected script result")
	}
	return decide(res[0], limit, time.Duration(res[1])*time.Millisecond), nil
}

// Memory is an in-process fixed-window limiter.
type Memory struct {
	mu    sync.Mutex
	clock clock.Clock
	m     map[string]*window
	ops   int
}

type window struct {
	count int64
	reset time.Time
}

// NewMemory returns an in-memory limiter.
func NewMemory(clk clock.Clock) *Memory {
	if clk == nil {
		clk = clock.System{}
	}
	return &Memory{clock: clk, m: map[string]*window{}}
}

// Allow implements Limiter.
func (m *Memory) Allow(_ context.Context, key string, limit int, win time.Duration) (Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.clock.Now()
	m.ops++
	if m.ops%1024 == 0 { // amortised cleanup of expired windows
		for k, w := range m.m {
			if !now.Before(w.reset) {
				delete(m.m, k)
			}
		}
	}
	w, ok := m.m[key]
	if !ok || !now.Before(w.reset) {
		w = &window{reset: now.Add(win)}
		m.m[key] = w
	}
	w.count++
	return decide(w.count, limit, w.reset.Sub(now)), nil
}

// Fallback uses Primary and falls back to Secondary when Primary errors.
type Fallback struct {
	Primary   Limiter
	Secondary Limiter
	Logger    *slog.Logger

	mu       sync.Mutex
	lastWarn time.Time
}

// Allow implements Limiter.
func (f *Fallback) Allow(ctx context.Context, key string, limit int, win time.Duration) (Decision, error) {
	d, err := f.Primary.Allow(ctx, key, limit, win)
	if err == nil {
		return d, nil
	}
	f.mu.Lock()
	if time.Since(f.lastWarn) > time.Minute && f.Logger != nil {
		f.lastWarn = time.Now()
		f.Logger.WarnContext(ctx, "rate limiter primary store unavailable, using in-memory fallback", slog.String("error", err.Error()))
	}
	f.mu.Unlock()
	return f.Secondary.Allow(ctx, key, limit, win)
}
