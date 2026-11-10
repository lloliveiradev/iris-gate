# Delivery specs

These specs take Iris Gate from [`../design/as-is.md`](../design/as-is.md) to [`../design/to-be.md`](../design/to-be.md). There is one spec per phase. A spec says **what** must be true when the phase ends and **which rules** the implementation must follow. It does not say how to split the work: the implementing agent writes that plan.

The design was approved by the project owner on 2026-11-10. The overall picture, including the finished phases 0 to 5 and the later phases without a spec yet, is in [`../roadmap.md`](../roadmap.md).

## Phases

| Phase | Spec | Depends on | Status |
| --- | --- | --- | --- |
| 06 | [Security baseline](phase-06-security-baseline.md) | none | ready |
| 07 | [Database roles and tenant isolation](phase-07-tenant-isolation.md) | 06 | ready |
| 08 | [Identity and access API](phase-08-identity-access-api.md) | 07 | ready |
| 09 | [Dashboard and console access](phase-09-dashboard-console-access.md) | 08 | ready |
| 10 | [Synchronous risk scoring](phase-10-sync-risk-scoring.md) | 06 | ready |
| 11 | [Payment lifecycle and review](phase-11-payment-lifecycle.md) | 09, 10 | ready |
| 12 | [Background jobs](phase-12-background-jobs.md) | 11 | ready |
| 13 | [Kafka event backbone](phase-13-kafka-backbone.md) | 11 | ready |
| 14 | [Card vault](phase-14-card-vault.md) | 11 | ready |
| 15 | [Hosted checkout, catalog and payment links](phase-15-checkout-catalog.md) | 12, 14 | ready |
| 16 | [Webhooks and request logs](phase-16-webhooks-request-logs.md) | 13, 15 | ready |
| 17 | [End-to-end tests and CI](phase-17-e2e-ci.md) | 16 | ready |
| 18 | [Visual identity refresh](phase-18-visual-identity.md) | 09 | **blocked: owner design session** |

Phase 10 can run in parallel with 07 to 09. Phases 12, 13 and 14 can run in parallel after 11.

Later, without a spec yet: observability, AWS deployment with Terraform, ledger, embedded card fields, 3-D Secure, partial capture, `platform_support` role and impersonation, MFA.

## How to run a phase

1. **Pick** the lowest-numbered phase whose dependencies are `done`. Set its status to `in progress` in the table above.
2. **Read** the spec, the to-be sections it links, `AGENTS.md`, and the as-is defects it closes. When the spec and the to-be disagree, the spec wins for scope and the to-be wins for design; raise the conflict (step 8).
3. **Plan.** Write `docs/plans/<YYYY-MM-DD>-phase-<NN>-<slug>.md` with the task list, the files each task touches, and the test list. Before coding, add a **pre-mortem** section that answers, for this phase:
   - which state transitions exist, including repeats (do it twice, undo, redo, edit after toggling);
   - what runs concurrently, and what lock or version check protects it;
   - every config default and feature flag involved, read from the code, not assumed;
   - which side effects happen before and after the database commit;
   - every permission and principal combination (secret key, restricted key, user per role, platform user, anonymous, other merchant);
   - what the phase **removes** (routes, columns, rules in `AGENTS.md`), not only what it adds.

   Every answer that names a risk becomes a test in the plan.
4. **Contracts first.** Change `libs/contracts/http/*.openapi.yaml` and `libs/contracts/events/**`, run `make gen-contracts`, then write code against the generated types.
5. **Implement** in small commits, one concern each. Commit messages follow Conventional Commits with the context as scope (`feat(payments): ...`, `fix(identity): ...`). Never skip the lefthook hooks.
6. **Verify.** Run every command in "Verify before you finish" of `AGENTS.md` that applies, plus each acceptance criterion of the spec. A phase is not done while any of them fails.
7. **Document.** Update `AGENTS.md` where the phase changed a documented behavior, the README if a user-visible command or flow changed, and write an ADR (`docs/adr/NNNN-*.md`) for every decision the spec leaves to the implementer. Set the phase status to `done`.
8. **Track on Trello** as `AGENTS.md` ("Track the work on Trello") describes, from the moment the plan exists:
   - create one Backlog card per plan task, then move each card through WIP to Done (or Review, when a human must look) in real time, with a commit-SHA comment on completion;
   - create a Backlog card assigned to the owner for anything only a human can do: a decision the spec does not cover, a conflict between documents, a credential, a design review, the merge of the phase PR;
   - when the phase ends, comment a final summary on its last card (what changed, tests added, ADRs written, known limits) and link the PR.

## Branches and merges

- Work on `phase/<NN>-<slug>`, branched from `main`. Open one pull request per phase.
- CI must be green before the PR is ready for review.
- The project owner merges. While a PR waits, the next phase may start on a branch stacked on the previous one.

## Rules for every phase

These rules apply on top of `AGENTS.md`.

- **English** for code, identifiers, comments, docs, commit messages and PR text. Dashboard copy goes in `en`, `pt` and `es`.
- **Stay in scope.** Do not refactor, rename or delete anything the phase does not need. If something outside scope is broken, open a card instead of fixing it.
- **Do not change a spec or the to-be** on your own. When a spec looks wrong, stop that part, open an owner card, and continue with the rest. You may proceed on an assumption only if it is cheap to reverse, and you must write it in the plan and in the card.
- **Money** is `int64` / `bigint` minor units with an ISO-4217 currency. No floats.
- **Never log or persist** PAN, CVV, passwords, API key secrets, refresh tokens, webhook secrets, raw email or raw document. Tests that use test card numbers must not print them.
- **Tenant scope.** From phase 07 on: every new merchant-scoped table has `merchant_id` and an RLS policy, every repository method takes the merchant id, and every new route is added to the cross-tenant suite.
- **Permissions.** From phase 08 on: every new route declares its permission (or is listed as public) in the route registry, and the registry test fails otherwise.
- **Actors.** From phase 08 on: every mutation records its actor (`to-be.md` section 4.5).
- **Migrations** are forward-only, numbered per context, and run with the migrator role. Migrations that change existing data include a test with rows in the old shape.
- **Tests.** Domain logic gets unit tests; repositories get integration tests against Postgres; routes get HTTP tests, including the 401, 403 and 404 cases. Go tests run with `-race`.
- **Dependencies.** Pin exact versions of new libraries and Docker images (no `latest`). Prefer the standard library and what the repo already uses.
- **Where files go.** Shared tool configs in `configs/`, infrastructure in `infra/`, developer scripts in `tools/scripts/` behind a root `make` target, design docs in `docs/design/`, plans in `docs/plans/`. A new bounded context (checkout, webhooks, vault) adds its isolation rule to `configs/go/golangci.yml` and its module to `PACKAGES` in the root Makefile.
- **Lint stays green.** `make lint` includes the architecture rules (depguard, `no-restricted-imports`). Do not silence them; fix the dependency.
- **Secrets** live in `.env` (gitignored), with a documented placeholder in `.env.example`.

## Definition of done (every phase)

- All acceptance criteria of the spec hold and are covered by automated tests.
- `make test`, `make lint`, `make gen-contracts-check` and the web typecheck pass.
- Docs, `AGENTS.md` and ADRs are updated as step 7 says.
- The phase card has its final summary.
