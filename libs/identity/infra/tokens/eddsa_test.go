package tokens_test

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/iris-gate/iris-gate/libs/identity/infra/tokens"
)

func testClaims() tokens.Claims {
	return tokens.Claims{
		Subject:    testSubject,
		Audience:   tokens.AudienceDashboard,
		MerchantID: testMerchantID,
		Role:       testRole,
		SessionID:  testSessionID,
		TokenID:    testTokenID,
		AuthTime:   testNow.Add(-5 * time.Minute),
	}
}

func clockAt(at *time.Time) tokens.Option {
	return tokens.WithClock(func() time.Time { return *at })
}

func segment(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeSegment(t *testing.T, s string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func validPayload() map[string]any {
	return map[string]any{
		"sub": testSubject, "aud": tokens.AudienceDashboard, "sid": testSessionID, "jti": testTokenID,
		"iat": testNow.Unix(), "exp": testNow.Add(tokens.AccessTokenTTL).Unix(), "auth_time": testNow.Unix(),
	}
}

// forge signs an arbitrary header and payload with the given Ed25519 key.
func forge(t *testing.T, key ed25519.PrivateKey, header, payload map[string]any) string {
	t.Helper()
	input := segment(t, header) + "." + segment(t, payload)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(input)))
}

// T1: issue/verify round trip, 15m expiry, kid header, auth_time claim.
func TestIssueVerify_RoundTrip(t *testing.T) {
	now := testNow
	key := newTestKey(t, testKIDOne, nil)
	keys := mustKeySet(t, testKIDOne, key)
	issuer := tokens.NewIssuer(keys, clockAt(&now))
	verifier := tokens.NewVerifier(keys, clockAt(&now))
	claims := testClaims()

	token, err := issuer.Issue(claims)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	parts := strings.Split(token, ".")
	header := decodeSegment(t, parts[0])
	if header["alg"] != "EdDSA" || header["kid"] != testKIDOne {
		t.Fatalf("header = %v", header)
	}
	payload := decodeSegment(t, parts[1])
	for _, name := range []string{"sub", "aud", "mid", "role", "sid", "jti", "iat", "exp", "auth_time"} {
		if _, ok := payload[name]; !ok {
			t.Errorf("claim %q missing", name)
		}
	}

	got, err := verifier.Verify(token, tokens.AudienceDashboard)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Subject != claims.Subject || got.MerchantID != claims.MerchantID || got.Role != claims.Role ||
		got.SessionID != claims.SessionID || got.TokenID != claims.TokenID || !got.AuthTime.Equal(claims.AuthTime) {
		t.Fatalf("claims = %+v", got)
	}
	if got.ExpiresAt.Sub(got.IssuedAt) != 15*time.Minute {
		t.Fatalf("ttl = %v", got.ExpiresAt.Sub(got.IssuedAt))
	}

	now = testNow.Add(tokens.AccessTokenTTL - time.Second)
	if _, err := verifier.Verify(token, tokens.AudienceDashboard); err != nil {
		t.Fatalf("token must verify just before expiry: %v", err)
	}
	now = testNow.Add(tokens.AccessTokenTTL)
	if _, err := verifier.Verify(token, tokens.AudienceDashboard); !errors.Is(err, tokens.ErrTokenExpired) ||
		!errors.Is(err, tokens.ErrInvalidToken) {
		t.Fatalf("expired token err = %v", err)
	}
}

