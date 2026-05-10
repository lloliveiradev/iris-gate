# iris-gate

A domain-first monorepo. The layout is language-agnostic — pick your stack(s)
and add the matching workspace manifest yourself (or rerun `/init-monorepo <stack>`).

## Layout

```
.
├── apps/        # entrypoints (deployable units: api, web, worker, cli)
├── libs/        # shared code, organized by bounded context
│   ├── billing/
│   ├── identity/
│   ├── shared/
├── configs/     # shared tool configs (lint, ci, formatter)
└── tools/       # repo-level scripts and codegen
```

## Dependency rules (DDD / hexagonal)

Inside each `libs/<domain>/src`:

- `domain/` — entities, value objects, aggregates, domain events, repository **interfaces**. No framework imports.
- `application/` — use cases, DTOs, ports. Depends on `domain/` only.
- `infra/` — persistence, HTTP, messaging. The only layer that touches frameworks/DBs. Depends on `domain/` + `application/`.

Cross-domain rules:

- **Apps** depend on **libs**. **Libs never depend on apps.**
- A domain lib (e.g. `libs/billing`) does **not** import from another domain lib (e.g. `libs/identity`). If they need to share, extract to `libs/shared` or use a domain event.
- `libs/shared` is for cross-cutting only (logger, config, base errors, generic types). It must not contain domain logic.

## Next steps

1. Pick a stack and rerun the skill: `/init-monorepo ts-nest`, `/init-monorepo python`, etc.
2. Or add manifests by hand — the folder skeleton is ready.
3. Read each `libs/<domain>/README.md` for the layered convention.
