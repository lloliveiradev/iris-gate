package tokens

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
	// ErrInvalidClaims is returned by Issue when required claims are missing, the
	// audience is unknown, or auth_time is not in 0 < auth_time <= iat.
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

// headerFields and claimFields are the only keys a token may carry, matched
// exactly. claimFields mirrors the json tags of wireClaims; Issue never sets
// nbf, but Verify honors it.
var (
	headerFields = []string{headerFieldAlg, headerFieldTyp, headerFieldKID}
	claimFields  = []string{"sub", "aud", "mid", "role", "sid", "jti", "iat", "exp", "nbf", "auth_time"}
)

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
// that has expired since startup, and returns ErrNoSigningKeys for a nil
// Issuer or a nil or zero KeySet. exp is always iat plus AccessTokenTTL, so
// exp > iat holds by construction.
func (i *Issuer) Issue(claims Claims) (string, error) {
	if i == nil {
		return "", ErrNoSigningKeys
	}
	active, ok := i.keys.active()
	if !ok {
		return "", ErrNoSigningKeys
	}
	now := i.opts.now()
	if err := validateIssueClaims(claims, now); err != nil {
		return "", err
	}
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

// validateIssueClaims applies at issue the same claim rules Verify enforces,
// so the issuer never signs a token its own verifier would reject. issuedAt is
// the iat the token will carry.
func validateIssueClaims(c Claims, issuedAt time.Time) error {
	if !knownAudience(c.Audience) {
		return fmt.Errorf("%w: unknown audience", ErrInvalidClaims)
	}
	if !hasIdentityClaims(c) {
		return fmt.Errorf("%w: sub, sid, role and, for the dashboard audience, mid are required", ErrInvalidClaims)
	}
	if !validAuthTime(c.AuthTime, issuedAt) {
		return fmt.Errorf("%w: auth_time must be positive and not after iat", ErrInvalidClaims)
	}
	return nil
}

func knownAudience(aud string) bool {
	return aud == AudienceDashboard || aud == AudiencePlatform
}

// hasIdentityClaims is the presence rule shared by Issue and Verify: sub, sid
// and role are always required, and a dashboard token is always scoped to one
// merchant, so it also needs mid.
func hasIdentityClaims(c Claims) bool {
	if c.Subject == "" || c.SessionID == "" || c.Role == "" {
		return false
	}
	return c.Audience != AudienceDashboard || c.MerchantID != ""
}

// validAuthTime is the auth_time rule shared by Issue and Verify, in whole
// seconds as on the wire: 0 < auth_time <= iat. A future auth_time would meet
// a step-up check (now - auth_time <= 5m) for the whole token lifetime.
func validAuthTime(authTime, issuedAt time.Time) bool {
	return authTime.Unix() > 0 && authTime.Unix() <= issuedAt.Unix()
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
// known unexpired kid (never trying other keys), a valid signature, only known
// claims, an unexpired exp after iat and within the maximum TTL, a sane
// iat/nbf/auth_time, the exact audience, and the identity claims of
// hasIdentityClaims. A nil Verifier or a nil or zero KeySet rejects every
// token with ErrInvalidToken wrapping ErrNoSigningKeys.
func (v *Verifier) Verify(token, expectedAudience string) (Claims, error) {
	if v == nil || !v.keys.usable() {
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, ErrNoSigningKeys)
	}
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

// decodeStrictSegment decodes one base64url JSON segment into v. Every key must
// be in allowed, compared exactly: encoding/json alone ignores unknown keys and
// matches keys case-insensitively, so "AUD" would otherwise fill aud.
func decodeStrictSegment(segment, part string, allowed []string, v any) error {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(segment)
	if err != nil {
		return reject("malformed " + part)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return reject("malformed " + part)
	}
	for name := range fields {
		if !slices.Contains(allowed, name) {
			return reject("unexpected " + part + " field")
		}
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return reject("malformed " + part)
	}
	return nil
}

// decodeHeader enforces the strict header: alg EdDSA, typ JWT when present,
// a kid, and no other field (so no embedded jwk, jku or crit).
func decodeHeader(segment string) (wireHeader, error) {
	var header wireHeader
	if err := decodeStrictSegment(segment, "header", headerFields, &header); err != nil {
		return wireHeader{}, err
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
	var wire wireClaims
	if err := decodeStrictSegment(segment, "payload", claimFields, &wire); err != nil {
		return Claims{}, err
	}
	if wire.IssuedAt == nil || wire.ExpiresAt == nil || wire.AuthTime == nil || wire.TokenID == "" {
		return Claims{}, reject("missing claim")
	}
	claims := Claims{
		Subject:    wire.Subject,
		Audience:   wire.Audience,
		MerchantID: wire.MerchantID,
		Role:       wire.Role,
		SessionID:  wire.SessionID,
		TokenID:    wire.TokenID,
		AuthTime:   time.Unix(*wire.AuthTime, 0),
		IssuedAt:   time.Unix(*wire.IssuedAt, 0),
		ExpiresAt:  time.Unix(*wire.ExpiresAt, 0),
	}
	if claims.Audience != expectedAudience {
		return Claims{}, reject("audience mismatch")
	}
	if !hasIdentityClaims(claims) {
		return Claims{}, reject("missing claim")
	}
	if err := validateTimes(claims, wire.NotBefore, now); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

// validateTimes enforces now < exp (no leeway), iat < exp <= iat + TTL, iat and
// nbf at most issuedAtLeeway ahead of now, and validAuthTime. Because iat is
// bounded by now plus the leeway, auth_time <= iat bounds auth_time as well.
func validateTimes(c Claims, notBefore *int64, now time.Time) error {
	if !now.Before(c.ExpiresAt) {
		return fmt.Errorf("%w: %w", ErrInvalidToken, ErrTokenExpired)
	}
	if !c.ExpiresAt.After(c.IssuedAt) {
		return reject("exp not after iat")
	}
	if c.ExpiresAt.Sub(c.IssuedAt) > AccessTokenTTL {
		return reject("lifetime too long")
	}
	latest := now.Add(issuedAtLeeway)
	if c.IssuedAt.After(latest) {
		return reject("issued in the future")
	}
	if notBefore != nil && time.Unix(*notBefore, 0).After(latest) {
		return reject("not yet valid")
	}
	if !validAuthTime(c.AuthTime, c.IssuedAt) {
		return reject("invalid auth_time")
	}
	return nil
}
