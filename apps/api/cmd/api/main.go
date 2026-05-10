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

	pool, err := pgxpool.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("pg connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.MigrateOnStartup {
		if err := identitypg.MigrateEmbedded(rootCtx, pool); err != nil {
			slog.Error("migrate identity", "err", err)
			os.Exit(1)
		}
		if err := paymentspg.MigrateEmbedded(rootCtx, pool); err != nil {
			slog.Error("migrate payments", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
	}

	// Identity wiring (Phase 2).
	identityRepo := identitypg.New(pool)
	merchants, keys := identityRepo.Repos()
	hasher := identityhasher.NewDefault()
	identityClock := identityapp.SystemClock{}
	identityIDs := identityapp.UUIDGen{}
	secrets := identityhasher.SecretGenerator{}

	identityDeps := identityhttp.Deps{
		CreateMerchant: identityapp.NewCreateMerchantUseCase(merchants, identityClock, identityIDs),
		IssueAPIKey:    identityapp.NewIssueAPIKeyUseCase(merchants, keys, hasher, secrets, identityClock, identityIDs),
		RevokeAPIKey:   identityapp.NewRevokeAPIKeyUseCase(keys, identityClock),
		Authenticate:   identityapp.NewAuthenticateAPIKeyUseCase(keys, merchants, hasher, identityClock),
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

	// Outbox relay worker.
	if cfg.RelayEnabled {
		brokers := strings.Split(cfg.KafkaBroker, ",")
		writer := kafkarelay.NewKafkaWriter(brokers)
		defer writer.Close()
		relay := kafkarelay.New(paymentsRepo, writer, kafkarelay.Config{})
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
