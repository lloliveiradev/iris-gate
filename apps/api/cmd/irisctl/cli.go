package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	"github.com/iris-gate/iris-gate/libs/identity/domain"
)

// Exit codes of irisctl.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const (
	cmdCreateMerchant = "create-merchant"
	cmdIssueAPIKey    = "issue-api-key"
	cmdRevokeAPIKey   = "revoke-api-key"
)

const usageText = `irisctl: operator CLI for merchants and API keys.

Usage:
  irisctl create-merchant --legal-name <name> --document <14 digits> --contact-email <email>
  irisctl issue-api-key   --merchant-id <uuid> [--scopes <a,b>]
  irisctl revoke-api-key  --id <uuid>

Output is one JSON object on stdout. The API key secret is printed once, by
issue-api-key, and is never stored or logged. Environment: DATABASE_URL.
`

// Ports consumed by the CLI, shaped by the commands that need them. The
// identity use cases satisfy them.
type (
	merchantCreator interface {
		Execute(ctx context.Context, in identityapp.CreateMerchantInput) (domain.Merchant, error)
	}
	apiKeyIssuer interface {
		Execute(ctx context.Context, in identityapp.IssueAPIKeyInput) (identityapp.IssueAPIKeyOutput, error)
	}
	apiKeyRevoker interface {
		Execute(ctx context.Context, id uuid.UUID) (domain.APIKey, error)
	}
)

// cli dispatches the subcommands to the identity use cases.
type cli struct {
	createMerchant merchantCreator
	issueAPIKey    apiKeyIssuer
	revokeAPIKey   apiKeyRevoker
	stdout         io.Writer
	stderr         io.Writer
}

// run executes one command and returns the process exit code.
func (c cli) run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(c.stderr, usageText)
		return exitUsage
	}
	name, rest := args[0], args[1:]

	var (
		out any
		err error
	)
	switch name {
	case cmdCreateMerchant:
		out, err = c.runCreateMerchant(ctx, rest)
	case cmdIssueAPIKey:
		out, err = c.runIssueAPIKey(ctx, rest)
	case cmdRevokeAPIKey:
		out, err = c.runRevokeAPIKey(ctx, rest)
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(c.stdout, usageText)
		return exitOK
	default:
		_, _ = fmt.Fprintf(c.stderr, "irisctl: unknown command %q\n\n%s", name, usageText)
		return exitUsage
	}

	var usageErr usageError
	switch {
	case errors.Is(err, flag.ErrHelp):
		return exitOK
	case errors.As(err, &usageErr):
		_, _ = fmt.Fprintf(c.stderr, "irisctl %s: %v\n", name, err)
		return exitUsage
	case err != nil:
		_, _ = fmt.Fprintf(c.stderr, "irisctl %s: %v\n", name, err)
		return exitError
	}
	if err := json.NewEncoder(c.stdout).Encode(out); err != nil {
		_, _ = fmt.Fprintf(c.stderr, "irisctl %s: write output: %v\n", name, err)
		return exitError
	}
	return exitOK
}

// usageError marks a mistake in the arguments, as opposed to a failed command.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func (c cli) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	return fs
}

// parse parses args and rejects stray positional arguments.
func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return usageError{msg: err.Error()}
	}
	if fs.NArg() > 0 {
		return usageError{msg: fmt.Sprintf("unexpected argument %q", fs.Arg(0))}
	}
	return nil
}

type merchantOutput struct {
	ID        string `json:"id"`
	LegalName string `json:"legal_name"`
	Status    string `json:"status"`
}

func (c cli) runCreateMerchant(ctx context.Context, args []string) (any, error) {
	fs := c.newFlagSet(cmdCreateMerchant)
	legalName := fs.String("legal-name", "", "merchant legal name (required)")
	document := fs.String("document", "", "14-digit document (required)")
	contactEmail := fs.String("contact-email", "", "contact email (required)")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	if *legalName == "" || *document == "" || *contactEmail == "" {
		return nil, usageError{msg: "--legal-name, --document and --contact-email are required"}
	}

	m, err := c.createMerchant.Execute(ctx, identityapp.CreateMerchantInput{
		LegalName:    *legalName,
		Document:     *document,
		ContactEmail: *contactEmail,
	})
	if err != nil {
		return nil, fmt.Errorf("create merchant: %w", err)
	}
	return merchantOutput{ID: m.ID.String(), LegalName: m.LegalName, Status: string(m.Status)}, nil
}

// issuedKeyOutput is the only output that carries a secret. It is produced
// once, by issue-api-key.
type issuedKeyOutput struct {
	ID         string   `json:"id"`
	MerchantID string   `json:"merchant_id"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	Secret     string   `json:"secret"`
}

func (c cli) runIssueAPIKey(ctx context.Context, args []string) (any, error) {
	fs := c.newFlagSet(cmdIssueAPIKey)
	merchantID := fs.String("merchant-id", "", "merchant UUID (required)")
	scopes := fs.String("scopes", "", "comma-separated scopes (optional)")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	id, err := parseUUIDFlag("--merchant-id", *merchantID)
	if err != nil {
		return nil, err
	}

	issued, err := c.issueAPIKey.Execute(ctx, identityapp.IssueAPIKeyInput{
		MerchantID: id,
		Scopes:     splitScopes(*scopes),
	})
	if err != nil {
		return nil, fmt.Errorf("issue api key: %w", err)
	}
	return issuedKeyOutput{
		ID:         issued.APIKey.ID.String(),
		MerchantID: issued.APIKey.MerchantID.String(),
		Prefix:     issued.APIKey.Prefix,
		Scopes:     issued.APIKey.Scopes,
		Secret:     issued.Token,
	}, nil
}

type revokedKeyOutput struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (c cli) runRevokeAPIKey(ctx context.Context, args []string) (any, error) {
	fs := c.newFlagSet(cmdRevokeAPIKey)
	keyID := fs.String("id", "", "API key UUID (required)")
	if err := parse(fs, args); err != nil {
		return nil, err
	}
	id, err := parseUUIDFlag("--id", *keyID)
	if err != nil {
		return nil, err
	}

	k, err := c.revokeAPIKey.Execute(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("revoke api key: %w", err)
	}
	return revokedKeyOutput{ID: k.ID.String(), Status: string(k.Status)}, nil
}

func parseUUIDFlag(flagName, value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.UUID{}, usageError{msg: flagName + " is required"}
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, usageError{msg: flagName + " must be a UUID"}
	}
	return id, nil
}

// splitScopes turns "a, b" into ["a" "b"]; an empty input yields no scopes.
func splitScopes(raw string) []string {
	scopes := []string{}
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}
