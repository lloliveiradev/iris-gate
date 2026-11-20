# ADR-0006 — Signing-key custody and rotation for EdDSA/JWKS

- **Status:** Proposed
- **Date:** 2026-11-20
- **Deciders:** project owner, identity & security engineering
- **Related:** [`../specs/phase-08-identity-access-api.md`](../specs/phase-08-identity-access-api.md), [`../security.md`](../security.md) (sections 5, 11), [`../design/to-be.md`](../design/to-be.md) (section 4.2)

## Context

Phase 08 replaces API-key-only dashboard authentication with 15-minute access JWTs signed with Ed25519 (`EdDSA`), carrying a `kid` header and verified against public keys exposed at `GET /.well-known/jwks.json`.

`docs/security.md` section 11 states that JWT signing keys come from the environment locally (secrets manager in cloud deployments) and support rotation by `kid` without invalidating live tokens. Phase 08 needs a concrete key-set representation, loading contract, rotation procedure, and fail-closed startup/verification behavior so keys never enter Postgres, source control, or logs, and a missing or expired key never falls back to an insecure default.

## Decision

### 1. Key custody and environment format

Signing keys live **only** in process environment variables (`apps/api`), injected from gitignored `.env` locally, CI secrets in tests/pipelines, or AWS Secrets Manager in cloud deployments. Keys are never stored in Postgres, `libs/contracts`, or source code.

Two environment variables configure the key set:

- `JWT_SIGNING_KEYS`: JSON array of Ed25519 private key entries:
  ```json
  [
    {
      "kid": "iris-2026-11-a",
      "private_key_b64": "<base64url raw 64-byte Ed25519 private key or PKCS#8 DER>",
      "not_after": "2027-02-20T00:00:00Z"
    }
  ]
  ```
  `not_after` is optional (RFC 3339 UTC timestamp); when set, keys past `not_after` are rejected for signing and, after overlap, for verification.
- `JWT_ACTIVE_KID`: the `kid` of the single key used to sign newly issued access JWTs.

A single JSON-encoded env var (`JWT_SIGNING_KEYS`) is used instead of mounted PEM files or indexed env vars (`JWT_KEY_1`, `JWT_KEY_2`) so rotation is atomic in `.env`, Docker Compose, and AWS Secrets Manager without filesystem mounts.

### 2. Startup loading and fail-closed validation

At startup (`apps/api` config/token-issuer initialization):

1. Parse `JWT_SIGNING_KEYS` and `JWT_ACTIVE_KID`.
2. Reject startup immediately (non-zero exit, **never** falling back to an embedded or randomly generated ephemeral key in `apps/api`) if:
   - `JWT_SIGNING_KEYS` is empty or invalid JSON,
   - any entry has an empty or duplicate `kid`, invalid base64, or non-Ed25519 key material,
   - `JWT_ACTIVE_KID` is empty, missing from `JWT_SIGNING_KEYS`, or its `not_after` is `<= now()`.
3. Derive the public JWK for each non-expired entry (`kty: "OKP"`, `crv: "Ed25519"`, `alg: "EdDSA"`, `use: "sig"`, `kid`, `x`) and hold it in memory to serve `GET /.well-known/jwks.json`. Private key bytes (`d` / `private_key_b64`) are never serialized into JWKS, HTTP errors, or logs.

Unit tests in `libs/identity` construct in-memory `KeySet` values directly with `crypto/ed25519.GenerateKey` rather than reading global env vars.

### 3. Verification rules (fail closed)

Every JWT verification:

1. Requires header `alg == "EdDSA"` strictly (rejects `none`, `HS256`, `RS256`, and embedded `jwk`/`jku` headers).
2. Requires a non-empty `kid` header matching an entry in the loaded `KeySet` whose `not_after` (if set) has not passed. Never tries all keys blindly when `kid` is missing or unknown.
3. Verifies signature, `exp` (`<= 15m` TTL), `nbf`/`iat`, and exact `aud` (`iris-dashboard` or `iris-platform`).
4. Any missing key, unknown `kid`, expired key, or signature failure returns `401 invalid_token`.

### 4. Rotation and overlap window

Because access JWTs have a 15-minute lifetime, a safe zero-downtime rotation uses three steps with an overlap window of at least **30 minutes** (15-minute max token TTL + 15-minute buffer for JWKS `Cache-Control` and clock skew):

1. **Publish new public key:** Add `K2` to `JWT_SIGNING_KEYS` while keeping `JWT_ACTIVE_KID = K1`. Restart/roll `apps/api`. `GET /.well-known/jwks.json` now serves `[K1, K2]` (`Cache-Control: public, max-age=300`) while tokens are still signed with `K1`.
2. **Switch active signer:** Change `JWT_ACTIVE_KID = K2` (keeping `K1` in `JWT_SIGNING_KEYS`). New tokens carry `kid: K2`; live tokens signed with `K1` continue to verify during their remaining `<= 15m` lifetime.
3. **Retire old key:** Once at least `30m` have elapsed after step 2 (or after `K1`'s `not_after`), remove `K1` from `JWT_SIGNING_KEYS`.

## Alternatives considered

### A. Single symmetric secret (`HS256`) or single Ed25519 key without `kid`

Simpler env config (`JWT_SECRET`). Rejected: `HS256` shares the signing secret with every verifier and prevents exposing public-only keys via `/.well-known/jwks.json`; omitting `kid` forces every rotation to either invalidate all active sessions or try multiple keys per request.

### B. PEM files mounted from disk

Requires mounting a secrets directory in Compose, CI, and container orchestrators. Rejected: harder to configure in `.env` and `apps/api` tests than a single JSON env var, and offers no security advantage locally over env injection.

### C. Signing JWTs inside OpenBao / AWS KMS

Eliminates private key bytes from `apps/api` memory. Deferred: OpenBao is not introduced until phase 14 (card vault), and adding a network KMS round-trip on every login/refresh/switch-merchant is unnecessary for 15-minute access tokens in a modular monolith. The `TokenIssuer` port keeps this migration path open later without changing use cases.

## Consequences

### Positive

- Zero-downtime rotation without invalidating live 15-minute sessions.
- Public verification keys are cleanly exposed via standard RFC 7517 JWKS (`/.well-known/jwks.json`) for `apps/web` or future services (`apps/vault`) without sharing private keys.
- Missing, misconfigured, or expired signing keys fail closed at startup and verification time.

### Negative

- Key rotation requires two config reloads (add `K2`, then switch `JWT_ACTIVE_KID`) separated by the JWKS cache window (`~5m`), plus waiting `>= 15m` before removing `K1`.
- Local developers and CI must generate/provide a dev `JWT_SIGNING_KEYS` entry in `.env` (a `make` helper or `.env.example` placeholder can be documented, never treated as a production key).
