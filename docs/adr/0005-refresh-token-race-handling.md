# ADR-0005 — Refresh-token race handling and reuse detection

- **Status:** Proposed
- **Date:** 2026-11-20
- **Deciders:** project owner, identity & web engineering
- **Related:** [`../specs/phase-08-identity-access-api.md`](../specs/phase-08-identity-access-api.md), [`../specs/phase-09-dashboard-console-access.md`](../specs/phase-09-dashboard-console-access.md), [`../design/to-be.md`](../design/to-be.md) (section 4.2)

## Context

In Iris Gate, access JWTs live for 15 minutes and refresh tokens are opaque, single-use, stored only as hashes in `refresh_tokens`, and rotated on every `POST /v1/auth/refresh` call (`to-be.md` section 4.2). Presenting an already-rotated refresh token normally indicates token theft and must revoke the entire session family (`family_id`).

In phase 09, Next.js acts as a backend-for-frontend (BFF) holding the access and refresh tokens in an encrypted `httpOnly` iron-session cookie. When the access token expires, parallel Next.js Server Component renders, concurrent Server Actions, or two open browser tabs can hit `POST /v1/auth/refresh` nearly simultaneously with the **same** refresh token `T1`.

Without benign-race handling, request B arrives milliseconds after request A rotated `T1 -> T2`, triggers reuse detection, revokes `family_id`, and logs the user out across all tabs (violating phase 08 guideline 5 and phase 09 acceptance criterion 4). Conversely, weakening rotation to allow unlimited reuse of `T1` defeats stolen-token detection.

Additionally, `docs/specs/README.md` forbids persisting plaintext refresh tokens in the database or logs, so the server cannot store `T2` in plaintext inside `refresh_tokens` to replay it to request B later.

## Decision

Handle concurrent refreshes with **three layers** so benign parallel requests succeed without storing plaintext refresh tokens in Postgres, while any replay outside the short race window revokes the entire family.

### 1. Serialized database rotation + `rotated_at` timestamp

Inside `POST /v1/auth/refresh`:

1. Hash the presented opaque token (`token_hash = SHA-256(token)`) and open a database transaction.
2. Lock the matching row with `SELECT ... FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE` so two concurrent requests with `T1` cannot both observe `replaced_by IS NULL` and mint two diverging child tokens (`T2a` and `T2b`).
3. Re-load the user, active membership (for `iris-dashboard`) or `platform_staff` (for `iris-platform`), and merchant status inside the transaction. If the user is disabled, the membership was removed, or the merchant is suspended, revoke the family and return `401` immediately.
4. **Fresh token (`replaced_by IS NULL AND revoked_at IS NULL AND expires_at > now()`):**
   - Generate `T2`, store `SHA-256(T2)` as a new row in the same `family_id`, and update `T1` with `replaced_by = T2.id` and `rotated_at = now()`.
   - Mint a fresh 15-minute EdDSA access JWT and commit.
   - Store `(access_jwt, T2)` in an **in-memory-only** grace cache keyed by `T1.id` with a TTL equal to `REFRESH_GRACE_WINDOW` (default **`10s`**). Plaintext `T2` is **never** written to Postgres or logs.

### 2. In-process single-flight and benign-race grace window

- **In-flight coalescing (`singleflight`):** When request B arrives for the same `token_hash` while request A is still executing on the same `apps/api` instance, request B waits for request A's result and returns the exact same `(access_jwt, T2)`.
- **Post-commit grace window (`now() - T1.rotated_at <= REFRESH_GRACE_WINDOW`, default `10s`, when `T1.revoked_at IS NULL`):**
  - If the in-memory grace cache still holds `(access_jwt, T2)` for `T1.id`, return HTTP `200` with `(access_jwt, T2)` and **do not revoke the family**. (Both tabs/components converge on `T2`.)
  - If the in-memory grace cache misses within the `10s` window (for example after an API restart or across two replicas without shared cache), return HTTP `409 concurrent_refresh` (retryable by the BFF) and **do not revoke the family**.

### 3. Reuse detection outside the grace window

If `T1` is presented when **any** of the following holds:

- `T1.revoked_at IS NOT NULL`, or
- `T1.replaced_by IS NOT NULL` and `now() - T1.rotated_at > REFRESH_GRACE_WINDOW`:

treat the call as **token reuse**:

1. Execute `UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL` in the transaction and evict `T1.id` from the grace cache.
2. Emit a structured security log with `user_id`, `family_id`, and `event = refresh_token_reused` (never log the raw token).
3. Return `401 invalid_token`.

### 4. BFF single-flight (phase 09, defense-in-depth)

In phase 09, `apps/web` also wraps cookie-based refreshes per session in a server-side single-flight lock so parallel Server Component calls within one Next.js process normally issue only a single `POST /v1/auth/refresh` request. The `apps/api` grace window remains authoritative for multi-instance BFFs and direct API clients.

## Alternatives considered

### A. Strict single-use with zero grace window

Every concurrent refresh after the first commit immediately revokes `family_id`. Rejected: two parallel Next.js server renders or two browser tabs waking after 15 minutes reliably trigger a false-positive logout.

### B. Persisting an encrypted copy of `T2` in Postgres for the grace window

Allows multi-replica API processes to replay `(access_jwt, T2)` from Postgres after a cache miss. Rejected for phase 08: violates the rule against persisting recoverable refresh tokens (`docs/specs/README.md`) and requires managing an extra symmetric wrapping key for short-lived session replay. Revisit with Redis/Valkey encrypted ephemeral storage only if multi-replica `apps/api` deployments see `409 concurrent_refresh` retries in practice.

### C. Allowing `T1` to mint a second valid sibling `T2b` within the grace window

Rejected: both `T2a` and `T2b` remain live in the family, so the two tabs hold different refresh tokens and whichever refreshes second still loses or forks the chain.

## Consequences

### Positive

- Parallel server renders and multi-tab wakes converge on one successor token `T2` without false-positive family revocation.
- Plaintext refresh tokens never touch disk, database tables, or logs.
- Stolen-token replay after `10s` deterministically revokes the entire session family.
- `SELECT ... FOR UPDATE` prevents split-brain token trees at the database level.

### Negative

- The grace cache is process-local: if `apps/api` restarts or scales horizontally during the `10s` window, a racing caller gets `409 concurrent_refresh` instead of `200` (safe, requires BFF retry or re-login).
- A thief racing within `10s` of a legitimate refresh on the same instance could obtain `T2` if they intercept the response; keeping `REFRESH_GRACE_WINDOW` short (`10s`) limits this window tightly.
