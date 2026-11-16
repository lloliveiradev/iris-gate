package config_test

import (
	"errors"
	"os"
	"testing"

	"github.com/iris-gate/iris-gate/apps/api/internal/config"
)

// overrideURL stands in for a connection URL set by the environment.
const overrideURL = "postgres://override@db.test:5432/iris?sslmode=disable"

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

// setEnv unsets the three URL variables, then sets env, for this test only.
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, name := range []string{config.EnvAppDatabaseURL, config.EnvWorkerDatabaseURL, config.EnvMigratorDatabaseURL} {
		t.Setenv(name, "") // registers the restore of the original value
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range env {
		t.Setenv(name, value)
	}
}
