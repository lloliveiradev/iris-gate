# Phase 08 plan: Identity and access API

Spec: [`../specs/phase-08-identity-access-api.md`](../specs/phase-08-identity-access-api.md). Workflow: [`../specs/README.md`](../specs/README.md), "How to run a phase". Design: [`../design/to-be.md`](../design/to-be.md) decisions 12 to 19, sections 4, 8.1, 11.2, 12.1 and 12.2. Related ADRs: [`../adr/0005-refresh-token-race-handling.md`](../adr/0005-refresh-token-race-handling.md), [`../adr/0006-signing-key-custody-and-rotation.md`](../adr/0006-signing-key-custody-and-rotation.md).

This plan is code-free: it only schedules the work, surfaces gaps, and defines tests. Nothing here changes the specs or `to-be.md`.

> [!IMPORTANT]
> **OWNER DECISION PENDING — RLS on identity tables (Section 2).** Tasks **08.2**, **08.3**, and **08.11** are written for both outcomes (Option B vs Option A/C) and are blocked on the owner's choice before implementation begins.

## 1. Delivery model

- **One pull request per task**, branched `phase/08-<task-slug>` from `main`, with its own Trello card (`Phase 08 · <task>`) and pull request.
- **Wave 3 in-flight work:** Task **08.4 (Mailer port, SMTP adapter, Mailpit)** is already briefed and running in wave 3. It is listed in Section 4 for completeness and dependency tracking so it is **not** planned or briefed a second time.
- **Upstream wave 3 dependencies:**
  - **07.2a** (`libs/shared` Go module, `go.work`, `golangci.yml`): required by **08.1** (`Actor` value object) and **08.3** (route registry types / tenant transaction helper).
  - **07.3a** (`apps/api/internal/httpapi/route_registry_test.go`): required by **08.3** (single route registry extension).
  - **07.2b** (`0005_rls_payments.sql`, `pgtenant.WithMerchant`): required by **08.2** / **08.3** / **08.11** if Option A or C is chosen, and for global migration order (`0005` precedes `0006`).
- **Migration numbering (`0006`–`0009` reserved globally across `iris`):**
  - Existing: `0001_init.sql` (`libs/identity`), `0002_payments.sql`, `0003_idempotency_per_merchant.sql`, `0004_payment_intent_version_and_failure.sql` (`libs/payments`), `0005_rls_payments.sql` (reserved for `07.2b`).
  - Phase 08 (`libs/identity/infra/pgstore/migrations/`, applied before `libs/payments` in `apps/api/cmd/migrate/main.go:55-60`):
    - **`0006_identity_users_and_tokens.sql`** — alters `merchants` (`onboarding`, `approved_at`), creates `users`, `user_tokens`, `refresh_tokens`.
    - **`0007_identity_memberships_invitations_staff.sql`** — creates `merchant_memberships`, `invitations`, `platform_staff`.
    - **`0008_identity_api_keys_kinds.sql`** — alters `api_keys` (`kind`, `name`, `expires_at`, nullable `hash` for `publishable`), backfills existing rows to `kind = 'secret'`, `name = 'legacy'`.
    - **`0009_identity_audit_events.sql`** — creates `audit_events` and executes `REVOKE UPDATE, DELETE ON audit_events FROM iris_app, iris_worker;` in the same file.
  - All four migration files are assigned to **08.2** so schema changes land once without migration-number collisions among parallel feature tasks (`08.5`–`08.12`). Every multi-statement migration file must wrap its statements in `BEGIN; ... COMMIT;` because `MigrateEmbedded` (`libs/identity/infra/pgstore/pgstore.go:247`) executes each file with a single `db.Exec`.

### REVOKE rule for append-only tables

`infra/docker/postgres/initdb/10-roles.sh:80-85` runs `ALTER DEFAULT PRIVILEGES FOR ROLE iris_migrator IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO iris_app, iris_worker;` (and `GRANT SELECT ON TABLES TO iris_platform;`), so every new table created by `iris_migrator` is writable (`UPDATE`, `DELETE`) by `iris_app` and `iris_worker` by default.

- `audit_events` is append-only: **`0009_identity_audit_events.sql`** (the migration that creates it) **must** include in the same file:

  ```sql
  REVOKE UPDATE, DELETE ON audit_events FROM iris_app, iris_worker;
  ```

- **Required test (spec 08 acceptance criterion 9):** one case per role (`iris_app`, `iris_worker`, `iris_platform`) in `apps/api/cmd/migrate/roles_integration_test.go`:
  - `iris_app` and `iris_worker`: `INSERT` and `SELECT` succeed; `UPDATE` and `DELETE` fail with SQLSTATE `42501` (`insufficient_privilege`).
  - `iris_platform`: `SELECT` succeeds; `INSERT`, `UPDATE`, and `DELETE` fail with SQLSTATE `42501`.
- **Important role semantics:**
  - `iris_worker` and `iris_platform` have `BYPASSRLS` (`10-roles.sh:46-49`), which does **not** bypass table privileges (`GRANT`/`REVOKE`), so the `REVOKE` still blocks `UPDATE` and `DELETE` for both roles.
  - `iris_platform` already has `SELECT` only (`10-roles.sh:82-83`) and **must not** be widened.
- **Phase 11 inheritance:** The exact same `REVOKE UPDATE, DELETE ON payment_intent_events FROM iris_app, iris_worker;` pattern and `roles_integration_test.go` test cases apply in phase 11 when `payment_intent_events` is created.

### Merge order and shared files

- **Wave A (foundations, after 07.2a / 07.3a and owner RLS decision):** `08.1` (contracts + `Actor`), `08.2` (schema `0006`–`0009`, blocked on RLS), `08.4` (already in flight in wave 3), `08.5` (EdDSA JWT/JWKS/reauth — domain/infra key loader can start immediately; DB `auth_time` wiring uses 08.2).
- **Wave B (core auth & enforcement):** `08.3` (Principal, RBAC, single route registry — depends on `07.2a`, `07.3a`, `08.1`, `08.5` verifier, RLS decision), `08.6` (refresh tokens & login/logout — depends on `08.2`, `08.5`), `08.11` (audit chain & `GET /v1/audit-events` — depends on `08.2`, `08.3`, RLS decision).
- **Wave C (vertical features, parallelizable after Wave A+B):** `08.7` (signup/verify/password-reset — depends on `08.2`, `08.4`, `08.6`), `08.8` (memberships/switch/ownership — depends on `08.2`, `08.3`, `08.5`, `08.11`), `08.9` (invitations — depends on `08.2`, `08.3`, `08.4`, `08.11`), `08.10` (API key kinds/roll/revoke — depends on `08.2`, `08.3`, `08.11`), `08.12` (platform staff/merchants/bootstrap-admin — depends on `08.2`, `08.3`, `08.4`, `08.9`, `08.11`).
- **Wave D (hardening & closeout):** `08.13` (rate limits, `/v1/me` final shape, phase verification & docs pass).
- **Shared files across tasks:** `libs/contracts/http/identity.openapi.yaml` (centralized in `08.1` to avoid merge conflicts), `apps/api/cmd/api/main.go` (`08.3`, `08.5`–`08.13` wire their deps cleanly), `apps/api/internal/config/config.go` (`08.4` in flight, `08.5`, `08.12`, `08.13`), `apps/api/internal/httpapi/route_registry.go` (`08.3` creates, `08.5`–`08.12` register their new routes), `libs/identity/infra/pgstore/pgstore.go` (`08.2` splits or extends cleanly per aggregate file).

---

## 2. RLS on identity tables (OWNER DECISION PENDING)

**Context and document conflict:**

- `docs/specs/README.md:67` states: *"From phase 07 on: every new merchant-scoped table has `merchant_id` and an RLS policy, every repository method takes the merchant id, and every new route is added to the cross-tenant suite."*
- `docs/specs/phase-07-tenant-isolation.md:23` states: *"Identity tables stay without RLS (to-be section 8.1); they are covered by the cross-tenant suite only."*
- `docs/design/to-be.md:547` states: *"Identity tables are **not** under RLS: authentication must look keys and users up before the merchant is known. They are protected by explicit merchant scoping in every query and by the cross-tenant test suite."*
- Phase 08 adds `merchant_memberships`, `invitations`, and `audit_events` (all carry `merchant_id`; `audit_events.merchant_id` is nullable for platform-wide actions) and extends `api_keys` (which already carries `merchant_id` and has no RLS), while login, refresh, API-key authentication, email verification, password reset, and invitation accept run **before a merchant is known**.

### Options

