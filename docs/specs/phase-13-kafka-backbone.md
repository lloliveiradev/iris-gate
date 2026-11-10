# Phase 13: Kafka event backbone

**Depends on:** 11. **Closes:** G6, G7. **Design:** [`to-be.md`](../design/to-be.md) section 7.

## Goal

Make Kafka the reliable carrier of payment facts: complete event set, correct keys, schema compatibility, consumers that dedupe and retry, and the anti-fraud consumer that turns outcomes into features and labels.

## In scope

1. **Contracts:** every topic of to-be section 7.1 in AsyncAPI with JSON Schema, registered in Karapace with `BACKWARD` compatibility. A CI check fails on an incompatible change.
2. **Producers:** payments (and later checkout) publish through the outbox only; key = aggregate id; headers `event-id`, `event-type`, `occurred-at`, `request-id`, `traceparent` when present. Payloads carry `merchant_id`, `actor_type`, `actor_id`.
3. **Retry and DLQ** for every consumer, Go and Nest: transient errors go to `<topic>.retry.<n>` with backoff (1 s, 10 s, 1 min, configurable), then `<topic>.dlq` with `failure-reason`, `original-topic`, `original-offset` headers. Poison messages (schema or parse errors) go straight to the DLQ. A `make dlq-replay TOPIC=...` target, backed by a script in `tools/scripts/`, re-publishes DLQ messages to the source topic.
4. **Anti-fraud `PaymentOutcomeConsumer`:** inbox dedupe, then updates `payment_outcomes` and `card_profiles` (attempts and declines in 24 h windows, average captured amount), and `review_labels` from `review_resolved`, review expiry and refunds.
5. **Topic provisioning:** `infra/docker/kafka/topics.txt` and `create-topics.sh` (run by `kafka-init`) already exist. Add every new topic, retry and DLQ topic there, extend the format if per-topic retention is needed, and pin the Kafka image instead of `latest`.

## Out of scope

Webhooks consumer (phase 16), ledger, analytics consumers.

## Guidelines

- Kafka never sits in a request path. No producer call from a use case.
- A consumer commits its offset only after its transaction (inbox row plus side effects) commits.
- Feature updates must be order-tolerant: an out-of-order `captured` before `authorized` for the same intent must not corrupt the profile (the key keeps per-intent order, but write the update so a replay is harmless).
- Keep per-merchant data out of message keys.

## Acceptance criteria

1. Replaying the same event twice changes nothing the second time.
2. A consumer that fails transiently three times processes the message from the retry topic; a poison message lands in the DLQ with the headers above; `dlq-replay` brings it back.
3. Five declines on one fingerprint raise `declines_24h` to 5, and the next risk assessment for that card sees it.
4. An incompatible schema change fails the compatibility check.
5. A review approved in the dashboard becomes an `approved` label in `review_labels`.

## Docs to update

`AGENTS.md` fraud and payments event bullets, ADR-0002 addendum for retry/DLQ, README (topics, DLQ replay).
