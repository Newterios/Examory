package authn

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// JWK is an Ed25519 public key in JWK form (RFC 8037). Only the fields the
// platform uses are modelled; unknown members are ignored when decoding.
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
}

// JWKSet is a JSON Web Key Set.
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// PublicJWK encodes an Ed25519 public key as a signing JWK.
func PublicJWK(kid string, pub ed25519.PublicKey) JWK {
	return JWK{Kty: "OKP", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(pub), Kid: kid, Use: "sig", Alg: "EdDSA"}
}

// Thumbprint returns the RFC 7638 JWK thumbprint (base64url SHA-256) of an
// Ed25519 public key; it is used as the default key id.
func Thumbprint(pub ed25519.PublicKey) string {
	// Members in lexicographic order, no whitespace (RFC 7638 §3.2).
	canonical := fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":"%s"}`, base64.RawURLEncoding.EncodeToString(pub))
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Ed25519Key validates the JWK and returns its public key.
func (k JWK) Ed25519Key() (ed25519.PublicKey, error) {
	if k.Kty != "OKP" || k.Crv != "Ed25519" {
		return nil, fmt.Errorf("unsupported key type %q/%q", k.Kty, k.Crv)
	}
	if k.Use != "" && k.Use != "sig" {
		return nil, fmt.Errorf("key use %q is not sig", k.Use)
	}
	if k.Alg != "" && k.Alg != "EdDSA" {
		return nil, fmt.Errorf("key alg %q is not EdDSA", k.Alg)
	}
	if k.Kid == "" || len(k.Kid) > 128 {
		return nil, fmt.Errorf("invalid kid")
	}
	raw, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 public key")
	}
	return ed25519.PublicKey(raw), nil
}

// ParseJWKSet decodes a JWKS document and returns the usable keys by kid.
// Keys that are not Ed25519 signing keys are skipped.
func ParseJWKSet(data []byte) (map[string]ed25519.PublicKey, error) {
	var set JWKSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}
	keys := make(map[string]ed25519.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		pub, err := k.Ed25519Key()
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks contains no usable Ed25519 keys")
	}
	return keys, nil
}
