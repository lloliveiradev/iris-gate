# Phase 06 plan: Security baseline

Spec: [`../specs/phase-06-security-baseline.md`](../specs/phase-06-security-baseline.md). Workflow: [`../specs/README.md`](../specs/README.md), "How to run a phase". Defects closed: D1, D2, D3, D4 (partially), D7, D8 from [`../design/as-is.md`](../design/as-is.md) section 8.1, plus the repository baseline (spec part B).

This plan is code-free: it only schedules the work. Nothing here changes the spec.

## 1. Delivery model

- **One pull request per task**, because the tasks run in parallel. Each task has its own branch `phase/06-<task-slug>` from `main`, its own Trello card (`Phase 06 · <task>`) and its own pull request.
- Part B items stay in their own pull requests, separate from the security fixes (spec guideline).
- **Migration numbers are global across contexts**: `0003` and `0004` are reserved below. No other task may add a migration. The next free number after this phase is `0005`. Both files go in `libs/payments/infra/pgstore/migrations/` (the context that owns the tables).
- **Merge order constraints** (the only dependencies between tasks):
  - T6 builds on T2 (same repository, use cases, handlers, and migration `0004` assumes `0003`). T6 starts from `main` after T2 merges, or from a branch stacked on T2.
  - T7 (audit) runs last, after T1 to T6 are merged.
  - T1, T2, T3, T4 and T5 are independent and can merge in any order.
- **Shared files** that several tasks touch and will conflict on: `README.md` (T1, T3, T5), `AGENTS.md` (T3, T5, T6), `Makefile` files (T1, T3, T5), `apps/api/cmd/api/main.go` (T2, T3). Each task edits only its own section and rebases before opening the PR.

## 2. Task list

Areas are Trello labels, not title prefixes.

### T1. `ig-` container prefix and host ports via `.env`

Labels: `infra`, `docs`.

Not an item of the spec's numbered scope; it is a repository baseline change requested for this phase (see section 7, assumption A1).

Steps:

1. Rename every `container_name: iris-*` in `infra/docker/docker-compose.yml` to `ig-*` (`ig-postgres`, `ig-kafka`, `ig-kafka-init`, `ig-karapace`, `ig-anti-fraud`, plus any other).
2. Make every host port come from `.env` with a documented default (`POSTGRES_PORT`, `KAFKA_PORT`, `KARAPACE_PORT`, `ANTI_FRAUD_PORT` already exist; add the API and web ports if they are bound anywhere).
3. Document each variable with a placeholder in `.env.example`; `.env` stays gitignored.
4. Update every reference to the old container names (`make psql`, `kafka-shell`, `ps`, docs, scripts).

Files: `infra/docker/docker-compose.yml`, `infra/docker/Makefile`, `Makefile`, `.env.example`, `README.md`, `tools/**` if it names a container, `docs/**` that names a container.

### T2. Merchant-scoped payments (D1, D2, D7)

Labels: `payments`, `api`, `security`, `contracts` (only if the 404 problem code needs a contract change).

Steps:

1. **D1.** Change the port to `GetByID(ctx, merchantID, id)`. The pgstore query filters `WHERE id = $2 AND merchant_id = $1`. Other merchant or missing id both return `ErrPaymentIntentNotFound`.
2. Pass the merchant id from the authenticated principal into `AuthorizePaymentUseCase.Execute`, `CapturePaymentUseCase.Execute` and the get handler. Drop the post-load `pi.MerchantID != merchant.ID` comparison in the get handler (the repository now enforces it) and map not-found to **404** `payment_intent_not_found` on all three routes.
3. **D2.** Migration `0003_idempotency_per_merchant.sql`: drop and recreate `idempotency_records` with primary key `(merchant_id, scope, key)` (`merchant_id UUID NOT NULL REFERENCES merchants (id)`). Existing rows are local test data and are deleted; say so in the migration comment.
4. Extend the idempotency `Store` port (`Existing`, `Save`) with the merchant id, read from the principal in the middleware. The middleware is passed the scope per route; `apps/api` keeps `create_payment_intent` and adds no scope for routes that do not use the middleware.
5. **D7.** Decide from the code: `GetByMerchantAndIdempotencyKey` in `libs/payments/infra/pgstore/pgstore.go` is a stub kept "for symmetry with the port", and idempotency is handled by the middleware. If CodeGraph and grep show no caller outside fakes and tests, **remove** it from the port, the stub and the fakes. Record the evidence in the PR. If a caller exists, implement it against Postgres.
6. Update `.http` files and `libs/payments` docs that mention the old signatures.

