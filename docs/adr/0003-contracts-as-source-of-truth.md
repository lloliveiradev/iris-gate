# ADR-0003 — OpenAPI and AsyncAPI contracts as the single source of truth

- **Status:** Accepted
- **Date:** 2026-05-09
- **Deciders:** founding team
- **Related:** [`../architecture.md`](../architecture.md), [`../roadmap.md`](../roadmap.md) (Phase 1)

## Context

`legacy-audit.md` records a drift that never got an owner:

- `Account` in Go has no `isSuspicious`. The Nest anti-fraud service does.
- `InvoiceStatus` has three values in Go and two in Nest.
- Kafka payloads are `snake_case` on the Go side and `camelCase` on the Nest side.
- The web app rebuilds each DTO by hand, with nothing to prove it still matches the backend.

Each repository holds its own private definition of the same concept. There is no artifact that can settle an argument. Integration bugs start there.

## Decision

Versioned contract files in `libs/contracts/` are the only source of truth for anything that crosses a boundary: HTTP between web and API, and events between services on Kafka.

- **HTTP:** OpenAPI 3.1, one file per context at `libs/contracts/http/<context>.openapi.yaml`.
- **Events:** AsyncAPI 2.6 plus JSON Schema 2020-12, under `libs/contracts/events/`.
- **Generated types,** never hand-written, committed under `libs/contracts/generated/{go,ts}/`.

### Pipeline

```
libs/contracts/http/*.openapi.yaml ──▶ oapi-codegen ──▶ generated/go/<ctx>/types.gen.go
                                  ╰─▶ openapi-typescript ──▶ generated/ts/<ctx>.d.ts
libs/contracts/events/schemas/**/*.json   (event codegen comes later)
```

One command from the repo root: `make gen-contracts`. CI runs `make gen-contracts-check` and fails when the generated tree drifts from what was committed.

### Conventions

- **Names.** Every JSON field is `snake_case`. Turning that into `camelCase` is the client or the UI's job, never the contract's.
- **HTTP versions.** Routes live under `/v1/...`. A breaking change adds `/v2/...` and both versions are served together for a transition window.
- **Event versions.** The topic name ends in `.vN`, for example `identity.merchant.created.v1`. A breaking change adds `.vN+1` and both versions are published for a transition window.
- **Schema registry, in development.** Karapace, compatibility `BACKWARD` by default. A breaking event is an explicit exception, not a silent overwrite.
- **The contract leads the code.** Edit the OpenAPI file, run codegen, then update handlers and clients. Never the other way around.

### Dependency rule

`apps/* → libs/contracts/generated/* → libs/contracts/{http,events}/*` (the YAML and JSON text).

`libs/<bounded-context>/domain/` does not import `libs/contracts/`. Generated types are delivery DTOs. The domain keeps its own types. The mapping sits in `infra/http` or `infra/kafka` of that context.

## Consequences

### Positive

- A broken field in the OpenAPI spec breaks the Go build (`apps/api`) and the TypeScript build (`apps/web`) in the same CI run, before merge.
- An external client can be onboarded from the spec. The same file feeds a Redocly or Swagger portal.
- Versioning makes a breaking change a conversation instead of a silent failure in someone else's consumer.
- An event can be audited from a versioned schema file, not reconstructed from logs.

### Negative, and how we live with it

- Codegen interrupts "edit and run". `make gen-contracts` is fast (under two seconds per language), and a pre-commit hook runs it.
- Generated files add noise to pull requests. Review `generated/` only when the source (`http/*.yaml`, `events/*.json`) changed in the same change.
- AsyncAPI is unfamiliar at first. Start with one event and add the next when a phase needs it.

## Not decided here

- **External acquirers.** Their contracts stay out of `libs/contracts/`. An adapter under `libs/payments/infra/acquirers/<name>/` keeps its own DTOs.
- **Internal gRPC.** If it arrives, it arrives as `libs/contracts/grpc/*.proto` under the same rule: the file is the source, the generated code is not edited by hand.
- **A mock server and contract tests.** A later ADR, once there are enough specs to justify the tooling.

## References

- OpenAPI 3.1: https://spec.openapis.org/oas/v3.1.0
- AsyncAPI 2.6: https://www.asyncapi.com/docs/reference/specification/v2.6.0
- oapi-codegen: https://github.com/oapi-codegen/oapi-codegen
- openapi-typescript: https://github.com/openapi-ts/openapi-typescript
- Karapace: https://github.com/Aiven-Open/karapace
