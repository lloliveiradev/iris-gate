# apps/api

Go HTTP process for Identity and Payments. chi and `log/slog` live here. Merchants, API keys, payment intents, the acquirer call, and the outbox live in `libs/identity` and `libs/payments`. `cmd/api/main.go` is the composition root: it opens Postgres, wires the use cases, and mounts the routers. `cmd/migrate` applies the migrations; the API process never does.

The module is Go 1.27 (`toolchain go1.27.4`), resolved through the repo `go.work`. This process does not read a `.env` file. Set variables in the shell or accept the defaults.

Postgres and Kafka come from `make up`. Then, from the repo root:

```bash
make migrate         # once, and after each new migration
make -C apps/api dev
```

The process listens on `http://localhost:8080` and logs JSON to stdout. `SIGINT` and `SIGTERM` stop the server with a 10 second shutdown timeout.

> [!IMPORTANT]
> Defaults are for localhost. The database roles are `iris_app`, `iris_worker` and `iris_migrator`, with the placeholder passwords of `infra/docker/.env.example`. An empty `ACQUIRER_URL` selects the in-memory fake acquirer. The API accepts an opaque `card_token`. It does not accept a PAN, CVV, or expiry.

## What main wires

1. One `pgxpool` from `IRIS_APP_DATABASE_URL`, as `iris_app`: DML only, no DDL. Identity and payments share the `iris` database. The fraud worker uses a different database.
2. No migrations. `make migrate` (repo root) runs `cmd/migrate`, which applies the embedded identity, then payments, SQL as `iris_migrator` from `IRIS_MIGRATOR_DATABASE_URL`.
3. Identity use cases: create merchant, issue API key (Argon2id, secret returned once), revoke, authenticate.
4. Payments use cases: create, authorize, capture. Authorize calls the acquirer port.
5. `RequireAPIKey` on payment routes and on `GET /v1/me`. The token is `Authorization: Bearer irk_<prefix>.<secret>`. Missing or invalid is 401. A revoked key is 403.
6. Idempotency middleware only on `POST /v1/payment-intents`.
7. The payments outbox relay, unless `RELAY_ENABLED=false`, on a second pool from `IRIS_WORKER_DATABASE_URL` (`iris_worker`). It polls every 250 ms, batches of 100, and publishes whatever topic is stored on the outbox row: `payments.payment_intent.created.v1`, `authorized.v1`, and `captured.v1`. It does not publish `fraud.assessment.requested.v1`.

chi adds request id, real IP, recover, and a 30 second timeout. `ReadHeaderTimeout` on the server is 5 seconds.

`/healthz` returns `{"status":"ok"}`. `/readyz` returns `{"status":"ready"}`. Readiness does not check Postgres or Kafka.

## Routes

Errors use `application/problem+json`. Contracts are `libs/contracts/http/identity.openapi.yaml` and `payments.openapi.yaml`. Handlers live in the libs, not in this app.

| Method | Path                                              | Auth                         |
| ------ | ------------------------------------------------- | ---------------------------- |
| `GET`  | `/healthz`, `/readyz`                             | no                           |
| `GET`  | `/v1/me`                                          | bearer                       |
| `GET`  | `/v1/payment-intents`, `/v1/payment-intents/{id}` | bearer                       |
| `POST` | `/v1/payment-intents`                             | bearer and `Idempotency-Key` |
| `POST` | `/v1/payment-intents/{id}/authorize`              | bearer                       |
| `POST` | `/v1/payment-intents/{id}/capture`                | bearer                       |

Merchants and API keys are managed with `irisctl`, not over HTTP (`POST /v1/merchants`, `POST /v1/api-keys` and the revoke route answer 404). From the repo root:

```bash
make irisctl ARGS="create-merchant --legal-name <name> --document <14 digits> --contact-email <email>"
make irisctl ARGS="issue-api-key --merchant-id <uuid> [--scopes a,b]"
make irisctl ARGS="revoke-api-key --id <uuid>"
```

Each command prints one JSON object on stdout. `issue-api-key` is the only one that prints a secret (field `secret`, the bearer token); it appears once and is not stored. A repeated document fails with `document already exists`. `irisctl` connects as `iris_app` through `IRIS_APP_DATABASE_URL` (default: local Compose Postgres) and expects `make migrate` to have run at least once.