Files: `libs/payments/application/ports.go`, `authorize_payment.go`, `capture_payment.go`, `authorize_capture_test.go`, `create_payment_intent_test.go`, `libs/payments/infra/pgstore/pgstore.go`, `libs/payments/infra/pgstore/migrations/0003_idempotency_per_merchant.sql`, `libs/payments/infra/idempotency/middleware.go`, `middleware_test.go`, `libs/payments/infra/httpapi/handlers.go` (+ new HTTP tests), `apps/api/cmd/api/main.go`, `apps/api/tests/payment-intents.http`. Conventions pass: translate Portuguese comments and messages in every touched file.

### T3. Close the public identity routes, add `irisctl` (D8)

Labels: `identity`, `api`, `security`, `docs`.

Steps:

1. Remove `POST /v1/merchants`, `POST /v1/api-keys` and `POST /v1/api-keys/{id}/revoke` from the identity HTTP router and the OpenAPI contract (`make gen-contracts`, commit the generated files with the spec). The bearer-authentication middleware stays.
2. Add `apps/api/cmd/irisctl` with `create-merchant`, `issue-api-key`, `revoke-api-key`. It reuses `CreateMerchant`, `IssueAPIKey` and `RevokeAPIKey` and the same Postgres and Argon2id adapters as `apps/api`; wiring stays in the command (composition root). It prints the new key secret once to stdout and nothing else secret; diagnostics go to stderr; non-zero exit on error.
3. Make targets: `make irisctl ARGS="..."` (or one target per command) in `apps/api/Makefile`, reachable from the root.
4. Update `AGENTS.md` (replace the D8 bullet, mention `irisctl`), README quickstart, `apps/api/README.md`, `apps/README.md`, `libs/README.md`, `apps/api/tests/identity.http`, and the dashboard docs that used the routes. Do not touch `docs/design/as-is.md`.
5. Check that the web dashboard and any script (`tools/`) do not call the removed routes.

Files: `libs/identity/infra/httpapi/handlers.go`, `handlers_test.go`, `libs/contracts/http/*.openapi.yaml` and `libs/contracts/generated/**` (generated), `apps/api/cmd/irisctl/**` (new), `apps/api/cmd/api/main.go`, `apps/api/Makefile`, `Makefile`, `AGENTS.md`, `README.md`, `apps/api/README.md`, `apps/README.md`, `libs/README.md`, `apps/api/tests/identity.http`.

### T4. Remove dead dependencies and scaffolds, migrate to `proxy.ts`

Labels: `web`, `core`, `docs`. Spec items 7, 8, 9 and 12.

Steps:

1. Confirm with CodeGraph and grep that each package is unused, then remove: `apps/web` (`sonner`, `@radix-ui/react-dialog`, `-dropdown-menu`, `-tabs`, `-tooltip`) and `apps/anti-fraud` (`@nestjs/microservices`, `ts-node`, `tsconfig-paths`). Keep any package whose removal breaks `nest build`, `nest start --watch` or Jest, and explain in the PR. Never remove the implicit-use packages listed in the spec.
2. Delete `libs/identity/src/` (only `.gitkeep` files) and any other empty scaffold folder; update `libs/README.md`.
3. Rename `apps/web/src/middleware.ts` to `proxy.ts` per the Next 16 migration guide, keeping the next-intl behavior (matcher, locale prefix, redirects). `next build` prints no deprecation warning.
4. Re-run `yarn npm audit`. Take a compatible fix if one exists; otherwise record `glob`, `minimatch` and `deepmerge` with reasons in `docs/security.md`, supply chain section, without changing any control's status.
5. Separate commits: `chore(web): remove unused dependencies`, `chore(anti-fraud): remove unused dependencies`, `chore(libs): remove empty identity scaffold`, `refactor(web): rename middleware to proxy`, `docs(security): record the dev advisory baseline`.

