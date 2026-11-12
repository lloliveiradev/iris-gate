# Security baseline

This is the security baseline of Iris Gate: the controls a payment gateway needs, how Iris Gate implements each one, and its status. A pull request that weakens an implemented control is rejected in review.

Iris Gate never runs in production and accepts only test card numbers. The controls are still built as a real gateway would need them, because demonstrating them is the point of the project.

**Status values:** `implemented` (in the code today), `phase NN` (specified in [`specs/`](specs/README.md), not built yet), `later` (in [`roadmap.md`](roadmap.md) without a spec). Do not describe a control as present before its status says so. Status as of 2026-11-10 (commit `7195707`).

## 1. Principles

1. **Defense in depth.** No layer trusts another blindly: handlers check permissions, repositories scope by merchant, and the database enforces row-level security.
2. **Least privilege.** Every service, database role, API key and key-manager policy has only the access it needs.
3. **Fail closed.** When the system cannot decide safely, it holds (`REVIEW`) or rejects, never approves.
4. **Auditable by design.** Every sensitive action records who did it (`actor_type`, `actor_id`), when, and the request id.
5. **No secrets in code.** Real values live in `.env` (gitignored) locally and in a secrets manager in a cloud deployment.

## 2. Status overview

| Area | Control | Status |
| --- | --- | --- |
| Card data | PAN never reaches apps/api or apps/web | phase 14 (today the dashboard sends an opaque `card_token` typed by hand) |
| Card data | Own vault with envelope encryption under a key manager | phase 14 |
| API keys | Argon2id hashing, secret shown once, prefix lookup | implemented |
| API keys | Key kinds (secret, restricted, publishable), roll with overlap | phase 8 |
| API keys | Key issuance and revocation not reachable over public HTTP | phase 6 (defect D8 today) |
| Users | Accounts, Argon2id passwords, JWT with rotating refresh tokens | phase 8 |
| Users | Dashboard stops using the merchant API key | phase 9 (defect D9 today) |
| Users | MFA | later |
| Authorization | Role-based permissions, route registry test | phase 8 |
| Tenant isolation | Merchant check on every by-id route | phase 6 (defect D1 today; `GET` already checks) |
| Tenant isolation | Postgres row-level security, cross-tenant test suite | phase 7 |
| Database | Separate migrator, app and worker roles per database | phase 7 |
| Database | TLS to Postgres | later (phase 21) |
| Idempotency | `Idempotency-Key` on intent creation with body hash | implemented (global scope, defect D2; per merchant in phase 6) |
| Messaging | Outbox on produce, inbox on consume | implemented |
| Messaging | Retry topics and DLQ for every consumer | phase 13 (anti-fraud DLQ for poison messages is implemented) |
| Messaging | Kafka TLS, SASL and per-topic ACLs | later |
| Service auth | Service token on the risk endpoint | phase 10 |
| Service auth | mTLS between apps/api and the vault | phase 14 |
| Audit | Append-only, hash-chained audit log | phase 8 |
| Webhooks | HMAC signatures, SSRF protection | phase 16 |
| Logging | Denylist enforced by a log-scan test | phase 14 (today the rule exists only in `AGENTS.md`) |
| Rate limiting | Login, sign-up and tokenization endpoints | phases 8 and 14 |
| Rate limiting | Per API key on the whole API | later (phase 20) |
| HTTP | Security headers and strict CSP | later (phase 20); hosted checkout CSP in phase 15 |
| Supply chain | gitleaks in pre-commit and CI | implemented |
| Supply chain | govulncheck and `yarn npm audit` blocking in CI | phase 17 |

## 3. Card data

**Rule:** apps/api and apps/web never receive, process or log a card number (PAN) or CVV.

- The hosted checkout page posts the card from the **browser directly to the vault** with the merchant's publishable key, and sends only the returned `card_token` to the server.
- The vault accepts only documented test card numbers. Anything else is rejected before any encryption or storage, without echoing the number.
- **Envelope encryption:** each PAN is encrypted with AES-256-GCM under its own data key; the data key is wrapped by a master key that never leaves the key manager (OpenBao Transit locally, AWS KMS later). Additional authenticated data binds each ciphertext to its token and merchant.
- **Fingerprints** are keyed HMACs computed by the key manager: one per merchant (returned to the merchant), one platform-wide (sent only to anti-fraud).
- **CVV** is validated and discarded. It is never stored, not even encrypted.
- **Detokenization** happens only in the acquirer adapter, over mTLS, once per single-use token, and each call is audited.
- **Key rotation** rewraps data keys without decrypting any PAN.
- In Go, the PAN is a `[]byte` zeroed after use; it never becomes a `string`, a log field, an error message, a metric label or a span attribute.
- Contracts: no OpenAPI or event schema outside the vault's tokenization request may contain `pan`, `card_number` or `cvv`.

Details: [`design/to-be.md`](design/to-be.md) section 10 and [phase 14](specs/phase-14-card-vault.md).

