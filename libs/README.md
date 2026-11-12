# libs

The domain code. Apps in `apps/` compose these packages. They do not own the rules.

Two shapes live here. A bounded context (`identity`, `payments`, `fraud`) keeps `domain/`, `application/`, and `infra/`, with the arrow pointing inward: `infra → application → domain`. `domain/` does not import chi, pgx, Nest, Prisma, Kafka, or another context. The other folders are not domains: `contracts` is the wire format, `sdk-web` is the browser client, `shared` is a slot for cross-cutting code that does not exist yet.

| Library | Language | Used by | State |
| --- | --- | --- | --- |
| [`identity`](identity/) | Go 1.27 | `apps/api` | Merchants and API keys |
| [`payments`](payments/) | Go 1.27 | `apps/api` | Payment intents, acquirer port, outbox relay |
| [`fraud`](fraud/) | TypeScript | `apps/anti-fraud` | Specification rules and `AssessRisk` |
| [`contracts`](contracts/) | OpenAPI, AsyncAPI | API, web, codegen | Source of truth for HTTP and events |
| [`sdk-web`](sdk-web/) | TypeScript | `apps/web` | `IrisClient` over the generated types |
| [`shared`](shared/) | none yet | nobody | Empty on purpose |

Go modules resolve through the repo `go.work`: `libs/identity`, `libs/payments`, and `libs/contracts/generated/go`. The TypeScript libraries are Yarn workspaces (`@iris-gate/fraud`, `@iris-gate/contracts`, `@iris-gate/sdk-web`).

> [!IMPORTANT]
> A context does not import another context's entities. The one cross-library edge that exists is at the HTTP boundary: `libs/payments/infra/httpapi` calls the identity API-key middleware. Do not push that import into `payments/domain` or `payments/application`.

## identity

Merchant and API key. `document` is 14 digits. A new merchant starts `pending`. The token format is `irk_<prefix>.<secret>`. Only the Argon2id PHC hash is stored. The plaintext secret leaves the process once, from issue.

Use cases: create merchant, issue key, revoke key, authenticate. Postgres migrations live in `infra/pgstore/migrations/` and run on API startup. HTTP handlers mount `/v1/merchants`, `/v1/api-keys`, revoke, and `/v1/me`. The library is the Go packages in this directory.

## payments

`PaymentIntent` and `Money`. Amount is a positive `int64` in minor units plus an ISO-4217 currency. Status moves `pending → authorized → captured`, or `pending → failed`. `refunded` and `canceled` are names on the type with no methods yet. Authorize and capture are no-ops when the intent is already in the target status.

Use cases: create, authorize, capture. Authorize calls an `Acquirer` port. An empty `ACQUIRER_URL` in `apps/api` selects the fake adapter in `infra/acquirer`. The aggregate and its outbox row commit in one transaction. `infra/kafkarelay` polls every 250 ms and publishes `payments.payment_intent.{created,authorized,captured}.v1`. `infra/idempotency` is the `Idempotency-Key` middleware for create.

## fraud

Pure TypeScript. No Nest, no Prisma, no Kafka. `apps/anti-fraud` implements the ports and calls `AssessRiskUseCase`.

Rules ship as specifications, with default weights:

| Code | Weight |
| --- | --- |
| `SUSPICIOUS_ACCOUNT` | 1.0 |
| `UNUSUAL_AMOUNT` | 0.6 |
| `FREQUENT_HIGH_VALUE` | 0.5 |

`DecisionThresholds.defaults()` is review at 30 and reject at 70. A score at or above reject is `REJECT`, at or above review is `REVIEW`, otherwise `APPROVE`. The use case dedupes through an `InboxStore` and writes the result through an outbox port. The package must be built (`make -C libs/fraud build`) before the Nest app runs. `predev` in `apps/anti-fraud` does that.

## contracts

Edit the spec, then generate. Do not hand-edit `generated/`.

| Spec | What it covers |
| --- | --- |
| `http/identity.openapi.yaml` | merchants and API keys |
| `http/payments.openapi.yaml` | payment intents |
| `events/asyncapi.yaml` | merchant created, payment-intent lifecycle, fraud assessment and its DLQ |

`make gen-contracts` from the repo root runs oapi-codegen and openapi-typescript. `make gen-contracts-check` fails CI when `generated/` drifts. JSON fields are `snake_case`. HTTP routes stay under `/v1`. A breaking event is a new `.vN` topic. Domain types are not these generated DTOs. Map them in `infra/`.

Event JSON Schemas under `events/schemas/` are not generated into Go or TypeScript yet.

## sdk-web

`IrisClient` for the dashboard. It knows `createMerchant`, `me`, `issueApiKey`, `revokeApiKey`, and the payment-intent calls. `createPaymentIntent` requires an `Idempotency-Key`. The web app constructs it in server actions. The browser bundle does not hold the API key.

## shared

`shared/` is the shared kernel: only what every context needs and no context owns. The phases add the `Actor` value object (phase 8) and the tenant transaction helper (phase 7). A domain concept does not land here to avoid a dependency.

## Commands

From the repo root. `make test` already includes `libs/identity`, `libs/payments`, and `libs/fraud`.

| Target | Effect |
| --- | --- |
| `make -C libs/identity test` | `go test -race` |
| `make -C libs/identity test-int` | the same with `-tags=integration` (needs Postgres) |
| `make -C libs/payments test` | `go test -race` |
| `make -C libs/fraud test` | Jest for the rules and `AssessRisk` |
| `make -C libs/fraud build` | `tsc` to `dist/`, which the Nest app imports |
| `make gen-contracts` | regenerate Go and TypeScript from the OpenAPI specs |
| `make gen-contracts-check` | regenerate and fail on drift |

`libs/sdk-web` typechecks through the web workspace. It has no Makefile, so root `make test` skips it.
