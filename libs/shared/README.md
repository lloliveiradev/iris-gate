# shared

Cross-cutting code with no business logic and no domain entities. It is not a
bounded context: contexts and `apps/api` may import it, and it imports none of
them (the `shared-isolation` depguard rule in `configs/go/golangci.yml`).

Acceptable here:

- Base `Error`/`Result`/`Either` types
- Logger, config loader, generic HTTP client
- Generic value-object base classes (e.g. `UniqueId`)
- Pure utility functions (`pipe`, `clamp`, date helpers)
- Infrastructure helpers every context needs, such as `pgtenant`

If you're tempted to add something domain-specific here, stop and put it in the
matching `libs/<context>/` instead. `shared/` should _shrink_ over time, not
grow.

## Go module

`github.com/iris-gate/iris-gate/libs/shared`, listed in `go.work` and in the root
`PACKAGES`. Because `go.work` lists it, `apps/api/Dockerfile` copies its
`go.mod`, `go.sum` and sources; `apps/api/Dockerfile.dockerignore` keeps `src/`
out of the image context.

### `pgtenant`

`pgtenant.WithMerchant(ctx, db, merchantID, fn)` runs `fn` in a transaction
scoped to one merchant. It is the helper the payments repositories will use for
every merchant-scoped statement once row-level security is on (phase 07).

- `merchantID` must be a UUID in the hyphenated form. Upper-case hex digits are
  normalized to lower case. Empty, malformed, braced, URN, unhyphenated and nil
  UUIDs return `ErrInvalidMerchantID` before any transaction is opened.
- The first statement is `SELECT set_config('app.merchant_id', $1, true)`: a
  bind parameter, and transaction-local, so the setting ends with the
  transaction and never reaches the next user of a pooled connection.
- `fn` commits by returning `nil`. An error, a done context, a panic (re-raised
  after the rollback) or `runtime.Goexit` rolls back. `fn`'s error is returned
  as is; a rollback failure is joined to it, never put in its place.
- Inside `fn`, use only the `tx` it receives. Nesting is not supported: `db` is
  a `TxBeginner` (`BeginTx`), which `*pgxpool.Pool` satisfies and `pgx.Tx` does
  not, so the transaction cannot be passed back in.

```go
var intent row
err := pgtenant.WithMerchant(ctx, pool, merchantID.String(), func(ctx context.Context, tx pgx.Tx) error {
	return tx.QueryRow(ctx, selectIntentSQL, id).Scan(&intent.ID, &intent.Status)
})
```

## Commands

```bash
make -C libs/shared test       # unit tests, -race
make -C libs/shared test-int   # adds the integration tag; needs Postgres and DATABASE_URL
make -C libs/shared lint
```

The integration test needs no table or migration; any role that can connect
runs it. It skips when `DATABASE_URL` is unset.

## TypeScript skeleton

`src/` (`domain/`, `infra/`, `utils/`) is an empty TypeScript skeleton, kept as
is. It is not a Yarn workspace yet and holds no code.
