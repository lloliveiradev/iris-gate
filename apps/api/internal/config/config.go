package config

import "os"

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	KafkaBroker      string
	Env              string
	MigrateOnStartup bool

	// Acquirer: se AcquirerURL for vazio, apps/api usa FakeAcquirer (dev/CI).
	// Apontar pro Stone (ou outro) sandbox quando a integração real entrar.
	AcquirerURL   string
	AcquirerToken string

	// Relay worker: pode ser desligado em ambientes sem Kafka (CI unit, etc.).
	RelayEnabled bool
}

func Load() (Config, error) {
	return Config{
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://iris:iris@localhost:5432/iris?sslmode=disable"),
		KafkaBroker:      env("KAFKA_BROKER", "localhost:9092"),
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
