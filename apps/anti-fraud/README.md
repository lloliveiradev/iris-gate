# apps/anti-fraud

NestJS 12 service that scores fraud assessments synchronously. The rules live in `@iris-gate/fraud`. This app wires Prisma, KafkaJS, the outbox relay, and the internal HTTP listener. It does not decide a payment and it does not serve the dashboard.

`make up` already builds `apps/anti-fraud/Dockerfile.dev` and runs the container `ig-anti-fraud`. The public listener binds port `3001` inside the container (Compose publishes host port `13001`) and serves `/healthz` and `/readyz`. The internal risk-scoring listener binds `ANTI_FRAUD_INTERNAL_PORT` (`13002`), published on the host at `127.0.0.1:13002:13002` only. `src/` and `prisma/` are mounted for reload. A host `make -C apps/anti-fraud dev` listens on `:13001` (health) and `127.0.0.1:13002` (internal risk API).

The Kafka assessment request topic (`fraud.assessment.requested.v1`) and legacy DLQ (`fraud.assessment.failed.dlq`) were retired in phase 10; synchronous scoring is defined by `libs/contracts/http/fraud-internal.openapi.yaml`.

## How scoring and publishing work

`POST /internal/v1/risk-assessments` runs on the separate internal listener (`ANTI_FRAUD_INTERNAL_PORT`, default `13002`, bound to `127.0.0.1` on the host) and is never mounted on the public port `13001`:

- `ServiceTokenGuard` requires `Authorization: Bearer <FRAUD_SERVICE_TOKEN>` and compares tokens with `crypto.timingSafeEqual` on equal-length buffers. Missing or wrong tokens return `401` (`application/problem+json`). Startup fails when `FRAUD_SERVICE_TOKEN` is unset or empty.
- Scoring is idempotent by `payment_intent_id`: a repeated request returns the stored assessment without rescoring or writing a second `attempted` row. A repeat whose `merchant_id`, `amount_cents`, or `currency` differs from the stored assessment returns `409` (`application/problem+json`).
- On a new `payment_intent_id`, `ScoreRiskUseCase` loads one `FeatureSnapshot` via `PrismaFeatureSnapshotReader`, evaluates the registered rules, and persists the completed `RiskAssessment` (with `features_snapshot`), the `attempted` row in `payment_outcomes`, and the `fraud.assessment.completed.v1` outbox event in one Prisma `$transaction` (`PrismaScoredAssessmentStore`).
- When `FRAUD_TEST_HOOKS=true` (code default `false`, enabled in `.env.example` and local Compose), `test_scenario = "blocked"` yields `REJECT` and `test_scenario = "review"` yields `REVIEW` with reason `TEST_SCENARIO`. When `false`, `test_scenario` is ignored.
- `OutboxRelayService` polls unpublished rows (default every `1000` ms, batches of `50`) and publishes `fraud.assessment.completed.v1` with headers `event-id`, `event-type`, `published-at`, and `traceparent` when the request had one.

`GET /healthz` returns `{ "status": "ok" }`. `GET /readyz` returns `{ "status": "ready" }`. Neither checks Postgres or Kafka.

## Rules

`FraudModule` loads `FraudRule` once, at startup, and the process stops when the registry rejects a row (unknown code, bad params, or `BLOCKED_CARD` missing, disabled or too light). The phase 10 migration deletes rows whose code is not one of the four below. An empty table logs a warning and uses `DEFAULT_RULES` from `@iris-gate/fraud`:

| Code             | Weight | Params                                                 |
| ---------------- | ------ | ------------------------------------------------------ |
| `BLOCKED_CARD`   | `1.0`  | none (mandatory, enabled, effective score >= reject)   |
| `UNUSUAL_AMOUNT` | `0.6`  | `thresholdPercent` 50 (= 1.5x average captured amount) |
| `VELOCITY`       | `0.5`  | `maxAttempts24h` 5 (> 5 attempts in 24 h)              |
| `DECLINE_BURST`  | `0.7`  | `maxDeclines24h` 3 (> 3 declines in 24 h)              |

