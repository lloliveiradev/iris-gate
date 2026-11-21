// Package config reads the apps/api commands' settings from the environment.
//
// Each database role has its own connection URL (to-be section 9.2): request
// handlers use iris_app, the outbox relay uses iris_worker and cmd/migrate
// uses iris_migrator. The API process never reads the migrator URL.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/iris-gate/iris-gate/libs/identity/infra/tokens"
)

// Environment variable names of the per-role connection URLs.
const (
	EnvAppDatabaseURL      = "IRIS_APP_DATABASE_URL"
	EnvWorkerDatabaseURL   = "IRIS_WORKER_DATABASE_URL"
	EnvMigratorDatabaseURL = "IRIS_MIGRATOR_DATABASE_URL"
)

// Environment variable names for the outbound SMTP mailer.
const (
	EnvMailerHost     = "MAILER_HOST"
	EnvMailerPort     = "MAILER_PORT"
	EnvMailerFrom     = "MAILER_FROM"
	EnvMailerUsername = "MAILER_USERNAME"
	EnvMailerPassword = "MAILER_PASSWORD"
	EnvMailerTLSMode  = "MAILER_TLS_MODE"
)

// Environment variable names of the access-token signing keys (ADR-0006).
const (
	EnvJWTSigningKeys = "JWT_SIGNING_KEYS"
	EnvJWTActiveKID   = "JWT_ACTIVE_KID"
)

// redactedValue replaces a secret wherever a Config is formatted.
const redactedValue = "[REDACTED]"

// Defaults point at the local Compose Postgres with the role passwords of
// infra/docker/.env.example, and at the local Mailpit SMTP container.
// Any other environment sets the variables above.
const (
	defaultAppDatabaseURL      = "postgres://iris_app:iris_app@localhost:15432/iris?sslmode=disable"
	defaultWorkerDatabaseURL   = "postgres://iris_worker:iris_worker@localhost:15432/iris?sslmode=disable"
	defaultMigratorDatabaseURL = "postgres://iris_migrator:iris_migrator@localhost:15432/iris?sslmode=disable"

	defaultMailerHost    = "localhost"
	defaultMailerPort    = 11025
	defaultMailerFrom    = "no-reply@iris-gate.local"
	defaultMailerTLSMode = MailerTLSNone

	minMailerPort = 1
	maxMailerPort = 65535
)

// MailerTLSMode controls how the SMTP mailer negotiates transport encryption.
type MailerTLSMode string

const (
	// MailerTLSNone uses plaintext SMTP (local Mailpit).
	MailerTLSNone MailerTLSMode = "none"
	// MailerTLSStartTLS upgrades the SMTP connection with STARTTLS.
	MailerTLSStartTLS MailerTLSMode = "starttls"
)

// Configuration validation errors. None of them echo secret values.
var (
	// ErrEmptyDatabaseURL is returned when a connection URL variable is set but
	// empty: pgx would otherwise fall back to libpq defaults and connect as
	// someone else.
	ErrEmptyDatabaseURL = errors.New("database URL is empty")
	// ErrInvalidMailerHost is returned when MAILER_HOST is empty or contains control characters.
	ErrInvalidMailerHost = errors.New("mailer host is invalid")
	// ErrInvalidMailerPort is returned when MAILER_PORT is not an integer in 1..65535.
	ErrInvalidMailerPort = errors.New("mailer port must be between 1 and 65535")
	// ErrInvalidMailerFrom is returned when MAILER_FROM is empty, contains control characters, or fails RFC 5322 parsing.
	ErrInvalidMailerFrom = errors.New("mailer from address is invalid")
	// ErrInvalidMailerTLSMode is returned when MAILER_TLS_MODE is neither "none" nor "starttls".
	ErrInvalidMailerTLSMode = errors.New("mailer TLS mode must be none or starttls")
	// ErrInvalidMailerAuth is returned when MAILER_PASSWORD is set without MAILER_USERNAME or credentials contain control characters.
	ErrInvalidMailerAuth = errors.New("mailer authentication settings are invalid")
)

