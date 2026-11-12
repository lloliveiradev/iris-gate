# apps

Deployable processes. Each directory here is one process you can run. Domain rules stay in `libs/`; these apps only compose them.

There are three apps. Nothing else under this folder is a service.

| App                         | Process                  | Port   | What it owns                                                               |
| --------------------------- | ------------------------ | ------ | -------------------------------------------------------------------------- |
| [`api`](api/)               | Go 1.27, chi, `slog`     | `8080` | HTTP composition, Postgres pool, startup migrations, payments outbox relay |
| [`web`](web/)               | Next.js 16.4, React 19.3 | `3000` | Merchant UI, i18n, iron-session                                            |
| [`anti-fraud`](anti-fraud/) | NestJS 12, Prisma 7      | `3001` | Kafka consumer, Prisma, inbox and outbox                                   |

An app may depend on `libs/`. A lib must not import an app. One app must not import another. If a rule is being written under `apps/`, it belongs in the matching `libs/<context>`.

`make up` starts Postgres, Kafka, Karapace, and the anti-fraud container. The API and the dashboard run on the host. Root `make dev` starts the stack if needed, then both of them; to run one alone:

```bash
make -C apps/api dev
make -C apps/web dev
```

## api

Composition root for Identity and Payments. `cmd/api/main.go` builds the use cases, and `internal/httpapi` only mounts what those libraries register. Handlers, repositories, and the acquirer adapter live in `libs/identity` and `libs/payments`.

With an empty `ACQUIRER_URL`, the process uses the in-memory fake acquirer. The relay publishes `payments.payment_intent.{created,authorized,captured}.v1`. This process does not publish `fraud.assessment.requested.v1`.

|               |                                                         |
| ------------- | ------------------------------------------------------- |
| Run           | `make -C apps/api dev`                                  |
| Test          | `make -C apps/api test` (`go test -race`)               |
| Image         | `apps/api/Dockerfile` (`make -C apps/api docker-build`) |
| HTTP examples | `apps/api/tests/*.http`                                 |

Defaults, overridable by env: `HTTP_ADDR=:8080`, `DATABASE_URL=postgres://iris:iris@localhost:5432/iris?sslmode=disable`, `KAFKA_BROKER=localhost:9092`, `ENV=dev`, `MIGRATE_ON_STARTUP=true`, `RELAY_ENABLED=true`.

Routes this process exposes:

| Method | Path                                              | Auth                         |
| ------ | ------------------------------------------------- | ---------------------------- |
| `GET`  | `/healthz`, `/readyz`                             | no                           |
| `GET`  | `/v1/me`                                          | bearer                       |
| `GET`  | `/v1/payment-intents`, `/v1/payment-intents/{id}` | bearer                       |
| `POST` | `/v1/payment-intents`                             | bearer and `Idempotency-Key` |
| `POST` | `/v1/payment-intents/{id}/authorize`, `/capture`  | bearer                       |

`/readyz` does not yet check Postgres or Kafka.

## web

Next.js App Router dashboard. It talks to `apps/api` through `@iris-gate/sdk-web`. It does not contain payment or identity rules. Copy lives in `messages/{en,pt,es}.json`. The default locale is `en`, and every route is prefixed.

| Route                                      | Screen                         |
| ------------------------------------------ | ------------------------------ |
| `/[locale]`                                | Landing                        |
| `/[locale]/login`                          | Sign-in with an `irk_` API key |
| `/[locale]/dashboard`                      | Merchant home                  |
| `/[locale]/dashboard/payment-intents`      | List                           |
| `/[locale]/dashboard/payment-intents/new`  | Create                         |
| `/[locale]/dashboard/payment-intents/[id]` | Detail, authorize, capture     |

The login server action stores the key in an httpOnly iron-session cookie. The browser bundle does not keep it.

|            |                                                                                                                                                               |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Run        | `make -C apps/web dev` (webpack, port 3000)                                                                                                                   |
| Faster dev | `make -C apps/web dev-turbopack`                                                                                                                              |
| Test       | `make -C apps/web test` (Vitest), `make -C apps/web e2e` (Playwright)                                                                                         |
| Env        | `IRIS_API_BASE_URL` defaults to `http://localhost:8080`. `IRIS_SESSION_PASSWORD` has a dev-only default. `IRIS_SITE_URL` defaults to `http://localhost:3000`. |

UI notes for this app are under `apps/web/docs/` (design system, i18n, tables). There is no Dockerfile here.

## anti-fraud

NestJS worker. `libs/fraud` holds the specification rules; this app wires Prisma, KafkaJS, the inbox, and the outbox. Its `dev`, `build` and `typecheck` scripts compile `@iris-gate/fraud` and run `prisma generate` first.

It consumes `fraud.assessment.requested.v1`, deduplicates by message id, and publishes `fraud.assessment.completed.v1`. Failures go to `fraud.assessment.failed.dlq`. No code in this repo produces the request topic yet.

`GET /healthz` and `GET /readyz` respond on port `3001`. Readiness does not yet check Postgres or Kafka.

Compose already runs this app (`iris-anti-fraud`) and mounts `src/` for reload. A second `make -C apps/anti-fraud dev` collides on port 3001. For a host process, stop the container and copy `apps/anti-fraud/.env.example` to `.env`. That file expects a separate `antifraud` database on the same Postgres:

```bash
docker compose -f infra/docker/docker-compose.yml exec postgres \
  psql -U iris -d iris -c "CREATE DATABASE antifraud OWNER iris;"
make -C apps/anti-fraud dev
```

|                       |                                                                                           |
| --------------------- | ----------------------------------------------------------------------------------------- |
| Test                  | `make -C apps/anti-fraud test`                                                            |
| Migrations            | `make -C apps/anti-fraud prisma-migrate` locally, `prisma-deploy` for a deployed database |
| Image used by Compose | `apps/anti-fraud/Dockerfile.dev`                                                          |

Prisma models and SQL migrations live in `apps/anti-fraud/prisma/`. The worker's own database is `antifraud`, not the `iris` database used by `apps/api`.