Files: `apps/web/package.json`, `apps/anti-fraud/package.json`, `yarn.lock`, `libs/identity/src/**` (deleted), `libs/README.md`, `apps/web/src/middleware.ts` → `apps/web/src/proxy.ts`, its tests, `docs/security.md`.

### T5. API image from the repo root, pinned make tools

Labels: `infra`, `ci`, `docs`. Spec items 10 and 11.

Steps:

1. Build context becomes the repo root: `docker build -f apps/api/Dockerfile .`, with `apps/api/Dockerfile.dockerignore` next to the Dockerfile. Keep the Go 1.27.4 build stage and the distroless Debian 13 runtime, and pin image tags or digests (no `latest`). Update `apps/api/Makefile` `docker-build`.
2. Prove the image starts and answers `/healthz` (document the run command in the PR).
3. `make tools` installs golangci-lint v2.15.0 and a pinned gofumpt version into `.tools/bin` (gitignored). The Go Makefiles and `lefthook.yml` prefer `PATH` and fall back to `.tools/bin`.
4. Document `make tools` and `make install-hooks` in the README quickstart; update `AGENTS.md` "Toolchain".
5. Check that CI uses the same versions, and open an owner card if it does not (the agent cannot edit workflows).

Files: `apps/api/Dockerfile`, `apps/api/Dockerfile.dockerignore` (new), `apps/api/Makefile`, `Makefile`, `libs/*/Makefile`, `apps/api/Makefile`, `lefthook.yml`, `.gitignore`, `tools/scripts/**` if the install logic needs a script, `README.md`, `AGENTS.md`.

### T6. Persisted declines and double-authorization guard (D3, D4)

Labels: `payments`, `contracts`, `messaging`, `security`. Starts after T2 merges.

Steps:

1. **Contracts first.** Add `payments.payment_intent.failed.v1` to `libs/contracts/events/asyncapi.yaml` and `infra/docker/kafka/topics.txt`; run `make gen-contracts`. Make sure the topic-creation script and relay allow-list pick up the new topic (AGENTS.md currently lists only three published topics; update it).
2. Migration `0004_payment_intent_version_failure.sql`: `payment_intents.version INT NOT NULL DEFAULT 0` and `failure_code TEXT NULL`, with a check that `failure_code` is set only when `status = 'failed'`. Existing rows get version 0.
3. Domain: `MarkFailed(code)` on the aggregate records the failure code and returns the event; aggregate carries `Version`. No status assigned from outside.
4. Repository: `UpdateWithOutbox` becomes `UPDATE ... WHERE id AND merchant_id AND version = $n`, sets `version = version + 1`; zero rows affected → `ErrConcurrentUpdate` (new sentinel). The outbox row is inserted only after the update succeeded, in the same transaction.
5. **D3.** On decline, the use case saves the intent `failed` plus the `failed.v1` outbox row in one transaction, then still returns `ErrAcquirerRejected`.
6. **D4.** Map `ErrConcurrentUpdate` to **409** `concurrent_update`. Pass the intent id as the acquirer idempotency key on every call (field on `AuthorizeInput`, header on the HTTP acquirer). The fake acquirer returns the same result (same `AcquirerRef`) for the same key and exposes a call counter for tests.
7. Add a decline trigger to the fake that tests can use without a real processor (for example a configurable declined token or an injected decision function). It must not change the default (always approves; empty `ACQUIRER_URL` still selects the fake).
8. Update `AGENTS.md` Payments and Fraud bullets (published topics, `pending → failed` now persisted, the version rule).

Files: `libs/contracts/events/asyncapi.yaml`, `libs/contracts/generated/**` (generated), `infra/docker/kafka/topics.txt`, `libs/payments/domain/{payment_intent,errors,events}.go` + tests, `libs/payments/application/{ports,authorize_payment,capture_payment}.go` + tests, `libs/payments/infra/pgstore/pgstore.go`, `libs/payments/infra/pgstore/migrations/0004_*.sql`, `libs/payments/infra/acquirer/{fake,http}.go` + tests, `libs/payments/infra/httpapi/handlers.go`, `libs/payments/infra/kafkarelay/**` if topics are allow-listed, `AGENTS.md`.

### T7. Final security audit

Labels: `security`, `docs`. Starts after T1 to T6 merge. Output is a report plus fixes only for gaps inside the phase scope.

Steps:

