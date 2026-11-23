// Package authn defines the authenticated principal, roles, the access-token
// claim set shared by the auth service (issuer) and every resource service
// (verifier), Ed25519 JWK encoding, and JWT verification against a JWKS.
package authn

import (
	"context"
	"errors"
)

// Role is a platform role (events.md actor.role).
type Role string

// Platform roles.
const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
	RoleSystem  Role = "system"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleStudent, RoleTeacher, RoleAdmin, RoleSystem:
		return true
	}
	return false
}

// Principal is the authenticated caller of a request.
type Principal struct {
	UserID  string
	Role    Role
	GroupID string // "" when the user is not in a group
	TokenID string // jti of the access token
}

// Is reports whether the principal has one of roles.
func (p Principal) Is(roles ...Role) bool {
	for _, r := range roles {
		if p.Role == r {
			return true
		}
	}
	return false
}

type ctxKey struct{}

// WithPrincipal stores p in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the principal stored in ctx.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// Errors returned by verification. They are deliberately coarse: callers map
// all of them to 401 without detail.
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrUnknownKey   = errors.New("unknown signing key")
)
