# AGENTS.md

Rules for anyone editing this repo, human or agent. This file is the single source of agent instructions: Cursor and Codex read it directly, and `CLAUDE.md` imports it. Do not duplicate these rules elsewhere; change them here.

Product overview, ports and the quickstart are in `README.md`. Design intent is in `docs/` (start at `docs/README.md`). When a doc and the code disagree, follow the code and `libs/contracts`, then fix the doc if the task is about that behavior.

## How to work

1. Don't assume. Don't hide confusion. Surface tradeoffs.
2. Minimum code that solves the problem. Nothing speculative.
3. Touch only what the task needs. Clean up only your own mess, plus the conventions pass below.
4. Define success criteria. Loop until verified.

### Delivery phases

Planned work is spec-driven. Before implementing a phase, read `docs/specs/README.md` (workflow, branch and reporting rules) and the phase spec. Plans go in `docs/plans/<YYYY-MM-DD>-phase-<NN>-<slug>.md`. Specs, plans and design docs live directly under `docs/`, never in folders named after a tool or framework.

### Explore with CodeGraph first

This repo is indexed by CodeGraph (`.codegraph/`, local to each machine and not committed). Use it before grep, find or opening files:

- MCP tool `codegraph_explore` when your client has it; otherwise the CLI: `codegraph explore "<symbols, files or a question>"`.
- One call returns the verbatim source of the relevant symbols, the call paths between them and what depends on them. Use it to find code, understand a flow, and check the blast radius before changing a symbol.
- If results look stale after large changes, run `codegraph sync`. If `.codegraph/` does not exist on your machine, run `codegraph init`.
- Fall back to grep and file reads only for what CodeGraph does not cover (Markdown, YAML, SQL, Makefiles).

## Design rules

### Domain-driven design

- One bounded context per `libs/<context>`: `identity`, `payments`, `fraud` today; `checkout`, `webhooks`, `vault` as the phases add them. Names in code follow the ubiquitous language of `docs/architecture.md` section 3.
- **Aggregates own their invariants.** State changes go through methods on the aggregate (`PaymentIntent.Confirm`, `Review.Resolve`), which validate and return domain events. No anemic structs mutated from use cases, and no status assigned from outside the aggregate.
- **Value objects** for concepts with rules (`Money`, `Email`, `CardRef`, `Score`): immutable, validated at construction, compared by value.
- One repository per aggregate, defined as a port in `application` and implemented in `infra`. Repositories load and save whole aggregates; they hold no business rules.
- Domain errors are typed (sentinel errors in Go, error classes in TypeScript) and mapped to HTTP or messages in `infra`.
- Contexts never share tables or import each other's entities. They talk through ports (in process), internal HTTP with a contract, or versioned events.

### Clean architecture

- Layers: `domain` → `application` → `infra`, with the dependency arrow `infra → application → domain`.
- `domain/` imports nothing from chi, pgx, Kafka, Nest, Prisma, HTTP, SQL or another context. `application/` imports only `domain/` and its own ports.
- `apps/*` are composition roots: config, pools, adapters, wiring, `Mount`. No business rule lives in `apps/`.
- Generated DTOs are not domain types. Map them in `infra`.
- Side effects (database, Kafka, acquirer, vault, risk service, mail, clock, id generation) sit behind ports so use cases are testable with fakes.

### SOLID, as applied here

- **Single responsibility:** one use case per type and file. A handler decodes, calls one use case, encodes. A repository persists.
- **Open/closed:** extend by adding, not by editing switch statements. A new fraud rule is a new specification registered in the rule registry; a new acquirer is a new adapter of the `Acquirer` port.
- **Liskov substitution:** every adapter honors its port's contract, including errors and idempotency. When a port has several adapters (fake and real), they share contract tests.
- **Interface segregation:** small ports shaped by the use case that needs them, not one large repository or client interface.
- **Dependency inversion:** `application` defines the ports; `infra` implements them; `apps` wires them. Constructors receive dependencies; nothing reaches for globals.

### Clean code

