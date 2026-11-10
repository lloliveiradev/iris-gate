# Phase 10: Synchronous risk scoring

**Depends on:** 06 (can run in parallel with 07 to 09). **Closes:** D6, G1 (anti-fraud side). **Design:** [`to-be.md`](../design/to-be.md) decisions 3 and 8, sections 7, 8.6 and 11.3.

## Goal

Turn the anti-fraud service into a synchronous scoring API that payments can call inside confirmation, and give payments the client for it. Wiring the client into confirm is phase 11.

## In scope

1. **Contract:** `libs/contracts/http/fraud-internal.openapi.yaml` for `POST /internal/v1/risk-assessments`. Request: `payment_intent_id`, `merchant_id`, `amount_cents`, `currency`, `card_fingerprint`, `card_brand`, `customer_ref` (optional), `test_scenario` (optional). Response: `assessment_id`, `decision`, `score`, `reasons`, `latency_ms`.
2. **Endpoint in apps/anti-fraud:**
   - idempotent by `payment_intent_id`: a repeat returns the stored assessment;
   - builds one `FeatureSnapshot` for the card and customer in a single query;
   - runs the specifications: `BlockedCardSpec`, `UnusualAmountSpec`, `VelocitySpec`, `DeclineBurstSpec`, with the existing weights and thresholds where they apply;
   - in one transaction: stores the assessment (with the feature snapshot), records an `attempted` row in `payment_outcomes`, and writes `fraud.assessment.completed.v1` to the outbox.
3. **Service authentication:** the internal endpoint listens on an internal port or path that is not exposed to browsers, and requires a service token from env. Requests without it get 401.
4. **Test hooks:** when `FRAUD_TEST_HOOKS=true` (default true in local `.env.example`, documented), `test_scenario = blocked` yields `REJECT` and `test_scenario = review` yields `REVIEW`, recorded with reason `TEST_SCENARIO`. When false, the field is ignored.
5. **Schema:** amounts become `BigInt` in Prisma (D6), with a migration. Replace `Account.isSuspicious` with `card_profiles.blocked` as in to-be section 8.6.
6. **Retire** the `fraud.assessment.requested.v1` consumer and topic from code and contracts.
7. **Go client:** `RiskAssessor` port in `libs/payments/application` and an HTTP adapter in `libs/payments/infra`, with a 300 ms total timeout and no retries. Timeout, network error, non-2xx or invalid body all return a `REVIEW` snapshot with reason `RISK_UNAVAILABLE` (fail closed), and log the cause without card data.

## Out of scope

- Calling the client from confirm (phase 11). The outcome consumer and labels (phase 13).

## Guidelines

- Rules live in `libs/fraud` (domain), not in the Nest app.
- The scoring path must not call Kafka and must not wait on any consumer.
- `features_snapshot` stores what each rule saw, so a decision can be explained later.
- Keep p99 latency under 100 ms on the local stack; measure it in a test or benchmark and record the number in the PR.

## Acceptance criteria

1. A first-time card with a normal amount gets `APPROVE`; the same card after N attempts within the velocity window gets `REVIEW` or `REJECT` as configured; a blocked card gets `REJECT`.
2. A repeated request for the same `payment_intent_id` returns the same assessment id and does not add a second `attempted` row.
3. Amounts above 21,474,836.47 are stored and scored correctly.
4. Without the service token: 401. With `FRAUD_TEST_HOOKS=false`, `test_scenario` has no effect.
5. The Go adapter returns `REVIEW` / `RISK_UNAVAILABLE` on timeout (test with a slow fake server) and on a 500.
6. `make -C libs/fraud test`, `make -C apps/anti-fraud test` and the payments tests pass.

## Docs to update

`AGENTS.md` fraud section (sync endpoint, retired topic, test hooks), `apps/anti-fraud` README if present, AsyncAPI.