1. Run the whole acceptance list (section 4) on `main` after the merges: `make test`, `make lint`, `make gen-contracts-check`, `make -C apps/web typecheck test`, `make -C libs/payments test-int` (or the existing integration targets), the image `/healthz` check, `make tools && make lint` on a clean clone.
2. Re-run the pre-mortem (section 3) against the merged code and tick each risk to its test.
3. Grep for leftovers: callers of the removed routes, `GetByID(ctx, id)` with the old signature, `idempotency_records` queries without `merchant_id`, Portuguese strings in touched files, PAN, `card_token` or key material in logs.
4. Update `docs/security.md` (statuses only where a control really is implemented; advisories baseline), set phase 06 to `done` in `docs/specs/README.md`, and write ADRs for decisions the spec left open (D7 outcome, acquirer idempotency key, optimistic version, decline trigger in the fake).
5. Comment the final summary on the last card: what changed, tests added, ADRs, known limits.

Files: `docs/security.md`, `docs/adr/*.md` (new), `docs/specs/README.md`, `docs/plans/` (this file, to tick results).

## 3. Pre-mortem

Each risk lists the test that covers it. Test IDs refer to section 5.

### 3.1 State transitions, including repeats

Existing lifecycle (unchanged): `pending → authorized → captured` and `pending → failed`. `refunded` and `canceled` exist in the database `CHECK` but have no domain methods; they stay unwired.

| Case                                                                   | Expected                                                                                                                                                               | Test |
| ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---- |
| Authorize twice (sequential)                                           | Second call returns the authorized intent, no second acquirer call, no second event                                                                                    | P1   |
| Capture twice                                                          | No-op, no second event                                                                                                                                                 | P2   |
| Capture a `pending` intent                                             | `ErrInvalidStatusTransition` (409 today); unchanged                                                                                                                    | P3   |
| Authorize a `captured` intent | The use case only short-circuits on `authorized`, so a captured intent reaches the acquirer. It must not; pin with a test and fix inside T6 if it fails | P4   |
| **Authorize a `failed` intent** (new risk, created by D3)              | Today `failed` is never persisted, so this path is new. A retry after a decline must not call the acquirer again or flip `failed` to `authorized`. Answer `ErrInvalidStatusTransition` (409) | P5   |
| Decline then replay of the same `Idempotency-Key` on create            | Create is unaffected; the stored response is the original 201                                                                                                          | P6   |
| `MarkFailed` on an already `failed` intent                             | No-op without a second `failed` event                                                                                                                                  | P7   |
| Decline, then capture                                                  | Rejected transition                                                                                                                                                    | P8   |
| Acquirer transport error (not a decline)                               | The intent stays `pending`, nothing is persisted as `failed`; the caller may retry with the same acquirer idempotency key                                              | P9   |

### 3.2 Concurrency and the protecting check

| What runs concurrently                                    | Today                                                                                                                                | Protection after the phase                                                                                                                                                       | Test |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---- |
| Two authorize calls on one intent                         | Both read `pending`, both call the acquirer, both write an `authorized` event (D4)                                                    | `version` check on update; loser gets 409 `concurrent_update` or re-reads the `authorized` result. Acquirer idempotency key (intent id) makes the second acquirer call return the same result | I1   |
| Authorize racing capture                                  | Capture reads `pending` and fails, or reads `authorized` after the commit                                                              | Version check; either outcome is valid, never two events of one kind                                                                                                              | I2   |
| Authorize racing a decline of the same intent             | Same read-modify-write                                                                                                                  | Version check; the fake returns one result for one key, so both callers see the same decision                                                                                     | I3   |
| Two creates with the same `Idempotency-Key`, same merchant | The middleware only saves **after** the handler returns and ignores `alreadyExists`, so both requests run and create two intents    | The primary key `(merchant_id, scope, key)` makes the second `Save` a no-op, but the intent is already created. **This is a known gap**: the fix (reserve the key before executing) is not in the spec | I4 (documents the current behavior), card C3 |
| Two merchants, same key, same body                        | Today the key is global, so the second merchant gets the first merchant's stored response (D2, data leak)                              | Key includes `merchant_id`                                                                                                                                                       | H4   |
| Failed `Save` after a successful create                   | Logged, response already sent; a retry creates a second intent                                                                         | Same gap as above; out of scope                                                                                                                                                  | C3   |
| Migration `0003`/`0004` running while the API serves      | `MIGRATE_ON_STARTUP` runs in `apps/api` start-up, before the server listens                                                            | Statements are in one migration transaction path (verify `MigrateEmbedded` executes the file atomically; multi-statement `Exec` without `BEGIN` is not transactional, so wrap each file in `BEGIN/COMMIT`, or keep one change per statement order that is safe to re-run) | M1   |
| Two API replicas migrating at once                        | `schema_migrations` check is not locked; both may apply the same file                                                                  | Out of scope for local dev (single instance); note in the plan, card C4 for phase 07 (migrator role)                                                                              | n/a  |

