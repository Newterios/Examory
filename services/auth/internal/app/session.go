package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Newterios/Examory/services/auth/internal/domain"
	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/events"
	"github.com/Newterios/Examory/services/platform/ids"
)

// Producer is the events.md producer name.
const Producer = "auth"

// Deps are the Service dependencies.
type Deps struct {
	Store    Store
	Sessions RefreshStore
	Lockout  Lockout
	Hasher   Hasher
	Tokens   TokenIssuer
	Clock    clock.Clock
	Logger   *slog.Logger
	Policy   domain.LockoutPolicy
}

// Service implements the auth use cases.
type Service struct {
	Deps
	dummyHash string
}

// New builds the service and pre-computes the dummy hash used to equalise
// timing for unknown accounts.
func New(ctx context.Context, d Deps) (*Service, error) {
	if d.Store == nil || d.Sessions == nil || d.Lockout == nil || d.Hasher == nil || d.Tokens == nil || d.Clock == nil || d.Logger == nil {
		return nil, errors.New("app: missing dependency")
	}
	if d.Policy.Threshold == 0 {
		d.Policy = domain.DefaultLockout
	}
	var seed [24]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	dummy, err := d.Hasher.Hash(ctx, base64.RawURLEncoding.EncodeToString(seed[:]))
	if err != nil {
		return nil, fmt.Errorf("app: dummy hash: %w", err)
	}
	return &Service{Deps: d, dummyHash: dummy}, nil
}

// Session is an authenticated session: an access token plus the opaque
// refresh token for the cookie.
type Session struct {
	AccessToken  string
	ExpiresAt    time.Time
	RefreshToken string
	User         domain.User
}

// RefreshTokenBytes is the entropy of refresh tokens (256 bit).
const RefreshTokenBytes = 32

// NewRefreshToken returns a random opaque token and its SHA-256 hex digest.
func NewRefreshToken() (token, digest string, err error) {
	var b [RefreshTokenBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b[:])
	return token, DigestToken(token), nil
}

// DigestToken returns the SHA-256 hex digest stored instead of the token.
func DigestToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}

// ValidRefreshTokenFormat reports whether s looks like a token we issued.
func ValidRefreshTokenFormat(s string) bool {
	if len(s) != 43 {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	return err == nil && len(b) == RefreshTokenBytes
}

// accountKey identifies an account for lockout without storing the e-mail.
func accountKey(email string) string {
	s := sha256.Sum256([]byte("login:" + email))
	return hex.EncodeToString(s[:])
}

// Login authenticates by e-mail and password. Every credential failure
// returns domain.ErrInvalidCredentials; a locked account returns
// *domain.LockedError without verifying the password.
func (s *Service) Login(ctx context.Context, email, password, ip string) (*Session, error) {
	email = domain.NormalizeEmail(email)
	key := accountKey(email)
	now := s.Clock.Now()
	until, err := s.Lockout.LockedUntil(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("lockout status: %w", err)
	}
	if until.After(now) {
		return nil, &domain.LockedError{Until: until}
	}

	var user *domain.User
	if len(email) <= domain.MaxEmailLen {
		user, err = s.Store.UserByEmail(ctx, email)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}
	hash := s.dummyHash
	if user != nil && user.Active() {
		hash = user.PasswordHash
	}
	pw := password
	if len(pw) > domain.MaxPasswordBytes {
		pw = pw[:domain.MaxPasswordBytes] // still spend the hash time; never matches a valid hash path below
	}
	ok, rehash, verr := s.Hasher.Verify(ctx, pw, hash)
	if verr != nil {
		s.Logger.ErrorContext(ctx, "password hash verification error", slog.String("error", verr.Error()))
		ok = false
	}
	if !ok || user == nil || !user.Active() || len(password) > domain.MaxPasswordBytes {
		return nil, s.loginFailed(ctx, key, user, ip)
	}

	if err := s.Lockout.Reset(ctx, key); err != nil {
		s.Logger.WarnContext(ctx, "lockout reset failed", slog.String("error", err.Error()))
	}
	if rehash {
		if nh, err := s.Hasher.Hash(ctx, password); err == nil {
			err = s.Store.InTx(ctx, func(tx Tx) error { return tx.UpdatePasswordHash(ctx, user.ID, nh, now) })
			if err != nil {
				s.Logger.WarnContext(ctx, "password rehash failed", slog.String("error", err.Error()))
			}
		}
	}
	sess, err := s.newSession(ctx, *user, now)
	if err != nil {
		return nil, err
	}
	err = s.Store.InTx(ctx, func(tx Tx) error {
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: user.ID, Action: "auth.login", TargetType: "user", TargetID: user.ID, Outcome: OutcomeSuccess, IP: ip}); err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TypeUserLoggedIn, events.Actor{UserID: user.ID, Role: string(user.Role)},
			userContext(*user), events.UserLoggedIn{UserID: user.ID, Method: "password"}, user.ID, now)
	})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *Service) loginFailed(ctx context.Context, key string, user *domain.User, ip string) error {
	now := s.Clock.Now()
	n, err := s.Lockout.Failure(ctx, key, s.Policy.Window)
	if err != nil {
		s.Logger.WarnContext(ctx, "lockout failure counter error", slog.String("error", err.Error()))
	}
	lockFor := s.Policy.LockDuration(n)
	if lockFor > 0 {
		if err := s.Lockout.Lock(ctx, key, lockFor); err != nil {
			s.Logger.WarnContext(ctx, "lockout lock error", slog.String("error", err.Error()))
		}
	}
	target := ""
	if user != nil {
		target = user.ID
	}
	detail := map[string]string{"account_key": key[:16], "failures": fmt.Sprint(n)}
	aerr := s.Store.InTx(ctx, func(tx Tx) error {
		if err := tx.Audit(ctx, AuditEntry{At: now, Action: "auth.login", TargetType: "user", TargetID: target, Outcome: OutcomeFailure, IP: ip, Detail: detail}); err != nil {
			return err
		}
		if lockFor > 0 {
			d := map[string]string{"account_key": key[:16], "lock_seconds": fmt.Sprint(int(lockFor.Seconds()))}
			return tx.Audit(ctx, AuditEntry{At: now, Action: "auth.lockout", TargetType: "user", TargetID: target, Outcome: OutcomeDenied, IP: ip, Detail: d})
		}
		return nil
	})
	if aerr != nil {
		s.Logger.ErrorContext(ctx, "audit write failed", slog.String("error", aerr.Error()))
	}
	return domain.ErrInvalidCredentials
}

