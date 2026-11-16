// Package config reads the apps/api commands' settings from the environment.
//
// Each database role has its own connection URL (to-be section 9.2): request
// handlers use iris_app, the outbox relay uses iris_worker and cmd/migrate
// uses iris_migrator. The API process never reads the migrator URL.
package config

import (
	"errors"
	"fmt"
	"os"
)

// Environment variable names of the per-role connection URLs.
const (
	EnvAppDatabaseURL      = "IRIS_APP_DATABASE_URL"
	EnvWorkerDatabaseURL   = "IRIS_WORKER_DATABASE_URL"
	EnvMigratorDatabaseURL = "IRIS_MIGRATOR_DATABASE_URL"
)

// Defaults point at the local Compose Postgres with the role passwords of
// infra/docker/.env.example. Any other environment sets the variables above.
const (
	defaultAppDatabaseURL      = "postgres://iris_app:iris_app@localhost:15432/iris?sslmode=disable"
	defaultWorkerDatabaseURL   = "postgres://iris_worker:iris_worker@localhost:15432/iris?sslmode=disable"
	defaultMigratorDatabaseURL = "postgres://iris_migrator:iris_migrator@localhost:15432/iris?sslmode=disable"
)

// ErrEmptyDatabaseURL is returned when a connection URL variable is set but
// empty: pgx would otherwise fall back to libpq defaults and connect as
// someone else.
var ErrEmptyDatabaseURL = errors.New("database URL is empty")

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
}

// MigratorConfig is the cmd/migrate configuration.
type MigratorConfig struct {
	// DatabaseURL connects as iris_migrator, the owner of the schema.
	DatabaseURL string
}

// Load reads the API process configuration.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		AppDatabaseURL:    env(EnvAppDatabaseURL, defaultAppDatabaseURL),
		WorkerDatabaseURL: env(EnvWorkerDatabaseURL, defaultWorkerDatabaseURL),
		KafkaBroker:       env("KAFKA_BROKER", "localhost:19092"),
		Env:               env("ENV", "dev"),
		AcquirerURL:       env("ACQUIRER_URL", ""),
		AcquirerToken:     env("ACQUIRER_TOKEN", ""),
		RelayEnabled:      env("RELAY_ENABLED", "true") == "true",
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
