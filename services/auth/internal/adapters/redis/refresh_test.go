package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Newterios/Examory/services/auth/internal/app"
)

func setup(t *testing.T) (*miniredis.Miniredis, *RefreshStore, *Lockout) {
	mr := miniredis.RunT(t)
	c := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	return mr, NewRefreshStore(c, "auth:", 7*24*time.Hour, 30*24*time.Hour), NewLockout(c, "auth:")
}

func TestRotateAndReuse(t *testing.T) {
	mr, s, _ := setup(t)
	ctx := context.Background()
	now := time.Now()
	if err := s.Create(ctx, "u1", "f1", "h1", now); err != nil {
		t.Fatal(err)
	}
	// Only the digest is a key; nothing stores a raw token.
	for _, k := range mr.Keys() {
		if !strings.HasPrefix(k, "auth:") {
			t.Fatalf("unprefixed key %s", k)
		}
	}
	r, err := s.Rotate(ctx, "h1", "h2", now)
	if err != nil || r.Outcome != app.RefreshOK || r.UserID != "u1" || r.FamilyID != "f1" {
		t.Fatalf("rotate: %+v %v", r, err)
	}
	if ttl := mr.TTL("auth:rt:h2"); ttl <= 0 || ttl > 7*24*time.Hour {
		t.Fatalf("new token ttl %v", ttl)
	}
	r, _ = s.Rotate(ctx, "h2", "h3", now)
	if r.Outcome != app.RefreshOK {
		t.Fatal("second rotation failed")
	}
	// Replay h1 => reuse, family revoked.
	if r, _ := s.Rotate(ctx, "h1", "hx", now); r.Outcome != app.RefreshReused {
		t.Fatalf("reuse not detected: %+v", r)
	}
	if r, _ := s.Rotate(ctx, "h3", "h4", now); r.Outcome != app.RefreshRevoked {
		t.Fatalf("family still usable: %+v", r)
	}
	if mr.Exists("auth:rt:hx") || mr.Exists("auth:rt:h4") {
		t.Fatal("token minted from revoked family")
	}
	if r, _ := s.Rotate(ctx, "unknown", "h5", now); r.Outcome != app.RefreshInvalid {
		t.Fatal("unknown token accepted")
	}
}

func TestFamilyMaxAgeAndExpiry(t *testing.T) {
	mr, s, _ := setup(t)
	ctx := context.Background()
	start := time.Now()
	_ = s.Create(ctx, "u1", "f1", "h1", start)
	// Absolute family age exceeded even though the token is still alive.
	if r, _ := s.Rotate(ctx, "h1", "h2", start.Add(31*24*time.Hour)); r.Outcome != app.RefreshInvalid {
		t.Fatalf("old family rotated: %+v", r)
	}
	_ = s.Create(ctx, "u1", "f2", "k1", start)
	mr.FastForward(8 * 24 * time.Hour) // token TTL elapsed
	if r, _ := s.Rotate(ctx, "k1", "k2", start); r.Outcome != app.RefreshInvalid {
		t.Fatalf("expired token rotated: %+v", r)
	}
}

func TestRevoke(t *testing.T) {
	_, s, _ := setup(t)
	ctx := context.Background()
	now := time.Now()
	_ = s.Create(ctx, "u1", "f1", "h1", now)
	_ = s.Create(ctx, "u1", "f2", "g1", now)
	_ = s.Create(ctx, "u2", "f3", "z1", now)
	uid, fam, ok, err := s.Revoke(ctx, "h1")
	if err != nil || !ok || uid != "u1" || fam != "f1" {
		t.Fatalf("revoke: %s %s %v %v", uid, fam, ok, err)
	}
	if _, _, ok, _ := s.Revoke(ctx, "h1"); ok {
		t.Fatal("second revoke reported ok")
	}
	if _, _, ok, _ := s.Revoke(ctx, "nope"); ok {
		t.Fatal("unknown revoke ok")
	}
	if err := s.RevokeAllForUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Rotate(ctx, "g1", "g2", now); r.Outcome != app.RefreshRevoked {
		t.Fatal("RevokeAllForUser missed a family")
	}
	if r, _ := s.Rotate(ctx, "z1", "z2", now); r.Outcome != app.RefreshOK {
		t.Fatal("other user's family revoked")
	}
}

func TestLockout(t *testing.T) {
	mr, _, l := setup(t)
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		n, err := l.Failure(ctx, "acct", time.Hour)
		if err != nil || n != i {
			t.Fatalf("failure %d: %d %v", i, n, err)
		}
	}
	if u, _ := l.LockedUntil(ctx, "acct"); !u.IsZero() {
		t.Fatal("locked without Lock")
	}
	_ = l.Lock(ctx, "acct", 30*time.Second)
	if u, _ := l.LockedUntil(ctx, "acct"); time.Until(u) < 25*time.Second {
		t.Fatalf("lock until %v", u)
	}
	mr.FastForward(31 * time.Second)
	if u, _ := l.LockedUntil(ctx, "acct"); !u.IsZero() {
		t.Fatal("lock did not expire")
	}
	_ = l.Reset(ctx, "acct")
	if n, _ := l.Failure(ctx, "acct", time.Hour); n != 1 {
		t.Fatal("reset did not clear failures")
	}
	mr.FastForward(2 * time.Hour)
	if n, _ := l.Failure(ctx, "acct", time.Hour); n != 1 {
		t.Fatal("failure window did not expire")
	}
}
