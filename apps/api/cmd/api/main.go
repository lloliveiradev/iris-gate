package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iris-gate/iris-gate/apps/api/internal/config"
	"github.com/iris-gate/iris-gate/apps/api/internal/httpapi"

	"github.com/iris-gate/iris-gate/libs/identity/application"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/identity/infra/hasher"
	"github.com/iris-gate/iris-gate/libs/identity/infra/pgstore"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load", "err", err)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("pg connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.MigrateOnStartup {
		if err := pgstore.MigrateEmbedded(rootCtx, pool); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
	}

	repo := pgstore.New(pool)
	merchants, keys := repo.Repos()
	h := hasher.NewDefault()
	clock := application.SystemClock{}
	ids := application.UUIDGen{}
	secrets := hasher.SecretGenerator{}

	deps := identityhttp.Deps{
		CreateMerchant: application.NewCreateMerchantUseCase(merchants, clock, ids),
		IssueAPIKey:    application.NewIssueAPIKeyUseCase(merchants, keys, h, secrets, clock, ids),
		RevokeAPIKey:   application.NewRevokeAPIKeyUseCase(keys, clock),
		Authenticate:   application.NewAuthenticateAPIKeyUseCase(keys, merchants, h, clock),
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(httpapi.Mounters{Identity: deps}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("http listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-rootCtx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}
