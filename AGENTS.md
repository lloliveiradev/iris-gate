# AGENTS.md

Rules for editing this repo. Product overview, ports, and the quickstart are in `README.md`. Design intent is in `docs/`. When a doc and the code disagree, follow the code and `libs/contracts`, then update the doc if the task is about that behavior.

## How to work

1. Don’t assume. Don’t hide confusion. Surface tradeoffs.
2. Minimum code that solves the problem. Nothing speculative.
3. Touch only what you must. Clean up only your own mess.
4. Define success criteria. Loop until verified.

There is no Python toolchain here. Do not create a `.venv` or a `pyproject.toml` for repo scripts.

## What is already true

- Go 1.25 (`go.work`), Node `>=20.18`, Yarn 4.14.1 workspaces. The task runner is Make. Do not add pnpm, npm, or Turborepo.
- Implemented contexts: `libs/identity`, `libs/payments`, `libs/fraud`. `libs/billing` and `libs/shared` are empty scaffolds. Ledger and notifications are not in the tree. Do not invent them unless the task says so.
- `docs/README.md` and `docs/architecture.md` §3 and §7 still describe the early skeleton (no toolchain, Go 1.24, Next 15, topic names like `payments.intent.created.v1`). Published topic names are the ones in `libs/contracts/events/asyncapi.yaml` (`payments.payment_intent.*.v1`, `fraud.assessment.*.v1`).
- `docs/security.md` is the target baseline. Do not pretend unimplemented controls (own vault, mTLS, row-level security, OIDC) already exist. Do not weaken the ones that do.

## Boundaries

- Business rules live in `libs/<context>/domain` and `application`. `apps/api` only composes chi, config, pools, and `Mount`.
- Dependency arrow: `infra → application → domain`. `domain/` imports nothing from chi, pgx, Kafka, Nest, Prisma, or another context.
- `libs/payments/infra/httpapi` may use the identity auth middleware. Do not push that import into `domain/` or `application/`.
- Generated DTOs are not domain types. Map them in infra.
- Cross-context facts move as versioned events. Do not share tables or import another context’s entities.

## Contracts

- Edit `libs/contracts/http/*.openapi.yaml` and `libs/contracts/events/**`, then `make gen-contracts`. Never hand-edit `libs/contracts/generated/**`.
- JSON fields are `snake_case`. Routes stay under `/v1`. A breaking change is `/v2` or a new `.vN` topic, not an in-place rename.
- Commit the spec and the generated files together. CI runs `make gen-contracts-check`.

## Payments

- `Money` is `AmountCents int64` plus ISO-4217 alpha-3. Amount must be `> 0`. No floats. No mixed currencies.
- Transitions that exist: `pending → authorized → captured`, and `pending → failed`. `MarkAuthorized` and `MarkCaptured` are no-ops when already in the target status. `refunded` and `canceled` have no methods; do not wire them without a domain change and a contract change.
- `card_token` is opaque, length 8–256. Do not add PAN, CVV, expiry, or `card_number` to a handler, schema, fixture, or log.
- `POST /v1/payment-intents` requires `Idempotency-Key`. Authorize and capture are idempotent in the domain; do not add a second side effect on retry.
- Insert the aggregate and the outbox row in the same transaction. Publishing belongs to `libs/payments/infra/kafkarelay`. Use cases do not call Kafka.
- Empty `ACQUIRER_URL` selects the fake acquirer. Do not point that default at a real processor.

## Identity, session, secrets

- Token shape is `irk_<prefix>.<secret>`. Persist only the Argon2id PHC hash. Return the plaintext secret once, at issue time.
- Bearer missing or invalid → 401. Revoked key → 403.
- New merchants start `pending`. `document` is 14 digits.
- `POST /v1/merchants`, `POST /v1/api-keys`, and revoke are public local bootstrap. Payment routes are not. Do not copy that openness onto payments, and do not lock the bootstrap routes down as a drive-by.
- The dashboard posts the key to a server action and stores it in the iron-session cookie. Do not put it in `localStorage`, client state, or logs.
- Never log PAN, CVV, `card_token`, API keys, passwords, raw email, or raw document. Never commit `.env` or a real secret. `iris`/`iris` and the dev session password stay local.

## Fraud

- The payments outbox publishes only `payments.payment_intent.{created,authorized,captured}.v1`. `apps/anti-fraud` consumes `fraud.assessment.requested.v1` and publishes `fraud.assessment.completed.v1`. Nothing in the tree produces the request topic yet. Do not invent that bridge unless the task asks for it.
- Rules are specifications in `libs/fraud`, configured by code and weight. Defaults: `SUSPICIOUS_ACCOUNT` 1.0, `UNUSUAL_AMOUNT` 0.6, `FREQUENT_HIGH_VALUE` 0.5. Thresholds default to review 30 and reject 70, both in `0..100`, with review `<=` reject.
- Uncertainty fails closed: `REVIEW` or `REJECT`, never a fallback `APPROVE`.
- `apps/anti-fraud` is the Nest composition root (Prisma, Kafka, inbox, outbox). It runs the built `@iris-gate/fraud` package. Change a rule in the lib. `predev` / `prebuild` already compile it.
- Consumers dedupe with the inbox. Do not apply one message id twice.

## Toolchain

- Run `yarn` from the repo root. Go modules resolve through `go.work`.
- `make test`, `make lint`, and `make build` fan out to `APPS` entries that have the target. Libraries have no `dev` target, so root `make dev` fails. Start `make -C apps/api dev` and `make -C apps/web dev`. Anti-fraud is already the Compose service on `:3001`.
- Go tests use `-race`. `make -C libs/identity test-int` needs Postgres.
- Format with `gofumpt` and Prettier. Lint with `golangci-lint` and ESLint. Lefthook rejects commits that are not `type(scope): subject`. Do not skip hooks unless the user asks.
- New dashboard copy goes in `apps/web/messages/en.json`, `pt.json`, and `es.json`. Default locale is `en`. Routes always wear a prefix.

## Verify before you finish

- Domain or use-case change: `make test` in that library (and `apps/api` if you touched composition).
- Contract change: `make gen-contracts-check`, then the handler or client that maps the DTO.
- Web change: `make -C apps/web typecheck` and `make -C apps/web test`. If copy or routing changed, check `/en`, `/pt`, and `/es`.
- Fraud rule change: `make -C libs/fraud test` and `make -C apps/anti-fraud test`.
