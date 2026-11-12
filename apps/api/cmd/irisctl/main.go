// Command irisctl is the operator CLI for merchants and API keys. It replaces
// the public bootstrap HTTP routes closed by phase 06 (defect D8) and reuses
// the identity use cases. It never logs: the only secret it handles, a new API
// key, goes to stdout once.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iris-gate/iris-gate/apps/api/internal/config"

	identityapp "github.com/iris-gate/iris-gate/libs/identity/application"
	identityhasher "github.com/iris-gate/iris-gate/libs/identity/infra/hasher"
	identitypg "github.com/iris-gate/iris-gate/libs/identity/infra/pgstore"
)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Help and unknown commands never build the database pool: only commands
	// that run a use case need one.
	probe := cli{stdout: os.Stdout, stderr: os.Stderr}
	if !needsDatabase(os.Args[1:]) {
		return probe.run(ctx, os.Args[1:])
	}

	cfg, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "irisctl: load config: %v\n", err)
		return exitError
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "irisctl: connect to the database failed")
		return exitError
	}
	defer pool.Close()

	merchants, keys := identitypg.New(pool).Repos()
	clock := identityapp.SystemClock{}
	ids := identityapp.UUIDGen{}
	return cli{
		createMerchant: identityapp.NewCreateMerchantUseCase(merchants, clock, ids),
		issueAPIKey: identityapp.NewIssueAPIKeyUseCase(
			merchants, keys, identityhasher.NewDefault(), identityhasher.SecretGenerator{}, clock, ids),
		revokeAPIKey: identityapp.NewRevokeAPIKeyUseCase(keys, clock),
		stdout:       os.Stdout,
		stderr:       os.Stderr,
	}.run(ctx, os.Args[1:])
}

// needsDatabase reports whether args name a command that runs a use case.
func needsDatabase(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case cmdCreateMerchant, cmdIssueAPIKey, cmdRevokeAPIKey:
		return true
	}
	return false
}
