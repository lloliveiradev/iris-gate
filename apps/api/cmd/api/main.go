package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iris-gate/iris-gate/apps/api/internal/config"
	"github.com/iris-gate/iris-gate/apps/api/internal/httpapi"

	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	identityhasher "github.com/iris-gate/iris-gate/libs/identity/infra/hasher"
	identityhttp "github.com/iris-gate/iris-gate/libs/identity/infra/httpapi"
	identitypg "github.com/iris-gate/iris-gate/libs/identity/infra/pgstore"

	paymentsapp "github.com/iris-gate/iris-gate/libs/payments/application"
	"github.com/iris-gate/iris-gate/libs/payments/infra/acquirer"
	paymentshttp "github.com/iris-gate/iris-gate/libs/payments/infra/httpapi"
	"github.com/iris-gate/iris-gate/libs/payments/infra/idempotency"
	"github.com/iris-gate/iris-gate/libs/payments/infra/kafkarelay"
	paymentspg "github.com/iris-gate/iris-gate/libs/payments/infra/pgstore"
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

	// Request handlers run as iris_app, which has DML only. Migrations are not
	// applied here: `make migrate` runs cmd/migrate as iris_migrator.
	pool, err := pgxpool.New(rootCtx, cfg.AppDatabaseURL)
	if err != nil {
		slog.Error("pg connect", "role", "app", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Identity wiring. Merchants and API keys are managed by cmd/irisctl, not HTTP.
	identityRepo := identitypg.New(pool)
	merchants, keys := identityRepo.Repos()
	identityDeps := identityhttp.Deps{
		Authenticate: identityapp.NewAuthenticateAPIKeyUseCase(
			keys, merchants, identityhasher.NewDefault(), identityapp.SystemClock{}),
	}

	// Payments wiring (Phase 3).
	paymentsRepo := paymentspg.New(pool)
	idempotencyStore := paymentspg.NewIdempotencyStore(pool)

	var acq paymentsapp.Acquirer
	if cfg.AcquirerURL != "" {
		acq = acquirer.NewHTTP(cfg.AcquirerURL, cfg.AcquirerToken)
		slog.Info("acquirer", "kind", "http", "base", cfg.AcquirerURL)
	} else {
		acq = acquirer.NewFake()
		slog.Info("acquirer", "kind", "fake")
	}

	paymentsClock := paymentsapp.SystemClock{}
	paymentsIDs := paymentsapp.UUIDGen{}
	paymentsDeps := paymentshttp.Deps{
		CreatePaymentIntent: paymentsapp.NewCreatePaymentIntentUseCase(paymentsRepo, paymentsClock, paymentsIDs),
		AuthorizePayment:    paymentsapp.NewAuthorizePaymentUseCase(paymentsRepo, acq, paymentsClock, paymentsIDs),
		CapturePayment:      paymentsapp.NewCapturePaymentUseCase(paymentsRepo, paymentsClock, paymentsIDs),
		Repo:                paymentsRepo,
		AuthRequired:        identityhttp.RequireAPIKey(identityDeps.Authenticate),
		IdempotencyMW:       idempotency.Middleware(idempotencyStore, "create_payment_intent"),
	}

	// Outbox relay worker, on its own pool as iris_worker (to-be section 9.2).
	if cfg.RelayEnabled {
		workerPool, err := pgxpool.New(rootCtx, cfg.WorkerDatabaseURL)
		if err != nil {
			slog.Error("pg connect", "role", "worker", "err", err)
			os.Exit(1)
		}
		defer workerPool.Close()

		brokers := strings.Split(cfg.KafkaBroker, ",")
		writer := kafkarelay.NewKafkaWriter(brokers)
		defer func() {
			if err := writer.Close(); err != nil {
				slog.Error("close kafka writer", "err", err)
			}
		}()
		relay := kafkarelay.New(paymentspg.New(workerPool), writer, kafkarelay.Config{})
		go func() {
			if err := relay.Run(rootCtx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("kafka relay stopped", "err", err)
			}
		}()
		slog.Info("kafka relay started", "brokers", cfg.KafkaBroker)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(httpapi.Mounters{Identity: identityDeps, Payments: paymentsDeps}),
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
