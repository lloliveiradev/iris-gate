<p align="center">
  <img src="apps/web/public/logo_horizontal.png" alt="Iris Gate" width="420" />
</p>

# Iris Gate

API-first payments gateway for local study. A Go service owns merchants, API keys, and payment intents. A NestJS worker scores fraud from Kafka. A Next.js app is the merchant dashboard. OpenAPI and AsyncAPI files in `libs/contracts` are the only shared language between them.

> [!IMPORTANT]
> This stack is for localhost. The default database password, the fake acquirer, and the dev session secret are not production settings. The API accepts an opaque `card_token`. It does not accept a PAN, CVV, or expiry.

## Features

- **Identity** — create a merchant, issue an API key (`irk_<prefix>.<secret>`), revoke it. Only the Argon2id hash is stored. `GET /v1/me` requires `Authorization: Bearer`.
- **Payments** — create, authorize, and capture a payment intent. Money is an integer in minor units plus an ISO-4217 currency. `POST /v1/payment-intents` requires `Idempotency-Key`.
- **Fraud** — specification rules (`SUSPICIOUS_ACCOUNT`, `UNUSUAL_AMOUNT`, `FREQUENT_HIGH_VALUE`) return `APPROVE`, `REVIEW`, or `REJECT`.
- **Delivery** — state and the Kafka outbox commit in one transaction. A relay publishes. The fraud worker deduplicates with an inbox.
- **Dashboard** — Next.js App Router, `en` / `pt` / `es`, landing page plus payment-intent screens. The browser session is an httpOnly iron-session cookie.
- **Local broker** — Postgres 18, Kafka in KRaft mode, and Karapace (schema registry) via Docker Compose.

Billing is a bounded-context scaffold. Its OpenAPI file has no paths yet.

## Getting started

### Prerequisites

- Go 1.25
- Node.js 20.18 or newer, with Corepack (`packageManager` is Yarn 4.14.1)
- Docker with Compose

Optional later: `golangci-lint`, `gofumpt`, `lefthook`, `gitleaks`.

### Run locally

```bash
make bootstrap          # yarn install, copy infra/docker/.env, start Compose
make -C apps/api dev    # http://localhost:8080
make -C apps/web dev    # http://localhost:3000
```

`make up` already starts the anti-fraud container on port `3001` and hot-reloads its source. Run `make -C apps/anti-fraud dev` only after stopping that container, with `apps/anti-fraud/.env` copied from `.env.example`.

> [!NOTE]
> Root `make dev` also enters library Makefiles. Those libraries have no `dev` target, so the command fails. Start `apps/api` and `apps/web` as shown above.

The API listens on `:8080` with `postgres://iris:iris@localhost:5432/iris?sslmode=disable` and `localhost:9092`. Leave `ACQUIRER_URL` empty to use the fake acquirer. Migrations run on startup (`MIGRATE_ON_STARTUP=true`).