### 3.3 Config defaults and flags, read from the code

- `MIGRATE_ON_STARTUP` (`apps/api/internal/config`, **default `true`**): gates `MigrateEmbedded` for identity then payments in `apps/api/cmd/api/main.go`. Order matters: payments `0003` has an FK to `merchants`, so identity runs first. T2 must keep that order. Test M2 runs both from an empty database.
- `ACQUIRER_URL` empty selects the fake acquirer. The fake must stay always-approve by default; the decline hook is test-only (T6 step 7). Test A3 asserts the default.
- `Mount` returns without mounting anything when `CreatePaymentIntent` or `AuthRequired` is nil, and mounts create only when `IdempotencyMW` is set. Optional fields on the handler dependencies: A nil middleware silently disables idempotency, and a nil use case silently drops a route (`if d.AuthorizePayment != nil`). After T2 and T6, a wiring test in `apps/api` asserts that every payments route is registered and that the middleware order is authentication, then idempotency (the merchant must exist in the context before the middleware reads it). Test W1.
- The idempotency header is `Idempotency-Key` and a missing header answers 400 on create. Unchanged.
- List limit defaults to 50, max 200. Unchanged; the list route already filters by merchant.
- `docker-compose` host ports and `container_name` (T1): `${VAR:-default}` defaults must match the values the README and `make dev` expect, and `apps/api` and `apps/anti-fraud` connection strings must use the same variables. Test: D1 (compose config check).
- Kafka topic creation reads `infra/docker/kafka/topics.txt` in `kafka-init`; a missing `failed.v1` makes the relay fail to publish (auto-create may be off). Test K1.
- Go toolchain pins (`go.work` toolchain go1.27.4, Dockerfile build stage) must be equal. Test D2.
- Next `proxy.ts` matcher must equal the old `middleware.ts` matcher. Test W2.

### 3.4 Side effects before and after the commit

- **Acquirer call happens before the database commit.** That is the root of D4 and stays until phase 11 (persisted `processing`). Failure windows:
  - acquirer approves, commit fails (database down, version conflict): money is authorized at the acquirer but the intent is `pending`. Mitigation in this phase: the acquirer idempotency key (intent id), so a retry gets the same authorization instead of a new one. Residual risk is documented, not fixed. Tests I1, A1 (fake returns the same ref for the same key), A2 (HTTP acquirer sends the key header).
  - acquirer declines, commit of `failed` fails: the intent stays `pending` and a retry re-asks the acquirer with the same key and gets the same decline. Test P10.
- **Outbox row and intent update are in one transaction** (`UpdateWithOutbox`). The `failed.v1` row must not be written when the version check fails (zero rows affected must abort before `insertOutbox`; today it does, keep it). Test I5 asserts no outbox row after a lost race.
- **Publishing is after commit**, by `kafkarelay`. A relay crash between publish and `MarkPublished` republishes (at least once); consumers dedupe by event id. The new topic reuses the same path. Test K2.
- **Idempotency record is written after the handler responded** (see 3.2); the response is already on the wire. Unchanged.
- `irisctl issue-api-key` prints the secret **after** the hash is committed. If the process dies between commit and print, the key exists but its secret is lost; the operator revokes and re-issues. If printing fails, do not retry the issue. Test C1.
- Merchant creation and key issue are separate commands, not one transaction. Test C2 covers creating a key for an unknown merchant (clear error, exit code non-zero).

### 3.5 Principal and permission combinations

Phase 06 has one principal type: a merchant API key. Users, roles and restricted keys arrive in phase 08. The matrix that applies today:

