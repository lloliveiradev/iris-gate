# libs/

Shared code, organized by bounded context (DDD). Each subfolder is one domain
(or `shared/` for cross-cutting). Inside every domain, code is split into:

- `domain/`      — pure business model (entities, VOs, aggregates, events, repo interfaces)
- `application/` — use cases / orchestration (depends on `domain/` only)
- `infra/`       — adapters for DB, HTTP, queues (depends on `domain/` + `application/`)

The dependency arrow points **inward**: `infra → application → domain`.