Thresholds are `DecisionThresholds.defaults()`: review at 30, reject at 70. They are not read from the database. Any `FraudRule` seed must keep `BLOCKED_CARD` enabled with weight >= 0.7 (so its effective score meets or exceeds the reject threshold 70). Editing `FraudRule`, a weight, or a rule class applies on the next process start. `dev` rebuilds `@iris-gate/fraud` before `nest start --watch`; a change inside `libs/fraud` needs that restart, because Nest watches only this app.

Change a specification in `libs/fraud`. This app only adapts ports to Prisma, Kafka, and HTTP.

## Database

Postgres is the same instance as `apps/api`, in a different database so the two migration sets do not share a schema. The API uses `iris`. This app uses `antifraud`.

The Postgres init scripts create the database and its two roles (`infra/docker/postgres/initdb/10-roles.sh`):

- `antifraud_migrator` owns the database and its schema. The Prisma CLI connects as this role (`ANTIFRAUD_MIGRATOR_DATABASE_URL` in `prisma.config.ts`).
- `antifraud_app` is the runtime (`DATABASE_URL`). Default privileges give it `SELECT, INSERT, UPDATE, DELETE` on every table the migrator creates and `USAGE` on its sequences; it cannot run DDL or connect to `iris`.

Apply the migrations in `prisma/migrations/` from the repo root with `make migrate`, which runs `prisma-deploy` and restarts the container. Compose and the app never migrate. A table created by any other role (the superuser in `make psql`, say) gets no grants for `antifraud_app`.

`prisma-migrate` is `prisma migrate dev` and is for creating a new migration. It needs a shadow database, which `antifraud_migrator` cannot create. Create one once and point `ANTIFRAUD_SHADOW_DATABASE_URL` at it (see `.env.example`):

```bash
make psql   # then: CREATE DATABASE antifraud_shadow OWNER antifraud_migrator;
            #       REVOKE ALL ON DATABASE antifraud_shadow FROM PUBLIC;
```

Models (to-be section 8.6): `CardProfile` (`card_profiles`), `PaymentOutcome` (`payment_outcomes`), `RiskAssessment` (with `features_snapshot`), `FraudRule`, `InboxMessage`, `OutboxMessage`. Amounts are `BIGINT` minor units. `src/infra/prisma/bigint.mapping.ts` maps them to numbers and throws outside the safe-integer range instead of rounding (fail closed).

`PrismaFeatureSnapshotReader` loads the `FeatureSnapshot` of a card in one query: `attempted` and `declined` outcomes with `occurred_at` in `[now - 24h, now)`, the floored average of every `captured` amount (`null` when none), and `card_profiles.blocked` (no row means not blocked). Nothing recorded for the payment intent being scored counts.

Prisma 7 specifics:

- `prisma.config.ts` points at the schema and migrations and reads `ANTIFRAUD_MIGRATOR_DATABASE_URL`, defaulting to the local Compose migrator (it loads `.env` when the file exists; variables already set win). The runtime never reads that variable.
- The client is generated into `src/infra/prisma/generated/` (gitignored). `yarn dev`, `yarn build` and `yarn typecheck` regenerate it; run `make prisma-generate` after editing the schema.
- `PrismaService` connects through the `@prisma/adapter-pg` driver adapter. There is no query engine binary any more.
- The process fails fast at startup when `DATABASE_URL` or `FRAUD_SERVICE_TOKEN` is missing.

## Run on the host

Stop the Compose service first, or host ports `13001` and `13002` are already taken. `yarn dev` loads `apps/anti-fraud/.env` when that file exists.

```bash
cp apps/anti-fraud/.env.example apps/anti-fraud/.env
make -C apps/anti-fraud dev
```

The image used by Compose is a dev image (`node:26.13.0-alpine`, `yarn dev`). There is no production image yet.

## Environment

`src/config/database.config.ts`, `src/config/risk.config.ts`, and `src/config/kafka.config.ts` are the sources of these names.

