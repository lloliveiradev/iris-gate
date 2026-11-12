package config

import "os"

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	KafkaBroker      string
	Env              string
	MigrateOnStartup bool

	// AcquirerURL empty selects FakeAcquirer (dev and CI).
	// Point it at a sandbox when a real acquirer integration lands.
	AcquirerURL   string
	AcquirerToken string

	// Relay worker: turn it off in environments without Kafka (CI unit tests).
	RelayEnabled bool
}

func Load() (Config, error) {
	return Config{
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://iris:iris@localhost:15432/iris?sslmode=disable"),
		KafkaBroker:      env("KAFKA_BROKER", "localhost:19092"),
		Env:              env("ENV", "dev"),
		MigrateOnStartup: env("MIGRATE_ON_STARTUP", "true") == "true",
		AcquirerURL:      env("ACQUIRER_URL", ""),
		AcquirerToken:    env("ACQUIRER_TOKEN", ""),
		RelayEnabled:     env("RELAY_ENABLED", "true") == "true",
	}, nil
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}
