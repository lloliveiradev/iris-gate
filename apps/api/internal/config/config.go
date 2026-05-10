package config

import "os"

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	KafkaBroker      string
	Env              string
	MigrateOnStartup bool
}

func Load() (Config, error) {
	return Config{
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://iris:iris@localhost:5432/iris?sslmode=disable"),
		KafkaBroker:      env("KAFKA_BROKER", "localhost:9092"),
		Env:              env("ENV", "dev"),
		MigrateOnStartup: env("MIGRATE_ON_STARTUP", "true") == "true",
	}, nil
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}
