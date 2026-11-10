# Phase 08: Identity and access API

**Depends on:** 07. **Design:** [`to-be.md`](../design/to-be.md) decisions 12 to 19, sections 4, 8.1, 11.2 and 12.1 to 12.2.

## Goal

Give Iris Gate real users, sessions, roles and permissions in the Go identity context, separate people from API keys, and let platform admins onboard merchants, all through the API. The web screens are phase 09.

## In scope

1. **Schema:** the identity tables of to-be section 8.1, including `audit_events`. Existing `api_keys` rows become `kind = secret`, `name = 'legacy'`.
2. **Users and passwords:** sign-up, email verification, login, logout, password reset, password change. Argon2id with at least the OWASP minimum parameters (reuse the existing hasher).
3. **Tokens:**
   - access JWT, `EdDSA` (Ed25519), 15 minutes, claims per to-be section 4.2, with `kid`;
   - JWKS at `GET /.well-known/jwks.json`;
   - signing keys from env (a key set with an active `kid`), rotation without invalidating live tokens;
   - opaque refresh tokens, hashed, rotated on every use, family revoked on reuse;
   - `auth_time` claim; `POST /v1/auth/reauthenticate` refreshes it. Routes marked "step-up" require `auth_time` within 5 minutes.
4. **Principal middleware** that accepts a secret, restricted or publishable API key or a JWT, checks the audience per route group, and builds `Principal` (to-be section 4.1). It rejects `pending` and `suspended` merchants on payment routes with 403 `merchant_not_active` (settings, members, keys and `/v1/me` stay open to `pending`).
5. **RBAC:** roles and the permission matrix of to-be section 4.3 as code in `libs/identity/domain`. A **route registry**: every mounted route declares its permission, `public`, or `platform:<permission>`. A test fails for any route without a declaration.
6. **Memberships** (many merchants per user), `switch-merchant`, exactly one owner per merchant, ownership transfer by the owner only.
7. **Invitations:** create, list, revoke, accept; single-use hashed tokens, 72 h.
8. **API key kinds:** `secret` (`irk_` prefix, existing format), `restricted` (`irr_`, scopes from the permission list), `publishable` (`ipk_`, stored in clear because it is public). Name, roll with overlap (default 24 h, max 7 days), revoke, `last_used_at`. Creating and rolling need step-up.
9. **Platform:** `platform_staff`, `/v1/platform/merchants` (list, create with owner invitation, approve, suspend, reactivate). `irisctl bootstrap-admin --email` (and `make bootstrap-admin EMAIL=...`) creates the first platform admin and prints a one-time password-setup link.
10. **Mail:** a `Mailer` port with an SMTP adapter; Mailpit added to Docker Compose (pinned image) and its UI port documented. Email templates are plain text plus simple HTML, in English.
11. **Audit:** `audit_events` rows for key create, roll, revoke; member invite, role change, removal; merchant create, approve, suspend, reactivate; staff changes. Same transaction as the change. `prev_hash`/`hash` chain, appended under a transaction-scoped advisory lock so the chain stays linear.
12. **Actor** value object in `libs/shared` (`type`, `id`, `request_id`), produced by `Principal.Actor()`.
13. **Rate limits** on signup, login, reset and invitation accept, per email and per IP, behind an interface (in-memory implementation now).

## Out of scope

- Web pages (phase 09). MFA. `platform_support`. Impersonation.
- Changing what payment routes do, beyond the middleware and the route registry.

## Guidelines

- Login answers the same error and takes about the same time for unknown email and wrong password.
- Do not put permissions in the JWT; resolve them from the role per request.
- Tokens issued for `iris-dashboard` must fail on `/v1/platform/*` and vice versa.
- A disabled user, a removed membership or a suspended merchant must stop working within one access-token lifetime; refresh must fail immediately.
- Refresh races: two parallel refreshes with the same token are normal for a browser. Design how the server tells a benign race from a stolen token (for example a short grace window after rotation), test it, and record the choice in an ADR. Phase 09 depends on this.
- Never return a secret twice. Never log a token, a key secret, a password or an email in clear.
- Secret keys get every merchant permission except `members.manage`, `settings.write` and `merchant.profile.write`.

## Acceptance criteria

1. Sign-up → verification email in Mailpit → verify → login works; the merchant is `pending` and payment routes answer 403 `merchant_not_active` until a platform admin approves it.
2. Platform admin creates a merchant → owner receives an invitation → accepts → is owner.
3. For every role in the matrix, each permission is allowed or denied exactly as the table says (table-driven test generated from the matrix).
4. A restricted key with only `payments.read` cannot create an intent (403). A publishable key cannot call any merchant API except the ones phase 14 and 15 will add.
5. Refresh rotation works; a reused rotated token (outside the benign-race rule) revokes the family; logout revokes it; password change revokes all families.
6. A dashboard token is rejected on platform routes and the reverse.
7. Creating an API key without a recent `auth_time` answers 403 `reauthentication_required`.
8. Every route is in the registry; the registry test and the cross-tenant suite pass with the new routes.
9. The audit chain verifies (a test recomputes the hashes) and an `UPDATE` on `audit_events` with `iris_app` fails.

## Docs to update

`AGENTS.md` identity section (token shapes, principal, registry, audit), `docs/security.md`, README (bootstrap admin, Mailpit), ADRs for the refresh-race rule and the signing-key format.
