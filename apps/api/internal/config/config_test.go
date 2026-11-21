package config_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/iris-gate/iris-gate/apps/api/internal/config"
	"github.com/iris-gate/iris-gate/libs/identity/infra/tokens"
)

// Shared fake, non-secret values used across configuration tests.
const (
	overrideURL          = "postgres://override@db.test:5432/iris?sslmode=disable"
	expectedMailerHost   = "localhost"
	expectedMailerPort   = 11025
	expectedMailerFrom   = "no-reply@iris-gate.local"
	overrideMailerHost   = "smtp.example.test"
	overrideMailerPort   = "587"
	overrideMailerFrom   = "Iris Gate <mail@example.test>"
	overrideMailerUser   = "smtp-test-user"
	overrideMailerSecret = "smtp-test-secret-credential"
	testJWTKID           = "test-kid"
	testJWTOtherKID      = "test-other-kid"
	testJWTMalformedKeys = "not json"
	redactedMarker       = "[REDACTED]"
)

func TestLoad_DatabaseURLs(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		wantApp    string
		wantWorker string
		wantErr    error
	}{
		{
			name:       "defaults use the local app and worker roles",
			wantApp:    "postgres://iris_app:iris_app@localhost:15432/iris?sslmode=disable",
			wantWorker: "postgres://iris_worker:iris_worker@localhost:15432/iris?sslmode=disable",
		},
		{
			name:       "environment overrides each role",
			env:        map[string]string{config.EnvAppDatabaseURL: overrideURL, config.EnvWorkerDatabaseURL: overrideURL},
			wantApp:    overrideURL,
			wantWorker: overrideURL,
		},
		{
			name:    "empty app URL fails",
			env:     map[string]string{config.EnvAppDatabaseURL: ""},
			wantErr: config.ErrEmptyDatabaseURL,
		},
		{
			name:    "empty worker URL fails",
			env:     map[string]string{config.EnvWorkerDatabaseURL: ""},
			wantErr: config.ErrEmptyDatabaseURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			cfg, err := config.Load()

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if cfg.AppDatabaseURL != tt.wantApp {
				t.Errorf("AppDatabaseURL = %q, want %q", cfg.AppDatabaseURL, tt.wantApp)
			}
			if cfg.WorkerDatabaseURL != tt.wantWorker {
				t.Errorf("WorkerDatabaseURL = %q, want %q", cfg.WorkerDatabaseURL, tt.wantWorker)
			}
		})
	}
}

func TestLoad_Mailer(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config.MailerConfig
		wantErr error
	}{
		{
			name: "defaults point at local Mailpit",
			want: config.MailerConfig{
				Host:     expectedMailerHost,
				Port:     expectedMailerPort,
				From:     expectedMailerFrom,
				Username: "",
				Password: "",
				TLSMode:  config.MailerTLSNone,
			},
		},
		{
			name: "environment overrides all mailer fields",
			env: map[string]string{
				config.EnvMailerHost:     overrideMailerHost,
				config.EnvMailerPort:     overrideMailerPort,
				config.EnvMailerFrom:     overrideMailerFrom,
				config.EnvMailerUsername: overrideMailerUser,
				config.EnvMailerPassword: overrideMailerSecret,
				config.EnvMailerTLSMode:  string(config.MailerTLSStartTLS),
			},
			want: config.MailerConfig{
				Host:     overrideMailerHost,
				Port:     587,
				From:     overrideMailerFrom,
				Username: overrideMailerUser,
				Password: overrideMailerSecret,
				TLSMode:  config.MailerTLSStartTLS,
			},
		},
		{
			name:    "empty host fails",
			env:     map[string]string{config.EnvMailerHost: "   ", config.EnvMailerPassword: overrideMailerSecret},
			wantErr: config.ErrInvalidMailerHost,
		},
		{
			name:    "host with control character fails",
			env:     map[string]string{config.EnvMailerHost: "localhost\r\n"},
			wantErr: config.ErrInvalidMailerHost,
		},
		{
			name:    "port zero fails",
			env:     map[string]string{config.EnvMailerPort: "0", config.EnvMailerPassword: overrideMailerSecret},
			wantErr: config.ErrInvalidMailerPort,
		},
		{
			name:    "port above 65535 fails",
			env:     map[string]string{config.EnvMailerPort: "65536", config.EnvMailerPassword: overrideMailerSecret},
			wantErr: config.ErrInvalidMailerPort,
		},
		{
			name:    "non-numeric port fails",
			env:     map[string]string{config.EnvMailerPort: "smtp", config.EnvMailerPassword: overrideMailerSecret},
			wantErr: config.ErrInvalidMailerPort,
		},
		{
			name:    "empty from fails",
			env:     map[string]string{config.EnvMailerFrom: "", config.EnvMailerPassword: overrideMailerSecret},
			wantErr: config.ErrInvalidMailerFrom,
		},
		{
			name:    "unparseable from fails",
			env:     map[string]string{config.EnvMailerFrom: "not-an-address", config.EnvMailerPassword: overrideMailerSecret},
			wantErr: config.ErrInvalidMailerFrom,
		},
		{
			name:    "crlf in from fails",
			env:     map[string]string{config.EnvMailerFrom: expectedMailerFrom + "\r\nBcc: evil@example.test"},
			wantErr: config.ErrInvalidMailerFrom,
		},
		{
			name:    "password without username fails without echoing password",
			env:     map[string]string{config.EnvMailerPassword: overrideMailerSecret},
			wantErr: config.ErrInvalidMailerAuth,
		},
		{
			name: "control char in password fails without echoing password",
			env: map[string]string{
				config.EnvMailerUsername: overrideMailerUser,
				config.EnvMailerPassword: overrideMailerSecret + "\n",
			},
			wantErr: config.ErrInvalidMailerAuth,
		},
		{
			name: "invalid TLS mode fails without echoing password",
			env: map[string]string{
				config.EnvMailerUsername: overrideMailerUser,
				config.EnvMailerPassword: overrideMailerSecret,
				config.EnvMailerTLSMode:  "invalid",
			},
			wantErr: config.ErrInvalidMailerTLSMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			cfg, err := config.Load()

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				if strings.Contains(err.Error(), overrideMailerSecret) {
					t.Fatalf("error echoed secret password: %q", err.Error())
				}
				return
			}
			if cfg.Mailer != tt.want {
				t.Errorf("Mailer = %+v, want %+v", cfg.Mailer, tt.want)
			}
		})
	}
}

