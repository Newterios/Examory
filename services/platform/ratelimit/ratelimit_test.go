package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Newterios/Examory/services/platform/clock"
)

func exercise(t *testing.T, l Limiter, advance func(time.Duration)) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		d, err := l.Allow(ctx, "k", 3, time.Minute)
		if err != nil || !d.Allowed || d.Remaining != 3-i {
			t.Fatalf("hit %d: %+v %v", i, d, err)
		}
	}
	d, _ := l.Allow(ctx, "k", 3, time.Minute)
	if d.Allowed || d.RetryAfter <= 0 || d.RetryAfter > time.Minute {
		t.Fatalf("4th hit should be denied: %+v", d)
	}
	if d2, _ := l.Allow(ctx, "other", 3, time.Minute); !d2.Allowed {
		t.Fatal("keys must be independent")
	}
	advance(61 * time.Second)
	if d, _ := l.Allow(ctx, "k", 3, time.Minute); !d.Allowed {
		t.Fatal("window should reset")
	}
}

func TestMemory(t *testing.T) {
	clk := clock.NewFake(time.Now())
	exercise(t, NewMemory(clk), clk.Advance)
}

func TestRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	exercise(t, NewRedis(c, "rl:"), mr.FastForward)
}

func TestFallback(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	clk := clock.NewFake(time.Now())
	f := &Fallback{Primary: NewRedis(c, "rl:"), Secondary: NewMemory(clk)}
	mr.Close() // Redis down
	for i := 0; i < 2; i++ {
		d, err := f.Allow(context.Background(), "k", 2, time.Minute)
		if err != nil || !d.Allowed {
			t.Fatalf("fallback should allow: %+v %v", d, err)
		}
	}
	if d, _ := f.Allow(context.Background(), "k", 2, time.Minute); d.Allowed {
		t.Fatal("fallback must still limit")
	}
}
