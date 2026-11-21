package tokens_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/iris-gate/iris-gate/libs/identity/infra/tokens"
)

// Shared fake, non-secret fixtures. Keys are generated at test time.
const (
	testKIDOne      = "test-kid-one"
	testKIDTwo      = "test-kid-two"
	testKIDUnknown  = "test-kid-unknown"
	testSubject     = "00000000-0000-4000-8000-000000000001"
	testMerchantID  = "00000000-0000-4000-8000-000000000002"
	testSessionID   = "00000000-0000-4000-8000-000000000003"
	testTokenID     = "00000000-0000-4000-8000-000000000004"
	testRole        = "owner"
	notJSON         = "not json"
	notBase64       = "!!!not-base64!!!"
	shortKeyB64     = "AAAA"
	unknownFieldDoc = `[{"kid":"x","private_key_b64":"AAAA","extra":1}]`
)

var testNow = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

type testKey struct {
	entry   map[string]any
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

func newTestKey(t *testing.T, kid string, notAfter *time.Time) testKey {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	entry := map[string]any{
		"kid":             kid,
		"private_key_b64": base64.RawURLEncoding.EncodeToString(private),
	}
	if notAfter != nil {
		entry["not_after"] = notAfter.Format(time.RFC3339)
	}
	return testKey{entry: entry, private: private, public: public}
}

func keysJSON(t *testing.T, keys ...testKey) string {
	t.Helper()
	entries := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, k.entry)
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal keys: %v", err)
	}
	return string(raw)
}

func mustKeySet(t *testing.T, activeKID string, keys ...testKey) *tokens.KeySet {
	t.Helper()
	set, err := tokens.ParseKeySet(keysJSON(t, keys...), activeKID, testNow)
	if err != nil {
		t.Fatalf("parse key set: %v", err)
	}
	return set
}

// T4: the loader fails closed.
func TestParseKeySet_FailsClosed(t *testing.T) {
	past := testNow.Add(-time.Minute)
	future := testNow.Add(time.Hour)
	good := newTestKey(t, testKIDOne, &future)
	expired := newTestKey(t, testKIDOne, &past)
	duplicate := newTestKey(t, testKIDOne, nil)
	noKID := newTestKey(t, "", nil)
	rsaLike := testKey{entry: map[string]any{"kid": testKIDOne, "private_key_b64": shortKeyB64}}
	badSeed := newTestKey(t, testKIDOne, nil)
	mismatched := append(ed25519.PrivateKey{}, badSeed.private...)
	mismatched[ed25519.PrivateKeySize-1] ^= 0xff
	badSeed.entry["private_key_b64"] = base64.RawURLEncoding.EncodeToString(mismatched)
	notEd25519 := testKey{entry: map[string]any{"kid": testKIDOne, "private_key_b64": notBase64}}
	ecdsaPKCS8 := testKey{entry: map[string]any{"kid": testKIDOne, "private_key_b64": ecdsaPKCS8B64(t)}}
	paddedKID := newTestKey(t, " "+testKIDOne, nil)
	controlKID := newTestKey(t, "test-kid\x07one", nil)

	tests := []struct {
		name    string
		keys    string
		active  string
		wantErr error
	}{
		{"empty config", "", testKIDOne, tokens.ErrNoSigningKeys},
		{"empty array", "[]", testKIDOne, tokens.ErrNoSigningKeys},
		{"not json", notJSON, testKIDOne, tokens.ErrMalformedSigningKeys},
		{"unknown field", unknownFieldDoc, testKIDOne, tokens.ErrMalformedSigningKeys},
		{"trailing data", keysJSON(t, good) + " []", testKIDOne, tokens.ErrMalformedSigningKeys},
		{"empty kid", keysJSON(t, noKID), testKIDOne, tokens.ErrInvalidKeyID},
		{"duplicate kid", keysJSON(t, good, duplicate), testKIDOne, tokens.ErrInvalidKeyID},
		{"short key material", keysJSON(t, rsaLike), testKIDOne, tokens.ErrInvalidKeyMaterial},
		{"invalid base64", keysJSON(t, notEd25519), testKIDOne, tokens.ErrInvalidKeyMaterial},
		{"public half does not match seed", keysJSON(t, badSeed), testKIDOne, tokens.ErrInvalidKeyMaterial},
		{"PKCS#8 key that is not Ed25519", keysJSON(t, ecdsaPKCS8), testKIDOne, tokens.ErrInvalidKeyMaterial},
		{"kid with surrounding whitespace", keysJSON(t, paddedKID), testKIDOne, tokens.ErrInvalidKeyID},
		{"kid with a control character", keysJSON(t, controlKID), testKIDOne, tokens.ErrInvalidKeyID},
		{"empty active kid", keysJSON(t, good), "", tokens.ErrNoActiveKeyID},
		{"whitespace-only active kid", keysJSON(t, good), "   ", tokens.ErrNoActiveKeyID},
		{"unknown active kid", keysJSON(t, good), testKIDUnknown, tokens.ErrUnknownActiveKeyID},
		{"expired active key", keysJSON(t, expired), testKIDOne, tokens.ErrActiveKeyExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := tokens.ParseKeySet(tt.keys, tt.active, testNow)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if set != nil {
				t.Fatal("a key set must not be returned on error")
			}
		})
	}
}

func TestParseKeySet_AcceptsRawAndPKCS8(t *testing.T) {
	raw := newTestKey(t, testKIDOne, nil)
	der, err := x509.MarshalPKCS8PrivateKey(raw.private)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pkcs8 := testKey{entry: map[string]any{
		"kid":             testKIDTwo,
		"private_key_b64": base64.RawURLEncoding.EncodeToString(der),
	}}
	set, err := tokens.ParseKeySet(keysJSON(t, raw, pkcs8), testKIDTwo, testNow)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if set.ActiveKID() != testKIDTwo {
		t.Fatalf("active kid = %q", set.ActiveKID())
	}
}

// ecdsaPKCS8B64 returns a well-formed PKCS#8 document for a P-256 key, so the
// loader reaches its Ed25519 type check instead of failing to parse.
func ecdsaPKCS8B64(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(der)
}

func TestKeySet_FormattingNeverExposesKeyMaterial(t *testing.T) {
	set := mustKeySet(t, testKIDOne, newTestKey(t, testKIDOne, nil), newTestKey(t, testKIDTwo, nil))
	want := `tokens.KeySet{ActiveKID:"test-kid-one" Keys:2}`
	for name, value := range map[string]any{"pointer": set, "value": *set} {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
			if got := fmt.Sprintf(verb, value); got != want {
				t.Errorf("%s %s = %s, want %s", name, verb, got, want)
			}
		}
	}

	var logs bytes.Buffer
	slog.New(slog.NewJSONHandler(&logs, nil)).Info("keys", "pointer", set, "value", *set)
	for _, name := range []string{"pointer", "value"} {
		if want := `"` + name + `":{"active_kid":"test-kid-one","keys":2}`; !strings.Contains(logs.String(), want) {
			t.Errorf("log = %s, want %s", logs.String(), want)
		}
	}
}

func TestParseKeySet_ExpiredInactiveKeyIsAccepted(t *testing.T) {
	past := testNow.Add(-time.Minute)
	old := newTestKey(t, testKIDOne, &past)
	current := newTestKey(t, testKIDTwo, nil)
	if _, err := tokens.ParseKeySet(keysJSON(t, old, current), testKIDTwo, testNow); err != nil {
		t.Fatalf("an expired non-active key must not stop startup: %v", err)
	}
}
