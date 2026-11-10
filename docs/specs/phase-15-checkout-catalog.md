# Phase 15: Hosted checkout, catalog and payment links

**Depends on:** 12, 14. **Closes:** G3 (checkout side). **Design:** [`to-be.md`](../design/to-be.md) decisions 23 to 25, sections 5, 8.3 and 12.4.

## Goal

Add the two redirect-based integration modes, hosted checkout sessions and payment links, backed by a product and price catalog and per-merchant branding.

## In scope

1. **Context `libs/checkout`** (`domain`, `application`, `infra`) for products, prices, checkout sessions, payment links and checkout settings. It creates and confirms payment intents through a port implemented by an adapter over the payments use cases; it never touches payments tables. New Go module in `go.work` and the root `PACKAGES`, with a `checkout-isolation` depguard rule in `configs/go/golangci.yml` (checkout may reach payments only through its port adapter in `infra/`).
2. **Catalog API:** products (create, update, archive, list), prices (create, archive, list; amount immutable).
3. **Checkout sessions API:** create (price ids and quantities, `success_url`, `cancel_url`, `client_reference_id`, optional `expires_in`, optional `locale`), get, expire, public get, public confirm. Session ids with at least 128 bits of randomness. Line items snapshotted. URL validation and `allowed_redirect_domains` (to-be section 5.2).
4. **Payment links API:** create, deactivate, list; public `POST /v1/payment-links/{id}/sessions` creates a session.
5. **Session expiry job** in the phase 12 worker: expired open sessions become `expired` and cancel their intent.
6. **Events:** `checkout.session.completed.v1` and `checkout.session.expired.v1` through the outbox.
7. **Hosted pages in apps/web:**
   - `/<locale>/pay/<session_id>`: merchant branding, line items, total, card form;
   - `/<locale>/l/<link_id>`: creates a session and redirects;
   - success and canceled states for sessions already finished or expired.

   The card form is a client component that posts the card **directly from the browser to the vault** with the merchant's publishable key, then sends only the `card_token` to the server action that confirms the session.
8. **Dashboard:** products and prices, payment links (with copy-link and a QR code), checkout settings (display name, logo URL, brand color, support email, allowed redirect domains), list of checkout sessions with status.
9. **Demo merchant:** a small page in apps/web, `/<locale>/demo/store`, clearly labeled as a demo, that plays the merchant server with a seeded secret key from server-side env: it lists the seeded products, creates a session and redirects. A `make demo-seed` target, backed by a script in `tools/scripts/` (or an `irisctl` command it calls), creates the demo merchant, products and keys.

## Out of scope

Embedded card fields on the merchant's page. Subscriptions and recurring prices. Coupons and taxes. Shipping.

## Guidelines

- The card number goes from the browser to the vault only. No Next server code, route handler or server action receives it. CSP `connect-src` allows the vault origin for that page.
- The public endpoints return display data only and are rate limited.
- The success redirect carries the session id; the merchant must still verify. Document this in the README integration guide.
- Brand color is validated as hex, logo URL as `https`, and both are rendered safely (no raw HTML, no CSS injection).
- Every new route goes into the route registry and the cross-tenant suite; public routes are listed as public with a reason.

## Acceptance criteria

1. Demo store → session → pay with `4242 4242 4242 4242` → success page; the session is `complete`, its intent `succeeded`, the completed event is in the outbox.
2. Tampering with the amount anywhere on the client has no effect: the charged amount always equals the session total.
3. A `success_url` outside the allowed domains is rejected at session creation; `javascript:` and relative URLs are rejected always.
4. Archiving a price does not change an open session that uses it; a new session with that price is rejected.
5. An expired session shows the expired state and its intent is `canceled / session_expired`.
6. A payment link opened twice creates two sessions; deactivated links answer 404.
7. Network capture in Playwright shows the card number only in the request to the vault.

## Docs to update

`AGENTS.md` (checkout context and its boundaries), README (integration guide for the three modes, demo store), `.http` files.
