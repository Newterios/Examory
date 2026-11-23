// Package config reads service configuration from environment variables
// with validation. Errors are accumulated so that a misconfigured service
// reports every problem at once. Secret values are never included in error
// messages.
//
// Usage:
//
//	env := config.New(os.LookupEnv)
//	addr := env.String("AUTH_HTTP_ADDR", ":8080")
//	dsn := env.Secret("AUTH_DATABASE_URL")          // required, never printed
//	ttl := env.Duration("AUTH_ACCESS_TTL", 10*time.Minute, config.Min(time.Minute))
//	if err := env.Err(); err != nil { ... }
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LookupFunc matches os.LookupEnv.
type LookupFunc func(key string) (string, bool)

// Env reads typed values and accumulates validation errors.
type Env struct {
	lookup LookupFunc
	errs   []error
}

// New returns an Env backed by lookup (normally os.LookupEnv).
func New(lookup LookupFunc) *Env { return &Env{lookup: lookup} }

// FromMap returns an Env backed by a map (for tests).
func FromMap(m map[string]string) *Env {
	return New(func(k string) (string, bool) { v, ok := m[k]; return v, ok })
}

func (e *Env) get(key string) (string, bool) {
	v, ok := e.lookup(key)
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	return v, v != ""
}

func (e *Env) fail(key, format string, args ...any) {
	e.errs = append(e.errs, fmt.Errorf("%s: %s", key, fmt.Sprintf(format, args...)))
}

// Err returns all accumulated errors joined, or nil.
func (e *Env) Err() error { return errors.Join(e.errs...) }

// String returns the value or def when unset.
func (e *Env) String(key, def string) string {
	if v, ok := e.get(key); ok {
		return v
	}
	return def
}

// Required returns a non-secret required value.
func (e *Env) Required(key string) string {
	v, ok := e.get(key)
	if !ok {
		e.fail(key, "is required")
	}
	return v
}

// Secret returns a required secret value. The value is never echoed in errors.
func (e *Env) Secret(key string) string {
	v, ok := e.get(key)
	if !ok {
		e.fail(key, "is required (secret)")
	}
	return v
}

// OptionalSecret returns a secret value or "" when unset.
func (e *Env) OptionalSecret(key string) string {
	v, _ := e.get(key)
	return v
}

// Enum returns the value, which must be one of allowed.
func (e *Env) Enum(key, def string, allowed ...string) string {
	v := e.String(key, def)
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	e.fail(key, "must be one of %s", strings.Join(allowed, ", "))
	return def
}

// Bool parses true/false/1/0.
func (e *Env) Bool(key string, def bool) bool {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		e.fail(key, "must be a boolean")
		return def
	}
	return b
}

// IntRange constrains an integer value.
type IntRange struct{ Min, Max int }

// Int parses an integer within r.
func (e *Env) Int(key string, def int, r IntRange) int {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		e.fail(key, "must be an integer")
		return def
	}
	if n < r.Min || n > r.Max {
		e.fail(key, "must be within [%d, %d]", r.Min, r.Max)
		return def
	}
	return n
}

// DurationRange constrains a duration value.
type DurationRange struct{ Min, Max time.Duration }

// Duration parses a Go duration (e.g. "10m") within r.
func (e *Env) Duration(key string, def time.Duration, r DurationRange) time.Duration {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		e.fail(key, "must be a duration such as 30s or 10m")
		return def
	}
	if d < r.Min || (r.Max > 0 && d > r.Max) {
		e.fail(key, "must be within [%s, %s]", r.Min, r.Max)
		return def
	}
	return d
}

// List splits a comma-separated value, trimming blanks.
func (e *Env) List(key string, def []string) []string {
	v, ok := e.get(key)
	if !ok {
		return def
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// URL parses an absolute http(s) URL.
func (e *Env) URL(key, def string) string {
	v := e.String(key, def)
	if v == "" {
		e.fail(key, "is required")
		return v
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		e.fail(key, "must be an absolute http(s) URL")
	}
	return v
}

// Check records a custom validation failure when ok is false.
func (e *Env) Check(ok bool, key, msg string) {
	if !ok {
		e.fail(key, "%s", msg)
	}
}