| Dimension | Option A: RLS on all merchant-scoped identity tables | Option B: No RLS on identity tables (spec 07 / `to-be.md` §8.1) | Option C: Hybrid (RLS on post-auth tenant tables; none on pre-auth lookup tables) |
| --- | --- | --- | --- |
| **How it works** | Enable `ENABLE` + `FORCE ROW LEVEL SECURITY` on `api_keys`, `merchant_memberships`, `invitations`, and `audit_events` with `merchant_id = current_setting('app.merchant_id', true)::uuid`. Pre-merchant lookups (`api_keys` by prefix in `AuthenticateAPIKey`, `invitations` by `token_hash` in `AcceptInvitation`, `merchant_memberships` at login/switch-merchant, platform writes with `merchant_id IS NULL`) run through narrow `SECURITY DEFINER` Postgres functions owned by `iris_migrator` (or a dedicated narrow lookup role). | No RLS policies on any identity table. Every merchant-scoped identity repository method requires `merchantID uuid.UUID` (`WHERE merchant_id = $1`), enforced by code review, the single route registry (`08.3`), and the cross-tenant HTTP suite. | RLS (`ENABLE` + `FORCE ROW LEVEL SECURITY`) on **`merchant_memberships`** and **`audit_events`** (with platform `NULL`-merchant `audit_events` inserts handled as described under Relay & platform reads/writes below), while **no RLS** on pre-auth lookup tables (`users`, `user_tokens`, `refresh_tokens`, `platform_staff`, `merchants`, `api_keys`, `invitations`). Alternatively, `invitations` and `api_keys` can also have RLS if pre-auth lookups (`GetByPrefix`, `GetByTokenHash`) use narrow `SECURITY DEFINER` functions. |
| **What it protects** | Protects all four merchant-scoped identity tables (`api_keys`, `merchant_memberships`, `invitations`, `audit_events`) at the Postgres layer against a forgotten `WHERE merchant_id = $1` in Go. | Protects via application-layer scoping (`merchantID` parameter on repo methods) + cross-tenant HTTP suite + route registry meta-test. Does **not** protect against a raw SQL bug inside `libs/identity/infra/pgstore` bypassing `WHERE merchant_id = $1`. | Protects `merchant_memberships` and `audit_events` (the tables read/mutated in merchant-dashboard context) at the Postgres layer. Leaves `api_keys` / `invitations` protected by repo scoping unless `SECURITY DEFINER` is used for `GetByPrefix` / `GetByTokenHash`. |
| **Login, refresh, invite-accept** | **Login / refresh:** `users` and `refresh_tokens` have no `merchant_id` (no RLS), but loading the user's memberships at login (`Authenticate` / `SwitchMerchant`) has no active `app.merchant_id` yet — requires a `SECURITY DEFINER` lookup `fn_list_user_memberships(user_id)`. **API key auth:** `GetByPrefix` runs before merchant is known — requires `SECURITY DEFINER` `fn_lookup_api_key_by_prefix(prefix)` + `fn_touch_api_key_last_used(id)`. **Invite accept:** `token_hash` lookup runs before merchant is known — requires `SECURITY DEFINER` `fn_accept_invitation(token_hash, ...)`. | Direct parameterized SQL on `iris_app` pool (`GetByPrefix`, `GetByEmail`, `ListMembershipsByUser`, `GetByTokenHash`). Zero `SECURITY DEFINER` functions needed. | **Login / refresh:** `users`, `refresh_tokens`, `user_tokens` have no RLS; `merchant_memberships` lookup by `user_id` (at login/refresh/switch-merchant) still crosses merchants and needs a narrow `SECURITY DEFINER` function `fn_list_user_memberships(user_id)`. **API key auth & invite-accept:** direct parameterized SQL on `iris_app` if `api_keys` and `invitations` stay without RLS. |
| **Relay & platform reads/writes** | **Relay:** `libs/identity` has no outbox in phase 08 (`iris_worker` unaffected). **Platform reads:** `iris_platform` has `BYPASSRLS` + `SELECT` (`10-roles.sh:48,82-83`), so `GET /v1/platform/merchants` and platform audit reads work. **Platform writes (`POST /v1/platform/merchants`, `approve`, `suspend`, `reactivate`, `audit_events` with `merchant_id IS NULL`):** `iris_platform` has `SELECT` only and must **not** be widened! Running platform writes on `iris_app` fails RLS on `invitations` (owner invite) and `audit_events` (`merchant_id IS NULL` or target merchant without setting `app.merchant_id`) unless platform handlers either set `app.merchant_id` for the target merchant (still fails for `merchant_id IS NULL` staff/platform audit rows!) or call `SECURITY DEFINER` writers. | **Platform reads:** `iris_platform` (`SELECT`, `BYPASSRLS`) or `iris_app` can read `merchants` / `audit_events`. **Platform writes:** `iris_app` writes `merchants`, `invitations`, and `audit_events` (including `merchant_id IS NULL`) without any RLS impedance mismatch or widening `iris_platform`. | **Platform reads:** `iris_platform` (`SELECT`, `BYPASSRLS`) reads across all tenants. **Platform writes:** `merchants` and `invitations` (if un-RLS'd) work on `iris_app`; however, writing `audit_events` with `merchant_id IS NULL` (staff changes) or target `merchant_id` from a platform admin request still requires either a policy exception / `SECURITY DEFINER` insert function on `audit_events` or setting `app.merchant_id` when `merchant_id` is non-null. |
| **Role & `BYPASSRLS` implications** | `iris_app` has `NOBYPASSRLS`. Every `SECURITY DEFINER` function runs as its owner, and Postgres `FORCE ROW LEVEL SECURITY` applies to the table owner too: a `SECURITY DEFINER` function owned by `iris_migrator` (`NOBYPASSRLS`, `10-roles.sh:42`) **is still subject to RLS** unless the function owner has `BYPASSRLS` or sets `row_security = off` with `BYPASSRLS`. Only a superuser can grant `BYPASSRLS` (`10-roles.sh:39`), so `SECURITY DEFINER` bypasses on `FORCE ROW LEVEL SECURITY` tables would have to be owned by a `BYPASSRLS` role created in `10-roles.sh` (or `postgres` init script). | `iris_app` (`NOBYPASSRLS`) runs all identity queries; `iris_platform` (`BYPASSRLS`, `SELECT` only) remains strictly read-only. No `SECURITY DEFINER` functions, no `10-roles.sh` changes, no `make nuke` required. | Same Postgres `FORCE ROW LEVEL SECURITY` constraint for `merchant_memberships` and `audit_events`: `iris_migrator` has `NOBYPASSRLS` (`10-roles.sh:42`), so a `SECURITY DEFINER` function owned by `iris_migrator` does **not** bypass `FORCE ROW LEVEL SECURITY`. Any `SECURITY DEFINER` bypass would require a `BYPASSRLS` owner role in `10-roles.sh` (requiring `make nuke && make bootstrap && make migrate`) or omitting `FORCE` (so `iris_migrator`-owned `SECURITY DEFINER` bypasses RLS while `iris_app` remains subject to it). |
| **Test cost** | High: must test every RLS policy (`iris_app` with no `app.merchant_id` -> 0 rows; A cannot see B), every `SECURITY DEFINER` function (privilege boundaries, `search_path` pinning), platform `merchant_id IS NULL` audit writes, plus the full cross-tenant HTTP suite. | Low-moderate: cross-tenant HTTP suite + repository integration tests proving `GetByID(merchantA, idOfB)` / `List(merchantA)` exclude B, plus `0` extra `SECURITY DEFINER` harness tests. | Moderate-high: RLS tests on `merchant_memberships` & `audit_events`, `SECURITY DEFINER` or non-`FORCE` tests for `fn_list_user_memberships` and platform `audit_events`, plus cross-tenant HTTP suite for all routes. |
| **Migration impact (`0006`–`0009`)** | `0007` (`merchant_memberships`, `invitations`), `0008` (`api_keys`), `0009` (`audit_events`) all add `ENABLE`/`FORCE ROW LEVEL SECURITY` + policies + `SECURITY DEFINER` functions (and potentially `10-roles.sh` changes if `FORCE` is kept). | `0006`–`0009` stay pure DDL + `REVOKE` on `audit_events`. Zero RLS statements on identity tables; zero `10-roles.sh` changes. | `0007` (`merchant_memberships`) and `0009` (`audit_events`) add RLS policies and carefully scoped helper/insert functions; `0006` and `0008` stay pure DDL. |
| **Failure modes** | 1. **`iris_migrator` `NOBYPASSRLS` + `FORCE RLS` deadlock:** `SECURITY DEFINER` owned by `iris_migrator` still returns 0 rows on `FORCE` tables unless `10-roles.sh` creates/grants a `BYPASSRLS` definer role. 2. Over-broad `SECURITY DEFINER` function becomes an RLS bypass gadget. 3. Platform `audit_events` with `merchant_id IS NULL` fails `WITH CHECK`. | 1. A future developer writes a pgstore query on `api_keys`, `merchant_memberships`, `invitations`, or `audit_events` without `WHERE merchant_id = $1`, and Postgres does not stop it (caught only by code review and the cross-tenant test suite). | 1. Same `iris_migrator` `NOBYPASSRLS` + `FORCE RLS` subtlety on `merchant_memberships` when listing all merchants for a user during login/switch-merchant. 2. Two mental models inside `libs/identity` (some tables use `pgtenant.WithMerchant`, others do not). |

### Recommendation

**Recommend Option B (no RLS on identity tables)**, with two explicit follow-up doc fixes if the owner accepts it:

1. **Matches the approved architecture (`to-be.md` §8.1, line 547) and spec 07 (`phase-07-tenant-isolation.md:23`)**, which explicitly carve out identity tables because authentication (`GetByPrefix`), login (`merchant_memberships` across merchants), and invitation acceptance (`token_hash`) happen before a merchant is selected, and `audit_events` holds platform-wide rows (`merchant_id IS NULL`).
2. **Respects the Postgres role model (`10-roles.sh:42,82-83`) without `make nuke` or `SECURITY DEFINER` escalation:** `iris_migrator` has `NOBYPASSRLS`, so `SECURITY DEFINER` functions owned by `iris_migrator` do not bypass `FORCE ROW LEVEL SECURITY` anyway; `iris_platform` has `SELECT` only and must not be widened. Option B lets `iris_app` handle identity queries cleanly with explicit `WHERE merchant_id = $1` on every merchant-scoped query, verified by repository integration tests and the mandatory cross-tenant HTTP test suite.
3. **If the owner prefers Option A or Option C** to satisfy `docs/specs/README.md:67` literally, the cheapest safe path is **Option C without `FORCE` on `merchant_memberships` (so `iris_migrator`-owned `SECURITY DEFINER` read-only `fn_list_user_memberships(uuid)` works without changing `10-roles.sh`)** and a `NULL`-safe `INSERT` policy / `SECURITY DEFINER` writer for `audit_events`, plus explicit `docs/specs/README.md` or `to-be.md` alignment.

**Status: OWNER DECISION PENDING.** Tasks **08.2**, **08.3**, and **08.11** below specify both outcomes (Option B vs Option A/C) and remain **blocked** until the owner chooses.

---

## 3. Single route registry

Owner decision 4 requires **one** route registry in phase 08, seeded by the list task **07.3a** creates at `apps/api/internal/httpapi/route_registry_test.go` (entries today: `method`, `pattern`, `scope`, `reason`).

### Architectural decision: promote the registry to production code

- **Decision:** **Promote the route registry from test-only to production code** (owned by **08.3**, building on **07.2a** `libs/shared` and **07.3a** `apps/api/internal/httpapi/route_registry_test.go`).
- **Why (not test-only):**
  - If the registry stayed test-only (`route_registry_test.go`), permission declarations in tests and `RequirePermission(...)` middleware calls inside `libs/identity/infra/httpapi` and `libs/payments/infra/httpapi` would live in two places. A developer could declare `payments.capture` in the test table while omitting or mis-wiring the middleware on the chi route, and the meta-test would still pass.
  - Promoting the registry into production code makes the declaration **the single source of truth** for both runtime authorization and test completeness:
    1. `libs/shared/authz` (or `libs/shared/httproute`, created in **08.3** inside the `libs/shared` module from `07.2a`) defines `RouteSpec`:
       - `Method string` (`GET`, `POST`, `PATCH`, `PUT`, `DELETE`)
       - `Pattern string` (chi route pattern, e.g. `/v1/payment-intents/{id}/capture`)
       - `TenantScope TenantScope` (`ScopeTenant` | `ScopePublic` | `ScopePlatform` | `ScopeSelf`, preserving 07.3a's cross-tenant scope classification)
       - `ScopeReason string` (why a route is non-tenant-scoped, preserving 07.3a's `reason` field)
       - `Access AccessRule` (`AccessPublic`, `AccessAuthenticated` [any valid principal, e.g. `/v1/me`, `/v1/auth/logout`, `/v1/auth/switch-merchant`, `/v1/auth/reauthenticate`], `AccessMerchant(permission)`, `AccessPlatform(permission)`)
       - `AllowPendingMerchant bool` (`true` for `/v1/me`, settings, members, invitations, API keys, audit-events per spec 08 item 4; `false` for payment routes -> `403 merchant_not_active`)
       - `AllowSuspendedRead bool` (or `BlockSuspended` per owner resolution of Q3)
       - `RequireStepUp bool` (`true` for creating/rolling API keys and changing member roles, checking `auth_time` within `5m` -> `403 reauthentication_required`)
    2. Each bounded context's `Mount(r, deps)` registers routes through a small helper (or attaches the policy middleware directly from the shared `RouteSpec` slice exported by the context / registered into a `Registry` passed to `Mount`), and `apps/api/internal/httpapi/route_registry.go` collects/exports the complete `[]RouteSpec` for `/healthz`, `/readyz`, `/.well-known/jwks.json`, `identity`, and `payments`.
    3. `apps/api/internal/httpapi/route_registry_test.go` (from `07.3a`) imports the **same** production `RouteRegistry` (no duplicate slice!), runs `chi.Walk(router, ...)` to fail on any mounted `(method, pattern)` missing from the registry (or any stale entry not mounted), and drives the cross-tenant suite for every `ScopeTenant` entry.
  - **Clean-architecture check:** `libs/shared` has no domain business rules (`libs/shared/README.md`) — `RouteSpec` uses string/typed permission constants or opaque permission strings (`Permission` type) so `libs/shared` never imports `libs/identity` or `libs/payments`. `libs/identity/infra/httpapi` and `libs/payments/infra/httpapi` remain testable standalone with `httptest`.

---

## 4. Task list (08.1 to 08.13)

Area labels follow `AGENTS.md`: `identity`, `api`, `core`, `security`, `contracts`, `infra`, `docs`.

### 08.1. Identity & platform OpenAPI contracts + `Actor` value object

- **Status:** Ready (depends on `07.2a` for `libs/shared` Go module).
- **Goal:** Define all phase 08 HTTP contracts first (`docs/specs/README.md` step 4) and add the `Actor` value object in `libs/shared` (`to-be.md` §4.5, spec item 12).
- **Labels:** `contracts`, `core`, `identity`.
- **Dependencies:** `07.2a` (`libs/shared` module in `go.work`).
- **Migration number:** None.
- **Estimated size:** M (3–4 h).
- **Files:**
  - `libs/contracts/http/identity.openapi.yaml` (extend with `/v1/auth/*`, `/v1/me`, `/v1/members*`, `/v1/invitations*`, `/v1/api-keys*`, `/v1/audit-events`, `/v1/platform/merchants*`, `/.well-known/jwks.json`; translate remaining Portuguese descriptions on touched schemas/responses per conventions pass)
  - `libs/contracts/Makefile` (verify `identity` codegen config includes all new schemas)
  - `libs/contracts/generated/go/identity/types.gen.go`, `libs/contracts/generated/ts/identity.d.ts` (generated via `make gen-contracts`)
  - `libs/shared/actor/actor.go`, `libs/shared/actor/actor_test.go` (`ActorType`: `user`, `api_key`, `platform_user`, `system`; `Actor { Type, ID, RequestID }`, validation constructor)
- **Steps:**
  1. Add `Actor` and `ActorType` in `libs/shared/actor/` with invariant checks (non-empty type in the closed enum, non-empty `ID`, `RequestID` preserved; no `nil` or fake system user).
  2. Author all phase 08 endpoints and `snake_case` DTOs in `libs/contracts/http/identity.openapi.yaml` matching `to-be.md` §§4.1–4.5, 12.1–12.2 (`Problem` errors, `auth_time`, `reauthentication_required`, `merchant_not_active`, key kinds `secret`/`restricted`/`publishable`, pagination/filters on `audit_events` and `platform/merchants`).
  3. Run `make gen-contracts` and `make gen-contracts-check` and `make -C libs/contracts typecheck`.
- **Tests:** `U-ACT1` (`Actor` validation for all 4 types, rejects unknown type or empty `ID`), contract codegen drift check (`make gen-contracts-check`).

---

### 08.2. Identity schema, migrations (`0006`–`0009`) & core repositories

- **Status:** **BLOCKED on owner RLS decision (Section 2).** Also depends on `07.2b` (`0005` migration number) and `08.1` (`Actor`).
- **Goal:** Create the identity database schema from `to-be.md` §8.1, backfill existing `api_keys` rows, enforce append-only privileges on `audit_events`, and implement pgstore repositories for the new entities.
- **Labels:** `identity`, `security`, `infra`.
- **Dependencies:** `07.2b` (migration `0005` precedes `0006`; `pgtenant` if Option A/C), `08.1` (`Actor`), **Owner RLS Decision**.
- **Migration numbers:** `0006`, `0007`, `0008`, `0009` (all in `libs/identity/infra/pgstore/migrations/`).
- **Estimated size:** L (4–6 h).
- **Files:**
  - `libs/identity/infra/pgstore/migrations/0006_identity_users_and_tokens.sql`
  - `libs/identity/infra/pgstore/migrations/0007_identity_memberships_invitations_staff.sql`
  - `libs/identity/infra/pgstore/migrations/0008_identity_api_keys_kinds.sql`
  - `libs/identity/infra/pgstore/migrations/0009_identity_audit_events.sql`
  - `libs/identity/domain/` (new entities & errors: `user.go`, `membership.go`, `invitation.go`, `platform_staff.go`, `refresh_token.go`, `user_token.go`, `audit_event.go`, updated `merchant.go` & `apikey.go`)
  - `libs/identity/application/ports.go` (ports for new repos)
  - `libs/identity/infra/pgstore/*.go` + integration tests (`*_test.go`)
  - `apps/api/cmd/migrate/roles_integration_test.go` (if adding `0006`–`0009` privilege/RLS smoke assertions alongside `08.11`)
- **Steps:**
  1. Write `0006_identity_users_and_tokens.sql` (`BEGIN; ... COMMIT;`):
     - `ALTER TABLE merchants` adding `onboarding TEXT NOT NULL DEFAULT 'sales_led' CHECK (onboarding IN ('sales_led', 'self_signup'))` and `approved_at TIMESTAMPTZ`.
     - `CREATE TABLE users` (`id`, `email CITEXT UNIQUE`, `name`, `password_hash`, `status IN ('active', 'disabled')`, `email_verified_at`, `password_changed_at`, `last_login_at`, `created_at`).
     - `CREATE TABLE refresh_tokens` (`id`, `user_id REFERENCES users`, `family_id`, `audience IN ('iris-dashboard', 'iris-platform')`, `token_hash UNIQUE`, `replaced_by`, `rotated_at`, `expires_at`, `revoked_at`, `created_at`) + indexes on `(family_id)`, `(user_id)`.
     - `CREATE TABLE user_tokens` (`id`, `user_id REFERENCES users`, `purpose IN ('email_verification', 'password_reset', 'password_setup')`, `token_hash UNIQUE`, `expires_at`, `used_at`, `created_at`).
  2. Write `0007_identity_memberships_invitations_staff.sql` (`BEGIN; ... COMMIT;`):
     - `CREATE TABLE merchant_memberships` (`merchant_id REFERENCES merchants`, `user_id REFERENCES users`, `role IN ('owner','admin','developer','operations','risk_analyst','viewer')`, `status IN ('active','removed')`, `created_at`, `PRIMARY KEY (merchant_id, user_id)`), plus partial unique index `CREATE UNIQUE INDEX merchant_memberships_one_owner_idx ON merchant_memberships (merchant_id) WHERE role = 'owner' AND status = 'active';` enforcing the single-owner invariant in Postgres!
     - `CREATE TABLE invitations` (`id`, `merchant_id REFERENCES merchants`, `email CITEXT`, `role`, `token_hash UNIQUE`, `invited_by_actor`, `expires_at`, `accepted_at`, `revoked_at`, `created_at`).
     - `CREATE TABLE platform_staff` (`user_id PK REFERENCES users`, `role IN ('platform_admin', 'platform_support')`, `created_at`).
  3. Write `0008_identity_api_keys_kinds.sql` (`BEGIN; ... COMMIT;`):
     - Add `kind TEXT`, `name TEXT`, `expires_at TIMESTAMPTZ` to `api_keys`; alter `hash` to nullable with `CHECK ((kind = 'publishable' AND hash IS NULL) OR (kind IN ('secret', 'restricted') AND hash IS NOT NULL))`; backfill existing rows (`UPDATE api_keys SET kind = 'secret', name = 'legacy' WHERE kind IS NULL;`), then set `NOT NULL` and `CHECK (kind IN ('secret', 'publishable', 'restricted'))`.
  4. Write `0009_identity_audit_events.sql` (`BEGIN; ... COMMIT;`):
     - `CREATE TABLE audit_events` (`id BIGSERIAL PRIMARY KEY`, `merchant_id UUID NULL`, `action TEXT NOT NULL`, `entity_type TEXT NOT NULL`, `entity_id TEXT NOT NULL`, `actor_type TEXT NOT NULL`, `actor_id TEXT NOT NULL`, `request_id TEXT NOT NULL`, `data JSONB NOT NULL DEFAULT '{}'::jsonb`, `prev_hash BYTEA NOT NULL`, `hash BYTEA NOT NULL`, `occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()`).
     - Index on `(merchant_id, occurred_at DESC, id DESC)`.
     - **REVOKE rule (mandatory):** In `0009_identity_audit_events.sql` (the migration that creates `audit_events`), run:
       ```sql
       REVOKE UPDATE, DELETE ON audit_events FROM iris_app, iris_worker;
       ```
       (`iris_worker` and `iris_platform` have `BYPASSRLS`, which does not bypass table privileges; `iris_platform` already has `SELECT` only and must not be widened.)
  5. **RLS branching by owner decision:**
     - **If Option B (no RLS on identity tables):** Do not add RLS policies to `0006`–`0009`. Every merchant-scoped repository method in `libs/identity/infra/pgstore` takes `merchantID uuid.UUID` as a required parameter and filters `WHERE merchant_id = $1`.
     - **If Option A (RLS on all merchant-scoped identity tables):** Add `ENABLE` + `FORCE ROW LEVEL SECURITY` and `merchant_id = current_setting('app.merchant_id', true)::uuid` to `merchant_memberships`, `invitations`, `api_keys`, and `audit_events` (with `NULL` / platform policy handling), plus narrow `SECURITY DEFINER` lookup functions (and resolve `iris_migrator` `NOBYPASSRLS` + `FORCE` with `10-roles.sh` if needed). Repositories use `pgtenant.WithMerchant`.
     - **If Option C (Hybrid):** Add RLS to `merchant_memberships` and `audit_events` (plus `invitations` / `api_keys` if chosen), leaving `users`, `user_tokens`, `refresh_tokens`, `platform_staff`, `merchants` un-RLS'd, and provide narrow lookup path for `ListMembershipsByUser` and platform `audit_events`.
  6. Translate Portuguese comments in `libs/identity/domain/*.go` and `libs/identity/infra/pgstore/pgstore.go` as touched (conventions pass).
- **Tests:** `M1`–`M4` (migrations from empty DB and from phase 07 state with pre-existing `merchants` and `api_keys` rows -> backfilled `kind='secret', name='legacy'`; partial unique index blocks two active owners), `R1`–`R5` (pgstore integration tests for new tables + Option A/C RLS tests if selected).

---

### 08.3. RBAC permission matrix, `Principal` middleware & single route registry

- **Status:** **BLOCKED on owner RLS decision (Section 2).** Depends on `07.2a`, `07.3a`, `08.1`, and `08.5` (`TokenVerifier` interface).
- **Goal:** Implement the role-to-permission matrix in `libs/identity/domain`, build unified `Principal` authentication/authorization middleware (API keys + EdDSA JWTs + audience/merchant-status/step-up checks), and establish the single production + test route registry (Section 3).
- **Labels:** `identity`, `api`, `security`, `core`.
- **Dependencies:** `07.2a` (`libs/shared`), `07.3a` (`apps/api/internal/httpapi/route_registry_test.go`), `08.1` (`Actor`), `08.5` (JWT verifier port; can code against port before 08.5 merges), **Owner RLS Decision**.
- **Migration number:** None.
- **Estimated size:** L (4–6 h).
- **Files:**
  - `libs/shared/httproute/registry.go`, `registry_test.go` (shared `RouteSpec`, `TenantScope`, `AccessRule`)
  - `libs/identity/domain/rbac.go`, `rbac_test.go` (all 14 merchant permissions + 2 publishable permissions `tokens.create`, `checkout.confirm` + platform permissions; role matrix matching `to-be.md` §4.3; secret-key permission set)
  - `libs/identity/domain/principal.go`, `principal_test.go` (`Principal { Type, ID, MerchantID, Role, Permissions, AuthTime, Audience }`, `Can(perm)`, `Actor() shared.Actor`)
  - `libs/identity/infra/httpapi/middleware.go`, `middleware_test.go` (unified `Authenticate` supporting `irk_`, `irr_`, `ipk_`, and JWT; audience enforcement; `pending`/`suspended` merchant status gate; `RequireAccess(spec)` middleware; context helpers replacing/wrapping `MerchantFromContext` safely during transition)
  - `libs/payments/infra/httpapi/handlers.go`, `handlers_test.go` (attach `RouteSpec` permissions to all 5 existing payment routes; read `Principal` + `Merchant` from context)
  - `apps/api/internal/httpapi/router.go`, `route_registry.go` (promoted production registry), `route_registry_test.go` (extended from 07.3a)
  - `apps/api/cmd/api/main.go` (wire unified auth middleware)
- **Steps:**
  1. Implement `Permission`, `Role`, `RolePermissions(role)`, and `SecretKeyPermissions()` in `libs/identity/domain/rbac.go` per `to-be.md` §4.3 and spec guideline 7 (secret keys get every merchant permission except `members.manage`, `settings.write`, `merchant.profile.write`).
  2. Implement `Principal` and `Principal.Actor()` returning `shared.Actor` (`user` -> `user.ID`, `api_key` -> `key.ID`, `platform_user` -> `user.ID`).
  3. Extend `Authenticate` middleware to accept:
     - Bearer `irk_` (secret), `irr_` (restricted, permissions intersected/validated from `key.Scopes`), `ipk_` (publishable, permissions `tokens.create`, `checkout.confirm` only -> `403 insufficient_scope` on merchant endpoints);
     - Bearer JWT (via `TokenVerifier` port): verify signature + expiry + `aud` (`iris-dashboard` vs `iris-platform`), resolve permissions from DB role/membership (never from JWT claims per spec guideline 2!), verify user is `active` and membership is `active`.
  4. Enforce merchant status per route `AllowPendingMerchant` flag: `pending` or `suspended` on payment routes -> `403 merchant_not_active`; `pending` allowed on settings, members, keys, `/v1/me`. Enforce `RequireStepUp` (`now - auth_time <= 5m`) -> `403 reauthentication_required`.
  5. Promote 07.3a's route registry into `libs/shared/httproute` + `apps/api/internal/httpapi/route_registry.go` and extend `apps/api/internal/httpapi/route_registry_test.go` with `Permission` / `Access` checks so **one** registry drives both runtime middleware and the `chi.Walk` + cross-tenant meta-tests.
  6. **RLS branching by owner decision:**
     - **If Option B (no RLS):** `Authenticate` looks up key by prefix / membership by `(merchant_id, user_id)` directly via `iris_app` pgstore; payments handlers continue passing `principal.MerchantID` into `pgtenant.WithMerchant` (from `07.2b`) on payments tables.
     - **If Option A or C:** `Authenticate` uses `SECURITY DEFINER` (or un-RLS'd lookup table in Option C) to resolve `api_key` / `membership`, then identity handlers handling `merchant_memberships` / `audit_events` (and `api_keys` / `invitations` in Option A) run inside `pgtenant.WithMerchant(ctx, pool, principal.MerchantID, ...)`; platform handlers use `iris_platform` pool for cross-tenant reads.
- **Tests:** `P1`–`P4` (generated table-driven matrix test for all 6 roles × all permissions; secret vs restricted vs publishable; audience isolation `iris-dashboard` vs `iris-platform`; `pending`/`suspended` 403 `merchant_not_active`), `REG1`–`REG2` (meta-test fails on unregistered route or route without permission/public declaration; cross-tenant suite passes on all mounted routes).

---

### 08.4. Mailer port, SMTP adapter & Mailpit *(IN FLIGHT — WAVE 3)*

- **Status:** **In flight in wave 3** (already briefed; **do not brief or implement again**).
- **Goal:** Provide the `Mailer` outbound port in `libs/identity/application`, an SMTP adapter, Mailpit in `infra/docker/docker-compose.yml` (pinned image, documented UI port), and plain-text + simple HTML English templates for verification, invitation, and password reset.
- **Labels:** `identity`, `infra`, `docs`.
- **Dependencies:** None (runs independently in wave 3).
- **Migration number:** None.
- **Estimated size:** M (3 h — tracked in wave 3, listed here only for dependency mapping).
- **Expected files (owned by wave 3 task `08.4`):** `libs/identity/application/mailer.go`, `libs/identity/infra/mailer/**`, `apps/api/internal/config/**`, `infra/docker/docker-compose.yml`, `infra/docker/.env.example`, `README.md`, `AGENTS.md`.
- **Expected interface consumed by `08.7`, `08.9`, `08.12`:**
  - `application.Mailer` port in `libs/identity/application` (e.g., `SendVerificationEmail`, `SendPasswordResetEmail`, `SendInvitationEmail` — check exact merged method signatures after wave 3 lands before wiring `08.7`, `08.9`, `08.12`).
  - SMTP config fields in `apps/api/internal/config` and Mailpit service in Compose.

---

### 08.5. EdDSA signing keys, access JWTs, JWKS & step-up re-authentication

- **Status:** Ready (implements ADR-0006). Can proceed in parallel with `08.2`; `reauthenticate` handler needs `users` repo from `08.2`.
- **Goal:** Implement Ed25519 (`EdDSA`) JWT issuance/verification with `kid`, key rotation with overlap, `GET /.well-known/jwks.json`, `auth_time` claim, and `POST /v1/auth/reauthenticate`.
- **Labels:** `identity`, `api`, `security`.
- **Dependencies:** `08.1` (OpenAPI contracts), `08.2` (for `reauthenticate` password check against `users` repo), `08.3` (registry registration).
- **Migration number:** None.
- **Estimated size:** M (3–4 h).
- **Files:**
  - `libs/identity/infra/tokens/eddsa.go`, `eddsa_test.go`, `jwks.go` (key set loader, `TokenIssuer`, `TokenVerifier`, JWKS JSON builder)
  - `libs/identity/application/reauthenticate.go`, `reauthenticate_test.go`
  - `libs/identity/infra/httpapi/auth_handlers.go` (`handleJWKS`, `handleReauthenticate`)
  - `apps/api/internal/config/config.go`, `config_test.go` (`JWT_SIGNING_KEYS`, `JWT_ACTIVE_KID`, fail-closed validation when auth is enabled), `apps/api/.env.example` (documented local dev placeholder or generation note)
  - `apps/api/cmd/api/main.go`, `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. Implement `KeySet` loader in `libs/identity/infra/tokens/`: parse Ed25519 private keys with `kid` (and optional `not_after`), validate `JWT_ACTIVE_KID` exists and is not expired, derive public OKP JWK set (`kty: "OKP"`, `crv: "Ed25519"`, `alg: "EdDSA"`, `use: "sig"`, `kid`, `x`). **Fail closed** at startup if missing, malformed, or active key expired.
  2. Implement `Issue(claims)` (15 min TTL, `sub`, `aud`, `mid`, `role`, `sid`, `jti`, `iat`, `exp`, `auth_time`, header `kid`, `alg: EdDSA`) and `Verify(token, expectedAudience)` (strict `alg == "EdDSA"`, known non-expired `kid`, valid signature/exp/aud).
  3. Mount `GET /.well-known/jwks.json` as explicit `ScopePublic` / `AccessPublic` in the route registry.
  4. Implement `ReauthenticateUseCase`: verify current user's password via `Hasher` (Argon2id) and issue a fresh access JWT with `auth_time = now` (preserving `sid`, `mid`, `role`). Mount `POST /v1/auth/reauthenticate`.
- **Tests:** `T1`–`T6` (EdDSA issue/verify round-trip, 15m TTL, rejects `alg=none`/`HS256`/wrong `aud`/unknown `kid`/expired token, rotation overlap accepts `K1` while signing with `K2`, fails closed on empty/invalid key config, JWKS never exposes `d` private key, `reauthenticate` updates `auth_time` and rejects wrong password with 401).

---

### 08.6. Refresh tokens & session lifecycle (`login`, `refresh`, `logout`)

- **Status:** Ready after `08.2` and `08.5` (implements ADR-0005).
- **Goal:** Implement `POST /v1/auth/login`, `POST /v1/auth/refresh` (with single-flight rotation, benign-race grace window, and family reuse revocation), and `POST /v1/auth/logout`.
- **Labels:** `identity`, `api`, `security`.
- **Dependencies:** `08.1`, `08.2`, `08.3`, `08.5`.
- **Migration number:** None (uses `0006` `refresh_tokens`).
- **Estimated size:** M (4–5 h).
- **Files:**
  - `libs/identity/application/login.go`, `refresh_session.go`, `logout.go` + unit tests
  - `libs/identity/infra/pgstore/refresh_tokens.go` + integration tests (row locking `SELECT ... FOR UPDATE`, family revocation)
  - `libs/identity/infra/httpapi/auth_handlers.go` + HTTP tests
  - `apps/api/internal/config/config.go` (`REFRESH_TOKEN_TTL`, default `7d` / `168h`; `REFRESH_GRACE_WINDOW`, default `10s`)
  - `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. Implement `LoginUseCase`: lookup user by email; if unknown, run dummy Argon2id verification against a precomputed dummy hash so timing matches wrong-password (`phase-08` guideline 1); check `user.Status == active` and `email_verified_at != nil`; resolve active memberships (or `platform_staff` when `audience = iris-platform`), pick default/requested `merchant_id`, create refresh token (`family_id = newUUID`, SHA-256 hash persisted — never plaintext), return access JWT + opaque refresh token. Return one generic `401 invalid_credentials` for unknown email, wrong password, or unverified/disabled user (avoid account enumeration).
  2. Implement `RefreshSessionUseCase` per **ADR-0005**:
     - Coalesce concurrent in-process refreshes for the same `token_hash`; in Postgres, lock the row with `SELECT ... FOR UPDATE`.
     - Re-check `user.Status == active`, active membership (or `platform_staff`), and merchant status (`pending` allowed for dashboard login/refresh; `suspended` rejected immediately per spec guideline 4 — see Open Question Q3).
     - Rotate `T1 -> T2` (`replaced_by = T2.id`, `rotated_at = now()`), cache `(accessJWT, T2)` in memory for `REFRESH_GRACE_WINDOW` (default `10s`).
     - Reuse inside `REFRESH_GRACE_WINDOW`: return cached `(accessJWT, T2)` (or `409 concurrent_refresh` on cache miss, never revoking family). Reuse outside window or on revoked token/family: revoke all rows with `family_id`, return `401`.
  3. Implement `LogoutUseCase`: revoke the current session's `family_id` (`POST /v1/auth/logout`).
- **Tests:** `S1`–`S7` (login parity timing/error, disabled/unverified rejected, concurrent refresh race returns 200 without family revocation, post-grace reuse revokes entire family, logout revokes family, removed membership/disabled user/suspended merchant fails refresh immediately).

---

### 08.7. User sign-up, email verification, password reset & password change

- **Status:** Ready after `08.2`, `08.4` (wave 3), and `08.6`.
- **Goal:** Implement self-signup (creates user + `pending` merchant + `owner` membership + verification email), email verification, password reset request & confirmation, and authenticated password change (revoking all refresh families).
- **Labels:** `identity`, `api`, `security`.
- **Dependencies:** `08.1`, `08.2`, `08.3`, `08.4` (Mailer), `08.6` (family revocation).
- **Migration number:** None (uses `0006`, `0007`).
- **Estimated size:** M (4–5 h).
- **Files:**
  - `libs/identity/domain/password.go` (12–128 char length validation, never logs plaintext)
  - `libs/identity/application/signup.go`, `verify_email.go`, `request_password_reset.go`, `confirm_password_reset.go`, `change_password.go` + unit tests
  - `libs/identity/infra/pgstore/users.go`, `user_tokens.go` (atomic signup tx: user + pending merchant + owner membership + verification `user_token`)
  - `libs/identity/infra/httpapi/auth_handlers.go` + HTTP tests
  - `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. `SignUpUseCase`: validate email, name, password (12–128 chars), legal name, 14-digit document; hash password with Argon2id; in one DB transaction create `User` (unverified), `Merchant` (`status = pending`, `onboarding = self_signup`), `Membership` (`owner`), and `UserToken` (`purpose = email_verification`, SHA-256 hash, TTL 24 h). Send verification email **after** DB commit via `Mailer`.
  2. `VerifyEmailUseCase`: atomically mark `user_tokens.used_at = now()` (`WHERE used_at IS NULL AND expires_at > now()`) and set `users.email_verified_at = now()`.
  3. `RequestPasswordResetUseCase` + `ConfirmPasswordResetUseCase`: always return `202 Accepted` regardless of whether email exists; create 1 h single-use hashed `user_token`; send reset mail after commit; on confirm, update `password_hash` + `password_changed_at`, mark token used, and **revoke all `refresh_tokens` families for the user** in the same transaction.
  4. `ChangePasswordUseCase` (`POST /v1/auth/password-change`, authenticated): verify current password, validate new password (12–128 chars), update hash, revoke all `refresh_tokens` families of the user in the same transaction (AC5).
- **Tests:** `U1`–`U6` (signup -> Mailpit -> verify -> login works and merchant is `pending`; payment routes return 403 `merchant_not_active`; duplicate email/document fails cleanly; expired/reused verify or reset token fails; password change & reset revoke all refresh families; reset request hides unknown email).

---

### 08.8. Memberships, `switch-merchant` & ownership transfer

- **Status:** Ready after `08.2`, `08.3`, `08.5`, `08.11`.
- **Goal:** Implement multi-merchant membership listing, active merchant switching (`POST /v1/auth/switch-merchant`), role updates (step-up required), member removal (guarding the single-owner invariant), and ownership transfer (`owner` only).
- **Labels:** `identity`, `api`, `security`.
- **Dependencies:** `08.1`, `08.2`, `08.3`, `08.5`, `08.11` (audit events).
- **Migration number:** None (uses `0007`).
- **Estimated size:** M (3–4 h).
- **Files:**
  - `libs/identity/application/list_members.go`, `change_member_role.go`, `remove_member.go`, `transfer_ownership.go`, `switch_merchant.go` + unit tests
  - `libs/identity/infra/pgstore/memberships.go` (transactional ownership swap under `SELECT ... FOR UPDATE` on `merchant_memberships`)
  - `libs/identity/infra/httpapi/member_handlers.go` + HTTP tests
  - `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. `GET /v1/members` (`members.manage` or read permission per `to-be.md` §12.1 — note `to-be.md:1223` lists `user members.manage`).
  2. `PATCH /v1/members/{user_id}` (`members.manage` + **step-up** `auth_time <= 5m` per `to-be.md:185`): change role among non-`owner` roles (`admin`, `developer`, `operations`, `risk_analyst`, `viewer`). Reject setting `role = owner` directly (must use ownership transfer) and reject demoting the `owner`. Write `audit_events` (`member.role_changed`) in the same tx.
  3. `DELETE /v1/members/{user_id}` (`members.manage`): lock merchant's active `owner` row; reject removing the sole `owner` (`409 cannot_remove_owner`); set `status = 'removed'` + write `audit_events` (`member.removed`) in the same tx.
  4. `POST /v1/members/transfer-ownership` (only `role == owner`, step-up required): in one tx lock both memberships (`ORDER BY user_id` to prevent deadlocks), demote current owner to `admin`, promote target active member to `owner`, write `audit_events` (`merchant.ownership_transferred`).
  5. `POST /v1/auth/switch-merchant` (`AccessAuthenticated`, dashboard audience): verify user has an active membership in `target_merchant_id`, verify merchant is not disabled/missing, issue new 15m access JWT with `mid = target_merchant_id` and new `role`.
- **Tests:** `MEM1`–`MEM6` (cannot remove or demote sole owner; concurrent ownership transfer & removal serialized without deadlock; role change requires step-up; `switch-merchant` rejects non-member merchant with 403/404 without leaking existence; removed member fails refresh immediately).

---

### 08.9. Invitations (`create`, `list`, `revoke`, `accept`)

- **Status:** Ready after `08.2`, `08.3`, `08.4` (wave 3), `08.11`.
- **Goal:** Implement merchant member invitations with 72-hour single-use hashed tokens, listing, revocation, and atomic acceptance (for both new users setting name/password and existing users).
- **Labels:** `identity`, `api`, `security`.
- **Dependencies:** `08.1`, `08.2`, `08.3`, `08.4` (Mailer), `08.11` (audit events).
- **Migration number:** None (uses `0007`).
- **Estimated size:** M (3–4 h).
- **Files:**
  - `libs/identity/application/invite_member.go`, `list_invitations.go`, `revoke_invitation.go`, `accept_invitation.go` + unit tests
  - `libs/identity/infra/pgstore/invitations.go` + integration tests
  - `libs/identity/infra/httpapi/invitation_handlers.go` + HTTP tests
  - `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. `POST /v1/invitations` (`members.manage`, `AllowPendingMerchant = true`): validate email and role (cannot invite as `owner` unless called by platform `CreateMerchant` flow in `08.12`; reject if user already has an active membership on that merchant); generate 32-byte random token, store SHA-256 `token_hash` with `expires_at = now + 72h`, `invited_by_actor = principal.Actor()`, write `audit_events` (`member.invited`) in tx; send email via `Mailer` **after** commit.
  2. `GET /v1/invitations` and `POST /v1/invitations/{id}/revoke` (or `DELETE /v1/invitations/{id}` — match `08.1` OpenAPI): scoped by `principal.MerchantID`; revoke sets `revoked_at = now()` and appends `audit_events` (`invitation.revoked`).
  3. `POST /v1/invitations/{token}/accept` (`AccessPublic`, rate-limited in `08.13`): hash token, `UPDATE invitations SET accepted_at = now() WHERE token_hash = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now() RETURNING ...` (atomic single-use claim!). If user does not exist, require `name` + `password` (12–128 chars), create active verified `User`; if user exists, require authenticated session or password confirmation (document rule in `08.1` contract so an unauthenticated attacker cannot hijack an existing user's account via invite email!); create/reactivate `merchant_memberships`, append `audit_events` (`member.joined`), return session tokens (or confirmation).
- **Tests:** `INV1`–`INV6` (72h expiry enforced; revoked invite rejected; concurrent double-accept allows only 1 winner and rejects the second with 409/410 without duplicate membership; cross-tenant list/revoke returns 404; email sent only after commit).

---

### 08.10. API key kinds (`secret`, `restricted`, `publishable`), roll & revoke

- **Status:** Ready after `08.2`, `08.3`, `08.11`.
- **Goal:** Support `secret` (`irk_`), `restricted` (`irr_`), and `publishable` (`ipk_`) API keys with names, permission subsets, roll with overlap window (default 24h, max 7d), revocation, `last_used_at`, and step-up enforcement on create & roll.
- **Labels:** `identity`, `api`, `security`.
- **Dependencies:** `08.1`, `08.2`, `08.3`, `08.11` (audit events).
- **Migration number:** None (uses `0008`).
- **Estimated size:** M (4–5 h).
- **Files:**
  - `libs/identity/domain/apikey.go`, `apikey_test.go` (`KeyKind`: `secret`, `restricted`, `publishable`; `irk_`/`irr_`/`ipk_` prefix formatting & parsing; `Roll(overlap)` validation `0..7d`, default `24h`; `Revoke`; `IsUsable(now)` checking `status == active` and `(expires_at == nil || now.Before(*expires_at))`)
  - `libs/identity/application/issue_api_key.go`, `roll_api_key.go`, `list_api_keys.go`, `revoke_api_key.go` + unit tests
  - `libs/identity/infra/pgstore/api_keys.go` (merchant-scoped `GetByID(ctx, merchantID, id)`, `ListByMerchant`, `Revoke(ctx, merchantID, id, at)`)
  - `libs/identity/infra/httpapi/api_key_handlers.go` + HTTP tests
  - `apps/api/cmd/irisctl/cli.go`, `cli_test.go` (update if `IssueAPIKeyInput` adds `Name`/`Kind`)
  - `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. Extend `domain.APIKey`: `Kind`, `Name`, `ExpiresAt`, `CreatedByActor`. `ParseToken` recognizes `irk_<prefix>.<secret>`, `irr_<prefix>.<secret>`, and `ipk_<prefix>` (no secret, `hash == nil`). Validate `restricted` scopes against `domain.MerchantPermissions()` (reject unknown permissions or `members.manage`/`settings.write`/`merchant.profile.write` if people-only, or allow subset per `to-be.md` §4.3 — allow valid merchant permissions; secret keys exclude those 3 per spec guideline 7).
  2. `GET /v1/api-keys` (`api_keys.manage`): returns metadata (`id`, `kind`, `name`, `prefix`, `scopes`, `status`, `created_at`, `expires_at`, `last_used_at`, `revoked_at`) + plaintext `publishable_key` (`ipk_<prefix>`) for publishable keys only; NEVER returns `hash` or `irk_`/`irr_` secrets.
  3. `POST /v1/api-keys` (`api_keys.manage`, **step-up required**): creates key, writes `audit_events` (`api_key.created`) in tx, returns plaintext token once (except publishable which is public).
  4. `POST /v1/api-keys/{id}/roll` (`api_keys.manage`, **step-up required**): loads key scoped by `merchantID`, rejects rolling an already-revoked or expired key, sets `old.expires_at = now + overlap` (default `24h`, max `7d` = `168h`), creates new active key of same `kind`/`name`/`scopes`, writes `audit_events` (`api_key.rolled`) in tx, returns new key secret once. Both keys work during overlap; old key stops working after `expires_at`.
  5. `POST /v1/api-keys/{id}/revoke` (`api_keys.manage`, merchant-scoped): revokes key + writes `audit_events` (`api_key.revoked`) in tx.
- **Tests:** `KEY1`–`KEY7` (create/roll without recent `auth_time` -> 403 `reauthentication_required`; roll overlap defaults 24h, rejects >7d, old key works before `expires_at` and fails 401/403 after; restricted key with `payments.read` gets 403 on `POST /v1/payment-intents`; publishable key gets 403 on merchant routes; cross-tenant roll/revoke gets 404; `irisctl` still works).

---

### 08.11. Audit log (`audit_events` hash chain, advisory lock, `GET /v1/audit-events` & REVOKE test)

- **Status:** **BLOCKED on owner RLS decision (Section 2).** Depends on `08.1`, `08.2`, `08.3`.
- **Goal:** Implement the tamper-evident, hash-chained `audit_events` writer under a transaction-scoped advisory lock, expose `GET /v1/audit-events`, and verify the `REVOKE UPDATE, DELETE` privilege enforcement across Postgres roles.
- **Labels:** `identity`, `security`, `api`.
- **Dependencies:** `08.1` (`Actor`), `08.2` (`0009_identity_audit_events.sql`), `08.3` (`Principal`, registry), **Owner RLS Decision**.
- **Migration number:** None (uses `0009` created by `08.2`).
- **Estimated size:** M (4–5 h).
- **Files:**
  - `libs/identity/domain/audit.go`, `audit_test.go` (canonical hash computation: SHA-256 over `prev_hash || id/seq || merchant_id || action || entity_type || entity_id || actor_type || actor_id || request_id || canonical_data || occurred_at_utc`, genesis `prev_hash` = 32 zero bytes)
  - `libs/identity/application/ports.go` (`AuditLog` port: `Append(ctx, txOrSession, event)`), `list_audit_events.go`
  - `libs/identity/infra/pgstore/audit_events.go`, `audit_events_test.go` (`pg_advisory_xact_lock`, read latest `hash`, insert next row in caller's tx)
  - `libs/identity/infra/httpapi/audit_handlers.go` (`GET /v1/audit-events` with action/cursor/limit filters)
  - `apps/api/cmd/migrate/roles_integration_test.go` (mandatory AC9 role tests)
  - `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. **REVOKE rule verification (from spec 08 AC9):** Ensure `0009_identity_audit_events.sql` includes:
     ```sql
     REVOKE UPDATE, DELETE ON audit_events FROM iris_app, iris_worker;
     ```
     and add integration test cases in `apps/api/cmd/migrate/roles_integration_test.go`, one case per role (`iris_app`, `iris_worker`, `iris_platform`):
     - `iris_app` & `iris_worker`: `INSERT` and `SELECT` succeed; `UPDATE` and `DELETE` fail with SQLSTATE `42501` (`permission denied`). (`iris_worker` has `BYPASSRLS`, which does **not** bypass table privileges, so the REVOKE still works.)
     - `iris_platform`: `SELECT` succeeds; `INSERT`, `UPDATE`, `DELETE` fail with SQLSTATE `42501` (`iris_platform` has `SELECT` only and must not be widened).
  2. Implement `AuditLog.Append` inside the same database transaction as the state change (never in a separate post-commit write!). Acquire a deterministic transaction-scoped advisory lock (`SELECT pg_advisory_xact_lock(hashtext('audit_events_chain'))`) so concurrent transactions linearize cleanly and `prev_hash` never forks.
  3. Enforce `data` payload sanitization: strip/reject any key matching secret/token/password/email-in-clear denylist before marshaling.
  4. Implement `GET /v1/audit-events` (`logs.read`, `ScopeTenant`, `AllowPendingMerchant = true`, filtered by `principal.MerchantID`, optional `action` filter).
  5. **RLS branching by owner decision:**
     - **If Option B (no RLS):** `audit_events` has no RLS policy. `Append` inserts directly as `iris_app` (both merchant-scoped and `merchant_id IS NULL` platform rows); `ListByMerchant` filters `WHERE merchant_id = $1`.
     - **If Option A or C (RLS on `audit_events`):**
       - `GET /v1/audit-events` runs inside `pgtenant.WithMerchant(ctx, pool, principal.MerchantID, ...)` so `iris_app` only sees rows where `merchant_id = current_setting('app.merchant_id', true)::uuid`.
       - Because the hash chain is global across `audit_events` (`prev_hash` links to the latest `id`), `Append` reading the previous row's `hash` **must** see the global tail row even when `app.merchant_id` is set to merchant A (or `NULL` for platform actions)! Under `FORCE ROW LEVEL SECURITY`, a merchant-scoped `iris_app` transaction would only see merchant A's rows and break the global chain! Therefore, under Option A/C either: (i) the chain is **per-merchant** (`merchant_id` partitioned chain + `platform` chain for `NULL`, though `to-be.md` shows one `audit_events` chain), OR (ii) `Append` inserts via a narrow `SECURITY DEFINER` function (owned by a `BYPASSRLS` role, since `iris_migrator` has `NOBYPASSRLS`) that takes the advisory lock, reads the global tail `hash`, and inserts the row! Document this exact trade-off for the owner in Section 2.
- **Tests:** `AUD1`–`AUD5` (recomputing SHA-256 chain across mixed merchant & platform events verifies 100%; concurrent goroutines appending audit events produce one linear unbranched chain under `pg_advisory_xact_lock`; `roles_integration_test.go` proves `42501` on `UPDATE`/`DELETE` for `iris_app`, `iris_worker`, `iris_platform`; `GET /v1/audit-events` cross-tenant returns only caller's merchant events and never leaks platform `NULL` events).

---

### 08.12. Platform staff, `/v1/platform/merchants` & `irisctl bootstrap-admin`

- **Status:** Ready after `08.2`, `08.3`, `08.4` (wave 3), `08.9`, `08.11`.
- **Goal:** Enable `platform_admin` console operations (`/v1/platform/merchants` list, sales-led create with owner invitation, approve, suspend, reactivate) and `irisctl bootstrap-admin --email` (`make bootstrap-admin EMAIL=...`).
- **Labels:** `identity`, `api`, `security`, `docs`.
- **Dependencies:** `08.1`, `08.2`, `08.3`, `08.4`, `08.9`, `08.11`.
- **Migration number:** None (uses `0006`, `0007`, `0009`).
- **Estimated size:** M (4–5 h).
- **Files:**
  - `libs/identity/domain/merchant.go` (`Approve(at)`, `Suspend(reason, at)`, `Reactivate(at)` state-machine methods)
  - `libs/identity/application/platform_create_merchant.go`, `approve_merchant.go`, `suspend_merchant.go`, `reactivate_merchant.go`, `list_merchants.go`, `bootstrap_admin.go` + unit tests
  - `libs/identity/infra/pgstore/platform.go`
  - `libs/identity/infra/httpapi/platform_handlers.go` + HTTP tests
  - `apps/api/cmd/irisctl/cli.go`, `main.go`, `cli_test.go` (`bootstrap-admin --email`)
  - `apps/api/Makefile` / root `Makefile` (check if `bootstrap-admin` target is added in `apps/api/Makefile` + root `Makefile` — coordinate with `17.1` if `apps/api/Makefile` is in flight)
  - `apps/api/internal/httpapi/route_registry.go`
- **Steps:**
  1. Add `Approve`, `Suspend`, `Reactivate` methods on `domain.Merchant`:
     - `pending -> active` (`Approve`, sets `approved_at`); `active -> suspended` (`Suspend`); `suspended -> active` (`Reactivate`). Reject invalid transitions (`suspended -> approve`? or idempotent repeat -> reject or no-op consistently and pin with tests!).
  2. Implement `/v1/platform/merchants`:
     - `GET /v1/platform/merchants` (`ScopePlatform`, `AccessPlatform("platform.merchants.read")` — uses `iris_platform` pool if configured or `iris_app` under Option B).
     - `POST /v1/platform/merchants`: in one tx creates `Merchant` (`status = active`, `onboarding = sales_led`, `approved_at = now`), `Invitation` for owner (`role = owner`, 72h hashed token), and `audit_events` (`merchant.created`, `actor_type = platform_user`); sends invitation email via `Mailer` **after** commit (AC2).
     - `POST /v1/platform/merchants/{id}/{approve,suspend,reactivate}`: transitions merchant status + writes `audit_events` in one tx.
  3. Implement `irisctl bootstrap-admin --email <email>`: creates active verified `User` (with no password yet, or unusable random placeholder hash) + `platform_staff` (`role = platform_admin`) + `UserToken` (`purpose = password_setup`, 1h single-use hashed token), and prints the one-time password-setup link to stdout once (never logs it). Refuse/fail cleanly if a `platform_admin` already exists unless idempotent flag or explicit error is specified.
- **Tests:** `PLT1`–`PLT6` (sales-led create -> Mailpit invite -> accept -> user is `owner` of `active` merchant; self-signup `pending` -> `approve` unlocks payment routes; `suspend` blocks payment routes; `reactivate` restores `active`; dashboard JWT gets 403/401 on `/v1/platform/*` and platform JWT gets 403/401 on merchant routes; `irisctl bootstrap-admin` prints setup link once and logs nothing secret).

---

### 08.13. Rate limiting, `/v1/me` completion & phase verification/docs pass

- **Status:** Ready after `08.3`–`08.12`.
- **Goal:** Add rate limiting behind a port on `signup`, `login`, `password-reset`, and `invitations/{token}/accept`; finalize `GET /v1/me` response (`principal`, `merchant`, `permissions`); and run the full phase 08 verification & docs update pass.
- **Labels:** `identity`, `api`, `security`, `docs`.
- **Dependencies:** `08.3`–`08.12`.
- **Migration number:** None.
- **Estimated size:** M (3–4 h).
- **Files:**
  - `libs/identity/application/ports.go` (`RateLimiter` port: `Allow(ctx, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)`)
  - `libs/identity/infra/ratelimit/memory.go`, `memory_test.go` (sliding-window or token-bucket in-memory implementation with bounded cleanup)
  - `libs/identity/infra/httpapi/ratelimit_middleware.go`, `handlers.go` (`GET /v1/me` returning principal, merchant, and resolved permissions)
  - `apps/api/internal/config/config.go` (`RATE_LIMIT_LOGIN_PER_MIN`, `RATE_LIMIT_SIGNUP_PER_MIN`, `TRUSTED_PROXIES` default empty)
  - `apps/api/tests/identity.http`, `apps/api/tests/payment-intents.http`
  - `AGENTS.md`, `docs/security.md`, `README.md`, `docs/specs/README.md` (status -> `done` in the final audit PR after all tasks merge)
- **Steps:**
  1. Implement `RateLimiter` port and in-memory adapter; key by both **client IP** (from `r.RemoteAddr` unless `TRUSTED_PROXIES` is configured) and **hashed email** (SHA-256 with salt, never raw email in logs/keys) on `POST /v1/auth/signup`, `POST /v1/auth/login`, `POST /v1/auth/password-reset`, `POST /v1/invitations/{token}/accept`. Return `429 rate_limited` with `Retry-After` header.
  2. Update `GET /v1/me` (`AccessAuthenticated`, `AllowPendingMerchant = true`) to return principal info (`type`, `id`, `email`, `name`, `audience`), active `merchant` (if merchant-scoped), and resolved `permissions[]` (used by phase 09 permission-aware UI).
  3. Run full phase 08 acceptance suite (`make test`, `make lint`, `make gen-contracts-check`, `make -C apps/api test-int`, `make -C libs/identity test-int`), verify zero Portuguese strings in touched files and zero secrets in logs, and update `AGENTS.md`, `docs/security.md`, `README.md`, and `docs/specs/README.md`.
- **Tests:** `RL1`–`RL3` (exceeding per-email or per-IP threshold returns `429` + `Retry-After`; spoofed `X-Forwarded-For` does not bypass IP limit when `TRUSTED_PROXIES` is empty; `GET /v1/me` returns exact permissions per role/key kind).

---

## 5. Pre-mortem (six points)

Each risk maps directly to a required test ID in Section 7.

### 5.1 State transitions, including repeats

| Entity / Flow | Transition & repeat / edge case | Expected behavior | Test |
| --- | --- | --- | --- |
| **User status** | `active -> disabled -> active` (re-enable) | Disabled user cannot log in or refresh (`401`); existing JWTs expire <= 15m (or fail if checked). Re-enabling restores login; previously revoked refresh families stay revoked. | `U5`, `S4` |
| **User email & edit** | Verify email twice (`used_at` set); change email/profile after disable or verify | Second verify fails (`400`/`410 token_used`); disabled user cannot verify or change password. | `U2`, `U5` |
| **Invitation** | `pending -> accepted`, `pending -> expired` (>72h), `pending -> revoked`, accept revoked/expired/used, edit role after invite | Only `pending` and unexpired can be accepted once; repeat accept, expired (>72h), and revoked all fail cleanly without creating a membership. | `INV2`, `INV3`, `INV4` |
| **API key** | Issue -> roll (with overlap `24h`) -> use old & new -> old expires -> revoke old/new -> roll a revoked key -> revoke twice | Rolling a revoked or expired key is rejected (`409`); revoking an already-revoked key is idempotent (`200` with `revoked`); old key authenticates until `expires_at` and fails `401`/`403` after. | `KEY2`, `KEY3`, `KEY4` |
| **Merchant status** | `pending -> active` (approve) -> `suspended` (suspend) -> `active` (reactivate); approve an already-active or suspended merchant | `pending` allows settings/members/keys/`/v1/me` and blocks payment routes (`403 merchant_not_active`); `approve` from `suspended` is rejected (`409`, must use `reactivate`). | `PLT2`, `PLT3`, `PLT4` |
| **Membership** | Remove member -> re-invite same email; change role -> edit again within/after step-up window | Re-inviting a `removed` member updates status back to `active` with the new role (not a duplicate PK failure); role change after 5m step-up expires returns `403 reauthentication_required`. | `INV5`, `MEM3` |

### 5.2 Concurrency and lock order

| Concurrent operation | Race condition | Protection & lock order | Test |
| --- | --- | --- | --- |
| **Refresh-token race** | Two parallel `POST /v1/auth/refresh` calls with the same `T1` (typical Next.js BFF page render with parallel server components) | In-process single-flight + DB `SELECT ... FOR UPDATE` on `refresh_tokens` by `token_hash`, plus `10s` benign-race grace window (ADR-0005). Within grace window: returns cached `(accessJWT, T2)` without revoking family. Outside grace window: revokes entire `family_id`. | `S2`, `S3` |
| **Invitation accepted twice** | Two requests race on `POST /v1/invitations/{token}/accept` | Atomic `UPDATE invitations SET accepted_at = now() WHERE token_hash = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now() RETURNING ...`. Only 1 caller gets a row; the second sees 0 rows and exits before touching `users` or `merchant_memberships`. | `INV4` |
| **Last-owner removal & ownership transfer** | `DELETE /v1/members/{owner_id}` races with another removal or `transfer-ownership` | Partial unique index `merchant_memberships_one_owner_idx` (`WHERE role = 'owner' AND status = 'active'`), plus `SELECT ... FROM merchant_memberships WHERE merchant_id = $1 ORDER BY user_id FOR UPDATE` inside a transaction before modifying roles/status. Prevents deadlocks and guarantees exactly 1 active owner. | `MEM1`, `MEM2` |
| **Audit hash chain append** | Two transactions (e.g. key issue + member invite) append to `audit_events` concurrently | Transaction-scoped advisory lock `SELECT pg_advisory_xact_lock(hashtext('audit_events_chain'))` acquired **after** domain row locks (consistent lock order: entity row -> audit lock -> read tail `hash` -> insert). Never forks `prev_hash`. | `AUD2` |
| **Concurrent sign-ups** | Two sign-ups with the same `email` or `document` | Postgres `UNIQUE` constraints on `users(email)` and `merchants(document)` map SQLSTATE `23505` to typed domain conflicts (`409`) without leaking which field collided if needed, or clean validation errors. | `U6` |
| **Refresh racing password change** | User refreshes token while password is changed in another session | Both lock/update `refresh_tokens` for `user_id`; password change sets `revoked_at` on all `family_id`s of `user_id` before commit. | `U4`, `S4` |

### 5.3 Config defaults and feature flags (read from the code)

Every existing default below was read directly from the codebase:

| Variable / Setting | Default in code | Source (`path:line`) | Phase 08 impact & required check |
| --- | --- | --- | --- |
| `HTTP_ADDR` | `":8080"` | `apps/api/internal/config/config.go:62` | Unchanged. |
| `IRIS_APP_DATABASE_URL` | `"postgres://iris_app:iris_app@localhost:15432/iris?sslmode=disable"` | `apps/api/internal/config/config.go:24,63` | Fails on empty string (`ErrEmptyDatabaseURL`, `:32,71,89-93`). Used by `apps/api` and `irisctl` (`apps/api/cmd/irisctl/main.go:43`). |
| `IRIS_WORKER_DATABASE_URL` | `"postgres://iris_worker:iris_worker@localhost:15432/iris?sslmode=disable"` | `apps/api/internal/config/config.go:25,64` | Relay pool; unaffected by phase 08 except `REVOKE` on `audit_events`. |
| `IRIS_MIGRATOR_DATABASE_URL` | `"postgres://iris_migrator:iris_migrator@localhost:15432/iris?sslmode=disable"` | `apps/api/internal/config/config.go:26,82` | Only read by `LoadMigrator()` (`cmd/migrate/main.go:41`); ignored by `Load()` (`config_test.go:106-116`). |
| `IRIS_PLATFORM_PASSWORD` | `iris_platform` (role created in Compose/initdb) | `infra/docker/.env.example:28`, `infra/docker/docker-compose.yml:26`, `infra/docker/postgres/initdb/10-roles.sh:48-49` | `apps/api/internal/config/config.go` has **no** `IRIS_PLATFORM_DATABASE_URL` yet. `08.12` (or `08.3`) adds `IRIS_PLATFORM_DATABASE_URL` (default matching `iris_platform` in `10-roles.sh`) if platform cross-tenant reads use a separate pool (`to-be.md:916`). Keep `iris_platform` `SELECT`-only! |
| `RELAY_ENABLED` | `"true"` (`true`) | `apps/api/internal/config/config.go:69` | Unchanged; tests without Kafka set `false`. |
| `ACQUIRER_URL` / `ACQUIRER_TOKEN` | `""` / `""` | `apps/api/internal/config/config.go:67-68` | Empty selects `FakeAcquirer` (`apps/api/cmd/api/main.go:67-73`). |
| `KAFKA_BROKER` / `ENV` | `"localhost:19092"` / `"dev"` | `apps/api/internal/config/config.go:65-66` | Unchanged. |
| `middleware.Timeout` / `ReadHeaderTimeout` / shutdown | `30s` / `5s` / `10s` | `apps/api/internal/httpapi/router.go:29`, `apps/api/cmd/api/main.go:114,128` | Argon2id login/signup stays well under `30s` (single hash ~50–150 ms). |
| **Client IP middleware** | **Disabled** (comment: chi `RealIP` trusts spoofable headers) | `apps/api/internal/httpapi/router.go:25-26` | **Risk:** Rate limiting (`08.13`) must NOT blindly trust `X-Forwarded-For`. Default `TRUSTED_PROXIES=""` uses `r.RemoteAddr` only! Test `RL2`. |
| **Mount nil-guards** | Silent return when `d.Authenticate == nil` or `d.AuthRequired == nil` | `libs/identity/infra/httpapi/handlers.go:34-36`, `libs/payments/infra/httpapi/handlers.go:44-46` | **Risk:** A nil `TokenVerifier` or `RouteRegistry` could silently drop routes. Startup wiring test (`REG1`, `W1`) must assert every expected route is mounted. |
| Argon2id `DefaultParams()` | `m=65536` (64 MiB), `t=1`, `p=4`, `SaltLen=16`, `KeyLen=32` | `libs/identity/infra/hasher/argon2.go:26-34` | Reused for user passwords and API keys (meets/exceeds OWASP minimum memory). Test `U1`. |
| `SecretGenerator` / `TokenScheme` | 16 hex prefix, 32 base64url secret, scheme `"irk"` | `libs/identity/infra/hasher/argon2.go:105-115`, `libs/identity/domain/apikey.go:36,39` | Extended in `08.10` with `"irr"` and `"ipk"`. Test `KEY1`. |
| `NewMerchant` initial status | `MerchantPending` (`"pending"`) | `libs/identity/domain/merchant.go:55` | Self-signup (`08.7`) stays `pending`; sales-led (`08.12`) sets `active`. Test `U1`, `PLT1`. |
| `idempotency.Middleware` | Reads `Identity` `MerchantFromContext`; no principal -> `401` | `libs/payments/infra/idempotency/middleware.go:48-52` | Must run **after** `08.3` unified auth + permission (`payments.write`) + active-merchant check so `pending` merchants get `403 merchant_not_active`, not `201`! Test `P3`. |
| `apps/web` session defaults | `IRIS_SESSION_COOKIE="iris_session"`, `IRIS_API_BASE_URL="http://localhost:8080"`, `IRIS_SESSION_PASSWORD` dev fallback | `apps/web/src/lib/env.ts:8-13`, `apps/web/src/lib/session.ts:18-22` | Unchanged in phase 08 (phase 09 migrates BFF session to JWT + refresh tokens). |
| **New phase 08 env vars (fail closed)** | `JWT_SIGNING_KEYS`, `JWT_ACTIVE_KID` (new in `08.5`); `REFRESH_GRACE_WINDOW` (`10s`), `REFRESH_TOKEN_TTL` (`168h`) (`08.6`); `SMTP_*` (`08.4`); `TRUSTED_PROXIES` (`""`, `08.13`) | n/a (new in phase 08) | Missing or invalid `JWT_SIGNING_KEYS` / `JWT_ACTIVE_KID` **must fail startup** (fail closed, ADR-0006). Test `T4`. |

### 5.4 Side effects before vs after commit

| Flow | Inside DB transaction (atomic) | After DB commit | Failure window & mitigation | Test |
| --- | --- | --- | --- | --- |
| **Signup / Invite / Password reset (`08.7`, `08.9`, `08.12`)** | Insert user/merchant/membership/invitation/`user_token` + `audit_events` | Send email via `Mailer` (SMTP / Mailpit) | If `Mailer` fails after commit, user/invitation exists; caller receives error/warning and can re-trigger verification/reset or re-send invite without creating duplicate rows. **Never** send email before commit (avoids ghost links for rolled-back rows). | `U1`, `INV6` |
| **All audited mutations (`08.8`–`08.12`)** | Domain state change + `audit_events` row (`pg_advisory_xact_lock` + hash chain) | None | Both commit or both roll back; no mutation without an audit event, no orphan audit event. | `AUD1`, `AUD2` |
| **Refresh rotation (`08.6`)** | Lock old token, mark `replaced_by` + `rotated_at`, insert new `refresh_tokens` hash | Store `(accessJWT, T2)` in in-memory grace cache (`10s` TTL) | If process dies right after commit before populating grace cache, parallel race within `10s` gets `409 concurrent_refresh` (retryable) instead of revoking family (ADR-0005). Plaintext `T2` is never persisted to DB. | `S2` |
| **`irisctl bootstrap-admin` (`08.12`)** | Insert `users` + `platform_staff` + `user_tokens` (`password_setup`) | Print one-time password-setup link to stdout | If stdout write fails after commit, token is unusable/unprinted; re-running `bootstrap-admin` (or issuing new setup token) recovers cleanly. Never logs link or token. | `PLT6` |
| **Identity Kafka outbox** | None in phase 08 (`to-be.md` §15 O4 defers `identity.merchant.*` events) | None | Explicitly out of scope; confirmed no outbox added to `libs/identity`. | n/a |

### 5.5 Permission and principal combinations

Every cell below must be covered by automated HTTP / unit tests (`P1`–`P4`, `KEY5`–`KEY6`, `PLT5`, `REG2`):

| Principal | Audience | `GET /v1/me` | `POST /v1/payment-intents` (`payments.write`) | `GET /v1/payment-intents` (`payments.read`) | `POST /v1/api-keys` (`api_keys.manage` + step-up) | `PATCH /v1/members/{id}` (`members.manage` + step-up) | `GET /v1/audit-events` (`logs.read`) | `/v1/platform/merchants*` | Other merchant's resource |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **Anonymous / invalid bearer** | n/a | 401 | 401 | 401 | 401 | 401 | 401 | 401 | 401 |
| **Secret key `irk_`** (active merchant) | n/a | 200 | 201 | 200 | **403** (people-only) | **403** (people-only) | 200 | 401/403 | **404** |
| **Restricted key `irr_`** (`[payments.read]`) | n/a | 200 | **403** | 200 | **403** | **403** | **403** | 401/403 | **404** |
| **Publishable key `ipk_`** | n/a | **403** (AC4: only `tokens.create`, `checkout.confirm`) | **403** | **403** | **403** | **403** | **403** | 401/403 | **403** |
| **Secret key `irk_`** (`pending` / `suspended` merchant) | n/a | 200 (for `pending`) | **403 `merchant_not_active`** | **403 `merchant_not_active`** | **403** | **403** | 200 (if `pending`) | 401/403 | **403** / **404** |
| **User JWT — `owner`** | `iris-dashboard` | 200 | 201 (active only) | 200 (active only) | 201 (with step-up; **403 `reauthentication_required`** without) | 200 (with step-up) | 200 | **403** (wrong aud) | **404** |
| **User JWT — `admin`** | `iris-dashboard` | 200 | 201 | 200 | 201 (step-up) | 200 (step-up; cannot touch owner) | 200 | **403** | **404** |
| **User JWT — `developer`** | `iris-dashboard` | 200 | 201 | 200 | 201 (step-up) | **403** | 200 | **403** | **404** |
| **User JWT — `operations`** | `iris-dashboard` | 200 | 201 | 200 | **403** | **403** | **403** | **403** | **404** |
| **User JWT — `risk_analyst`** | `iris-dashboard` | 200 | **403** | 200 | **403** | **403** | **403** | **403** | **404** |
| **User JWT — `viewer`** | `iris-dashboard` | 200 | **403** | 200 | **403** | **403** | **403** | **403** | **404** |
| **Platform JWT — `platform_admin`** | `iris-platform` | **403** (wrong aud on merchant routes) | **403** (wrong aud) | **403** (wrong aud) | **403** | **403** | **403** | **200/201** | n/a |

`Principal.Actor()` mapping verified for all 4 types (`U-ACT1`, `P1`):

- User JWT (`iris-dashboard`) -> `Actor{Type: "user", ID: userID, RequestID: reqID}`
- Platform JWT (`iris-platform`) -> `Actor{Type: "platform_user", ID: userID, RequestID: reqID}`
- Secret / Restricted / Publishable API key -> `Actor{Type: "api_key", ID: keyID, RequestID: reqID}`
- Automatic jobs (future phases) -> `Actor{Type: "system", ID: jobName, RequestID: reqID}`

### 5.6 What the phase removes

| Removed or replaced | Where it exists today | Replacement & follow-up in phase 08 |
| --- | --- | --- |
| `pending` merchants allowed on payment routes (phase 06 assumption A2) | `libs/identity/infra/httpapi/middleware.go` (checks only key revocation), `libs/payments/infra/httpapi/handlers.go` | `08.3` rejects `pending` and `suspended` merchants on payment routes with `403 merchant_not_active`; update phase 06 assumption note / `AGENTS.md`. |
| `MerchantFromContext` as the sole auth context contract | `libs/identity/infra/httpapi/handlers.go:108-118`, `libs/payments/infra/httpapi/handlers.go:76,102,123,162,185`, `libs/payments/infra/idempotency/middleware.go:48` | `08.3` introduces `PrincipalFromContext` (keeping `MerchantFromContext` as a helper or migrating callers cleanly so `idempotency.Middleware` and payments handlers get `Principal`). |
| `GET /v1/me` returning only `Merchant` | `libs/contracts/http/identity.openapi.yaml:19-39`, `libs/identity/infra/httpapi/handlers.go:44-64` | `08.1` + `08.13` expand `/v1/me` response to include principal, merchant, and resolved permissions. |
| Single implicit API key shape (`irk_` only, no `kind`, no `name`, no `expires_at`) | `libs/identity/domain/apikey.go:39`, `libs/identity/infra/pgstore/migrations/0001_init.sql:19-30` | `08.2` (`0008`) + `08.10` add `kind` (`secret`/`restricted`/`publishable`), `name`, `expires_at`; legacy rows backfilled as `kind='secret', name='legacy'`. |
| Test-only `route_registry_test.go` as an isolated test list | `07.3a` `apps/api/internal/httpapi/route_registry_test.go` | `08.3` promotes it into the single production + test `RouteRegistry` (zero duplicate route tables). |
| `AGENTS.md` / `docs/security.md` bullets stating users/roles/audit are future phase 08 | `AGENTS.md:153`, `docs/security.md:24,26,29,40,70-84` | `08.13` updates `AGENTS.md` (token shapes, principal, registry, audit) and `docs/security.md` (statuses -> `implemented`) once all phase 08 PRs merge. |

---

## 6. Acceptance criteria to task map

| AC # | Criterion (`docs/specs/phase-08-identity-access-api.md:47-55`) | Proving task(s) | Test IDs |
| --- | --- | --- | --- |
| 1 | Sign-up -> verification email in Mailpit -> verify -> login works; merchant is `pending` and payment routes answer `403 merchant_not_active` until a platform admin approves it | `08.3`, `08.4`, `08.6`, `08.7`, `08.12` | `U1`, `U2`, `P3`, `PLT2` |
| 2 | Platform admin creates a merchant -> owner receives an invitation -> accepts -> is owner | `08.4`, `08.9`, `08.12` | `PLT1`, `INV1` |
| 3 | For every role in the matrix, each permission is allowed or denied exactly as the table says (table-driven test generated from the matrix) | `08.3` | `P1` |
| 4 | Restricted key with only `payments.read` cannot create an intent (`403`); publishable key cannot call any merchant API except future phases 14/15 | `08.3`, `08.10` | `KEY5`, `KEY6`, `P2` |
| 5 | Refresh rotation works; reused rotated token (outside benign-race rule) revokes the family; logout revokes it; password change revokes all families | `08.6`, `08.7` | `S1`, `S2`, `S3`, `S5`, `U4` |
| 6 | Dashboard token is rejected on platform routes and the reverse | `08.3`, `08.5`, `08.12` | `P4`, `PLT5`, `T2` |
| 7 | Creating an API key without a recent `auth_time` answers `403 reauthentication_required` | `08.5`, `08.10` | `KEY1`, `T6` |
| 8 | Every route is in the registry; the registry test and the cross-tenant suite pass with the new routes | `08.3` (and `08.5`–`08.13`) | `REG1`, `REG2` |
| 9 | Audit chain verifies (a test recomputes the hashes) and an `UPDATE` on `audit_events` with `iris_app` fails | `08.2`, `08.11` | `AUD1`, `AUD3` |

---

## 7. Test list

All Go tests run with `-race`. Integration tests use `-tags=integration` against Compose Postgres. Fake tokens, keys, passwords, and emails live in shared named test fixtures, never repeated literals or logged.

### Unit (domain & application)

- **U-ACT1** (`08.1`): `shared.Actor` validates all 4 `ActorType` values and rejects unknown type or empty `ID`. [`libs/shared/actor`]
- **P1** (`08.3`): Generated table-driven test over all 6 roles × 14 merchant permissions + 2 publishable permissions + secret-key exclusions (`members.manage`, `settings.write`, `merchant.profile.write`). [`libs/identity/domain`]
- **P2** (`08.3`): `Principal.Can` and `Principal.Actor()` for secret, restricted, publishable keys, merchant user, and platform user. [`libs/identity/domain`]
- **T1** (`08.5`): EdDSA JWT issue/verify round-trip, 15m expiry, `kid` header, `auth_time` claim. [`libs/identity/infra/tokens`]
- **T2** (`08.5`): Rejects `alg=none`, `HS256`, unknown `kid`, expired token, wrong `aud` (`iris-dashboard` vs `iris-platform`).
- **T3** (`08.5`): Key rotation overlap: signing key `K2` signs new tokens while `K1` stays valid in verifier & JWKS until removed.
- **T4** (`08.5`): KeySet loader fails closed on empty env, malformed key, missing `JWT_ACTIVE_KID`, or expired active key.
- **U1**–**U6** (`08.7`): Password length (12–128), Argon2id hash/verify, signup -> verify -> login, reused/expired verify & reset tokens rejected, password change/reset revokes all refresh families, disabled user rejected. [`libs/identity/application`]
- **KEY1**–**KEY4** (`08.10`): Key parsing (`irk_`/`irr_`/`ipk_`), `Roll` overlap bounds (`0..7d`, default `24h`), rolling revoked key rejected, revoking twice is idempotent, `expires_at` enforcement. [`libs/identity/domain`, `application`]
- **AUD1** (`08.11`): Hash-chain computation and recomputation verifier detects any modified field or reordered row. [`libs/identity/domain`]

### HTTP, middleware & route registry (`libs/*/infra/httpapi`, `apps/api/internal/httpapi`)

- **P3** (`08.3`): `pending` and `suspended` merchants get `403 merchant_not_active` on payment routes; `pending` allowed on `/v1/me`, settings, members, invitations, keys, audit-events.
- **P4** (`08.3`): Audience split: `iris-dashboard` JWT rejected on `/v1/platform/*`; `iris-platform` JWT rejected on `/v1/*` merchant routes.
- **T5** (`08.5`): `GET /.well-known/jwks.json` returns public OKP Ed25519 keys with `kid` and never leaks private field `d`.
- **T6** (`08.5`): `POST /v1/auth/reauthenticate` refreshes `auth_time`; wrong password answers `401`.
- **KEY5**–**KEY7** (`08.10`): Restricted key (`payments.read`) gets `403` on `POST /v1/payment-intents`; publishable key (`ipk_`) gets `403` on all merchant routes; create/roll key without recent `auth_time` returns `403 reauthentication_required`; cross-tenant key roll/revoke returns `404`.
- **PLT1**–**PLT5** (`08.12`): Platform merchant create + owner invite + accept; approve/suspend/reactivate state transitions; platform audience enforcement.
- **RL1**–**RL3** (`08.13`): Per-email and per-IP rate limiting returns `429` + `Retry-After`; `X-Forwarded-For` ignored when `TRUSTED_PROXIES` is empty; `GET /v1/me` returns principal, merchant, and resolved permissions.
- **REG1** (`08.3`): `chi.Walk` meta-test fails if any mounted route is missing from `apps/api/internal/httpapi/route_registry.go` or lacks an explicit `AccessRule` / `TenantScope`. [`apps/api/internal/httpapi`]
- **REG2** (`08.3`–`08.13`): Cross-tenant HTTP suite passes across **all** `ScopeTenant` routes (merchant B on merchant A's keys, invitations, members, audit-events, payment-intents -> `404` and zero side effect).

### Integration against Postgres (`-race`, `-tags=integration`)

- **M1**–**M4** (`08.2`): Empty DB -> `0001`..`0009` idempotent; phase 07 DB with existing `merchants` & `api_keys` -> `0006`..`0009` succeeds and backfills `kind='secret', name='legacy'`; `merchant_memberships_one_owner_idx` rejects second active owner.
- **R1**–**R5** (`08.2`): Repository tests for users, refresh tokens, user tokens, memberships, invitations, platform staff, API keys (plus RLS `app.merchant_id` isolation tests if Option A or C).
- **S1**–**S7** (`08.6`): Login parity (`unknown email` vs `wrong password`), N concurrent refreshes with same `T1` (benign race within `10s` grace window returns 200 without revoking family), reuse after grace window revokes entire `family_id`, logout revokes family, removed membership / disabled user / suspended merchant fails refresh immediately.
- **INV1**–**INV6** (`08.9`): 72h expiry, revoke, concurrent double-accept allows exactly 1 membership, re-inviting removed member, mailer invoked only after commit.
- **MEM1**–**MEM6** (`08.8`): Concurrent last-owner removal and ownership transfer serialized; role change step-up; `switch-merchant`.
- **AUD2**–**AUD3** (`08.11`):
  - `AUD2`: N concurrent transactions appending `audit_events` produce one linear, unbroken SHA-256 chain under `pg_advisory_xact_lock`.
  - `AUD3` (`apps/api/cmd/migrate/roles_integration_test.go`, AC9): `iris_app` and `iris_worker` succeed on `INSERT`/`SELECT` and fail with SQLSTATE `42501` on `UPDATE`/`DELETE` of `audit_events`; `iris_platform` succeeds on `SELECT` and fails with SQLSTATE `42501` on `INSERT`/`UPDATE`/`DELETE`.

### CLI (`apps/api/cmd/irisctl`)

- **PLT6** (`08.12`): `irisctl bootstrap-admin --email ...` creates active user + `platform_admin`, prints one-time `password_setup` link once to stdout, logs no secrets, and `make bootstrap-admin EMAIL=...` works end-to-end.

---

## 8. Open questions for the owner

1. **RLS on identity tables (Section 2 — BLOCKING `08.2`, `08.3`, `08.11`):** Choose Option A, B, or C.
   - *Cheapest reversible assumption:* **Do not guess** — `08.2`, `08.3`, and `08.11` stay blocked until answered. If Option B is chosen, `docs/specs/README.md:67` should get a one-line carve-out matching `docs/design/to-be.md:547` and `docs/specs/phase-07-tenant-isolation.md:23`.
2. **`iris_platform` writes vs `SELECT`-only role (`to-be.md:916` vs `10-roles.sh:82-83`):** `to-be.md` §9.2 says `iris_platform` has "`SELECT` on merchant tables, writes on identity", whereas `infra/docker/postgres/initdb/10-roles.sh:82-83` grants `SELECT` only on tables to `iris_platform` (and the REVOKE rule notes `iris_platform` must not be widened).
   - *Cheapest reversible assumption:* Keep `iris_platform` strictly `SELECT`-only (for platform cross-tenant reads) and run `/v1/platform/merchants` write handlers on `iris_app` (works natively under RLS Option B; if RLS Option A/C is chosen, requires `SECURITY DEFINER` or explicit grant).
3. **`suspended` merchant behavior (`to-be.md:243` vs `phase-08:19,40`):** `to-be.md:243` says *"API keys get 403, the dashboard is read-only"*, whereas `phase-08:40` says *"a suspended merchant must stop working within one access-token lifetime; refresh must fail immediately"* and `phase-08:19` says *"rejects `pending` and `suspended` merchants on payment routes with 403 `merchant_not_active`"*.
   - *Cheapest reversible assumption:* Follow `phase-08` spec (`403 merchant_not_active` on payment routes; fail refresh immediately when active merchant is `suspended`), because reversing it later only requires relaxing the `suspended` check on refresh and allowing `GET` dashboard routes.
4. **Publishable API key (`ipk_`) HTTP transport on merchant routes (AC4):** In phases 14–15, `ipk_` is passed to vault/public checkout. In phase 08 (before vault/checkout exist), how should AC4 (*"publishable key cannot call any merchant API"*) be exercised?
   - *Cheapest reversible assumption:* Accept `Authorization: Bearer ipk_<prefix>` in `Authenticate` middleware, build a `Principal` with permissions `[tokens.create, checkout.confirm]` only, so every merchant route answers `403 insufficient_scope` (distinct from `401 invalid_credentials`).
5. **Existing user accepting an invitation (`08.9`):** `to-be.md:227-228` shows a new user setting name and password at `POST /v1/invitations/{token}/accept`. When the invited email already belongs to an existing `User`, accepting unauthenticated with only the token could attach a membership without authenticating the existing user (or allow account takeover if password is overwritten).
   - *Cheapest reversible assumption:* Require either an authenticated session for that `user.Email` or the existing user's current password on `accept`, and reject overwriting an existing user's password via an invitation link.
6. **`to-be.md` §8.1 schema omissions (`to-be.md:502-530` vs `phase-08` items 3, 7, 9):** `to-be.md` §8.1 omits `refresh_tokens.rotated_at` (needed for the ADR-0005 benign-race grace window), `invitations.revoked_at` and `invitations.created_at` (needed for `08.9` revoke and list), and `user_tokens.purpose = 'password_setup'` (needed for `08.12` `irisctl bootstrap-admin`).
   - *Cheapest reversible assumption:* Include those columns/enum value in migrations `0006` and `0007` (`08.2`), and update `to-be.md` §8.1 in the phase 08 closeout PR (`08.13`).

---

## 9. Assumptions

- **A1.** Phase 08 does **not** add an `outbox_messages` table or Kafka producer to `libs/identity` (`identity.merchant.*` events remain deferred per `to-be.md` §15 O4).
- **A2.** `libs/identity` keeps `apps/api/internal/config` non-empty validation fail-closed for `JWT_SIGNING_KEYS` / `JWT_ACTIVE_KID` (ADR-0006), with a documented local dev key setup in `.env.example` / Compose (handled in `08.5`, not hardcoded in Go source).
- **A3.** `08.4` (Mailer) delivers `application.Mailer` in wave 3; `08.7`, `08.9`, and `08.12` rebase on `08.4` and wire its interface without modifying Compose or Mailpit config.
- **A4.** `PaymentIntent` routes in `libs/payments/infra/httpapi` keep their current phase 06 paths (`/v1/payment-intents`, `/authorize`, `/capture`) during phase 08 (`phase-08` out of scope: *"Changing what payment routes do, beyond the middleware and the route registry"*), and gain `RouteSpec` permissions (`payments.read`, `payments.write`, `payments.capture`) plus `403 merchant_not_active` for `pending`/`suspended` merchants.
