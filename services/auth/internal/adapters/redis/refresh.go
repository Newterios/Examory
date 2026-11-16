// Package redis implements the refresh-token store and the login lockout
// on Redis. Only SHA-256 digests of refresh tokens are stored.
//
// Keys (prefix "auth:"):
//
//	rt:<digest>  hash {fam, uid, used}         TTL = token TTL (7d)
//	rf:<family>  hash {uid, created, revoked}  TTL = family max age (30d)
//	ru:<user>    set of family ids             TTL = family max age
//	lf:<acct>    failure counter                TTL = failure window
//	ll:<acct>    lock marker                    TTL = lock duration
//
// A consumed token keeps its key (used=1) until its TTL expires so that a
// replay is detected and revokes the family.
package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Newterios/Examory/services/auth/internal/app"
)

// RefreshStore implements app.RefreshStore.
type RefreshStore struct {
	c        redis.UniversalClient
	prefix   string
	tokenTTL time.Duration
	maxAge   time.Duration
}

// NewRefreshStore returns a store. tokenTTL is the lifetime of each refresh
// token (7d); maxAge bounds a whole family regardless of rotation (30d).
func NewRefreshStore(c redis.UniversalClient, prefix string, tokenTTL, maxAge time.Duration) *RefreshStore {
	return &RefreshStore{c: c, prefix: prefix, tokenTTL: tokenTTL, maxAge: maxAge}
}

func (s *RefreshStore) tk(d string) string { return s.prefix + "rt:" + d }
func (s *RefreshStore) fk(f string) string { return s.prefix + "rf:" + f }
func (s *RefreshStore) uk(u string) string { return s.prefix + "ru:" + u }
func ms(d time.Duration) int64             { return d.Milliseconds() }
func errUnexpected(v any) error            { return fmt.Errorf("refresh store: unexpected script result %v", v) }
func toStrings(v any) ([]string, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, errUnexpected(v)
	}
	out := make([]string, len(arr))
	for i, x := range arr {
		s, ok := x.(string)
		if !ok {
			return nil, errUnexpected(v)
		}
		out[i] = s
	}
	return out, nil
}

var createScript = redis.NewScript(`
redis.call('HSET', KEYS[2], 'uid', ARGV[1], 'created', ARGV[2], 'revoked', '0')
redis.call('PEXPIRE', KEYS[2], ARGV[4])
redis.call('HSET', KEYS[1], 'fam', ARGV[3], 'uid', ARGV[1], 'used', '0')
redis.call('PEXPIRE', KEYS[1], ARGV[5])
redis.call('SADD', KEYS[3], ARGV[3])
redis.call('PEXPIRE', KEYS[3], ARGV[4])
return 1
`)

// Create implements app.RefreshStore.
func (s *RefreshStore) Create(ctx context.Context, userID, familyID, tokenHash string, now time.Time) error {
	return createScript.Run(ctx, s.c, []string{s.tk(tokenHash), s.fk(familyID), s.uk(userID)},
		userID, now.UnixMilli(), familyID, ms(s.maxAge), ms(s.tokenTTL)).Err()
}

// rotateScript: KEYS[1]=old token, KEYS[2]=new token; ARGV: family prefix,
// now ms, family max age ms, token ttl ms.
var rotateScript = redis.NewScript(`
local v = redis.call('HMGET', KEYS[1], 'fam', 'uid', 'used')
local fam, uid, used = v[1], v[2], v[3]
if not fam then return {'invalid', '', ''} end
local fkey = ARGV[1] .. fam
local f = redis.call('HMGET', fkey, 'revoked', 'created')
local revoked, created = f[1], f[2]
if not revoked then return {'invalid', uid, fam} end
if revoked == '1' then return {'revoked', uid, fam} end
if used == '1' then
  redis.call('HSET', fkey, 'revoked', '1')
  return {'reused', uid, fam}
end
if tonumber(ARGV[2]) - tonumber(created) > tonumber(ARGV[3]) then return {'invalid', uid, fam} end
redis.call('HSET', KEYS[1], 'used', '1')
redis.call('HSET', KEYS[2], 'fam', fam, 'uid', uid, 'used', '0')
redis.call('PEXPIRE', KEYS[2], ARGV[4])
return {'ok', uid, fam}
`)

