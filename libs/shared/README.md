# shared

Cross-cutting only. No business logic, no domain entities.

Acceptable here:
- Base `Error`/`Result`/`Either` types
- Logger, config loader, generic HTTP client
- Generic value-object base classes (e.g. `UniqueId`)
- Pure utility functions (`pipe`, `clamp`, date helpers)

If you're tempted to add something domain-specific here — stop, and put it in
the matching `libs/<domain>/` instead. `shared/` should *shrink* over time, not
grow.
