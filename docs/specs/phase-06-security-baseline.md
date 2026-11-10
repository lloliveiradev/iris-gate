# Phase 06: Security baseline

**Depends on:** none. **Closes:** D1, D2, D3, D4 (partially), D7, D8 from [`as-is.md`](../design/as-is.md) section 8.1, plus the repository cleanup left by the 2026-11-10 stack upgrade.

## Goal

Fix the security and correctness defects of the current code, and leave the repository clean and fully tooled, before any feature is added on top of it. The payment lifecycle stays as it is (`pending → authorized → captured`); the new lifecycle is phase 11.

## In scope

### A. Security and correctness

1. **D1, tenant check.** Every payments repository read by id takes the merchant id: `GetByID(ctx, merchantID, id)`. Authorize, capture and get load through it. An intent of another merchant answers **404** `payment_intent_not_found`, the same as an id that does not exist.
2. **D2, idempotency per merchant.** `idempotency_records` key becomes `(merchant_id, scope, key)`. The scope is the route (`create_payment_intent`, and any other route that uses the middleware), not a global constant. The middleware reads the merchant from the authenticated principal.
3. **D3, persist declines.** An acquirer decline saves the intent as `failed` with a `failure_code`, and writes `payments.payment_intent.failed.v1` to the outbox in the same transaction. Add the event to the AsyncAPI contract and to `infra/docker/kafka/topics.txt`.
4. **D4, concurrency.** Add `version int` to `payment_intents`. Every update is conditional on `id`, `merchant_id` and `version` and increments it. A lost race answers **409** `concurrent_update`. Pass the intent id as the idempotency key of every acquirer call, and make the fake acquirer return the same result for the same key. The full fix (a persisted `processing` status before the acquirer call) is phase 11.
5. **D7.** Implement `GetByMerchantAndIdempotencyKey` against Postgres, or remove it if nothing calls it. Decide from the code.
6. **D8, close the public identity routes.** Remove `POST /v1/merchants`, `POST /v1/api-keys` and `POST /v1/api-keys/{id}/revoke` from the HTTP router. Add an operator CLI, `apps/api/cmd/irisctl`, with `create-merchant`, `issue-api-key` and `revoke-api-key`, which reuse the existing identity use cases. Add Make targets (`make irisctl ARGS=...` or one target per command) and update the README quickstart, the `.http` files and the dashboard docs that used the routes.

### B. Repository baseline

These items were found during the stack upgrade. They are mandatory so that later phases start from a clean tree; do not defer them.

7. **Remove unused dependencies.** Verified as not imported anywhere on 2026-11-10; confirm with CodeGraph and grep, remove, then prove nothing breaks (install, typecheck, lint, test, build):
   - `apps/web`: `sonner`, `@radix-ui/react-dialog`, `@radix-ui/react-dropdown-menu`, `@radix-ui/react-tabs`, `@radix-ui/react-tooltip` (the components come from the `radix-ui` package);
   - `apps/anti-fraud`: `@nestjs/microservices` (Kafka is used through KafkaJS directly), `ts-node`, `tsconfig-paths`. Keep a package if removing it breaks `nest build`, `nest start --watch` or Jest, and say why in the PR.
   Packages used implicitly by a framework (`react-dom`, `@nestjs/platform-express`, `reflect-metadata`, `rxjs`, `@prisma/client`, `@nestjs/cli`, `ts-jest`, `@types/*`) stay.