func TestIssue_GeneratesTokenIDAndValidatesClaims(t *testing.T) {
	now := testNow
	keys := mustKeySet(t, testKIDOne, newTestKey(t, testKIDOne, nil))
	issuer := tokens.NewIssuer(keys, clockAt(&now), tokens.WithIDGenerator(func() string { return testTokenID }))
	verifier := tokens.NewVerifier(keys, clockAt(&now))

	claims := testClaims()
	claims.TokenID = ""
	token, err := issuer.Issue(claims)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := verifier.Verify(token, tokens.AudienceDashboard)
	if err != nil || got.TokenID != testTokenID {
		t.Fatalf("jti = %q, err = %v", got.TokenID, err)
	}

	for name, mutate := range map[string]func(*tokens.Claims){
		"no subject":       func(c *tokens.Claims) { c.Subject = "" },
		"no session":       func(c *tokens.Claims) { c.SessionID = "" },
		"no auth_time":     func(c *tokens.Claims) { c.AuthTime = time.Time{} },
		"unknown audience": func(c *tokens.Claims) { c.Audience = "other" },
	} {
		c := testClaims()
		mutate(&c)
		if _, err := issuer.Issue(c); !errors.Is(err, tokens.ErrInvalidClaims) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestIssue_RefusesExpiredActiveKey(t *testing.T) {
	now := testNow
	notAfter := testNow.Add(time.Hour)
	keys := mustKeySet(t, testKIDOne, newTestKey(t, testKIDOne, &notAfter))
	issuer := tokens.NewIssuer(keys, clockAt(&now))
	now = notAfter
	if _, err := issuer.Issue(testClaims()); !errors.Is(err, tokens.ErrActiveKeyExpired) {
		t.Fatalf("err = %v", err)
	}
}

// T2: strict verification.
func TestVerify_Rejects(t *testing.T) {
	now := testNow
	one := newTestKey(t, testKIDOne, nil)
	other := newTestKey(t, testKIDUnknown, nil)
	keys := mustKeySet(t, testKIDOne, one)
	issuer := tokens.NewIssuer(keys, clockAt(&now))
	verifier := tokens.NewVerifier(keys, clockAt(&now))
	valid, err := issuer.Issue(testClaims())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	validParts := strings.Split(valid, ".")

	hmacToken := func(secret []byte) string {
		input := segment(t, map[string]any{"alg": "HS256", "typ": "JWT", "kid": testKIDOne}) + "." + segment(t, validPayload())
		mac := hmac.New(sha256.New, secret)
		mac.Write([]byte(input))
		return input + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	header := func(extra map[string]any) map[string]any {
		h := map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": testKIDOne}
		for k, v := range extra {
			h[k] = v
		}
		return h
	}
	tamperedPayload := validPayload()
	tamperedPayload["sub"] = "someone-else"
	farFuture := validPayload()
	farFuture["exp"] = testNow.Add(24 * time.Hour).Unix()
	futureIssued := validPayload()
	futureIssued["iat"] = testNow.Add(time.Hour).Unix()
	noExp := validPayload()
	delete(noExp, "exp")
	noAuthTime := validPayload()
	delete(noAuthTime, "auth_time")

	tests := []struct {
		name  string
		token string
		aud   string
	}{
		{"alg none", segment(t, map[string]any{"alg": "none", "kid": testKIDOne}) + "." + segment(t, validPayload()) + ".", tokens.AudienceDashboard},
		{"alg none with signature part", forge(t, one.private, header(map[string]any{"alg": "none"}), validPayload()), tokens.AudienceDashboard},
		{"HS256 with garbage secret", hmacToken([]byte("secret")), tokens.AudienceDashboard},
		{"HS256 signed with public key bytes", hmacToken(one.public), tokens.AudienceDashboard},
		{"unknown kid", forge(t, other.private, header(map[string]any{"kid": testKIDUnknown}), validPayload()), tokens.AudienceDashboard},
		{"missing kid", forge(t, one.private, map[string]any{"alg": "EdDSA", "typ": "JWT"}, validPayload()), tokens.AudienceDashboard},
		{"known kid, other key signature", forge(t, other.private, header(nil), validPayload()), tokens.AudienceDashboard},
		{"embedded jwk", forge(t, one.private, header(map[string]any{"jwk": map[string]any{"kty": "OKP"}}), validPayload()), tokens.AudienceDashboard},
		{"jku header", forge(t, one.private, header(map[string]any{"jku": "https://example.test/jwks"}), validPayload()), tokens.AudienceDashboard},
		{"wrong typ", forge(t, one.private, header(map[string]any{"typ": "at+jwt"}), validPayload()), tokens.AudienceDashboard},
		{"wrong audience", valid, tokens.AudiencePlatform},
		{"unknown expected audience", valid, "other"},
		{"tampered payload", validParts[0] + "." + segment(t, tamperedPayload) + "." + validParts[2], tokens.AudienceDashboard},
		{"truncated signature", validParts[0] + "." + validParts[1] + "." + validParts[2][:10], tokens.AudienceDashboard},
		{"lifetime beyond ttl", forge(t, one.private, header(nil), farFuture), tokens.AudienceDashboard},
		{"issued in the future", forge(t, one.private, header(nil), futureIssued), tokens.AudienceDashboard},
		{"missing exp", forge(t, one.private, header(nil), noExp), tokens.AudienceDashboard},
		{"missing auth_time", forge(t, one.private, header(nil), noAuthTime), tokens.AudienceDashboard},
		{"two parts", validParts[0] + "." + validParts[1], tokens.AudienceDashboard},
		{"four parts", valid + ".x", tokens.AudienceDashboard},
		{"empty", "", tokens.AudienceDashboard},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := verifier.Verify(tt.token, tt.aud); !errors.Is(err, tokens.ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerify_PlatformAudienceIsDistinct(t *testing.T) {
	now := testNow
	keys := mustKeySet(t, testKIDOne, newTestKey(t, testKIDOne, nil))
	issuer := tokens.NewIssuer(keys, clockAt(&now))
	verifier := tokens.NewVerifier(keys, clockAt(&now))
	claims := testClaims()
	claims.Audience = tokens.AudiencePlatform
	token, err := issuer.Issue(claims)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.Verify(token, tokens.AudiencePlatform); err != nil {
		t.Fatalf("platform: %v", err)
	}
	if _, err := verifier.Verify(token, tokens.AudienceDashboard); !errors.Is(err, tokens.ErrInvalidToken) {
		t.Fatalf("dashboard err = %v", err)
	}
}

func TestVerify_RejectsKeyPastNotAfter(t *testing.T) {
	now := testNow
	notAfter := testNow.Add(10 * time.Minute)
	old := newTestKey(t, testKIDOne, &notAfter)
	current := newTestKey(t, testKIDTwo, nil)
	both := mustKeySet(t, testKIDOne, old, current)
	token, err := tokens.NewIssuer(both, clockAt(&now)).Issue(testClaims())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	verifier := tokens.NewVerifier(both, clockAt(&now))
	now = notAfter
	if _, err := verifier.Verify(token, tokens.AudienceDashboard); !errors.Is(err, tokens.ErrInvalidToken) {
		t.Fatalf("err = %v", err)
	}
}

// T3: rotation overlap.
func TestRotation_OverlapAcceptsOldKeyUntilRemoved(t *testing.T) {
	now := testNow
	k1 := newTestKey(t, testKIDOne, nil)
	k2 := newTestKey(t, testKIDTwo, nil)

	beforeRotation := mustKeySet(t, testKIDOne, k1)
	tokenK1, err := tokens.NewIssuer(beforeRotation, clockAt(&now)).Issue(testClaims())
	if err != nil {
		t.Fatalf("issue k1: %v", err)
	}

	overlap := mustKeySet(t, testKIDTwo, k1, k2)
	tokenK2, err := tokens.NewIssuer(overlap, clockAt(&now)).Issue(testClaims())
	if err != nil {
		t.Fatalf("issue k2: %v", err)
	}
	if kid := decodeSegment(t, strings.Split(tokenK2, ".")[0])["kid"]; kid != testKIDTwo {
		t.Fatalf("new token kid = %v, want %s", kid, testKIDTwo)
	}
	verifier := tokens.NewVerifier(overlap, clockAt(&now))
	for name, token := range map[string]string{"k1": tokenK1, "k2": tokenK2} {
		if _, err := verifier.Verify(token, tokens.AudienceDashboard); err != nil {
			t.Errorf("%s token must verify during overlap: %v", name, err)
		}
	}

	afterRetire := mustKeySet(t, testKIDTwo, k2)
	retired := tokens.NewVerifier(afterRetire, clockAt(&now))
	if _, err := retired.Verify(tokenK1, tokens.AudienceDashboard); !errors.Is(err, tokens.ErrInvalidToken) {
		t.Fatalf("k1 token after removal err = %v", err)
	}
	if _, err := retired.Verify(tokenK2, tokens.AudienceDashboard); err != nil {
		t.Fatalf("k2 token: %v", err)
	}
}