`POST /v1/payment-intents` without `Idempotency-Key` returns 400. The same key and body replay the stored response and set `Idempotent-Replay`. The same key with a different body returns 409. Authorize and capture are idempotent in the domain: repeating the current status is a no-op. `pending → authorized → captured`; any other transition is 409. When the acquirer rejects, the domain marks the intent `failed` in memory, but the row stays `pending` and no event is written. `GET /v1/payment-intents/{id}` returns 404 for an intent owned by another merchant; authorize and capture do not check ownership yet. These are defects D1 and D3 in [`docs/design/as-is.md`](../../docs/design/as-is.md), fixed in phase 6.

REST Client files in `tests/` exercise this surface. Run `tests/identity.http` before `tests/payment-intents.http`. The document in the identity file is unique. Change it, or reset the database with `make nuke`, before a second bootstrap.

## Environment

| Variable                     | Default                                                                                                   |
| ---------------------------- | --------------------------------------------------------------------------------------------------------- |
| `HTTP_ADDR`                  | `:8080`                                                                                                   |
| `IRIS_APP_DATABASE_URL`      | `postgres://iris_app:iris_app@localhost:15432/iris?sslmode=disable`                                       |
| `IRIS_WORKER_DATABASE_URL`   | `postgres://iris_worker:iris_worker@localhost:15432/iris?sslmode=disable`                                 |
| `IRIS_MIGRATOR_DATABASE_URL` | `postgres://iris_migrator:iris_migrator@localhost:15432/iris?sslmode=disable`; read only by `cmd/migrate` |
| `KAFKA_BROKER`               | `localhost:19092` (comma-separated)                                                                       |
| `ENV`                        | `dev` (log field only)                                                                                    |
| `ACQUIRER_URL`               | empty, which selects the fake acquirer                                                                    |
| `ACQUIRER_TOKEN`             | empty, sent only with the HTTP acquirer                                                                   |
| `RELAY_ENABLED`              | `true`                                                                                                    |

A database URL variable that is set but empty stops the process: pgx would otherwise connect with libpq defaults as another role.

## Layout

```text
cmd/api/main.go          composition root and process lifecycle
cmd/migrate/             applies the embedded migrations as iris_migrator; role integration tests
internal/config/         environment
internal/httpapi/        chi router plus /healthz and /readyz
tests/                   identity.http, payment-intents.http, health.http
Dockerfile               distroless nonroot image, port 8080
```

A behavior change belongs in `libs/identity` or `libs/payments`. This directory changes when a use case is constructed, a middleware is attached, or a process setting is added.

## Commands

From the repo root. `make -C apps/api <target>` matches a Make run inside this directory.

| Target         | Effect                                                                                            |
| -------------- | ------------------------------------------------------------------------------------------------- |
| `dev`          | `go run ./cmd/api`                                                                                |
| `migrate`      | `go run ./cmd/migrate` (as `iris_migrator`)                                                       |
| `test`         | `go test -race -count=1 ./...`                                                                    |
| `test-int`     | adds `-tags=integration`: role tests against the migrated stack; export `infra/docker/.env` first |
| `cover`        | race test plus a function coverage summary                                                        |
| `build`        | static `bin/api` (`CGO_ENABLED=0`)                                                                |
| `vet`          | `go vet`                                                                                          |
| `lint`         | `golangci-lint` (must be installed)                                                               |
| `fmt`          | `gofumpt` (must be installed)                                                                     |
| `tidy`         | `go mod tidy`                                                                                     |
| `clean`        | removes `bin/` and `coverage.out`                                                                 |
| `docker-build` | `docker build -t iris-gate/api:dev .`                                                             |

> [!WARNING]
> `docker-build` uses this directory as the context. `go.mod` replaces `libs/identity`, `libs/payments`, and the generated contracts with paths outside that context, and the Dockerfile `FROM` is `golang:1.24-alpine` while the module is Go 1.25. Local `make dev` and `make test` go through `go.work` and are the paths CI runs.
