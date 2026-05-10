# billing

Bounded context for **billing**.

```
src/
├── domain/             # pure business model — no framework imports
│   ├── entities/
│   ├── value-objects/
│   └── repositories/   # INTERFACES only; concrete impls live in infra/
├── application/        # use cases (depends on domain/ only)
│   ├── use-cases/
│   └── ports/          # outbound interfaces (e.g. EmailSender)
└── infra/              # adapters (depends on domain/ + application/)
    ├── persistence/    # ORM models, concrete repository implementations
    └── http/           # HTTP controllers / handlers
```

## Rules

- `domain/` imports from nothing inside this monorepo except `libs/shared/domain`.
- `application/` imports from `domain/` and `libs/shared/`.
- `infra/` is the only layer allowed to import frameworks (Express, FastAPI, Spring, etc.) and DB clients.
- This lib must not import directly from `libs/<other-domain>`. Use `libs/shared` or domain events.
