# ADR-0001 — Strategic DDD and hexagonal architecture per bounded context

- **Status:** Accepted
- **Date:** 2026-05-09
- **Deciders:** founding team
- **Related:** [`../architecture.md`](../architecture.md)

## Context

The three legacy repositories (`go-gateway`, `anti-fraud`, `gateway-web`) describe the same payments domain and disagree about it. `Account` in Go has no `isSuspicious`. `InvoiceStatus` has three values in Go and two in Nest. Kafka payloads mix `snake_case` and `camelCase`. Every new payment method would have to be reconciled by hand across three models. That cost is what blocks the roadmap, more than any missing feature.

Iris Gate needs an architecture that can do four things at once:

1. Let payments, fraud, identity, and ledger change without dragging each other along.
2. Keep the domain free of the framework. chi, Nest, and Next are replaceable.
3. Show, for a PCI-DSS review, exactly where sensitive data enters and leaves.
4. Scale on two axes: more owners for the code, and sharding by merchant for the traffic.

## Decision

Use strategic DDD to name the bounded contexts, and hexagonal architecture (ports and adapters) inside each one. The contexts are Identity, Billing, Payments, Fraud, Ledger, and Notifications. Each lives in `libs/<context>/` with a fixed layout:

```
libs/<context>/
├── domain/        entities, value objects, events, repository interfaces, errors
├── application/   use cases, inbound and outbound ports, policies
└── infra/         adapters: persistence (sqlc/Prisma), HTTP, Kafka, acquirers, observability
```

The dependency arrow points inward: `infra → application → domain`. `domain/` imports nothing from outside its own package.

This layout is already scaffolded in `libs/identity/` and `libs/billing/`.

## Alternatives considered

### A. Traditional layers (handler → service → repository) — rejected

This is what the legacy `go-gateway` does. It is easy to start. The domain then imports `time`, `uuid`, and `sync.RWMutex`. Business rules leak into the service. Replacing the framework means rewriting the rules with it.

### B. Microservices from day one — deferred past the MVP

Each bounded context would be its own deployable. For one or two developers, the cost of deploys, distributed tracing, and schema coordination is too high. The MVP starts with three apps (`web`, `api-gateway`, `anti-fraud`). `api-gateway` hosts Identity, Billing, Payments, Ledger, and Notifications, but each context already lives in its own library. Extracting one into a service later is a packaging change, not a redesign.

### C. A modular monolith without hexagonal boundaries — rejected

Well-named Go packages, and no ports. Less code. PCI-DSS still needs a clear trail of where sensitive data crosses a boundary. Without explicit ports, that trail is a reading of the call graph.

## Consequences

### Positive

- The domain can be tested without a database or a broker.
- A new acquirer is a new file under `libs/payments/infra/acquirers/`. `domain/` does not change.
- A context can later move into its own service because it is already isolated.
- A new developer reads `domain/` to learn the rules before reading `infra/`.

### Negative

- More files and more imports than a layered service.
- A learning curve for anyone who has not worked with DDD.
- A risk of ceremony in a trivial context. `notifications/` may keep `application/` thin.

### Mitigations

- A scaffold generator at `tools/scripts/new-context.sh` removes the boilerplate.
- Review blocks a pull request that imports `pgx` or `chi` from `domain/`.
- ADR-0002, still to be written, will state exactly what each layer may import.