| Principal                                                            | Create               | Authorize / capture / get own intent | Authorize / capture / get other merchant's intent | List                    | Removed identity routes |
| -------------------------------------------------------------------- | -------------------- | ------------------------------------ | ------------------------------------------------- | ----------------------- | ----------------------- |
| Anonymous (no bearer)                                                | 401                  | 401                                  | 401                                               | 401                     | 404 or 405              |
| Invalid bearer                                                       | 401                  | 401                                  | 401                                               | 401                     | 404 or 405              |
| Revoked key                                                          | 403                  | 403                                  | 403                                               | 403                     | 404 or 405              |
| Valid key of merchant A                                              | 201 / 200            | 200                                  | n/a                                               | A's intents only        | 404 or 405              |
| Valid key of merchant B on A's intent                                | n/a                  | n/a                                  | **404** `payment_intent_not_found`, A unchanged   | no A rows               | n/a                     |
| Key of a `pending` merchant                                          | Behavior as today, pinned by a test (see assumption A2) | same                                 | same                                              | same                    | n/a                     |
| Restricted key, user per role, platform user                         | not applicable until phase 08; no test | not applicable | not applicable | not applicable | not applicable |

Extra checks:

- Same-key, different-merchant on create: each merchant has its own intent and stored response (H4).
- Non-existent id and other-merchant id must be **indistinguishable**: same status, same problem code, same body shape, and no timing-visible second query (H1 compares full responses).
- A malformed id returns 400 `invalid_id` for both own and foreign callers, so it leaks nothing.
- The 404 applies to **every by-id route that exists today**: authorize, capture, get (list is by merchant). No 403 for another merchant's resource.
- The stored idempotent response for merchant A is never replayed to B (H4).
- `irisctl` is a local operator tool; its only "principal" is access to the database. It must not print `DATABASE_URL`, the hash or the secret except the new key's secret once. Test C3.

### 3.6 What the phase removes

| Removed                                                                                             | Where it is referenced                                                                         | Follow-up in the same phase                                                                 |
| --------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `POST /v1/merchants`, `POST /v1/api-keys`, `POST /v1/api-keys/{id}/revoke`                          | identity router, OpenAPI contract and generated code, `.http` files, README, `apps/*/README.md`, `libs/README.md`, `AGENTS.md` (D8 bullet), dashboard docs, `tools/` | T3 step 4 and 5; grep test G1 over the repo                                                   |
| Global idempotency key `(scope, key)`                                                               | `idempotency_records` table, `Store` port, middleware, fakes                                   | Migration `0003`; existing rows deleted on purpose                                          |
| `GetByID(ctx, id)` (no merchant)                                                                    | port, pgstore, fakes, use cases, handlers, tests                                               | T2; compile errors enumerate every call site                                                |
| `GetByMerchantAndIdempotencyKey` (if no caller, D7)                                                 | port, pgstore stub, fakes                                                                      | T2; evidence in the PR                                                                      |
| AGENTS.md rules that become false: "bootstrap routes are public (D8)", "Published topics: only created/authorized/captured", "authorize/capture idempotent in the domain; do not add a second side effect" (still true, reword for versioning) | `AGENTS.md` Identity and Payments sections | T3 and T6 edit their bullets in the same PR (AGENTS.md rule)                                 |
| Dev dependencies and scaffold: web radix packages and `sonner`; anti-fraud `@nestjs/microservices`, `ts-node`, `tsconfig-paths`; `libs/identity/src/` | `package.json`, `yarn.lock`, Nest and Jest configs, `libs/README.md`                           | T4; install, typecheck, lint, test and build all pass                                       |
| `apps/web/src/middleware.ts`                                                                        | next-intl, tests, docs                                                                         | T4; replaced by `proxy.ts`                                                                  |
| Old container names `iris-*`                                                                        | compose, Makefiles, docs, scripts                                                              | T1; grep test G2                                                                            |
| The "failed is registered but not published" behavior and its comment in `authorize_payment.go`     | use case                                                                                       | T6                                                                                          |
| Docker build context `apps/api`                                                                     | `apps/api/Makefile`, Dockerfile, any CI step                                                   | T5; owner card if a workflow needs a change                                                 |

## 4. Acceptance criteria to task map

