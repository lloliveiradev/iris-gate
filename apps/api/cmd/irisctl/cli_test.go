package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

const (
	testSecret     = "irk_pfx123.supersecretvalue" // gitleaks:allow fixture: not a real API key
	testMerchantID = "11111111-1111-1111-1111-111111111111"
	testKeyID      = "22222222-2222-2222-2222-222222222222"
)

type fakeCreator struct {
	got identityapp.CreateMerchantInput
	err error
	n   int
}

func (f *fakeCreator) Execute(_ context.Context, in identityapp.CreateMerchantInput) (domain.Merchant, error) {
	f.n++
	f.got = in
	if f.err != nil {
		return domain.Merchant{}, f.err
	}
	return domain.Merchant{
		ID:        uuid.MustParse(testMerchantID),
		LegalName: in.LegalName,
		Status:    domain.MerchantPending,
	}, nil
}

type fakeIssuer struct {
	got identityapp.IssueAPIKeyInput
	err error
	n   int
}

func (f *fakeIssuer) Execute(_ context.Context, in identityapp.IssueAPIKeyInput) (identityapp.IssueAPIKeyOutput, error) {
	f.n++
	f.got = in
	if f.err != nil {
		return identityapp.IssueAPIKeyOutput{}, f.err
	}
	return identityapp.IssueAPIKeyOutput{
		APIKey: domain.APIKey{
			ID:         uuid.MustParse(testKeyID),
			MerchantID: in.MerchantID,
			Prefix:     "pfx123",
			Scopes:     in.Scopes,
		},
		Token: testSecret,
	}, nil
}

type fakeRevoker struct {
	got uuid.UUID
	err error
	n   int
}

func (f *fakeRevoker) Execute(_ context.Context, id uuid.UUID) (domain.APIKey, error) {
	f.n++
	f.got = id
	if f.err != nil {
		return domain.APIKey{}, f.err
	}
	return domain.APIKey{ID: id, Status: domain.APIKeyRevoked}, nil
}

type rig struct {
	cli     cli
	creator *fakeCreator
	issuer  *fakeIssuer
	revoker *fakeRevoker
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
}

func newRig() *rig {
	r := &rig{
		creator: &fakeCreator{},
		issuer:  &fakeIssuer{},
		revoker: &fakeRevoker{},
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
	}
	r.cli = cli{
		createMerchant: r.creator,
		issueAPIKey:    r.issuer,
		revokeAPIKey:   r.revoker,
		stdout:         r.stdout,
		stderr:         r.stderr,
	}
	return r
}

func (r *rig) run(args ...string) int {
	return r.cli.run(context.Background(), args)
}

func TestCreateMerchant_PassesFlagsAndPrintsMerchant(t *testing.T) {
	t.Parallel()
	r := newRig()

	code := r.run("create-merchant", "--legal-name", "ACME", "--document", "12345678000199", "--contact-email", "ops@acme.test")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr=%s", code, r.stderr)
	}
	want := identityapp.CreateMerchantInput{LegalName: "ACME", Document: "12345678000199", ContactEmail: "ops@acme.test"}
	if r.creator.got != want {
		t.Fatalf("input = %+v, want %+v", r.creator.got, want)
	}
	var out merchantOutput
	if err := json.Unmarshal(r.stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not JSON: %q", r.stdout)
	}
	if out.ID != testMerchantID || out.Status != "pending" {
		t.Fatalf("output = %+v", out)
	}
}

func TestIssueAPIKey_PrintsSecretExactlyOnceAndNothingElseSecret(t *testing.T) {
	t.Parallel()
	r := newRig()

	code := r.run("issue-api-key", "--merchant-id", testMerchantID, "--scopes", "payments:read, payments:write,")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr=%s", code, r.stderr)
	}
	if n := strings.Count(r.stdout.String(), testSecret); n != 1 {
		t.Fatalf("secret printed %d times on stdout, want 1", n)
	}
	if strings.Contains(r.stderr.String(), testSecret) {
		t.Fatalf("secret leaked to stderr: %q", r.stderr)
	}
	if r.stderr.Len() != 0 {
		t.Fatalf("stderr should be empty on success, got %q", r.stderr)
	}
	if got := strings.Join(r.issuer.got.Scopes, "|"); got != "payments:read|payments:write" {
		t.Fatalf("scopes = %q", got)
	}
	if r.issuer.got.MerchantID.String() != testMerchantID {
		t.Fatalf("merchant id = %s", r.issuer.got.MerchantID)
	}
	var out issuedKeyOutput
	if err := json.Unmarshal(r.stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not JSON: %q", r.stdout)
	}
	if out.Secret != testSecret || out.Prefix != "pfx123" || out.ID != testKeyID {
		t.Fatalf("output = %+v", out)
	}
}

