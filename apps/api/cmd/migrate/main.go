// Command migrate applies the embedded identity and payments migrations to the
// iris database as iris_migrator, the role that owns the schema. It is the only
// apps/api command that runs DDL; the API process connects as iris_app and
// cannot. Run it with `make migrate` once the stack is up.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5"

	"github.com/iris-gate/iris-gate/apps/api/internal/config"

	identitypg "github.com/iris-gate/iris-gate/libs/identity/infra/pgstore"
	paymentspg "github.com/iris-gate/iris-gate/libs/payments/infra/pgstore"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("migrate", "err", err)
		stop()
		os.Exit(1)
	}
	slog.Info("migrations applied")
}

// run connects as the migrator and applies identity before payments, whose
// tables reference merchants. The connection URL carries a password, so it is
// never logged.
func run(ctx context.Context) error {
	cfg, err := config.LoadMigrator()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect as migrator: %w", err)
	}
	defer func() {
		if err := conn.Close(context.WithoutCancel(ctx)); err != nil {
			slog.Error("close connection", "err", err)
		}
	}()

	if err := identitypg.MigrateEmbedded(ctx, conn); err != nil {
		return fmt.Errorf("migrate identity: %w", err)
	}
	if err := paymentspg.MigrateEmbedded(ctx, conn); err != nil {
		return fmt.Errorf("migrate payments: %w", err)
	}
	return nil
}