| #   | Criterion                                                              | Task | Tests                       |
| --- | ---------------------------------------------------------------------- | ---- | --------------------------- |
| 1   | Merchant B gets 404 on authorize, capture, get of A's intent; A unchanged | T2   | H1, H2, H3, R1              |
| 2   | Two merchants, same key and body, each get their own intent             | T2   | H4, R2, M3                  |
| 3   | Same merchant replay returns the stored response; different body is 409 | T2   | H5, H6                      |
| 4   | Decline leaves `failed` with a failure code and a `failed` outbox row   | T6   | I6, P5, P7, P10             |
| 5   | Two concurrent authorizes: one acquirer authorization, one event        | T6   | I1, I2, I3, I5, A1          |
| 6   | Identity routes 404/405; `irisctl` produces a merchant and a working key | T3   | H7, H8, C1, C2, C3          |
| 7   | Dependencies gone, `libs/identity/src/` gone, no `proxy` warning        | T4   | G3, W2, build checks        |
| 8   | `make -C apps/api docker-build` image answers `/healthz`                | T5   | D3                          |
| 9   | `make tools && make lint` works without global tools                    | T5   | D4                          |
| 10  | `make test`, `make lint`, `make gen-contracts-check` pass               | all, T7 | full suite run in T7     |

## 5. Test list

Test location in brackets. HTTP tests use the existing fakes and `httptest`; integration tests need Postgres and run with `-race`.

### Domain and application (unit)

- **P1** Authorize twice returns the authorized intent, one acquirer call, one event. [`libs/payments/application`]
- **P2** Capture twice is a no-op with one event.
- **P3** Capture of a `pending` intent is `ErrInvalidStatusTransition`.
- **P4** Authorize of a `captured` intent does not call the acquirer.
- **P5** Authorize of a `failed` intent does not call the acquirer and returns a transition error. (T6)
- **P6** Replay of create after a decline elsewhere returns the stored 201. [`libs/payments/infra/idempotency`]
- **P7** `MarkFailed` records the code and returns one `failed` event; twice is a no-op. [`libs/payments/domain`] (T6)
- **P8** Capture after a decline is rejected. (T6)
- **P9** Acquirer transport error leaves the intent `pending` and writes nothing. (T6)
- **P10** Commit failure after a decline leaves `pending`; retry returns the same decline via the same key. (T6)
- **A1** Fake acquirer returns the same `AcquirerRef` for the same idempotency key and different refs for different keys; call counter increments only on first use. [`libs/payments/infra/acquirer`] (T6)
- **A2** HTTP acquirer sends the intent id as the idempotency key header. [`libs/payments/infra/acquirer/http_test.go`] (T6)
- **A3** Default fake approves; empty `ACQUIRER_URL` selects the fake. [`apps/api`] (T6)

### HTTP (`libs/payments/infra/httpapi`, `libs/identity/infra/httpapi`)

- **H1** B authorizing A's intent gets 404 `payment_intent_not_found`, byte-equal in shape to a random unknown id. (T2)
- **H2** Same for capture. (T2)
- **H3** Same for get; list for B excludes A's intent. (T2)
- **H4** Two merchants, same key and body: two intents, two different stored responses, no cross-replay. (T2)
- **H5** Same merchant, same key and body: stored response with `Idempotent-Replay: true`, one intent. (T2)
- **H6** Same merchant, same key, different body: 409 `idempotency_conflict`. (T2)
- **H7** The three removed routes answer 404 or 405. (T3)
- **H8** Bearer missing 401, invalid 401, revoked 403 on every payments route (pins the matrix in 3.5). (T2)
- **H9** Lost version race answers 409 `concurrent_update`. (T6)

### Integration against Postgres (`-race`)

- **I1** N goroutines authorize one intent: fake counter shows one authorization, one `authorized` outbox row, others get 409 or the idempotent result. (T6)
- **I2** Authorize racing capture: never two events of one kind, final state valid. (T6)
- **I3** Authorize racing a decline: both callers see one decision. (T6)
- **I4** Two creates with one key and merchant: documents the current duplicate-intent behavior and fails if it silently changes (see C3 card). (T2)
- **I5** A lost race writes no outbox row. (T6)
- **I6** A decline persists `failed`, `failure_code` and one `failed.v1` outbox row in one transaction. (T6)
- **R1** Repository `GetByID(merchantA, intentOfB)` returns not found; `UpdateWithOutbox` with the wrong merchant or wrong version affects zero rows. (T2, T6)
- **R2** `idempotency_records` accepts the same `(scope, key)` for two merchants and rejects it for one. (T2)