## 4. API keys

- **Format (implemented):** `irk_<prefix>.<secret>`; prefix 16 hex characters (8 bytes), secret 32 base64url characters (24 bytes), from `crypto/rand`.
- **Storage (implemented):** Argon2id PHC string (`m=64 MiB, t=1, p=4`); only the hash is stored; the secret is returned once. Lookup by prefix, then constant-time verification.
- **Responses (implemented):** missing or invalid key 401, revoked key 403.
- **Kinds (phase 8):** `irk_` secret, `irr_` restricted (subset of permissions), `ipk_` publishable (public, browser side, tokenization and checkout only).
- **Lifecycle (phase 8):** named keys, `last_used_at`, roll with an overlap window (default 24 h, max 7 days), revoke; creating and rolling require re-authentication; every change goes to the audit log.

## 5. Users, sessions and authorization

Phases 8 and 9. Details in [`design/to-be.md`](design/to-be.md) section 4.

- **Passwords:** Argon2id, 12 to 128 characters. Login returns the same error for unknown email and wrong password, in about the same time, and is rate limited per email and per IP.
- **Tokens:** access JWT signed with Ed25519 (`EdDSA`), 15 minutes, audience `iris-dashboard` or `iris-platform`, public keys at `/.well-known/jwks.json`. Refresh tokens are opaque, hashed, rotated on every use; reuse of a rotated token revokes the family.
- **Backend for frontend:** Next.js keeps the tokens in the encrypted `httpOnly` iron-session cookie (`SameSite=Lax`, `Secure` outside local development). Browser JavaScript never sees a token or an API key.
- **Step-up:** creating or rolling API keys and changing member roles require a password re-check within 5 minutes (`auth_time`).
- **One-time links** (email verification, invitation, password reset) are single-use, hashed and short-lived.
- **Permissions:** roles map to permissions in code; handlers check permissions; a route registry test fails if a route has no declared permission. The UI only hides controls; the API enforces.
- **Platform staff** tokens work only on `/v1/platform/*`; merchant tokens never do.

## 6. Tenant isolation

Phases 6 and 7. Details in [`design/to-be.md`](design/to-be.md) section 9.

- The merchant id comes only from the authenticated principal.
- Repository methods take the merchant id as a parameter.
- Postgres RLS (`FORCE ROW LEVEL SECURITY`) on merchant-scoped tables, with the merchant set per transaction through `set_config('app.merchant_id', ..., true)`.
- A resource of another merchant answers 404.
- A cross-tenant test suite covers every route, and a meta-test fails when a new route is not covered.

## 7. Service-to-service authentication

| Path | Control | Status |
| --- | --- | --- |
| apps/api → anti-fraud risk endpoint | internal listener, service token, 300 ms timeout, fail closed | phase 10 |
| acquirer adapter → vault detokenize | internal listener, mTLS with a local dev CA, client identity checked | phase 14 |
| vault → OpenBao | AppRole (or token) with a policy limited to its Transit keys and operations; never the root token | phase 14 |
| services → Kafka | TLS, SASL/SCRAM and per-topic ACLs | later |

## 8. Idempotency

| Layer | Mechanism | Status |
| --- | --- | --- |
| HTTP | `Idempotency-Key` on routes that create resources; records keyed by `(merchant_id, scope, key)`, body hash checked on replay | implemented for intent creation, per-merchant scope in phase 6, more routes in phases 11 and 15 |
| Acquirer | attempt id as the acquirer idempotency key | phase 11 |
| Kafka consumers | inbox keyed by event id before any side effect | implemented |
| Webhooks | every delivery carries the event id; receivers must dedupe | phase 16 |

## 9. Input validation

- Request and response types are generated from OpenAPI; handlers decode into them and reject invalid input before any use case runs.
- Kafka messages are validated against their schema; invalid messages go to the DLQ with a reason.
- SQL is parameterized only (pgx placeholders, Prisma); no string concatenation.
- URLs supplied by merchants (checkout redirects, webhook endpoints) are validated: absolute `https` (local exceptions behind a dev flag), allowed redirect domains, and no private or link-local addresses for webhooks, re-checked at send time.

## 10. Logging and personal data

| Class | Fields |
| --- | --- |
| Safe | `request_id`, `trace_id`, `merchant_id`, `payment_intent_id`, status, latency, error code, HTTP status |
| Never | PAN, CVV, `card_token`, API key secrets, passwords, JWTs, refresh tokens, webhook secrets, raw email, raw document, raw IP |
| Hashed when needed | email and IP (SHA-256 with a salt) |

- Go uses `slog`; Nest uses its logger. A redacting handler and a log-scan test that runs the payment flows with every test card and fails on any card number in the logs arrive in phase 14.
- Request logs and webhook delivery logs store metadata only, never bodies (phase 16).

## 11. Secrets