- Intention-revealing names in English, from the ubiquitous language. No abbreviations beyond the common ones (`ctx`, `id`, `tx`).
- Small functions with one level of abstraction. Return early instead of nesting.
- No dead code, commented-out code, unused parameters or speculative options.
- No magic numbers or strings: name them as constants or configuration.
- Comments explain why, not what. Exported Go identifiers and public TypeScript APIs have doc comments.
- Errors are wrapped with context (`fmt.Errorf("load intent: %w", err)` in Go) and never swallowed.
- Go: `context.Context` is the first parameter of anything that does I/O; no `panic` outside `main` and tests; table-driven tests.
- TypeScript: `strict` on, no `any`, no non-null assertions to silence the compiler.

### Conventions pass on every file you touch

When a task makes you edit a file, bring **that file** up to these conventions in the same change:

- translate Portuguese comments, docstrings, log messages, error messages and Makefile help text to English;
- fix naming and structure that break the rules above, as long as the fix stays inside that file and does not change behavior.

Do not open files only to translate or tidy them. Keep these edits in the same commit as the change, or in a separate `style:` or `docs:` commit when the diff would otherwise be hard to review.

## What is already true

- Go 1.27 (`go.work`, toolchain go1.27.4), Node 26 LTS as the runtime (`engines` accepts `>=24.11.0`), Yarn 4.18.1 workspaces through Corepack. The task runner is Make. Do not add pnpm, npm or Turborepo. There is no Python toolchain; do not create a `.venv` or `pyproject.toml`.
- Implemented contexts: `libs/identity`, `libs/payments`, `libs/fraud`. `libs/shared` is the empty shared-kernel slot. A new context needs a phase spec.
- Published topic names are the ones in `libs/contracts/events/asyncapi.yaml` (`payments.payment_intent.*.v1`, `fraud.assessment.*.v1`).
- Planned work follows `docs/design/to-be.md` and the phase specs in `docs/specs/`. `docs/architecture.md` and `docs/security.md` mark each planned part with its phase.
- `docs/security.md` lists every control with its status. Do not describe a control as present before its status says `implemented`. Do not weaken the ones that are. When a phase implements a control, update its status there.

The sections below describe current behavior. When a phase changes one of them, that phase updates the section in the same change.

## Contracts

- Edit `libs/contracts/http/*.openapi.yaml` and `libs/contracts/events/**`, then `make gen-contracts`. Never hand-edit `libs/contracts/generated/**`.
- JSON fields are `snake_case`. Routes stay under `/v1`. A breaking change is `/v2` or a new `.vN` topic, not an in-place rename (the one exception is the payments v1 change recorded in phase 11).
- Commit the spec and the generated files together. CI runs `make gen-contracts-check`.

## Payments

- `Money` is `AmountCents int64` plus ISO-4217 alpha-3. Amount must be `> 0`. No floats. No mixed currencies.
- Transitions that exist: `pending → authorized → captured`, and `pending → failed`. `MarkAuthorized` and `MarkCaptured` are no-ops when already in the target status. `refunded` and `canceled` have no methods; do not wire them without a domain change and a contract change.
- `card_token` is opaque, length 8–256. Do not add PAN, CVV, expiry or `card_number` to a handler, schema, fixture or log.
- `POST /v1/payment-intents` requires `Idempotency-Key`. Authorize and capture are idempotent in the domain; do not add a second side effect on retry.
- Insert the aggregate and the outbox row in the same transaction. Publishing belongs to `libs/payments/infra/kafkarelay`. Use cases do not call Kafka.
- Empty `ACQUIRER_URL` selects the fake acquirer. Do not point that default at a real processor.

## Identity, session, secrets

