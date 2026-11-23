package authn

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Newterios/Examory/services/platform/clock"
	"github.com/Newterios/Examory/services/platform/ids"
)

// TokenType is the JWT "typ" header of access tokens (RFC 9068). Verifiers
// reject any other type so that other JWTs signed with the same key cannot be
// replayed as access tokens.
const TokenType = "at+jwt"

// Algorithm is the only accepted JWS algorithm.
const Algorithm = "EdDSA"

// MaxTokenBytes bounds the size of a bearer token accepted for parsing.
const MaxTokenBytes = 4096

// Claims is the access-token claim set.
type Claims struct {
	Role    Role    `json:"role"`
	GroupID *string `json:"group_id"`
	jwt.RegisteredClaims
}

// Signer issues access tokens with an Ed25519 key.
type Signer struct {
	kid      string
	key      ed25519.PrivateKey
	issuer   string
	audience string
	ttl      time.Duration
	clock    clock.Clock
}

// NewSigner returns a signer. kid identifies the key in the JWKS.
func NewSigner(kid string, key ed25519.PrivateKey, issuer, audience string, ttl time.Duration, clk clock.Clock) (*Signer, error) {
	if len(key) != ed25519.PrivateKeySize || kid == "" || issuer == "" || audience == "" || ttl <= 0 {
		return nil, errors.New("authn: invalid signer configuration")
	}
	return &Signer{kid: kid, key: key, issuer: issuer, audience: audience, ttl: ttl, clock: clk}, nil
}

// KeyID returns the signing key id.
func (s *Signer) KeyID() string { return s.kid }

// TTL returns the access-token lifetime.
func (s *Signer) TTL() time.Duration { return s.ttl }

// Issue signs an access token for the subject. groupID may be "".
func (s *Signer) Issue(userID string, role Role, groupID string) (token string, expiresAt time.Time, err error) {
	now := s.clock.Now().Truncate(time.Second)
	exp := now.Add(s.ttl)
	var gid *string
	if groupID != "" {
		gid = &groupID
	}
	claims := Claims{
		Role:    role,
		GroupID: gid,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(exp),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        ids.NewV7At(now),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	t.Header["kid"] = s.kid
	t.Header["typ"] = TokenType
	signed, err := t.SignedString(s.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// KeySource resolves verification keys by kid.
type KeySource interface {
	Key(ctx context.Context, kid string) (ed25519.PublicKey, error)
}

// StaticKeys is a fixed KeySource (tests, or the issuer verifying itself).
type StaticKeys map[string]ed25519.PublicKey

// Key implements KeySource.
func (s StaticKeys) Key(_ context.Context, kid string) (ed25519.PublicKey, error) {
	if k, ok := s[kid]; ok {
		return k, nil
	}
	return nil, ErrUnknownKey
}

// VerifierConfig configures token verification.
type VerifierConfig struct {
	Issuer   string
	Audience string
	// Leeway is the allowed clock skew for exp/nbf/iat (default 30s, max 2m).
	Leeway time.Duration
	// MaxTTL rejects tokens whose exp-iat exceeds it (default 1h).
	MaxTTL time.Duration
	Clock  clock.Clock
}

// Verifier validates access tokens.
type Verifier struct {
	src    KeySource
	cfg    VerifierConfig
	parser *jwt.Parser
}

// NewVerifier returns a verifier using src for key lookup.
func NewVerifier(src KeySource, cfg VerifierConfig) (*Verifier, error) {
	if cfg.Issuer == "" || cfg.Audience == "" {
		return nil, errors.New("authn: issuer and audience are required")
	}
	if cfg.Leeway <= 0 {
		cfg.Leeway = 30 * time.Second
	}
	if cfg.Leeway > 2*time.Minute {
		return nil, errors.New("authn: leeway above 2m is not allowed")
	}
	if cfg.MaxTTL <= 0 {
		cfg.MaxTTL = time.Hour
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	p := jwt.NewParser(
		jwt.WithValidMethods([]string{Algorithm}),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithAudience(cfg.Audience),
		jwt.WithLeeway(cfg.Leeway),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithStrictDecoding(),
		jwt.WithTimeFunc(cfg.Clock.Now),
	)
	return &Verifier{src: src, cfg: cfg, parser: p}, nil
}

// Verify parses and validates a compact JWS access token.
func (v *Verifier) Verify(ctx context.Context, raw string) (Principal, error) {
	if raw == "" || len(raw) > MaxTokenBytes {
		return Principal{}, ErrInvalidToken
	}
	var claims Claims
	_, err := v.parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if typ, _ := t.Header["typ"].(string); typ != TokenType {
			return nil, ErrInvalidToken
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" || len(kid) > 128 {
			return nil, ErrInvalidToken
		}
		return v.src.Key(ctx, kid)
	})
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	now := v.cfg.Clock.Now()
	switch {
	case claims.NotBefore == nil, claims.IssuedAt == nil, claims.ExpiresAt == nil:
		return Principal{}, fmt.Errorf("%w: missing time claims", ErrInvalidToken)
	case claims.ExpiresAt.Sub(claims.IssuedAt.Time) > v.cfg.MaxTTL:
		return Principal{}, fmt.Errorf("%w: lifetime too long", ErrInvalidToken)
	case claims.IssuedAt.After(now.Add(v.cfg.Leeway)):
		return Principal{}, fmt.Errorf("%w: issued in the future", ErrInvalidToken)
	case !ids.Valid(claims.Subject), !ids.Valid(claims.ID):
		return Principal{}, fmt.Errorf("%w: bad sub or jti", ErrInvalidToken)
	case !claims.Role.Valid():
		return Principal{}, fmt.Errorf("%w: bad role", ErrInvalidToken)
	}
	p := Principal{UserID: claims.Subject, Role: claims.Role, TokenID: claims.ID}
	if claims.GroupID != nil {
		if !ids.Valid(*claims.GroupID) {
			return Principal{}, fmt.Errorf("%w: bad group_id", ErrInvalidToken)
		}
		p.GroupID = *claims.GroupID
	}
	return p, nil
}