func TestMailerConfig_RedactsPasswordInStringAndLogs(t *testing.T) {
	cfg := config.MailerConfig{
		Host:     overrideMailerHost,
		Port:     587,
		From:     overrideMailerFrom,
		Username: overrideMailerUser,
		Password: overrideMailerSecret,
		TLSMode:  config.MailerTLSStartTLS,
	}

	if strings.Contains(cfg.String(), overrideMailerSecret) {
		t.Fatalf("String() leaked password: %s", cfg.String())
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	logger.Info("loaded mailer config", "mailer", cfg)
	if strings.Contains(logBuf.String(), overrideMailerSecret) {
		t.Fatalf("LogValue() leaked password: %s", logBuf.String())
	}
}

func TestLoadMigrator_DatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr error
	}{
		{
			name: "default uses the local migrator role",
			want: "postgres://iris_migrator:iris_migrator@localhost:15432/iris?sslmode=disable",
		},
		{
			name: "environment overrides it",
			env:  map[string]string{config.EnvMigratorDatabaseURL: overrideURL},
			want: overrideURL,
		},
		{
			name:    "empty URL fails",
			env:     map[string]string{config.EnvMigratorDatabaseURL: ""},
			wantErr: config.ErrEmptyDatabaseURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			cfg, err := config.LoadMigrator()

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && cfg.DatabaseURL != tt.want {
				t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, tt.want)
			}
		})
	}
}

// TestLoad_IgnoresMigratorURL guards the split: the API process must not pick
// up the migrator role even when its variable is set.
func TestLoad_IgnoresMigratorURL(t *testing.T) {
	setEnv(t, map[string]string{config.EnvMigratorDatabaseURL: overrideURL})

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppDatabaseURL == overrideURL || cfg.WorkerDatabaseURL == overrideURL {
		t.Fatalf("API config uses the migrator URL: %+v", cfg)
	}
}

// setEnv unsets the database URL and mailer variables, then sets env, for this test only.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	vars := []string{
		config.EnvAppDatabaseURL,
		config.EnvWorkerDatabaseURL,
		config.EnvMigratorDatabaseURL,
		config.EnvMailerHost,
		config.EnvMailerPort,
		config.EnvMailerFrom,
		config.EnvMailerUsername,
		config.EnvMailerPassword,
		config.EnvMailerTLSMode,
		config.EnvJWTSigningKeys,
		config.EnvJWTActiveKID,
	}
	for _, name := range vars {
		t.Setenv(name, "") // registers the restore of the original value
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range env {
		t.Setenv(name, value)
	}
}

// jwtKeysJSON generates one fake Ed25519 key set at test time.
func jwtKeysJSON(t *testing.T, kid string, notAfter *time.Time) (string, string) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	entry := map[string]any{"kid": kid, "private_key_b64": base64.RawURLEncoding.EncodeToString(private)}
	if notAfter != nil {
		entry["not_after"] = notAfter.Format(time.RFC3339)
	}
	raw, err := json.Marshal([]any{entry})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw), base64.RawURLEncoding.EncodeToString(private)
}

