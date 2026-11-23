package authn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/ids"
)

var t0 = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func setup(t *testing.T) (*Signer, *Verifier, *clock.Fake, ed25519.PrivateKey) {
	pub, priv := newKey(t)
	clk := clock.NewFake(t0)
	s, err := NewSigner("k1", priv, "examory-auth", "examory", 10*time.Minute, clk)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewVerifier(StaticKeys{"k1": pub}, VerifierConfig{Issuer: "examory-auth", Audience: "examory", Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	return s, v, clk, priv
}

func craft(t *testing.T, method jwt.SigningMethod, key any, header map[string]any, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	for k, v := range header {
		tok.Header[k] = v
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func baseClaims() Claims {
	return Claims{
		Role: RoleStudent,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "examory-auth", Subject: ids.NewV7(), Audience: jwt.ClaimStrings{"examory"},
			IssuedAt: jwt.NewNumericDate(t0), NotBefore: jwt.NewNumericDate(t0),
			ExpiresAt: jwt.NewNumericDate(t0.Add(10 * time.Minute)), ID: ids.NewV7(),
		},
	}
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	s, v, _, _ := setup(t)
	uid, gid := ids.NewV7(), ids.NewV7()
	tok, exp, err := s.Issue(uid, RoleStudent, gid)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(t0.Add(10 * time.Minute)) {
		t.Fatalf("exp %v", exp)
	}
	p, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != uid || p.Role != RoleStudent || p.GroupID != gid || !ids.Valid(p.TokenID) {
		t.Fatalf("principal %+v", p)
	}
	tok2, _, _ := s.Issue(uid, RoleTeacher, "")
	p2, err := v.Verify(context.Background(), tok2)
	if err != nil || p2.GroupID != "" {
		t.Fatalf("no group: %+v %v", p2, err)
	}
}

func TestVerifyRejects(t *testing.T) {
	s, v, clk, priv := setup(t)
	_, otherPriv := newKey(t)
	good, _, _ := s.Issue(ids.NewV7(), RoleStudent, "")
	hdr := map[string]any{"kid": "k1", "typ": TokenType}

	wrongIss := baseClaims()
	wrongIss.Issuer = "evil"
	wrongAud := baseClaims()
	wrongAud.Audience = jwt.ClaimStrings{"other"}
	longLife := baseClaims()
	longLife.ExpiresAt = jwt.NewNumericDate(t0.Add(48 * time.Hour))
	badRole := baseClaims()
	badRole.Role = "root"
	noNbf := baseClaims()
	noNbf.NotBefore = nil
	badSub := baseClaims()
	badSub.Subject = "1 OR 1=1"
	badGroup := baseClaims()
	g := "not-a-uuid"
	badGroup.GroupID = &g

	cases := map[string]string{
		"empty":         "",
		"garbage":       "a.b.c",
		"tampered":      tamper(good),
		"wrong key":     craft(t, jwt.SigningMethodEdDSA, otherPriv, hdr, baseClaims()),
		"unknown kid":   craft(t, jwt.SigningMethodEdDSA, priv, map[string]any{"kid": "zz", "typ": TokenType}, baseClaims()),
		"no kid":        craft(t, jwt.SigningMethodEdDSA, priv, map[string]any{"typ": TokenType}, baseClaims()),
		"wrong typ":     craft(t, jwt.SigningMethodEdDSA, priv, map[string]any{"kid": "k1", "typ": "JWT"}, baseClaims()),
		"hs256 confuse": craft(t, jwt.SigningMethodHS256, []byte(priv.Public().(ed25519.PublicKey)), hdr, baseClaims()),
		"alg none":      craft(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, hdr, baseClaims()),
		"wrong iss":     craft(t, jwt.SigningMethodEdDSA, priv, hdr, wrongIss),
		"wrong aud":     craft(t, jwt.SigningMethodEdDSA, priv, hdr, wrongAud),
		"long lifetime": craft(t, jwt.SigningMethodEdDSA, priv, hdr, longLife),
		"bad role":      craft(t, jwt.SigningMethodEdDSA, priv, hdr, badRole),
		"no nbf":        craft(t, jwt.SigningMethodEdDSA, priv, hdr, noNbf),
		"bad sub":       craft(t, jwt.SigningMethodEdDSA, priv, hdr, badSub),
		"bad group":     craft(t, jwt.SigningMethodEdDSA, priv, hdr, badGroup),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(context.Background(), tok); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("expected ErrInvalidToken, got %v", err)
			}
		})
	}

	// Expiry with leeway: valid 20s after exp (leeway 30s), invalid after 31s.
	clk.Set(t0.Add(10*time.Minute + 20*time.Second))
	if _, err := v.Verify(context.Background(), good); err != nil {
		t.Fatalf("within leeway: %v", err)
	}
	clk.Set(t0.Add(10*time.Minute + 31*time.Second))
	if _, err := v.Verify(context.Background(), good); err == nil {
		t.Fatal("expired token accepted")
	}
	// Not yet valid.
	clk.Set(t0.Add(-time.Minute))
	if _, err := v.Verify(context.Background(), good); err == nil {
		t.Fatal("nbf in future accepted")
	}
}

func TestJWKSClientCachingAndRefreshLimit(t *testing.T) {
	pub1, _ := newKey(t)
	pub2, _ := newKey(t)
	var hits atomic.Int32
	var serveSecond atomic.Bool
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if fail.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		set := JWKSet{Keys: []JWK{PublicJWK("k1", pub1), {Kty: "RSA", Kid: "rsa"}}}
		if serveSecond.Load() {
			set.Keys = append(set.Keys, PublicJWK("k2", pub2))
		}
		_ = json.NewEncoder(w).Encode(set)
	}))
	defer srv.Close()
	clk := clock.NewFake(t0)
	c := NewJWKSClient(srv.URL, JWKSOptions{Clock: clk, TTL: time.Minute, MinRefreshInterval: 10 * time.Second})
	ctx := context.Background()

	if _, err := c.Key(ctx, "k1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Key(ctx, "k1"); err != nil || hits.Load() != 1 {
		t.Fatalf("expected cached, hits=%d err=%v", hits.Load(), err)
	}
	// Unknown kid: one refresh after the interval, then rate limited.
	clk.Advance(11 * time.Second)
	for i := 0; i < 5; i++ {
		if _, err := c.Key(ctx, "nope"); !errors.Is(err, ErrUnknownKey) {
			t.Fatal("unknown kid accepted")
		}
	}
	if hits.Load() != 2 {
		t.Fatalf("refresh not rate limited: hits=%d", hits.Load())
	}
	if _, err := c.Key(ctx, "rsa"); err == nil {
		t.Fatal("non-Ed25519 key accepted")
	}
	// Rotation: new key appears; after the refresh interval it is picked up.
	serveSecond.Store(true)
	clk.Advance(11 * time.Second)
	if _, err := c.Key(ctx, "k2"); err != nil {
		t.Fatalf("rotated key not fetched: %v", err)
	}
	// Issuer down after TTL: cached keys still served.
	fail.Store(true)
	clk.Advance(2 * time.Minute)
	if _, err := c.Key(ctx, "k1"); err != nil {
		t.Fatalf("stale key should still be served: %v", err)
	}
}

// tamper flips one character inside the signature (not the last one, whose
// low bits are base64 padding and may decode to the same bytes).
func tamper(tok string) string {
	b := []byte(tok)
	i := len(b) - 10
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}
