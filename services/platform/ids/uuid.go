// Package ids generates and validates UUIDv7 identifiers (RFC 9562).
//
// All Examory identifiers (entities and event_id) are UUIDv7 in the canonical
// lowercase 8-4-4-4-12 form. The 48-bit timestamp of a UUIDv7 is the Unix time
// in milliseconds; for events it MUST equal occurred_at (events.md §1), which
// is why NewV7At takes the timestamp explicitly.
package ids

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrInvalid is returned for malformed identifiers.
var ErrInvalid = errors.New("invalid uuid")

// NewV7 returns a UUIDv7 for the current wall-clock time.
func NewV7() string { return NewV7At(time.Now()) }

// NewV7At returns a UUIDv7 whose timestamp is t truncated to milliseconds.
// The remaining 74 bits are cryptographically random.
func NewV7At(t time.Time) string {
	var b [16]byte
	if _, err := rand.Read(b[6:]); err != nil {
		// crypto/rand never fails on supported platforms (Go 1.24+ panics
		// internally instead); keep the explicit check for older behaviour.
		panic(fmt.Sprintf("ids: crypto/rand failed: %v", err))
	}
	// 48-bit big-endian Unix milliseconds: write 8 bytes, keep the low 6.
	ms := t.UnixMilli()
	if ms < 0 {
		ms = 0
	}
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(ms)) // #nosec G115 -- ms >= 0
	copy(b[0:6], ts[2:8])
	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 9562 variant
	return format(b)
}

func format(b [16]byte) string {
	var dst [36]byte
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst[:])
}

// Parse parses a canonical (lowercase or uppercase) UUID string of any version.
func Parse(s string) ([16]byte, error) {
	var b [16]byte
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return b, ErrInvalid
	}
	src := s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:36]
	if _, err := hex.Decode(b[:], []byte(src)); err != nil {
		return b, ErrInvalid
	}
	return b, nil
}

// Valid reports whether s is a syntactically valid UUID (any version).
func Valid(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// Canonical returns the lowercase canonical form of a valid UUID.
func Canonical(s string) (string, error) {
	b, err := Parse(s)
	if err != nil {
		return "", err
	}
	return format(b), nil
}

// V7Time extracts the millisecond timestamp of a UUIDv7.
func V7Time(s string) (time.Time, error) {
	b, err := Parse(s)
	if err != nil {
		return time.Time{}, err
	}
	if b[6]>>4 != 7 {
		return time.Time{}, fmt.Errorf("%w: not version 7", ErrInvalid)
	}
	ms := int64(b[0])<<40 | int64(b[1])<<32 | int64(b[2])<<24 | int64(b[3])<<16 | int64(b[4])<<8 | int64(b[5])
	return time.UnixMilli(ms).UTC(), nil
}
