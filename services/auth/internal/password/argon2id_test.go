package password

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

var phcRe = regexp.MustCompile(`^\$argon2id\$v=19\$m=19456,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)

func TestHashVerify(t *testing.T) {
	ctx := context.Background()
	h := New(OWASP, 2)
	phc, err := h.Hash(ctx, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !phcRe.MatchString(phc) {
		t.Fatalf("not PHC: %s", phc)
	}
	phc2, _ := h.Hash(ctx, "correct horse battery")
	if phc == phc2 {
		t.Fatal("salt not random")
	}
	ok, rehash, err := h.Verify(ctx, "correct horse battery", phc)
	if !ok || rehash || err != nil {
		t.Fatalf("verify: %v %v %v", ok, rehash, err)
	}
	if ok, _, _ := h.Verify(ctx, "correct horse batterY", phc); ok {
		t.Fatal("wrong password accepted")
	}
}

func TestNeedsRehashOnParamUpgrade(t *testing.T) {
	ctx := context.Background()
	weak := New(Params{MemoryKiB: 8192, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}, 1)
	old, _ := weak.Hash(ctx, "correct horse battery")
	ok, rehash, err := New(OWASP, 1).Verify(ctx, "correct horse battery", old)
	if !ok || !rehash || err != nil {
		t.Fatalf("expected ok+rehash: %v %v %v", ok, rehash, err)
	}
}

func TestMalformedAndCaps(t *testing.T) {
	h := New(OWASP, 1)
	salt := "c29tZXNhbHRzb21lc2FsdA"
	key := "aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhcw"
	bad := []string{
		"",
		"$2a$10$abcdefghijklmnopqrstuv",
		"$argon2i$v=19$m=19456,t=2,p=1$" + salt + "$" + key,
		"$argon2id$v=16$m=19456,t=2,p=1$" + salt + "$" + key,
		"$argon2id$v=19$m=4194304,t=2,p=1$" + salt + "$" + key, // 4 GiB: memory DoS
		"$argon2id$v=19$m=19456,t=1000,p=1$" + salt + "$" + key,
		"$argon2id$v=19$m=19456,t=2,p=255$" + salt + "$" + key,
		"$argon2id$v=19$m=19456,t=2$" + salt + "$" + key,
		"$argon2id$v=19$m=19456,t=2,p=1$!!$" + key,
		"$argon2id$v=19$m=19456,t=2,p=1$" + salt + "$" + strings.Repeat("A", 300),
	}
	for _, phc := range bad {
		if _, _, err := h.Verify(context.Background(), "x", phc); err == nil {
			t.Errorf("accepted malformed %q", phc)
		}
	}
}

func TestContextCancelledWhileWaiting(t *testing.T) {
	h := New(OWASP, 1)
	h.sem <- struct{}{} // saturate
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Hash(ctx, "correct horse battery"); err == nil {
		t.Fatal("expected context error")
	}
}
