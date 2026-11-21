package tokens

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Audiences an access token can be issued for.
const (
	AudienceDashboard = "iris-dashboard"
	AudiencePlatform  = "iris-platform"
)

const (
	// AccessTokenTTL is the lifetime of an access token (ADR-0006).
	AccessTokenTTL = 15 * time.Minute
	// issuedAtLeeway tolerates a verifier clock slightly behind the issuer.
	issuedAtLeeway = 30 * time.Second

	jwtType        = "JWT"
	jwtPartCount   = 3
	headerFieldAlg = "alg"
	headerFieldTyp = "typ"
	headerFieldKID = "kid"
)

// Token errors. Verification failures all wrap ErrInvalidToken (401 invalid_token).
var (
	// ErrInvalidToken is returned for any token that must not be trusted.
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired is wrapped together with ErrInvalidToken when exp has passed.
	ErrTokenExpired = errors.New("token expired")
	// ErrInvalidClaims is returned by Issue when required claims are missing or the audience is unknown.
	ErrInvalidClaims = errors.New("invalid token claims")
)

// Claims are the access-token claims. IssuedAt and ExpiresAt are set by Issue
// and filled by Verify; callers do not choose them.
type Claims struct {
	Subject    string
	Audience   string
	MerchantID string
	Role       string
	SessionID  string
	TokenID    string
	AuthTime   time.Time
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

type wireClaims struct {
	Subject    string `json:"sub"`
	Audience   string `json:"aud"`
	MerchantID string `json:"mid,omitempty"`
	Role       string `json:"role,omitempty"`
	SessionID  string `json:"sid"`
	TokenID    string `json:"jti"`
	IssuedAt   *int64 `json:"iat"`
	ExpiresAt  *int64 `json:"exp"`
	NotBefore  *int64 `json:"nbf,omitempty"`
	AuthTime   *int64 `json:"auth_time"`
}

type wireHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

// Option customizes an Issuer or Verifier.
type Option func(*options)

type options struct {
	now   func() time.Time
	newID func() string
}

// WithClock injects the clock so expiry is deterministic in tests.
func WithClock(now func() time.Time) Option {
	return func(o *options) { o.now = now }
}

// WithIDGenerator injects the jti generator.
func WithIDGenerator(newID func() string) Option {
	return func(o *options) { o.newID = newID }
}

func buildOptions(opts []Option) options {
	o := options{now: time.Now, newID: uuid.NewString}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Issuer signs access tokens with the active key of a KeySet.
type Issuer struct {
	keys *KeySet
	opts options
}

// NewIssuer returns an Issuer over keys.
func NewIssuer(keys *KeySet, opts ...Option) *Issuer {
	return &Issuer{keys: keys, opts: buildOptions(opts)}
}

// Issue signs a 15-minute access token. It refuses to sign with an active key
// that has expired since startup.
func (i *Issuer) Issue(claims Claims) (string, error) {
	if err := validateIssueClaims(claims); err != nil {
		return "", err
	}
	now := i.opts.now()
	active := i.keys.active()
	if active.expiredAt(now) {
		return "", ErrActiveKeyExpired
	}
	tokenID := claims.TokenID
	if tokenID == "" {
		tokenID = i.opts.newID()
	}
	issuedAt, expiresAt, authTime := now.Unix(), now.Add(AccessTokenTTL).Unix(), claims.AuthTime.Unix()

	header, err := encodeJSONSegment(wireHeader{Algorithm: jwtAlgorithm, Type: jwtType, KeyID: active.kid})
	if err != nil {
		return "", fmt.Errorf("encode header: %w", err)
	}
	payload, err := encodeJSONSegment(wireClaims{
		Subject:    claims.Subject,
		Audience:   claims.Audience,
		MerchantID: claims.MerchantID,
		Role:       claims.Role,
		SessionID:  claims.SessionID,
		TokenID:    tokenID,
		IssuedAt:   &issuedAt,
		ExpiresAt:  &expiresAt,
		AuthTime:   &authTime,
	})
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}
	signingInput := header + "." + payload
	signature := ed25519.Sign(active.private, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func validateIssueClaims(c Claims) error {
	if c.Subject == "" || c.SessionID == "" || c.AuthTime.IsZero() {
		return fmt.Errorf("%w: sub, sid and auth_time are required", ErrInvalidClaims)
	}
	if !knownAudience(c.Audience) {
		return fmt.Errorf("%w: unknown audience", ErrInvalidClaims)
	}
	return nil
}

func knownAudience(aud string) bool {
	return aud == AudienceDashboard || aud == AudiencePlatform
}

func encodeJSONSegment(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Verifier validates access tokens against a KeySet.
type Verifier struct {
	keys *KeySet
	opts options
}

// NewVerifier returns a Verifier over keys.
func NewVerifier(keys *KeySet, opts ...Option) *Verifier {
	return &Verifier{keys: keys, opts: buildOptions(opts)}
}

// Verify checks the token and returns its claims. It requires alg EdDSA, a
// known unexpired kid (never trying other keys), a valid signature, an
// unexpired exp within the maximum TTL, a sane iat/nbf and the exact audience.
func (v *Verifier) Verify(token, expectedAudience string) (Claims, error) {
	if !knownAudience(expectedAudience) {
		return Claims{}, fmt.Errorf("%w: unknown expected audience", ErrInvalidToken)
	}
	parts := strings.Split(token, ".")
	if len(parts) != jwtPartCount {
		return Claims{}, reject("malformed")
	}
	header, err := decodeHeader(parts[0])
	if err != nil {
		return Claims{}, err
	}
	pair, ok := v.keys.lookup(header.KeyID)
	if !ok || header.KeyID == "" {
		return Claims{}, reject("unknown kid")
	}
	now := v.opts.now()
	if pair.expiredAt(now) {
		return Claims{}, reject("key expired")
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if err != nil || !ed25519.Verify(pair.public, []byte(parts[0]+"."+parts[1]), signature) {
		return Claims{}, reject("bad signature")
	}
	return decodeClaims(parts[1], expectedAudience, now)
}

func reject(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidToken, reason)
}

// decodeHeader enforces the strict header: alg EdDSA, typ JWT when present,
// a kid, and no other field (so no embedded jwk, jku or crit).
func decodeHeader(segment string) (wireHeader, error) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(segment)
	if err != nil {
		return wireHeader{}, reject("malformed header")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return wireHeader{}, reject("malformed header")
	}
	for name := range fields {
		switch name {
		case headerFieldAlg, headerFieldTyp, headerFieldKID:
		default:
			return wireHeader{}, reject("unexpected header field")
		}
	}
	var header wireHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return wireHeader{}, reject("malformed header")
	}
	if header.Algorithm != jwtAlgorithm {
		return wireHeader{}, reject("unsupported alg")
	}
	if header.Type != "" && header.Type != jwtType {
		return wireHeader{}, reject("unsupported typ")
	}
	return header, nil
}

func decodeClaims(segment, expectedAudience string, now time.Time) (Claims, error) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(segment)
	if err != nil {
		return Claims{}, reject("malformed payload")
	}
	var wire wireClaims
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Claims{}, reject("malformed payload")
	}
	if wire.IssuedAt == nil || wire.ExpiresAt == nil || wire.AuthTime == nil ||
		wire.Subject == "" || wire.SessionID == "" || wire.TokenID == "" {
		return Claims{}, reject("missing claim")
	}
	if wire.Audience != expectedAudience {
		return Claims{}, reject("audience mismatch")
	}
	issuedAt, expiresAt := time.Unix(*wire.IssuedAt, 0), time.Unix(*wire.ExpiresAt, 0)
	if !now.Before(expiresAt) {
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, ErrTokenExpired)
	}
	if expiresAt.Sub(issuedAt) > AccessTokenTTL {
		return Claims{}, reject("lifetime too long")
	}
	if issuedAt.After(now.Add(issuedAtLeeway)) {
		return Claims{}, reject("issued in the future")
	}
	if wire.NotBefore != nil && time.Unix(*wire.NotBefore, 0).After(now.Add(issuedAtLeeway)) {
		return Claims{}, reject("not yet valid")
	}
	return Claims{
		Subject:    wire.Subject,
		Audience:   wire.Audience,
		MerchantID: wire.MerchantID,
		Role:       wire.Role,
		SessionID:  wire.SessionID,
		TokenID:    wire.TokenID,
		AuthTime:   time.Unix(*wire.AuthTime, 0),
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
	}, nil
}