8. **Remove dead scaffolds.** `libs/identity/src/` holds only `.gitkeep` files from an early TypeScript layout; the Go packages next to it are the library. Delete it and any other empty scaffold folder you find, and update `libs/README.md`.
9. **Next.js 16 proxy.** Next 16 deprecates `middleware.ts`: rename `apps/web/src/middleware.ts` to `proxy.ts` following the Next 16 migration guide (same next-intl behavior), and make sure `next build` prints no deprecation warning.
10. **API image builds.** `make -C apps/api docker-build` fails today: the build context is `apps/api`, but `go.mod` replaces `libs/*` and the generated contracts with paths outside it. Build from the repo root (Dockerfile path `apps/api/Dockerfile`, context `.`, with a `Dockerfile.dockerignore` next to it), keep the Go 1.27.4 build stage and the distroless Debian 13 runtime, and prove the image starts and answers `/healthz`.
11. **Pinned local tools.** Add `make tools`, which installs the exact tool versions CI uses into a gitignored repo-local folder (for example `.tools/bin`): golangci-lint v2.15.0 and gofumpt (latest stable, pinned). Make the Go Makefiles and lefthook use that folder when the tool is not on `PATH`, and document `make tools` and `make install-hooks` in the README quickstart. Nobody should need a global install to run `make lint`.
12. **Dependency advisories.** `yarn npm audit` reports three dev-only advisories (`glob`, `minimatch`, `deepmerge`, through Jest and ESLint) with no compatible fix on 2026-11-10. Re-run the audit; if a compatible fix exists, take it. Otherwise record the advisories and their reason in `docs/security.md` (supply chain section) so phase 17's blocking audit has a documented baseline.

## Out of scope

- New statuses, routes or tables beyond what is listed above.
- Users, JWT, roles (phase 08). The dashboard keeps logging in with an API key until phase 09.
- Row-level security and database roles (phase 07).
- Upgrading dependencies again (done on 2026-11-10), except what item 12 needs.

## Guidelines

- `AGENTS.md` documents the bootstrap routes as defect D8; this phase is the task that removes them. Update that bullet.
- The 404-for-other-merchants rule applies to every by-id route that exists today. Do not return 403 for a resource of another merchant.
- Keep the change to `idempotency_records` a forward migration. Existing rows can be deleted by the migration (they are local test data); say so in the migration comment.
- The CLI prints a new API key's secret once, to stdout, and nothing else that is secret.
- Part B changes go in their own commits (`chore(web): remove unused dependencies`, `build(api): build the image from the repo root`, ...), separate from the security fixes.

## Acceptance criteria

1. Merchant B calling authorize, capture or get on merchant A's intent receives 404, and A's intent is unchanged.
2. Two merchants sending the same `Idempotency-Key` and the same body each get their own intent.
3. The same merchant repeating a request with the same key and body gets the stored response; with a different body, 409 (existing behavior kept).
4. A declined authorization leaves the intent `failed` with a failure code in the database, plus a `failed` outbox row.
5. Two concurrent authorize calls on one intent result in one acquirer authorization (counted by the fake) and one `authorized` event; the other call gets 409 or the idempotent result.
6. The three identity routes answer 404 (or 405) over HTTP. `irisctl create-merchant` and `issue-api-key` produce a merchant and a working key.
7. No dependency listed in item 7 remains unless the PR explains why; `libs/identity/src/` is gone; `next build` shows no middleware deprecation warning.
8. `make -C apps/api docker-build` produces an image that answers `/healthz`.
9. On a machine without golangci-lint or gofumpt installed globally, `make tools && make lint` works.
10. `make test`, `make lint`, `make gen-contracts-check` pass.

## Required tests

- HTTP tests for 1 to 3 and 6, in `libs/payments/infra/httpapi` and `libs/identity/infra/httpapi`.
- Integration test for 4 and 5 against Postgres, with goroutines for 5 (`-race`).
- Migration test for the new idempotency key.

## Docs to update

`AGENTS.md` (identity bootstrap bullets, payments transitions, tool setup), the README quickstart (`make tools`, `make install-hooks`, `irisctl`), `libs/README.md`, `docs/security.md` (advisories baseline). Do not edit `docs/design/as-is.md`: it is a snapshot. List the defects fixed (D1 to D3, D7, D8, part of D4) in the PR description instead.
