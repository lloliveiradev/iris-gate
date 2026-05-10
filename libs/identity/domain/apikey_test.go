package domain_test

import (
	"errors"
	"testing"

	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

func TestParseToken_RoundTrip(t *testing.T) {
	t.Parallel()

	got := domain.FormatToken("abcdef0123456789", "secret-payload-base64url-x")
	prefix, secret, err := domain.ParseToken(got)
	if err != nil {
		t.Fatalf("ParseToken err = %v", err)
	}
	if prefix != "abcdef0123456789" {
		t.Errorf("prefix = %q", prefix)
	}
	if secret != "secret-payload-base64url-x" {
		t.Errorf("secret = %q", secret)
	}
}

func TestParseToken_Invalid(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"plain-string",
		"irk_only-prefix",
		"irk_.empty-prefix",
		"irk_prefix.",
		"wrong_prefix.secret",
	}
	for _, in := range cases {
		in := in
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			_, _, err := domain.ParseToken(in)
			if !errors.Is(err, domain.ErrAPIKeyInvalid) {
				t.Fatalf("ParseToken(%q) err = %v, want ErrAPIKeyInvalid", in, err)
			}
		})
	}
}
