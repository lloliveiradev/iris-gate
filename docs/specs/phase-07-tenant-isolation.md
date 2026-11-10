# Phase 07: Database roles and tenant isolation

**Depends on:** 06. **Design:** [`to-be.md`](../design/to-be.md) section 9.

## Goal

Make tenant isolation a property of the database and of the type system, not only of careful handlers, and give each service the least database privilege it needs.

## In scope

1. **Roles.** SQL init scripts in `infra/docker/postgres/initdb/` create, with passwords from `infra/docker/.env` (placeholders in `.env.example`):
   - `iris_migrator`, `iris_app`, `iris_worker`, `iris_platform` for database `iris`;
   - `antifraud_migrator`, `antifraud_app` for database `antifraud`.

   `REVOKE CONNECT ... FROM PUBLIC` on each database, then `GRANT CONNECT` only to its own roles. The migrator owns the tables; the app role gets `SELECT, INSERT, UPDATE, DELETE` on its tables and `USAGE` on sequences, nothing else.
2. **Wiring.** Migrations (Go `make migrate`, Prisma `migrate deploy`) use the migrator DSN. apps/api request handlers use `iris_app`. The outbox relay uses `iris_worker`. apps/anti-fraud runtime uses `antifraud_app`. Separate env vars per role, documented in `.env.example`.
3. **RLS** on every merchant-scoped table of the payments context that exists after phase 06 (`payment_intents`, `idempotency_records`, and any other with `merchant_id`). Policy: `merchant_id = current_setting('app.merchant_id', true)::uuid`. `ENABLE` and `FORCE ROW LEVEL SECURITY`. `iris_worker` and `iris_platform` have `BYPASSRLS`; `iris_app` does not.
4. **Tenant transaction helper** in `libs/shared` (for example `pgtenant.WithMerchant(ctx, pool, merchantID, fn)`), which opens a transaction and runs `SELECT set_config('app.merchant_id', $1, true)` first. Payments repositories run every merchant-scoped statement inside it.
5. **Cross-tenant test suite.** A table-driven HTTP test in apps/api that, for each route touching a merchant resource, creates the resource as merchant A and calls the route as merchant B. Expected: 404 and no change. A **meta-test** walks the chi router and fails if a mounted route is neither in the suite nor in an explicit "not tenant-scoped" list with a reason.

## Out of scope

- Identity tables stay without RLS (to-be section 8.1); they are covered by the cross-tenant suite only.
- Anti-fraud gets roles but no RLS.
- The vault database (phase 14 adds its own roles).

## Guidelines

- `set_config(..., true)` is transaction-local. Never use plain `SET`, because pooled connections would leak the setting to the next request.
- A statement that runs on `iris_app` outside the helper must return zero rows from an RLS table. Prove it with a test.
- Keep migrations runnable from an empty database and from the state after phase 06.
- The meta-test is the guard for every later phase: make its failure message say exactly which route is missing and where to add it.

## Acceptance criteria

1. `antifraud_app` cannot connect to `iris`; `iris_app` cannot connect to `antifraud`.
2. `iris_app` cannot `CREATE`, `ALTER` or `DROP` anything.
3. With `iris_app` and no `app.merchant_id`, `SELECT * FROM payment_intents` returns zero rows, even when rows exist.
4. With `app.merchant_id` set to A, merchant B's rows are invisible and cannot be updated.
5. The cross-tenant suite and its meta-test pass in `make test`.
6. A fresh `make nuke && make bootstrap && make migrate` reaches a working stack, and the README says how.

## Required tests

Integration tests for 1 to 4 (they need Postgres), the HTTP suite and meta-test for 5.

## Docs to update

`AGENTS.md` (remove "do not pretend row-level security exists" once it does; add the tenant helper and meta-test rules), `docs/security.md` (mark RLS and DB roles as implemented), README (env vars).
