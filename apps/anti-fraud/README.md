# apps/anti-fraud

NestJS 11 worker that scores one fraud assessment per Kafka message. The rules live in `@iris-gate/fraud`. This app wires Prisma, KafkaJS, the inbox, and the outbox. It does not decide a payment and it does not serve the dashboard.

`make up` already builds `apps/anti-fraud/Dockerfile.dev` and runs the container `iris-anti-fraud` on port `3001`, with `src/` and `prisma/` mounted for reload. A host `make -C apps/anti-fraud dev` uses the same port.

Nothing in this repo publishes `fraud.assessment.requested.v1` yet. The process still boots, listens, and answers `/healthz`.

## What a message does

The consumer subscribes to `fraud.assessment.requested.v1` from the end of the log (`fromBeginning: false`).

1. The idempotency id is the `event-id` header, or the Kafka message key when the header is absent. A message with neither goes to `fraud.assessment.failed.dlq`.
2. The body must be JSON with `event_id`, `occurred_at`, `assessment_id`, `payment_intent_id`, `merchant_id`, `account_ref`, `amount_cents` (positive integer), and `currency` (`A-Z` × 3). A bad body or a failed parse goes to the DLQ. The consumer keeps running.
3. `AssessRiskUseCase` inserts the id into `InboxMessage`. A duplicate id returns the previous decision and does not score again.
4. Rules from `libs/fraud` run against `Account` and `InvoiceAttempt`. The result is stored on `RiskAssessment` and enqueued in `OutboxMessage` in the same database transaction.
5. `OutboxRelayService` polls unpublished rows (default every `1000` ms, batches of `50`) and publishes `fraud.assessment.completed.v1` with headers `event-id`, `event-type`, `published-at`, and `traceparent` when the request had one.

`GET /healthz` returns `{ "status": "ok" }`. `GET /readyz` returns `{ "status": "ready" }`. Neither checks Postgres or Kafka.

## Rules

`FraudModule` loads `FraudRule` once, at startup. An empty table logs a warning and uses `DEFAULT_RULES` from `@iris-gate/fraud`:

| Code                  | Weight | Params                                   |
| --------------------- | ------ | ---------------------------------------- |
| `SUSPICIOUS_ACCOUNT`  | `1.0`  | none                                     |
| `UNUSUAL_AMOUNT`      | `0.6`  | `historyCount` 10, `thresholdPercent` 50 |
| `FREQUENT_HIGH_VALUE` | `0.5`  | `maxRecentCount` 5, `windowHours` 1      |

Thresholds are `DecisionThresholds.defaults()`: review at 30, reject at 70. They are not read from the database. Editing `FraudRule`, a weight, or a rule class applies on the next process start. `predev` rebuilds `@iris-gate/fraud` before `nest start --watch`; a change inside `libs/fraud` needs that restart, because Nest watches only this app.

Change a specification in `libs/fraud`. This app only adapts ports to Prisma and Kafka.

## Database

Postgres is the same instance as `apps/api`, in a different database so the two migration sets do not share a schema. The API uses `iris`. This app uses `antifraud`.

Create it once, then apply the migration in `prisma/migrations/`:

```bash
docker compose -f infra/docker/docker-compose.yml exec postgres \
  psql -U iris -d iris -c "CREATE DATABASE antifraud OWNER iris;"
make -C apps/anti-fraud prisma-deploy
```

`prisma-migrate` is `prisma migrate dev` and is for creating a new migration. Compose does not run migrations.

Models: `Account`, `InvoiceAttempt`, `RiskAssessment`, `FraudRule`, `InboxMessage`, `OutboxMessage`.

## Run on the host

Stop the Compose service first, or port `3001` is already taken. `yarn dev` loads `apps/anti-fraud/.env` when that file exists.

```bash
cp apps/anti-fraud/.env.example apps/anti-fraud/.env
make -C apps/anti-fraud dev
```

The image used by Compose is a dev image (`node:20-alpine`, `yarn dev`). There is no production image yet.

## Environment

`src/config/kafka.config.ts` is the source of these names. The group variable is `KAFKA_CONSUMER_GROUP`, as in `.env.example`; the Compose service leaves it unset, so the default applies.

| Variable                  | Default in code                 | Used for                                              |
| ------------------------- | ------------------------------- | ----------------------------------------------------- |
| `PORT`                    | `3001`                          | HTTP listen                                           |
| `DATABASE_URL`            | required by Prisma              | `antifraud` database                                  |
| `KAFKA_BROKERS`           | `localhost:9092`                | comma-separated                                       |
| `KAFKA_CLIENT_ID`         | `iris-anti-fraud`               | KafkaJS client                                        |
| `KAFKA_CONSUMER_GROUP`    | `iris-anti-fraud`               | consumer group                                        |
| `KAFKA_TOPIC_REQUEST`     | `fraud.assessment.requested.v1` | subscribe                                             |
| `KAFKA_TOPIC_COMPLETED`   | `fraud.assessment.completed.v1` | configured; the use case publishes this name directly |
| `KAFKA_TOPIC_DLQ`         | `fraud.assessment.failed.dlq`   | invalid messages                                      |
| `OUTBOX_POLL_INTERVAL_MS` | `1000`                          | relay delay                                           |
| `OUTBOX_BATCH_SIZE`       | `50`                            | rows per tick                                         |

Inside Compose the broker hostname is `kafka:9092`, not `localhost`.

## Layout

```text
src/main.ts                 Nest bootstrap, shutdown hooks
src/modules/fraud/          wires AssessRiskUseCase to Prisma adapters
src/infra/kafka/            consumer, DLQ, outbox relay, payload parser
src/infra/prisma/           repositories, inbox, outbox, rule config
src/config/kafka.config.ts  env → KafkaConfig
prisma/                     schema and SQL migrations
```

## Commands

From the repo root:

| Target               | Effect                                                                                                        |
| -------------------- | ------------------------------------------------------------------------------------------------------------- |
| `dev`                | rebuild `@iris-gate/fraud`, then `nest start --watch`                                                         |
| `build` / `start`    | compile the lib and this app; start runs `node dist/main.js`                                                  |
| `test`               | Jest. Current specs cover `parseAssessmentRequest` and the health controller. Rule tests live in `libs/fraud` |
| `lint` / `typecheck` | ESLint and `tsc --noEmit`                                                                                     |
| `prisma-generate`    | Prisma client                                                                                                 |
| `prisma-migrate`     | `prisma migrate dev`                                                                                          |
| `prisma-deploy`      | `prisma migrate deploy`                                                                                       |
| `clean`              | `node_modules`, `dist`, coverage, `tsconfig.tsbuildinfo`                                                      |

## Boundaries

- A new rule is a specification in `libs/fraud`, registered in its rule registry, then covered by that library's tests. Do not score inside a Nest provider.
- The inbox id is `event-id` or the message key. Do not dedupe on `assessment_id` alone.
- Persist the assessment and its outbox row together. Do not call `producer.send` from the use case.
- This database stays `antifraud`. Do not point `DATABASE_URL` at the `iris` database used by `apps/api`.