// ErrInvalidJWTKeys is returned when the access-token signing key settings are
// incomplete or invalid. Auth is enabled as soon as either JWT variable is
// set, and then both must describe a usable key set: there is no fallback key.
var ErrInvalidJWTKeys = errors.New("JWT signing key settings are invalid")

// SecretString holds a secret and prints as [REDACTED] in fmt and slog output.
type SecretString string

// String redacts the secret.
func (SecretString) String() string { return redactedValue }

// GoString redacts the secret under %#v.
func (SecretString) GoString() string { return redactedValue }

// LogValue implements slog.LogValuer.
func (SecretString) LogValue() slog.Value { return slog.StringValue(redactedValue) }

// MailerConfig holds the outbound SMTP settings.
type MailerConfig struct {
	Host     string
	Port     int
	From     string
	Username string
	// Password is optional and must never appear in logs or error text.
	Password string
	TLSMode  MailerTLSMode
}

// String formats MailerConfig with Password redacted so formatting cannot leak credentials.
func (m MailerConfig) String() string {
	password := ""
	if m.Password != "" {
		password = "[REDACTED]"
	}
	return fmt.Sprintf("MailerConfig{Host:%q Port:%d From:%q Username:%q Password:%q TLSMode:%q}",
		m.Host, m.Port, m.From, m.Username, password, m.TLSMode)
}

// LogValue implements slog.LogValuer so structured logs never emit Password in clear.
func (m MailerConfig) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("host", m.Host),
		slog.Int("port", m.Port),
		slog.String("from", m.From),
		slog.String("tls_mode", string(m.TLSMode)),
		slog.Bool("auth_configured", m.Username != ""),
	}
	return slog.GroupValue(attrs...)
}

// Config is the API process configuration (cmd/api and cmd/irisctl).
type Config struct {
	HTTPAddr string
	// AppDatabaseURL connects as iris_app: DML only, never DDL.
	AppDatabaseURL string
	// WorkerDatabaseURL connects as iris_worker, for the outbox relay.
	WorkerDatabaseURL string
	KafkaBroker       string
	Env               string

	// AcquirerURL empty selects FakeAcquirer (dev and CI).
	// Point it at a sandbox when a real acquirer integration lands.
	AcquirerURL   string
	AcquirerToken string

	// Relay worker: turn it off in environments without Kafka (CI unit tests).
	RelayEnabled bool

	// Mailer holds outbound SMTP settings (defaults target local Mailpit).
	Mailer MailerConfig

	// JWTSigningKeys is the JSON array of Ed25519 private keys and JWTActiveKID
	// the kid that signs new tokens (ADR-0006). Both empty means auth is not
	// enabled; either one set means both are validated at load. Nothing reads
	// them yet (wired by a later phase 08 card).
	JWTSigningKeys SecretString
	JWTActiveKID   string
}

// MigratorConfig is the cmd/migrate configuration.
type MigratorConfig struct {
	// DatabaseURL connects as iris_migrator, the owner of the schema.
	DatabaseURL string
}

// Load reads the API process configuration.
func Load() (Config, error) {
	mailerCfg, err := loadMailer()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		AppDatabaseURL:    env(EnvAppDatabaseURL, defaultAppDatabaseURL),
		WorkerDatabaseURL: env(EnvWorkerDatabaseURL, defaultWorkerDatabaseURL),
		KafkaBroker:       env("KAFKA_BROKER", "localhost:19092"),
		Env:               env("ENV", "dev"),
		AcquirerURL:       env("ACQUIRER_URL", ""),
		AcquirerToken:     env("ACQUIRER_TOKEN", ""),
		RelayEnabled:      env("RELAY_ENABLED", "true") == "true",
		Mailer:            mailerCfg,
		JWTSigningKeys:    SecretString(env(EnvJWTSigningKeys, "")),
		JWTActiveKID:      env(EnvJWTActiveKID, ""),
	}
	if err := validateJWTKeys(cfg.JWTSigningKeys, cfg.JWTActiveKID); err != nil {
		return Config{}, err
	}
	if err := requireURL(EnvAppDatabaseURL, cfg.AppDatabaseURL); err != nil {
		return Config{}, err
	}
	if err := requireURL(EnvWorkerDatabaseURL, cfg.WorkerDatabaseURL); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LoadMigrator reads the cmd/migrate configuration.
