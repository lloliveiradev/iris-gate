# Phase 12: Background jobs

**Depends on:** 11. **Design:** [`to-be.md`](../design/to-be.md) sections 6.1, 6.3 and 9.2.

## Goal

Run the time-driven parts of the lifecycle safely: authorization expiry, review deadline, and reconciliation of payments stuck in `processing`.

## In scope

1. A worker process, `apps/api/cmd/worker`, using the `iris_worker` role. The outbox relay moves into it if it is not already separate. Each job has its own interval in config.
2. **Authorization expiry:** intents in `requires_capture` or `requires_review` past `expires_at` are voided and set `canceled / authorization_expired`.
3. **Review timeout:** open reviews past `due_at` are voided, the review becomes `expired`, the intent `canceled / review_expired`.
4. **Reconciliation:** intents in `processing` for more than N minutes (config, default 2) call `Acquirer.Lookup(attempt_id)` and finish the transition the lookup reports; if the acquirer has no record, the intent fails `acquirer_unavailable`.
5. Every action is recorded with `actor_type = system` and the job name as `actor_id`, with events and outbox rows like any other transition.

## Out of scope

Checkout session expiry (phase 15), vault rewrap (phase 14), webhook dispatcher (phase 16).

## Guidelines

- Claim work with `SELECT ... FOR UPDATE SKIP LOCKED` in small batches, so two workers can run at once without doing the same item twice.
- Each item is its own transaction; one failure does not stop the batch.
- Jobs reuse the use cases (`ExpireAuthorizations`, `ExpireReviews`, `ReconcileProcessing`); they contain no business rules of their own.
- Use an injectable clock so tests can move time.
- A void that fails leaves the item for the next run and logs it; after M failures (config) it is logged at error level with the intent id.

## Acceptance criteria

1. An authorization past its TTL is voided once, even with two workers running.
2. A review past its deadline ends `expired` and `canceled / review_expired`, and the acquirer saw one void.
3. `tok_test_timeout` leaves `processing`; after the threshold the reconciliation finishes it according to the fake acquirer's lookup.
4. A review resolved by a user at the same moment the timeout job runs ends in exactly one final state (version check), and the loser does nothing.
5. Worker starts with `make -C apps/api dev-worker` (or the equivalent target) and in Docker Compose.

## Docs to update

`AGENTS.md` (worker process and role), README (how to run the worker), job intervals in `.env.example`.