func TestIssueAPIKey_NoScopesMeansEmptyList(t *testing.T) {
	t.Parallel()
	r := newRig()

	if code := r.run("issue-api-key", "--merchant-id", testMerchantID); code != exitOK {
		t.Fatalf("exit = %d, stderr=%s", code, r.stderr)
	}
	if r.issuer.got.Scopes == nil || len(r.issuer.got.Scopes) != 0 {
		t.Fatalf("scopes = %#v, want empty non-nil", r.issuer.got.Scopes)
	}
}

func TestRevokeAPIKey_PrintsStatusWithoutSecret(t *testing.T) {
	t.Parallel()
	r := newRig()

	code := r.run("revoke-api-key", "--id", testKeyID)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr=%s", code, r.stderr)
	}
	if r.revoker.got.String() != testKeyID {
		t.Fatalf("id = %s", r.revoker.got)
	}
	var out revokedKeyOutput
	if err := json.Unmarshal(r.stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not JSON: %q", r.stdout)
	}
	if out.ID != testKeyID || out.Status != "revoked" {
		t.Fatalf("output = %+v", out)
	}
}

func TestUsageErrors_ExitTwoAndRunNoUseCase(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no command":                  {},
		"unknown command":             {"delete-everything"},
		"create missing flags":        {"create-merchant", "--legal-name", "ACME"},
		"create unknown flag":         {"create-merchant", "--nope", "x"},
		"create stray argument":       {"create-merchant", "--legal-name", "A", "--document", "1", "--contact-email", "e", "extra"},
		"issue missing merchant":      {"issue-api-key"},
		"issue malformed merchant":    {"issue-api-key", "--merchant-id", "not-a-uuid"},
		"revoke missing id":           {"revoke-api-key"},
		"revoke malformed id":         {"revoke-api-key", "--id", "not-a-uuid"},
		"secret flag is not accepted": {"issue-api-key", "--merchant-id", testMerchantID, "--secret", "x"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig()
			if code := r.run(args...); code != exitUsage {
				t.Fatalf("exit = %d, want %d; stderr=%s", code, exitUsage, r.stderr)
			}
			if r.creator.n+r.issuer.n+r.revoker.n != 0 {
				t.Fatalf("a use case ran on a usage error")
			}
			if r.stdout.Len() != 0 {
				t.Fatalf("stdout must stay empty on usage errors, got %q", r.stdout)
			}
		})
	}
}

func TestHelp_ExitsZero(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"help"}, {"--help"}, {"create-merchant", "-h"}} {
		r := newRig()
		if code := r.run(args...); code != exitOK {
			t.Fatalf("%v: exit = %d", args, code)
		}
	}
}

func TestUseCaseError_ExitOneWithoutOutputOnStdout(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.issuer.err = domain.ErrMerchantNotFound

	code := r.run("issue-api-key", "--merchant-id", testMerchantID)
	if code != exitError {
		t.Fatalf("exit = %d, want %d", code, exitError)
	}
	if r.stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", r.stdout)
	}
	if !strings.Contains(r.stderr.String(), domain.ErrMerchantNotFound.Error()) {
		t.Fatalf("stderr = %q", r.stderr)
	}
}

func TestErrors_DoNotEchoMerchantData(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.creator.err = errors.Join(domain.ErrValidation)

	code := r.run("create-merchant", "--legal-name", "ACME", "--document", "99999999999999", "--contact-email", "private@acme.test")
	if code != exitError {
		t.Fatalf("exit = %d", code)
	}
	for _, leaked := range []string{"99999999999999", "private@acme.test"} {
		if strings.Contains(r.stderr.String(), leaked) {
			t.Fatalf("stderr echoes %q", leaked)
		}
	}
}

func TestNeedsDatabase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"help"}, false},
		{[]string{"bogus"}, false},
		{[]string{"create-merchant"}, true},
		{[]string{"issue-api-key"}, true},
		{[]string{"revoke-api-key"}, true},
	}
	for _, c := range cases {
		if got := needsDatabase(c.args); got != c.want {
			t.Errorf("needsDatabase(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}
