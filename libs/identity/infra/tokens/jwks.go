package tokens

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// JWK constants for an Ed25519 public key (RFC 8037).
const (
	jwkKeyType   = "OKP"
	jwkCurve     = "Ed25519"
	jwkUse       = "sig"
	jwtAlgorithm = "EdDSA"
)

// publicJWK has no private field by construction: only public data can be serialized.
type publicJWK struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	ALG string `json:"alg"`
	USE string `json:"use"`
	KID string `json:"kid"`
	X   string `json:"x"`
}

type jwksDocument struct {
	Keys []publicJWK `json:"keys"`
}

// JWKS renders the public key set as JSON, in configuration order, leaving out
// keys whose not_after has passed at now. Private key bytes are never included.
// A nil or zero KeySet returns ErrNoSigningKeys.
func (s *KeySet) JWKS(now time.Time) ([]byte, error) {
	if !s.usable() {
		return nil, ErrNoSigningKeys
	}
	doc := jwksDocument{Keys: make([]publicJWK, 0, len(s.order))}
	for _, kid := range s.order {
		pair := s.byKID[kid]
		if pair.expiredAt(now) {
			continue
		}
		doc.Keys = append(doc.Keys, publicJWK{
			KTY: jwkKeyType,
			CRV: jwkCurve,
			ALG: jwtAlgorithm,
			USE: jwkUse,
			KID: pair.kid,
			X:   base64.RawURLEncoding.EncodeToString(pair.public),
		})
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal jwks: %w", err)
	}
	return out, nil
}