func (s *Service) newSession(ctx context.Context, u domain.User, now time.Time) (*Session, error) {
	tok, exp, err := s.Tokens.Issue(u.ID, u.Role, u.GroupID)
	if err != nil {
		return nil, err
	}
	rt, digest, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	if err := s.Sessions.Create(ctx, u.ID, ids.NewV7At(now), digest, now); err != nil {
		return nil, fmt.Errorf("create refresh family: %w", err)
	}
	return &Session{AccessToken: tok, ExpiresAt: exp, RefreshToken: rt, User: u}, nil
}

// Refresh rotates a refresh token. A replayed (already rotated) token
// revokes the whole family and is audited as token reuse.
func (s *Service) Refresh(ctx context.Context, token, ip string) (*Session, error) {
	if !ValidRefreshTokenFormat(token) {
		return nil, domain.ErrInvalidSession
	}
	now := s.Clock.Now()
	newTok, newDigest, err := NewRefreshToken()
	if err != nil {
		return nil, err
	}
	res, err := s.Sessions.Rotate(ctx, DigestToken(token), newDigest, now)
	if err != nil {
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}
	switch res.Outcome {
	case RefreshOK:
	case RefreshReused:
		s.Logger.WarnContext(ctx, "refresh token reuse detected; session family revoked",
			slog.String("user_id", res.UserID), slog.String("family_id", res.FamilyID))
		err := s.Store.InTx(ctx, func(tx Tx) error {
			if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: res.UserID, Action: "auth.token_reuse", TargetType: "user", TargetID: res.UserID,
				Outcome: OutcomeDenied, IP: ip, Detail: map[string]string{"family_id": res.FamilyID}}); err != nil {
				return err
			}
			u, err := s.Store.UserByID(ctx, res.UserID)
			if err != nil {
				return err
			}
			return s.publish(ctx, tx, events.TypeUserLoggedOut, events.Actor{UserID: u.ID, Role: string(u.Role)}, userContext(*u),
				events.UserLoggedOut{UserID: u.ID, Reason: events.LogoutTokenReuse}, u.ID, now)
		})
		if err != nil {
			s.Logger.ErrorContext(ctx, "token reuse audit failed", slog.String("error", err.Error()))
		}
		return nil, domain.ErrInvalidSession
	default:
		return nil, domain.ErrInvalidSession
	}
	u, err := s.Store.UserByID(ctx, res.UserID)
	if err != nil || !u.Active() {
		_ = s.Sessions.RevokeFamily(ctx, res.FamilyID)
		return nil, domain.ErrInvalidSession
	}
	tok, exp, err := s.Tokens.Issue(u.ID, u.Role, u.GroupID)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: tok, ExpiresAt: exp, RefreshToken: newTok, User: *u}, nil
}

// Logout revokes the family of the presented refresh token. It is
// idempotent: unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, token, ip string) error {
	if !ValidRefreshTokenFormat(token) {
		return nil
	}
	userID, famID, ok, err := s.Sessions.Revoke(ctx, DigestToken(token))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	now := s.Clock.Now()
	u, err := s.Store.UserByID(ctx, userID)
	if err != nil {
		return nil // account gone; nothing to announce
	}
	return s.Store.InTx(ctx, func(tx Tx) error {
		if err := tx.Audit(ctx, AuditEntry{At: now, ActorID: u.ID, Action: "auth.logout", TargetType: "user", TargetID: u.ID,
			Outcome: OutcomeSuccess, IP: ip, Detail: map[string]string{"family_id": famID}}); err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TypeUserLoggedOut, events.Actor{UserID: u.ID, Role: string(u.Role)}, userContext(*u),
			events.UserLoggedOut{UserID: u.ID, Reason: events.LogoutUser}, u.ID, now)
	})
}

// Me returns the caller's account.
func (s *Service) Me(ctx context.Context, userID string) (*domain.User, error) {
	u, err := s.Store.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !u.Active() {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func userContext(u domain.User) events.Context {
	return events.Context{CourseID: events.Ptr(u.GroupCourseID), GroupID: events.Ptr(u.GroupID)}
}

func (s *Service) publish(ctx context.Context, tx Tx, typ string, actor events.Actor, ectx events.Context, payload any, key string, at time.Time) error {
	env, err := events.New(typ, 1, Producer, actor, ectx, payload, at)
	if err != nil {
		return err
	}
	return tx.Publish(ctx, events.TopicUsers, key, env)
}
