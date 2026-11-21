// Package tokens implements ADR-0006: Ed25519 signing keys with rotation by
// kid, access-token issue and strict verification (EdDSA only), and the public
// JWKS document. Keys come from the process environment only and a missing,
// malformed or expired configuration fails closed: there is no fallback key.
package tokens

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Key set configuration errors. None of them echo key material.
var (
	// ErrNoSigningKeys is returned when JWT_SIGNING_KEYS is empty or lists no key.
	ErrNoSigningKeys = errors.New("no signing keys configured")
	// ErrMalformedSigningKeys is returned when JWT_SIGNING_KEYS is not the documented JSON array.
	ErrMalformedSigningKeys = errors.New("signing keys are malformed")
	// ErrInvalidKeyID is returned when an entry has an empty or duplicate kid.
	ErrInvalidKeyID = errors.New("signing key id is empty or duplicated")
	// ErrInvalidKeyMaterial is returned when an entry is not valid Ed25519 private key material.
	ErrInvalidKeyMaterial = errors.New("signing key material is not a valid Ed25519 private key")
	// ErrNoActiveKeyID is returned when JWT_ACTIVE_KID is empty.
	ErrNoActiveKeyID = errors.New("active key id is empty")
	// ErrUnknownActiveKeyID is returned when JWT_ACTIVE_KID is not in JWT_SIGNING_KEYS.
	ErrUnknownActiveKeyID = errors.New("active key id is not in the signing keys")
	// ErrActiveKeyExpired is returned when the active key's not_after has passed.
	ErrActiveKeyExpired = errors.New("active signing key is expired")
)

// ed25519PrivateKeySize is the length of a raw Ed25519 private key (seed plus public key).
const ed25519PrivateKeySize = ed25519.PrivateKeySize

// signingKeyEntry is one element of the JWT_SIGNING_KEYS JSON array.
type signingKeyEntry struct {
	KID           string     `json:"kid"`
	PrivateKeyB64 string     `json:"private_key_b64"`
	NotAfter      *time.Time `json:"not_after,omitempty"`
}

type keyPair struct {
	kid      string
	private  ed25519.PrivateKey
	public   ed25519.PublicKey
	notAfter *time.Time
}

func (k keyPair) expiredAt(now time.Time) bool {
	return k.notAfter != nil && !now.Before(*k.notAfter)
}

// KeySet holds the loaded signing keys and the kid that signs new tokens.
// It is immutable after construction.
type KeySet struct {
	activeKID string
	byKID     map[string]keyPair
	order     []string
}

// ParseKeySet parses JWT_SIGNING_KEYS and JWT_ACTIVE_KID and validates them
// against now. It fails closed on empty or malformed configuration, an empty,
// duplicate or unknown kid, bad key material and an expired active key.
func ParseKeySet(signingKeysJSON, activeKID string, now time.Time) (*KeySet, error) {
	if strings.TrimSpace(signingKeysJSON) == "" {
		return nil, ErrNoSigningKeys
	}
	entries, err := decodeEntries(signingKeysJSON)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrNoSigningKeys
	}

	set := &KeySet{byKID: make(map[string]keyPair, len(entries))}
	for i, entry := range entries {
		pair, err := newKeyPair(entry)
		if err != nil {
			return nil, fmt.Errorf("signing key %d: %w", i, err)
		}
		if _, dup := set.byKID[pair.kid]; dup {
			return nil, fmt.Errorf("signing key %d: %w", i, ErrInvalidKeyID)
		}
		set.byKID[pair.kid] = pair
		set.order = append(set.order, pair.kid)
	}

	if activeKID == "" {
		return nil, ErrNoActiveKeyID
	}
	active, ok := set.byKID[activeKID]
	if !ok {
		return nil, ErrUnknownActiveKeyID
	}
	if active.expiredAt(now) {
		return nil, ErrActiveKeyExpired
	}
	set.activeKID = activeKID
	return set, nil
}

func decodeEntries(raw string) ([]signingKeyEntry, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var entries []signingKeyEntry
	if err := decoder.Decode(&entries); err != nil {
		// The decoder error can quote input; keep only the sentinel.
		return nil, ErrMalformedSigningKeys
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrMalformedSigningKeys
	}
	return entries, nil
}

func newKeyPair(entry signingKeyEntry) (keyPair, error) {
	if strings.TrimSpace(entry.KID) == "" {
		return keyPair{}, ErrInvalidKeyID
	}
	private, err := decodePrivateKey(entry.PrivateKeyB64)
	if err != nil {
		return keyPair{}, err
	}
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok {
		return keyPair{}, ErrInvalidKeyMaterial
	}
	return keyPair{kid: entry.KID, private: private, public: public, notAfter: entry.NotAfter}, nil
}

// decodePrivateKey accepts a base64url raw 64-byte Ed25519 private key or a
// PKCS#8 DER document, and rejects a raw key whose public half does not match
// its seed.
func decodePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(encoded, "="))
	if err != nil || len(raw) == 0 {
		return nil, ErrInvalidKeyMaterial
	}
	if len(raw) == ed25519PrivateKeySize {
		private := ed25519.PrivateKey(raw)
		derived := ed25519.NewKeyFromSeed(private.Seed())
		if !bytes.Equal(private, derived) {
			return nil, ErrInvalidKeyMaterial
		}
		return derived, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(raw)
	if err != nil {
		return nil, ErrInvalidKeyMaterial
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, ErrInvalidKeyMaterial
	}
	return private, nil
}

// ActiveKID returns the kid that signs newly issued tokens.
func (s *KeySet) ActiveKID() string { return s.activeKID }

func (s *KeySet) active() keyPair { return s.byKID[s.activeKID] }

func (s *KeySet) lookup(kid string) (keyPair, bool) {
	pair, ok := s.byKID[kid]
	return pair, ok
}