### Migrations

- **M1** `0003` applied on a database holding rows in the old shape: succeeds, old rows deleted, new primary key in place. (T2)
- **M1b** `0004` applied on rows in the old shape: `version` is 0, `failure_code` null, check constraint holds. (T6)
- **M2** Identity then payments migrations from an empty database, run twice (idempotent through `schema_migrations`). (T2, T6)
- **M3** After `0003`, inserting the same key for two merchants works (also covered by R2). (T2)

### CLI (`apps/api/cmd/irisctl`)

- **C1** `create-merchant` then `issue-api-key` yields a key that authenticates against a payments route; the secret appears once on stdout and nowhere on stderr or in logs. (T3)
- **C2** `issue-api-key` for an unknown merchant exits non-zero with a clear error; `revoke-api-key` makes the key answer 403. (T3)
- **C3** Output contains no `DATABASE_URL`, hash or second secret. (T3)

### Repository and tooling checks

- **G1** `grep` finds no reference to the three removed routes outside `docs/design/as-is.md`. (T3, T7)
- **G2** `grep` finds no `iris-` container name left in compose, Makefiles or docs. (T1)
- **G3** `libs/identity/src/` does not exist; the removed packages are absent from both `package.json` files. (T4)
- **D1** `docker compose config` renders with the default `.env.example` values and with overridden ports. (T1)
- **D2** Dockerfile Go version equals `go.work` toolchain. (T5)
- **D3** `docker build -f apps/api/Dockerfile .`, run the image against Postgres, `GET /healthz` is 200. (T5)
- **D4** On a clean environment without global tools, `make tools && make lint`. (T5)
- **K1** `topics.txt` contains `payments.payment_intent.failed.v1` and AsyncAPI documents it. (T6)
- **K2** The relay publishes a `failed.v1` outbox row with `event-id` and `event-type` headers. [`libs/payments/infra/kafkarelay`] (T6)
- **W1** `apps/api` wiring: all five payments routes registered; unauthenticated calls get 401 before the idempotency middleware runs. (T2)
- **W2** `proxy.ts` matcher and locale behavior equal the old `middleware.ts` for `/en`, `/pt`, `/es` and an unprefixed path; `next build` prints no deprecation. (T4)

Gates for each PR (`AGENTS.md` "Verify before you finish"): `make test` in the touched libraries and `apps/api`; `make gen-contracts-check` for T3 and T6; `make -C apps/web typecheck` and `make -C apps/web test` for T4; `make lint` everywhere.

## 6. Owner cards (human-only work)

Created in Backlog assigned to `owner`, mentioned on the card they block:

- **C1.** Review and merge each phase 06 pull request (one per task).
- **C2.** If CI does not use the golangci-lint and gofumpt versions that T5 pins, or needs a workflow change for the new image build context: update `.github/workflows` (agents cannot edit workflows).
- **C3.** Decision needed: concurrent creates with the same `Idempotency-Key` can create two intents because the key is stored after the handler runs (section 3.2). The spec does not cover reserving the key before execution. Proposal: schedule it in phase 11 or a follow-up task.
- **C4.** Two API replicas can migrate at once (no migration lock); phase 07 introduces the migrator role. Confirm that phase 07 covers a lock.

## 7. Assumptions

Cheap to reverse; each is repeated on the relevant Trello card.

- **A1.** T1 (`ig-` prefix and ports via `.env`) is not in the spec's numbered scope; it is included at the owner's request in the issue that opened this plan. If the spec should list it, the owner updates the spec.
- **A2.** Key of a `pending` merchant: the current authentication does not check merchant status beyond key revocation; this phase does not change it. A test pins the current behavior so that phase 08 changes it knowingly.
- **A3.** The 404 problem code for a missing or foreign intent is `payment_intent_not_found` (spec). The handler's current `not_found` code for the get route is replaced by it so the three routes agree; if the contract documents the old code, T2 updates the OpenAPI file and regenerates.
- **A4.** Migration numbers are global: `0003` is reserved for T2, `0004` for T6. If a task needs another migration, it asks the owner for a number instead of taking one.