// Rotate implements app.RefreshStore.
func (s *RefreshStore) Rotate(ctx context.Context, oldHash, newHash string, now time.Time) (app.RefreshResult, error) {
	v, err := rotateScript.Run(ctx, s.c, []string{s.tk(oldHash), s.tk(newHash)},
		s.prefix+"rf:", now.UnixMilli(), ms(s.maxAge), ms(s.tokenTTL)).Result()
	if err != nil {
		return app.RefreshResult{}, err
	}
	r, err := toStrings(v)
	if err != nil || len(r) != 3 {
		return app.RefreshResult{}, errUnexpected(v)
	}
	res := app.RefreshResult{UserID: r[1], FamilyID: r[2]}
	switch r[0] {
	case "ok":
		res.Outcome = app.RefreshOK
	case "reused":
		res.Outcome = app.RefreshReused
	case "revoked":
		res.Outcome = app.RefreshRevoked
	default:
		res.Outcome = app.RefreshInvalid
	}
	return res, nil
}

var revokeScript = redis.NewScript(`
local v = redis.call('HMGET', KEYS[1], 'fam', 'uid')
if not v[1] then return {'', ''} end
local fkey = ARGV[1] .. v[1]
local r = redis.call('HGET', fkey, 'revoked')
if not r or r == '1' then return {'', ''} end
redis.call('HSET', fkey, 'revoked', '1')
return {v[2], v[1]}
`)

// Revoke implements app.RefreshStore. ok is false when the token is unknown
// or its family is already revoked or expired.
func (s *RefreshStore) Revoke(ctx context.Context, tokenHash string) (string, string, bool, error) {
	v, err := revokeScript.Run(ctx, s.c, []string{s.tk(tokenHash)}, s.prefix+"rf:").Result()
	if err != nil {
		return "", "", false, err
	}
	r, err := toStrings(v)
	if err != nil || len(r) != 2 {
		return "", "", false, errUnexpected(v)
	}
	if r[1] == "" {
		return "", "", false, nil
	}
	return r[0], r[1], true, nil
}

// RevokeFamily implements app.RefreshStore.
func (s *RefreshStore) RevokeFamily(ctx context.Context, familyID string) error {
	n, err := s.c.Exists(ctx, s.fk(familyID)).Result()
	if err != nil || n == 0 {
		return err
	}
	return s.c.HSet(ctx, s.fk(familyID), "revoked", "1").Err()
}

// RevokeAllForUser implements app.RefreshStore.
func (s *RefreshStore) RevokeAllForUser(ctx context.Context, userID string) error {
	fams, err := s.c.SMembers(ctx, s.uk(userID)).Result()
	if err != nil {
		return err
	}
	var errs []error
	for _, f := range fams {
		if err := s.RevokeFamily(ctx, f); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Lockout implements app.Lockout.
type Lockout struct {
	c      redis.UniversalClient
	prefix string
}

// NewLockout returns a Redis lockout tracker.
func NewLockout(c redis.UniversalClient, prefix string) *Lockout {
	return &Lockout{c: c, prefix: prefix}
}

// LockedUntil implements app.Lockout.
func (l *Lockout) LockedUntil(ctx context.Context, key string) (time.Time, error) {
	ttl, err := l.c.PTTL(ctx, l.prefix+"ll:"+key).Result()
	if err != nil {
		return time.Time{}, err
	}
	if ttl <= 0 {
		return time.Time{}, nil
	}
	return time.Now().Add(ttl), nil
}

var failScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return n
`)

// Failure implements app.Lockout.
func (l *Lockout) Failure(ctx context.Context, key string, window time.Duration) (int, error) {
	n, err := failScript.Run(ctx, l.c, []string{l.prefix + "lf:" + key}, ms(window)).Int64()
	return int(n), err
}

// Lock implements app.Lockout.
func (l *Lockout) Lock(ctx context.Context, key string, d time.Duration) error {
	return l.c.Set(ctx, l.prefix+"ll:"+key, strconv.FormatInt(time.Now().Add(d).Unix(), 10), d).Err()
}

// Reset implements app.Lockout.
func (l *Lockout) Reset(ctx context.Context, key string) error {
	return l.c.Del(ctx, l.prefix+"lf:"+key, l.prefix+"ll:"+key).Err()
}