Open the dashboard at [http://localhost:3000/en](http://localhost:3000/en). Create a merchant and an API key against the API, then sign in with that `irk_` token.

### Calls worth trying

| Method | Path | Auth |
| --- | --- | --- |
| `GET` | `/healthz`, `/readyz` | no |
| `POST` | `/v1/merchants` | no (local bootstrap) |
| `POST` | `/v1/api-keys` | no (local bootstrap) |
| `POST` | `/v1/api-keys/{id}/revoke` | no (local bootstrap) |
| `GET` | `/v1/me` | bearer |
| `GET` | `/v1/payment-intents`, `/v1/payment-intents/{id}` | bearer |
| `POST` | `/v1/payment-intents` | bearer + `Idempotency-Key` |
| `POST` | `/v1/payment-intents/{id}/authorize` | bearer |
| `POST` | `/v1/payment-intents/{id}/capture` | bearer |

HTTP examples live in `apps/api/tests/*.http`. The typed browser client is `@iris-gate/sdk-web`.

## How a payment moves

```text
dashboard / SDK
    │  POST /v1/payment-intents   (pending)
    ▼
apps/api  ── one transaction ──►  payment_intents + outbox
    │
    │  relay
    ▼
Kafka topics
    payments.payment_intent.created.v1
    payments.payment_intent.authorized.v1
    payments.payment_intent.captured.v1

authorize: pending → authorized     (a repeat call is a no-op)
capture:   authorized → captured    (a repeat call is a no-op)
fail:      pending → failed
```

`apps/anti-fraud` consumes `fraud.assessment.requested.v1`, deduplicates with its inbox, runs the rules in `libs/fraud`, and publishes `fraud.assessment.completed.v1` through its own outbox. The AsyncAPI file defines the request topic. No producer in this repo writes it yet.

`refunded` and `canceled` are status names only. There is no use case for them yet.

## Repository layout

```text
apps/api            Go HTTP process. Composition root only (chi, slog).
apps/web            Next.js 16 dashboard and landing page.
apps/anti-fraud     NestJS 11 worker. Prisma, KafkaJS, inbox, outbox.
libs/identity       Merchants and API keys. domain → application → infra.
libs/payments       Payment intents, acquirer port, Postgres, outbox relay.
libs/fraud          TypeScript rules consumed by apps/anti-fraud.
libs/contracts      OpenAPI, AsyncAPI, and generated Go/TS types.
libs/sdk-web        Fetch client for the dashboard.
libs/billing        Scaffold. No behavior yet.
libs/shared         Cross-cutting slot. Keep domain logic out of it.
infra/docker        Postgres, Kafka, Karapace, anti-fraud.
docs                Architecture, security baseline, ADRs.
```

Inside a context the arrow points inward: `infra` implements ports, `application` orchestrates, `domain` stays free of chi, pgx, Nest, Prisma, and Kafka. Apps wire libraries. Libraries do not import apps.

## Development

| Command | What it does |
| --- | --- |
| `make help` | List root targets |
| `make test` | Race-enabled Go tests plus Jest and Vitest where a Makefile exists |
| `make -C libs/identity test-int` | Identity tests that need Postgres (`-tags=integration`) |
| `make -C apps/web e2e` | Playwright |
| `make gen-contracts` | Regenerate Go and TypeScript from the OpenAPI specs |
| `make gen-contracts-check` | Fail if generated files drifted (CI does this) |
| `make psql` / `make kafka-topics` | Local Postgres shell and topic list |
| `make down` | Stop Compose, keep volumes |
| `make nuke` | Stop Compose and delete volumes |

Edit `libs/contracts/http/*.openapi.yaml` or `libs/contracts/events/**`, then run `make gen-contracts`. Do not hand-edit `libs/contracts/generated`. JSON fields are `snake_case`. New HTTP routes stay under `/v1`. A breaking event is a new `.vN` topic, not a silent field rename.

Format Go with `gofumpt` and TypeScript, JSON, YAML, and Markdown with Prettier. `make install-hooks` enables lefthook (format plus Conventional Commits). CI runs contracts, `apps/api`, `libs/sdk-web`, `apps/web`, `libs/fraud`, `apps/anti-fraud`, and gitleaks.

Web copy goes in `apps/web/messages/{en,es,pt}.json`. The default locale is `en`, and every route is prefixed (`/en`, `/pt`, `/es`).

## Documentation

| Doc | Use it for |
| --- | --- |
| [`docs/architecture.md`](docs/architecture.md) | Bounded contexts, hexagonal layout, outbox/inbox |
| [`docs/security.md`](docs/security.md) | Target baseline: PAN, API keys, logs, idempotency |
| [`docs/adr`](docs/adr) | Accepted decisions (DDD, outbox, contracts) |
| [`AGENTS.md`](AGENTS.md) | Rules that are easy to break while editing |

> [!WARNING]
> `docs/README.md` and parts of `docs/architecture.md` (the "current layout" and "toolchain still to create" sections) describe an earlier skeleton: Go 1.24, Next 15, and no workspace files. The modules, Makefiles, and OpenAPI specs in the tree are the current source of truth.

## Troubleshooting

- **Port 3001 is taken.** Compose already runs anti-fraud. Stop it before a host `make -C apps/anti-fraud dev`.
- **`make dev` exits on a library.** Expected. Those Makefiles have `test` and `build`, not `dev`.
- **Contract check fails.** Run `make gen-contracts` and commit the generated files together with the spec.
- **API cannot reach Postgres or Kafka.** `make ps` should show healthy `iris-postgres` and the Kafka container. Defaults assume `localhost:5432` and `localhost:9092`.