| Environment | Where |
| --- | --- |
| Local | `.env` files, gitignored; `.env.example` documents each variable with a placeholder |
| CI | GitHub Actions secrets |
| Cloud (later) | AWS Secrets Manager and IAM database authentication; KMS for key material |

The vault's master key and the webhook-secret wrapping key live only in the key manager. JWT signing keys come from env locally and support rotation by `kid`. gitleaks runs in pre-commit and CI.

## 12. HTTP hardening

Later (phase 20), except where noted.

| Header | Value |
| --- | --- |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` (when served over TLS) |
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `geolocation=(), microphone=(), camera=()` |
| `Content-Security-Policy` | strict, no `unsafe-inline`; the hosted checkout gets its own policy with the vault in `connect-src` in phase 15 |

CORS uses an explicit allowlist; `*` is never allowed. The vault's tokenization endpoint allows only the web origin.

## 13. Database

- One database per service: `iris`, `antifraud`, `vault`. App roles cannot connect to another service's database (phase 7, vault in phase 14).
- Migrator roles own the schema; app roles have DML only and no `BYPASSRLS`; worker and platform roles are separate (phase 7).
- Append-only tables (`audit_events`, `payment_intent_events`) grant `INSERT` and `SELECT` only.
- TLS, backups, point-in-time recovery and encryption at rest belong to the cloud deployment (later).

## 14. Supply chain

- Lockfiles committed; CI installs with `--immutable`.
- New dependencies and Docker images pinned to exact versions.
- gitleaks (implemented); govulncheck and `yarn npm audit` blocking on high severity (phase 17).
- Distroless runtime image for apps/api (implemented).

### Advisory baseline (2026-11-12)

`yarn npm audit` at the repo root only inspects the root workspace's direct dependencies and reports no suggestions. `yarn npm audit --all --recursive` reports the three advisories below. Re-checked on 2026-11-12. Each patched release sits outside the range declared by the current Jest 30.5.2 and ESLint 10.14.0 (the latest releases of those majors), so no compatible fix was applied. Both tools are devDependencies. This baseline is the input for the blocking audit in phase 17. It does not change the status of the controls above.

| Package | Advisory | Severity | Installed | Why it stays |
| --- | --- | --- | --- | --- |
| `deepmerge` | [GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4) (`mergeObject` prototype poisoning) | low | 4.3.1 | `jest-config@30.5.2` requires `^4.3.1`. 4.3.1 is the latest 4.x release and is still inside `>=4.2.0 <=4.3.1`. |
| `glob` | [GHSA-3g3m-2fjr-6pw4](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4) (AbortSignal cancellation bypass) | moderate | 10.5.0, 13.0.7 | The patched line starts at 14.0.2. Jest 30.5.2 requires `^13.0.6` (latest in range: 13.0.7, via `jest-config` and `@jest/reporters`). `test-exclude@7.0.2` requires `^10.4.1` (latest in range: 10.5.0). |
| `minimatch` | [GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249) (globparts denial of service) | moderate | 9.0.9, 10.2.6 | The patched line starts at 11.0.1. ESLint 10.14.0 and `typescript-eslint` require `^10.2.x` (installed 10.2.6). `glob@10.5.0` requires `^9.0.4` (installed 9.0.9). `@nestjs/cli` already depends on 11.0.5, which is outside the advisory. |

## 15. Threat model (STRIDE)

| Threat | Example | Mitigations |
| --- | --- | --- |
| Spoofing | stolen API key or session | Argon2id hashes, scopes, roll and revoke, short-lived JWTs, refresh reuse detection, step-up for sensitive actions |
| Tampering | altered amount at checkout, altered card ciphertext, forged webhook | amounts from the server-side session, AAD-bound AES-GCM, HMAC webhook signatures with timestamps |
| Repudiation | a merchant denies approving a review or issuing a refund | actor on every event, hash-chained audit log |
| Information disclosure | another merchant's data, PAN in logs | merchant from the principal, RLS, 404 policy, vault isolation, log denylist and log-scan test |
| Denial of service | login brute force, request bursts, slow webhook endpoints | rate limits, timeouts, bounded webhook concurrency, risk-call budget |
| Elevation of privilege | a `viewer` calling a capture route, a dashboard token on platform routes | permission checks on every route, registry test, token audiences, cross-tenant suite |

## 16. Pull request checklist

- [ ] No new log line or error message contains a field from the "never" list.
- [ ] New routes declare a permission (or are listed as public with a reason) and are in the cross-tenant suite.
- [ ] New merchant-scoped tables have `merchant_id` and an RLS policy.
- [ ] Routes that create resources accept `Idempotency-Key`.
- [ ] Mutations record their actor.
- [ ] Request and response types come from `libs/contracts`.
- [ ] New consumers use the inbox and the retry/DLQ topics; new producers use the outbox.
- [ ] Migrations grant only what the role needs.
- [ ] New dependencies are pinned and pass the vulnerability checks.