| Variable | Default in code | Used for |
| -------------------------- | ------------------------------- | |
| `PORT` | `13001` | Public HTTP listener (`/healthz`, `/readyz`). Compose sets `3001` |
| `ANTI_FRAUD_INTERNAL_PORT` | `13002` | Internal risk-scoring listener (`POST /internal/v1/risk-assessments`) |
| `ANTI_FRAUD_INTERNAL_HOST` | `127.0.0.1` | Bind address of the internal listener (`0.0.0.0` in Compose container) |
| `FRAUD_SERVICE_TOKEN` | required | Bearer token for `POST /internal/v1/risk-assessments` (boot fails if unset) |
| `FRAUD_TEST_HOOKS` | `false` | Honors `request.test_scenario` (`blocked` / `review`) only when `true` |
| `DATABASE_URL` | required | `antifraud` database as `antifraud_app` (runtime, DML only) |
| `KAFKA_BROKERS` | `localhost:19092` | comma-separated host listener. Compose sets `kafka:9092` |
| `KAFKA_CLIENT_ID` | `iris-anti-fraud` | KafkaJS client |
| `KAFKA_CONSUMER_GROUP` | `iris-anti-fraud` | consumer group |
| `KAFKA_TOPIC_COMPLETED` | `fraud.assessment.completed.v1` | configured; the use case publishes this name directly |
| `OUTBOX_POLL_INTERVAL_MS` | `1000` | relay delay |
| `OUTBOX_BATCH_SIZE` | `50` | rows per tick |

Inside Compose the broker hostname is `kafka:9092`, not `localhost`.

The Prisma CLI reads `ANTIFRAUD_MIGRATOR_DATABASE_URL` (default: local Compose `antifraud_migrator`) and, for `prisma migrate dev` only, `ANTIFRAUD_SHADOW_DATABASE_URL`.

## Layout

```text
src/main.ts                 Nest bootstrap, public and internal listeners, shutdown hooks
src/modules/fraud/          wires the Prisma adapters and the rules loaded at startup
src/modules/risk/           internal risk-assessment controller, service-token guard, DTOs, problem filter
src/infra/kafka/            Kafka client, event publisher, outbox relay
src/infra/prisma/           scored assessment store, feature reader, BigInt mapping, outbox, rule config
src/config/                 env → DatabaseConfig, RiskConfig, KafkaConfig
prisma/                     schema and SQL migrations
```

## Commands

From the repo root:

| Target               | Effect                                                                    |
| -------------------- | ------------------------------------------------------------------------- |
| `dev`                | rebuild `@iris-gate/fraud`, then `nest start --watch`                     |
| `build` / `start`    | compile the lib and this app; start runs `node dist/main.js`              |
| `test`               | Jest. Integration specs skip without `ANTIFRAUD_TEST_DATABASE_URL`        |
| `test-int`           | migrate `ANTIFRAUD_TEST_DATABASE_URL` (disposable), run integration specs |
| `lint` / `typecheck` | ESLint and `tsc --noEmit`                                                 |
| `prisma-generate`    | Prisma client                                                             |
| `prisma-migrate`     | `prisma migrate dev` (needs `ANTIFRAUD_SHADOW_DATABASE_URL`)              |
| `prisma-deploy`      | `prisma migrate deploy` as the migrator, then revoke `_prisma_migrations` |
| `clean`              | `node_modules`, `dist`, coverage, `tsconfig.tsbuildinfo`                  |

## Boundaries

- A new rule is a specification in `libs/fraud`, registered in its rule registry, then covered by that library's tests. Do not score inside a Nest provider.
- The `InboxMessage` table is reserved for phase 13 consumers, which must insert the inbox row inside their work transaction and read the topic name from configuration.
- Persist the assessment, its `features_snapshot`, the `attempted` outcome, and its outbox row together in one transaction. Do not call `producer.send` from the use case.
- This database stays `antifraud`. Do not point `DATABASE_URL` at the `iris` database used by `apps/api`.