func TestLoad_JWTKeys(t *testing.T) {
	valid, _ := jwtKeysJSON(t, testJWTKID, nil)
	past := time.Now().Add(-time.Hour)
	expired, _ := jwtKeysJSON(t, testJWTKID, &past)

	tests := []struct {
		name    string
		env     map[string]string
		wantErr error
		wantSet bool
	}{
		{name: "auth not enabled when both are unset"},
		{
			name:    "valid keys and active kid",
			env:     map[string]string{config.EnvJWTSigningKeys: valid, config.EnvJWTActiveKID: testJWTKID},
			wantSet: true,
		},
		{
			name:    "keys without active kid fail",
			env:     map[string]string{config.EnvJWTSigningKeys: valid},
			wantErr: tokens.ErrNoActiveKeyID,
		},
		{
			name:    "active kid without keys fail",
			env:     map[string]string{config.EnvJWTActiveKID: testJWTKID},
			wantErr: tokens.ErrNoSigningKeys,
		},
		{
			name:    "unknown active kid fails",
			env:     map[string]string{config.EnvJWTSigningKeys: valid, config.EnvJWTActiveKID: testJWTOtherKID},
			wantErr: tokens.ErrUnknownActiveKeyID,
		},
		{
			name:    "malformed keys fail",
			env:     map[string]string{config.EnvJWTSigningKeys: testJWTMalformedKeys, config.EnvJWTActiveKID: testJWTKID},
			wantErr: tokens.ErrMalformedSigningKeys,
		},
		{
			name:    "expired active key fails",
			env:     map[string]string{config.EnvJWTSigningKeys: expired, config.EnvJWTActiveKID: testJWTKID},
			wantErr: tokens.ErrActiveKeyExpired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			cfg, err := config.Load()

			if tt.wantErr != nil {
				if !errors.Is(err, config.ErrInvalidJWTKeys) || !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want ErrInvalidJWTKeys wrapping %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if tt.wantSet && (string(cfg.JWTSigningKeys) != valid || cfg.JWTActiveKID != testJWTKID) {
				t.Fatalf("JWT settings were not loaded")
			}
		})
	}
}

func TestLoad_JWTKeysNeverAppearInFormattedConfigOrErrors(t *testing.T) {
	valid, secret := jwtKeysJSON(t, testJWTKID, nil)
	setEnv(t, map[string]string{config.EnvJWTSigningKeys: valid, config.EnvJWTActiveKID: testJWTKID})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var logs bytes.Buffer
	slog.New(slog.NewTextHandler(&logs, nil)).Info("cfg", "keys", cfg.JWTSigningKeys)
	for _, out := range []string{fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), logs.String()} {
		if strings.Contains(out, secret) {
			t.Fatal("signing key leaked through formatting or logs")
		}
	}

	setEnv(t, map[string]string{config.EnvJWTSigningKeys: valid, config.EnvJWTActiveKID: testJWTOtherKID})
	_, err = config.Load()
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("error must exist and not echo the key: %v", err)
	}
}

// slog resolves LogValue only for a top-level attribute, so logging the whole
// Config, or marshaling it, reaches the nested SecretString through
// encoding/json or encoding.TextMarshaler instead.
func TestLoad_JWTKeysNeverAppearInJSONOrStructLogs(t *testing.T) {
	valid, secret := jwtKeysJSON(t, testJWTKID, nil)
	setEnv(t, map[string]string{config.EnvJWTSigningKeys: valid, config.EnvJWTActiveKID: testJWTKID})
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Control: the base64url key is found verbatim in an unredacted JSON
	// encoding (JSON escapes none of its characters), so the checks below can fail.
	if unredacted, _ := json.Marshal(string(cfg.JWTSigningKeys)); !strings.Contains(string(unredacted), secret) {
		t.Fatal("control: the raw key must be visible in an unredacted encoding")
	}

	byValue, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	byPointer, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config pointer: %v", err)
	}
	text, err := cfg.JWTSigningKeys.MarshalText()
	if err != nil {
		t.Fatalf("marshal text: %v", err)
	}
	var jsonLogs, textLogs bytes.Buffer
	slog.New(slog.NewJSONHandler(&jsonLogs, nil)).Info("startup", "cfg", cfg)
	slog.New(slog.NewTextHandler(&textLogs, nil)).Info("startup", "cfg", cfg)

	for name, out := range map[string]string{
		"json.Marshal(cfg)":  string(byValue),
		"json.Marshal(&cfg)": string(byPointer),
		"MarshalText":        string(text),
		"slog JSON handler":  jsonLogs.String(),
		"slog text handler":  textLogs.String(),
	} {
		if strings.Contains(out, secret) || strings.Contains(out, valid) {
			t.Errorf("%s leaked the signing key", name)
		}
		if !strings.Contains(out, redactedMarker) {
			t.Errorf("%s does not carry the redacted marker: %s", name, out)
		}
	}
}