- Token shape is `irk_<prefix>.<secret>`. Persist only the Argon2id PHC hash. Return the plaintext secret once, at issue time.
- Bearer missing or invalid → 401. Revoked key → 403.
- New merchants start `pending`. `document` is 14 digits.
- `POST /v1/merchants`, `POST /v1/api-keys` and revoke are public local bootstrap (defect D8). Payment routes are not. Do not copy that openness onto other routes. Phase 6 replaces these routes with the `irisctl` CLI.
- The dashboard posts the key to a server action and stores it in the iron-session cookie (defect D9, replaced by user accounts in phase 9). Do not put it in `localStorage`, client state or logs.
- Never log PAN, CVV, `card_token`, API keys, passwords, tokens, raw email or raw document. Never commit `.env` or a real secret. `iris`/`iris` and the dev session password stay local.

## Fraud

- The payments outbox publishes only `payments.payment_intent.{created,authorized,captured}.v1`. `apps/anti-fraud` consumes `fraud.assessment.requested.v1` and publishes `fraud.assessment.completed.v1`. Nothing in the tree produces the request topic; phase 10 replaces it with a synchronous endpoint. Do not build that bridge.
- Rules are specifications in `libs/fraud`, configured by code and weight. Defaults: `SUSPICIOUS_ACCOUNT` 1.0, `UNUSUAL_AMOUNT` 0.6, `FREQUENT_HIGH_VALUE` 0.5. Thresholds default to review 30 and reject 70, both in `0..100`, with review `<=` reject.
- Uncertainty fails closed: `REVIEW` or `REJECT`, never a fallback `APPROVE`.
- `apps/anti-fraud` is the Nest composition root (Prisma, Kafka, inbox, outbox). It runs the built `@iris-gate/fraud` package. Change a rule in the lib. Yarn 4 does not run `pre*` scripts, so the `dev`, `build` and `typecheck` scripts build `@iris-gate/fraud` and run `prisma generate` themselves.
- Prisma 7: the client is generated into `apps/anti-fraud/src/infra/prisma/generated/` (gitignored, never edited), `prisma.config.ts` holds the schema path and `DATABASE_URL`, and `PrismaService` connects through the `@prisma/adapter-pg` driver adapter. Schema changes still go through a migration in `prisma/migrations/`.
- Consumers dedupe with the inbox. Do not apply one message id twice.

## Toolchain

- Run `yarn` from the repo root. Go modules resolve through `go.work`.
- `make test`, `make lint` and `make build` run in every package (`PACKAGES` in the root Makefile) that defines the target. `make dev` starts the stack, then `apps/api` and `apps/web` on the host; anti-fraud is already the Compose service on `:3001`.
- Go tests use `-race`. `make -C libs/identity test-int` needs Postgres.
- Format with `gofumpt` and Prettier. Lint with golangci-lint v2 and ESLint. Lefthook rejects commits that are not `type(scope): subject`. Do not skip hooks unless the owner asks.
- Shared tool configs live in `configs/` (see `configs/README.md`): `typescript/` bases that every `tsconfig.json` extends, `eslint/node.mjs`, and `go/golangci.yml`. Their architecture rules (depguard in Go, `no-restricted-imports` in TypeScript) enforce the clean-architecture and context boundaries above. Never silence them with `//nolint` or `eslint-disable`; fix the dependency. A new Go module or context gets its isolation rule in `configs/go/golangci.yml`.
- Infra files go in `infra/`, developer scripts in `tools/` behind a root `make` target, design docs in `docs/design/` (the web design system is in `docs/design/design-system/`).
- New dashboard copy goes in `apps/web/messages/en.json`, `pt.json` and `es.json`. Default locale is `en`. Routes always wear a prefix.
- Code, identifiers, comments, docs and commit messages are in English. Dashboard copy is the only place for Portuguese and Spanish.

## Verify before you finish

- Domain or use-case change: `make test` in that library (and `apps/api` if you touched composition).
- Contract change: `make gen-contracts-check`, then the handler or client that maps the DTO.
- Web change: `make -C apps/web typecheck` and `make -C apps/web test`. If copy or routing changed, check `/en`, `/pt` and `/es`.
- Fraud rule change: `make -C libs/fraud test` and `make -C apps/anti-fraud test`.
- Phase work: every acceptance criterion of the spec, as `docs/specs/README.md` describes.
