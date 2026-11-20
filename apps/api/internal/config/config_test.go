package config_test

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/iris-gate/iris-gate/apps/api/internal/config"
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
