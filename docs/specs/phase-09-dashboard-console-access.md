# Phase 09: Dashboard and console access

**Depends on:** 08. **Closes:** D9. **Design:** [`to-be.md`](../design/to-be.md) sections 4.2 to 4.4.

## Goal

Replace the API-key login of the dashboard with user accounts, and add the screens for team, API keys, audit log and the platform console, using the phase 08 API.

## In scope

1. **Session in the BFF.** iron-session stores the access token, refresh token, access expiry and active merchant, nothing else. Remove the stored API key and every code path that used it (D9).
2. **Token refresh** on the server, before calling the API, with single-flight per session so parallel server components and actions do not refresh twice. It must work with the refresh-race rule chosen in phase 08.
3. **Pages** (all under the locale prefix, copy in `en`, `pt`, `es`):
   - sign-up, verify email, login, logout, forgot and reset password, accept invitation;
   - merchant switcher (users with several memberships);
   - pending-merchant banner explaining that payments unlock after approval;
   - team: list members and invitations, invite, change role (step-up), remove;
   - API keys: list with kind, name, prefix, created, last used, status; create (step-up) with the secret shown once and a copy button; roll; revoke; the publishable key always visible;
   - audit log (read-only, filterable by action);
   - profile: name, change password.
4. **Platform console** at `/<locale>/admin`: platform login (audience `iris-platform`), merchant list with status filter, create merchant, approve, suspend, reactivate.
5. **Permission-aware UI:** read the permissions from `/v1/me`; hide or disable what the principal cannot do. The API still enforces everything.
6. **Step-up flow:** when the API answers `reauthentication_required`, ask for the password in a dialog, call reauthenticate, and retry the action.

## Out of scope

- Payment screens changes (phase 11). Visual redesign (phase 18): use the current components and tokens.

## Guidelines

- Tokens never reach client components, `localStorage`, URLs or logs.
- Server actions keep Next's origin checks; do not add routes that accept cross-site form posts.
- A 401 from the API after a failed refresh clears the session and redirects to login with the original path in a validated `next` parameter (same-origin paths only).
- Do not duplicate the permission matrix in the web app. Use the permissions the API returns.

## Acceptance criteria

1. No code in apps/web reads or stores an API key secret for authentication. `grep` for the old session field finds nothing.
2. Playwright: sign up, verify through Mailpit's API, see the pending banner; platform admin approves in `/admin`; the user creates a secret key and sees it exactly once.
3. Playwright: owner invites a `viewer`, who accepts and cannot see the invite or key-creation controls; a direct server-action call by the viewer is refused by the API.
4. Opening two tabs and letting the access token expire does not log the user out.
5. `/en`, `/pt` and `/es` render every new page; `make -C apps/web typecheck` and `make -C apps/web test` pass.

## Docs to update

`AGENTS.md` (session bullets), README (how to sign in locally, Mailpit URL, bootstrap admin), `apps/web` docs if any describe the old login.
