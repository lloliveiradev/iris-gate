# Design system (apps/web)

Tokens, components, patterns and accessibility rules for the dashboard, landing page and, later, the platform console and hosted checkout. Moved from `apps/web/docs/design-system/` on 2026-11-10 so every design document lives under `docs/design/`.

- Code paths in these documents (`app/...`, `components/...`) are relative to `apps/web/src/`.
- The runtime source of the tokens is `apps/web/src/app/globals.css`; `TOKENS.md` is its specification.
- The current palette (petrol, sage, mint, copper, cream) is the one the project owner wants to replace. Phase 18 ([spec](../../specs/phase-18-visual-identity.md)) defines the new visual identity in `docs/design/visual-identity.md` and then updates these documents.

| Document | Covers |
| --- | --- |
| [`PRINCIPLES.md`](PRINCIPLES.md) | Design principles, theming, typography, motion, surface rules |
| [`TOKENS.md`](TOKENS.md) | Color, typography, spacing, radius, shadow and motion tokens |
| [`COMPONENTS.md`](COMPONENTS.md) | Component inventory and usage |
| [`PATTERNS.md`](PATTERNS.md) | Page and interaction patterns |
| [`ACCESSIBILITY.md`](ACCESSIBILITY.md) | Accessibility rules |
