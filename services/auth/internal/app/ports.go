// Package app implements the auth use cases on top of ports (interfaces)
// that the adapters implement.
package app

import (
	"context"
	"time"

	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/platform/events"
)

// Store is the relational store (Postgres adapter).
type Store interface {
	InTx(ctx context.Context, fn func(Tx) error) error

	UserByEmail(ctx context.Context, email string) (*domain.User, error)
	UserByID(ctx context.Context, id string) (*domain.User, error)
	ListUsers(ctx context.Context, limit int, afterID string) ([]domain.User, error)
	GroupByID(ctx context.Context, id string) (*domain.Group, error)
	ListGroups(ctx context.Context, limit int, afterID string) ([]domain.Group, error)
	GroupMembers(ctx context.Context, groupID string) ([]domain.User, error)
}

// AuditEntry is one security/admin audit record (NFR-16). It never holds
// secrets; Detail keys are fixed by the use cases.
type AuditEntry struct {
	At         time.Time
	ActorID    string // "" for anonymous
	Action     string
	TargetType string
	TargetID   string
	Outcome    string // success | failure | denied
	IP         string
	Detail     map[string]string
}

// Audit outcomes.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
	OutcomeDenied  = "denied"
)

// Tx is a unit of work: every state change and its outbox rows commit together.
type Tx interface {
	InsertUser(ctx context.Context, u domain.User) error
	UserByIDForUpdate(ctx context.Context, id string) (*domain.User, error)
	UpdateUser(ctx context.Context, id string, role domain.Role, status domain.Status, at time.Time) error
	UpdatePasswordHash(ctx context.Context, id, hash string, at time.Time) error
	CountActiveAdmins(ctx context.Context) (int, error)
	LockBootstrap(ctx context.Context) error

	InsertGroup(ctx context.Context, g domain.Group) error
	GroupByIDForUpdate(ctx context.Context, id string) (*domain.Group, error)
	// SetMembership puts the user into groupID, replacing any previous group.
	SetMembership(ctx context.Context, userID, groupID, addedBy string, at time.Time) error
	RemoveMembership(ctx context.Context, userID string) error
	CountMembers(ctx context.Context, groupID string) (int, error)

	Audit(ctx context.Context, e AuditEntry) error
	Publish(ctx context.Context, topic, key string, env events.Envelope) error
}

// RefreshOutcome is the result of a refresh-token rotation.
type RefreshOutcome int

// Outcomes.
const (
	RefreshInvalid RefreshOutcome = iota // unknown/expired token or family
	RefreshOK                            // rotated
	RefreshReused                        // replay of a rotated token: family revoked now
	RefreshRevoked                       // family already revoked
)

// RefreshResult describes a rotation.
type RefreshResult struct {
	Outcome  RefreshOutcome
	UserID   string
	FamilyID string
}

// RefreshStore keeps refresh-token state. Only SHA-256 digests of tokens
// are passed to it.
type RefreshStore interface {
	// Create starts a new family containing tokenHash.
	Create(ctx context.Context, userID, familyID, tokenHash string, now time.Time) error
	// Rotate atomically consumes oldHash and, on success, adds newHash to the
	// same family. Replaying a consumed hash revokes the family.
	Rotate(ctx context.Context, oldHash, newHash string, now time.Time) (RefreshResult, error)
	// Revoke revokes the family of tokenHash (logout). ok=false if unknown.
	Revoke(ctx context.Context, tokenHash string) (userID, familyID string, ok bool, err error)
	// RevokeFamily revokes one family.
	RevokeFamily(ctx context.Context, familyID string) error
	// RevokeAllForUser revokes every family of the user.
	RevokeAllForUser(ctx context.Context, userID string) error
}

// Lockout tracks failed logins per account key.
type Lockout interface {
	// LockedUntil returns a time in the future while locked, else zero.
	LockedUntil(ctx context.Context, key string) (time.Time, error)
	// Failure records one failure and returns the new failure count.
	Failure(ctx context.Context, key string, window time.Duration) (int, error)
	// Lock locks the key for d.
	Lock(ctx context.Context, key string, d time.Duration) error
	// Reset clears failures and lock.
	Reset(ctx context.Context, key string) error
}

// Hasher hashes passwords.
type Hasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, password, phc string) (ok, needsRehash bool, err error)
}

// TokenIssuer signs access tokens.
type TokenIssuer interface {
	Issue(userID string, role domain.Role, groupID string) (token string, expiresAt time.Time, err error)
}
