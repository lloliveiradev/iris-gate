package tokens_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/iris-gate/iris-gate/libs/identity/infra/tokens"
)

type jwksView struct {
	Keys []map[string]string `json:"keys"`
}

func TestJWKS_PublicOKPKeysOnly(t *testing.T) {
	one := newTestKey(t, testKIDOne, nil)
	two := newTestKey(t, testKIDTwo, nil)
	keys := mustKeySet(t, testKIDTwo, one, two)

	raw, err := keys.JWKS(testNow)
	if err != nil {
		t.Fatalf("jwks: %v", err)
	}
	var doc jwksView
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Keys) != 2 {
		t.Fatalf("keys = %d, want 2", len(doc.Keys))
	}
	for i, want := range []testKey{one, two} {
		jwk := doc.Keys[i]
		if jwk["kty"] != "OKP" || jwk["crv"] != "Ed25519" || jwk["alg"] != "EdDSA" || jwk["use"] != "sig" {
			t.Errorf("jwk %d = %v", i, jwk)
		}
		if jwk["kid"] != want.entry["kid"] || jwk["x"] != base64.RawURLEncoding.EncodeToString(want.public) {
			t.Errorf("jwk %d kid/x = %v", i, jwk)
		}
		if len(jwk) != 6 {
			t.Errorf("jwk %d has unexpected fields: %v", i, jwk)
		}
		if _, present := jwk["d"]; present {
			t.Errorf("jwk %d exposes the private part d", i)
		}
	}
	for _, key := range []testKey{one, two} {
		secret := key.entry["private_key_b64"].(string)
		seed := base64.RawURLEncoding.EncodeToString(key.private.Seed())
		if strings.Contains(string(raw), secret) || strings.Contains(string(raw), seed) {
			t.Fatal("private key material leaked into the JWKS")
		}
	}
}

func TestJWKS_OmitsExpiredKeys(t *testing.T) {
	notAfter := testNow.Add(time.Hour)
	old := newTestKey(t, testKIDOne, &notAfter)
	current := newTestKey(t, testKIDTwo, nil)
	keys := mustKeySet(t, testKIDTwo, old, current)

	raw, err := keys.JWKS(notAfter)
	if err != nil {
		t.Fatalf("jwks: %v", err)
	}
	var doc jwksView
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Keys) != 1 || doc.Keys[0]["kid"] != testKIDTwo {
		t.Fatalf("keys = %v", doc.Keys)
	}
}

func TestJWKS_NilOrZeroKeySetFailsClosed(t *testing.T) {
	var nilSet *tokens.KeySet
	for name, keys := range map[string]*tokens.KeySet{"nil": nilSet, "zero": {}} {
		raw, err := keys.JWKS(testNow)
		if !errors.Is(err, tokens.ErrNoSigningKeys) || raw != nil {
			t.Errorf("%s key set: jwks = %s, err = %v, want ErrNoSigningKeys", name, raw, err)
		}
		if kid := keys.ActiveKID(); kid != "" {
			t.Errorf("%s key set: active kid = %q", name, kid)
		}
	}
}