func LoadMigrator() (MigratorConfig, error) {
	cfg := MigratorConfig{DatabaseURL: env(EnvMigratorDatabaseURL, defaultMigratorDatabaseURL)}
	if err := requireURL(EnvMigratorDatabaseURL, cfg.DatabaseURL); err != nil {
		return MigratorConfig{}, err
	}
	return cfg, nil
}

// validateJWTKeys fails closed: when auth is enabled (either variable set) the
// key set must parse and its active key must be usable now. Errors never echo
// key material.
func validateJWTKeys(signingKeys SecretString, activeKID string) error {
	if signingKeys == "" && activeKID == "" {
		return nil
	}
	if _, err := tokens.ParseKeySet(string(signingKeys), activeKID, time.Now()); err != nil {
		return fmt.Errorf("%s/%s: %w: %w", EnvJWTSigningKeys, EnvJWTActiveKID, ErrInvalidJWTKeys, err)
	}
	return nil
}

func loadMailer() (MailerConfig, error) {
	host := env(EnvMailerHost, defaultMailerHost)
	if strings.TrimSpace(host) == "" || hasControlChar(host) {
		return MailerConfig{}, fmt.Errorf("%s: %w", EnvMailerHost, ErrInvalidMailerHost)
	}

	rawPort := env(EnvMailerPort, strconv.Itoa(defaultMailerPort))
	port, err := strconv.Atoi(strings.TrimSpace(rawPort))
	if err != nil || port < minMailerPort || port > maxMailerPort {
		return MailerConfig{}, fmt.Errorf("%s: %w", EnvMailerPort, ErrInvalidMailerPort)
	}

	from := env(EnvMailerFrom, defaultMailerFrom)
	if !isValidMailAddress(from) {
		return MailerConfig{}, fmt.Errorf("%s: %w", EnvMailerFrom, ErrInvalidMailerFrom)
	}

	username := env(EnvMailerUsername, "")
	password := env(EnvMailerPassword, "")
	if (password != "" && username == "") || hasControlChar(username) || hasControlChar(password) {
		return MailerConfig{}, fmt.Errorf("%s: %w", EnvMailerUsername, ErrInvalidMailerAuth)
	}

	tlsMode := MailerTLSMode(env(EnvMailerTLSMode, string(defaultMailerTLSMode)))
	if tlsMode != MailerTLSNone && tlsMode != MailerTLSStartTLS {
		return MailerConfig{}, fmt.Errorf("%s: %w", EnvMailerTLSMode, ErrInvalidMailerTLSMode)
	}

	return MailerConfig{
		Host:     strings.TrimSpace(host),
		Port:     port,
		From:     from,
		Username: username,
		Password: password,
		TLSMode:  tlsMode,
	}, nil
}

func isValidMailAddress(raw string) bool {
	if strings.TrimSpace(raw) == "" || hasControlChar(raw) {
		return false
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil || hasControlChar(addr.Address) || hasControlChar(addr.Name) {
		return false
	}
	local, domain, ok := strings.Cut(addr.Address, "@")
	return ok && local != "" && domain != "" && !strings.ContainsAny(addr.Address, " \t")
}

func hasControlChar(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func requireURL(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s: %w", name, ErrEmptyDatabaseURL)
	}
	return nil
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}
