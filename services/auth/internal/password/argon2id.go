// Package password hashes and verifies passwords with Argon2id in PHC
// string format: $argon2id$v=19$m=<KiB>,t=<iterations>,p=<parallelism>$<salt>$<hash>
// (salt and hash in unpadded standard base64).
package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are Argon2id cost parameters.
type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     int
	KeyLen      int
}

// OWASP is the OWASP Password Storage Cheat Sheet minimum for Argon2id
// (m=19 MiB, t=2, p=1).
var OWASP = Params{MemoryKiB: 19456, Iterations: 2, Parallelism: 1, SaltLen: 16, KeyLen: 32}

// Caps applied when parsing stored hashes, so a tampered row cannot make a
// login allocate gigabytes or spin for minutes.
const (
	maxMemoryKiB  = 256 * 1024
	maxIterations = 10
	maxParallel   = 8
	maxSaltLen    = 64
	maxKeyLen     = 64
)

// ErrMalformed is returned for an unparsable PHC string.
var ErrMalformed = errors.New("password: malformed hash")

// Hasher hashes with Params and bounds concurrent hash computations, since
// each one allocates MemoryKiB (memory-exhaustion DoS protection).
type Hasher struct {
	p   Params
	sem chan struct{}
}

// New returns a Hasher allowing at most concurrency simultaneous hashes.
// Key and salt lengths are clamped to [16, 64] bytes.
func New(p Params, concurrency int) *Hasher {
	if concurrency < 1 {
		concurrency = 1
	}
	p.KeyLen = min(max(p.KeyLen, 16), maxKeyLen)
	p.SaltLen = min(max(p.SaltLen, 16), maxSaltLen)
	return &Hasher{p: p, sem: make(chan struct{}, concurrency)}
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.sem }

// Hash returns the PHC string for password with a fresh random salt.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, h.p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password: salt: %w", err)
	}
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, h.p.Iterations, h.p.MemoryKiB, h.p.Parallelism, uint32(h.p.KeyLen)) // #nosec G115 -- KeyLen clamped to <= 64 in New
	h.release()
	return encode(h.p, salt, key), nil
}

// Verify checks password against phc in constant time. needsRehash is true
// when the stored parameters differ from the hasher's current parameters.
func (h *Hasher) Verify(ctx context.Context, password, phc string) (ok, needsRehash bool, err error) {
	p, salt, key, err := decode(phc)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(key))) // #nosec G115 -- len(key) <= maxKeyLen checked in decode
	h.release()
	if subtle.ConstantTimeCompare(got, key) != 1 {
		return false, false, nil
	}
	needs := p.MemoryKiB != h.p.MemoryKiB || p.Iterations != h.p.Iterations ||
		p.Parallelism != h.p.Parallelism || len(salt) != h.p.SaltLen || len(key) != h.p.KeyLen
	return true, needs, nil
}

var b64 = base64.RawStdEncoding

func encode(p Params, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Iterations, p.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key))
}

func decode(phc string) (Params, []byte, []byte, error) {
	var p Params
	if len(phc) > 256 {
		return p, nil, nil, ErrMalformed
	}
	parts := strings.Split(phc, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash]
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return p, nil, nil, ErrMalformed
	}
	kv := strings.Split(parts[3], ",")
	if len(kv) != 3 {
		return p, nil, nil, ErrMalformed
	}
	vals := map[string]uint64{}
	for _, item := range kv {
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			return p, nil, nil, ErrMalformed
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return p, nil, nil, ErrMalformed
		}
		vals[k] = n
	}
	m, t, par := vals["m"], vals["t"], vals["p"]
	if m < 8 || m > maxMemoryKiB || t < 1 || t > maxIterations || par < 1 || par > maxParallel {
		return p, nil, nil, ErrMalformed
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > maxSaltLen {
		return p, nil, nil, ErrMalformed
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > maxKeyLen {
		return p, nil, nil, ErrMalformed
	}
	p = Params{MemoryKiB: uint32(m), Iterations: uint32(t), Parallelism: uint8(par), SaltLen: len(salt), KeyLen: len(key)}
	return p, salt, key, nil
}
