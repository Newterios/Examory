package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Password policy (OWASP ASVS 2.1): length-based, no composition rules.
const (
	MinPasswordRunes = 12
	MaxPasswordRunes = 128
	// MaxPasswordBytes caps input before hashing to prevent hash-DoS with
	// very long multi-byte strings.
	MaxPasswordBytes = 256
)

// ValidatePassword checks the policy for a new password.
func ValidatePassword(v *Validator, password, email string) {
	switch {
	case len(password) > MaxPasswordBytes || !utf8.ValidString(password):
		v.Add("password", "is too long or not valid UTF-8")
	case utf8.RuneCountInString(password) < MinPasswordRunes:
		v.Add("password", "must be at least 12 characters")
	case utf8.RuneCountInString(password) > MaxPasswordRunes:
		v.Add("password", "must be at most 128 characters")
	case email != "" && strings.EqualFold(strings.TrimSpace(password), strings.TrimSpace(email)):
		v.Add("password", "must not equal the e-mail address")
	}
}

// LockoutPolicy defines exponential account lockout after failed logins.
type LockoutPolicy struct {
	Threshold int           // failures before the first lock (5)
	Base      time.Duration // first lock duration (30s)
	Max       time.Duration // cap (15m)
	Window    time.Duration // failure counter lifetime (24h)
}

// DefaultLockout is the production policy.
var DefaultLockout = LockoutPolicy{Threshold: 5, Base: 30 * time.Second, Max: 15 * time.Minute, Window: 24 * time.Hour}

// LockDuration returns how long to lock after `failures` consecutive
// failures: 0 below the threshold, then Base * 2^(failures-Threshold),
// capped at Max.
func (p LockoutPolicy) LockDuration(failures int) time.Duration {
	if failures < p.Threshold {
		return 0
	}
	d := p.Base
	for i := p.Threshold; i < failures; i++ {
		d *= 2
		if d >= p.Max {
			return p.Max
		}
	}
	if d > p.Max {
		return p.Max
	}
	return d
}
