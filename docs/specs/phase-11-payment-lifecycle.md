# Phase 11: Payment lifecycle and review

**Depends on:** 09, 10. **Closes:** D4 (fully), D5, G2, G4, G5. **Design:** [`to-be.md`](../design/to-be.md) decisions 1 to 11, sections 6, 8.2, 11.1, 12.3 and 13.

## Goal

Replace the manual authorize-then-capture flow with the market lifecycle: confirm scores the risk, authorizes and, by default, captures automatically. Add manual capture as a setting, the review flow in both review modes, cancel, refunds, payment history with actors, and the dashboard and console screens for them.

## In scope

1. **ADR** "Change the v1 payments contract in place" (resolution O1), written first.
2. **Domain:** statuses, transitions, failure codes and cancellation reasons of to-be section 6.1, with the methods of section 11.1. Each method returns domain events. Illegal transitions return typed errors.
3. **Settings:** `payment_settings` with `capture_method`, `authorization_ttl_hours`, `review_mode`, `review_timeout_hours`. Validation: TTL 1 to 168 h; review timeout 1 h to (TTL - 1 h). An intent copies `capture_method` (or the per-intent override) and the review mode at creation; later setting changes do not affect it.
4. **Confirm use case:** persist `processing` (conditional on version) → `RiskAssessor` → `Acquirer.Authorize` → `Capture` when automatic → final state, attempts, events and outbox in one transaction. `REJECT` fails without calling the acquirer. `REVIEW` authorizes and opens a review with `due_at`.
5. **Acquirer port** with `Authorize`, `Capture`, `Void`, `Refund`, `Lookup`, and `payment_attempts`; the attempt id is the acquirer idempotency key. The fake acquirer decides by magic test token (to-be section 13) and later by card number (phase 14). `tok_test_timeout` leaves the attempt `unknown` and the intent `processing` for phase 12 to reconcile. Payments sends `test_scenario = blocked` or `review` in the risk request for `tok_test_blocked` and `tok_test_review` (phase 10 test hooks).
6. **Review:** `POST /v1/payment-intents/{id}/review` for `review_mode = merchant`, and `/v1/platform/reviews` for `platform`. Approve captures (or moves to `requires_capture`); reject voids. A review can be resolved once.
7. **Capture, cancel, refunds** routes with their permissions. Refunds: partial or full, sum never above `amount_captured`, `Idempotency-Key` required.
8. **History:** `payment_intent_events` with actor for every transition, and `GET /v1/payment-intents/{id}/events`.
9. **Contract:** update the payments OpenAPI (remove `/authorize`, add the routes of section 12.3) and the AsyncAPI topics of section 7.1 that payments produces. Message keys become `payment_intent_id`.
10. **Data migration:** `pending → requires_payment_method`, `authorized → requires_capture`, `captured → succeeded`, `failed` stays. Tested with rows in the old shape.
11. **Dashboard:** remove the Authorize button; list with status filters; detail with timeline (actor shown as user name, key name, "Iris Gate risk team" or system job); capture, cancel and refund actions gated by permission; review queue with approve and reject (note required on reject); payment settings page. The "new intent" form selects a test scenario (magic token) instead of a typed card token.
12. **Console:** platform review queue for merchants in `platform` mode; `GET /v1/platform/metrics` and a page with volume, approval rate, fraud-reject rate and open reviews per merchant.

## Out of scope

- Expiry, review timeout and reconciliation jobs (phase 12). Consumers of the new events (phase 13). Vault (phase 14). Partial capture, 3-D Secure.

## Guidelines

- `processing` is committed **before** the acquirer call. Never hold a database transaction open across the risk or acquirer call.
- Risk unavailable means `REVIEW`, never `APPROVE`.
- Capture failure after a successful authorization leaves the intent in `requires_capture` with a failed capture attempt and an event; it does not fail the payment.
- Review rejection and cancel void the authorization through the acquirer before the state change is committed as `canceled`; a void failure keeps the intent where it was and returns 502 `acquirer_unavailable`.
- Every new route goes into the route registry and the cross-tenant suite.

## Acceptance criteria

1. `tok_test_approved` with automatic capture ends `succeeded` in one confirm call, with authorize and capture attempts and `authorized` and `captured` outbox rows.
2. With manual capture it ends `requires_capture`; capture moves it to `succeeded`; capture twice is idempotent or 409, never two acquirer captures.
3. `tok_test_review` ends `requires_review`; approve by a `risk_analyst` captures; reject by another user voids; a `viewer` gets 403; in `platform` mode a merchant user gets 403 and a platform admin can resolve it.
4. `tok_test_blocked` ends `failed / fraud_rejected` with no acquirer call; `tok_test_declined` ends `failed / card_declined`.
5. Risk service down: the payment ends `requires_review` with `RISK_UNAVAILABLE`.
6. Two concurrent confirms: one acquirer authorization.
7. Refunds above the captured amount are rejected; partial refunds sum correctly.
8. Every transition has one event row with the right actor type.
9. Dashboard and console flows above work in Playwright for one happy path and one review path.

## Docs to update

`AGENTS.md` payments section (new transitions, routes, events), README API examples and `.http` files, the ADR from item 1.
