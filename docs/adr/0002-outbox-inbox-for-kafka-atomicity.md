# ADR-0002 — Outbox and inbox for atomicity between Postgres and Kafka

- **Status:** Accepted
- **Date:** 2026-05-09
- **Deciders:** founding team
- **Related:** [`../architecture.md`](../architecture.md), [`../legacy-audit.md`](../legacy-audit.md)

## Context

The audit of the legacy `go-gateway` found a textbook dual write. `InvoiceService.Create()` calls `repo.Save(invoice)` and then `kafkaProducer.Publish(...)` outside the transaction. The two writes can part company in three ways:

| Failure | What the system is left with |
| --- | --- |
| Save succeeds, publish fails | An invoice in the database and no event. Anti-fraud never scores it. The status stays `pending`. |
| Save fails after a publish that already happened | Anti-fraud scores an invoice that does not exist. |
| The process crashes between the two calls | The same gap as the first row. |

The audit of the legacy `anti-fraud` found the matching bug on the read side. The consumer is not idempotent. A Kafka redelivery, which happens on rebalance and on a failed offset commit, runs the handler again and inserts a second `FraudHistory` row.

A payments gateway cannot accept either outcome. State and the fact that state changed have to commit together, and a message that arrives twice has to be safe.

## Decision

Every write that must publish an event uses a transactional outbox. Every consumer uses an idempotent inbox.

### Outbox, on the producer

Each context owns a table:

```sql
CREATE TABLE outbox_events (
  id              UUID PRIMARY KEY,
  aggregate_type  TEXT NOT NULL,           -- 'PaymentIntent'
  aggregate_id    UUID NOT NULL,
  event_type      TEXT NOT NULL,           -- 'payments.intent.created.v1'
  payload         JSONB NOT NULL,
  headers         JSONB NOT NULL,          -- traceparent, idempotency-key, ...
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at    TIMESTAMPTZ,
  attempts        INT NOT NULL DEFAULT 0,
  last_error      TEXT
);
CREATE INDEX outbox_unpublished ON outbox_events (created_at) WHERE published_at IS NULL;
```

The use case inserts the aggregate and the outbox row in the same transaction. A dedicated relay (`libs/platform/outbox/relay`) polls every 200 ms, or wakes on `LISTEN/NOTIFY`, publishes to Kafka, and sets `published_at`. On failure it increments `attempts` and retries with backoff. After N attempts it alerts.

The relay can still publish twice if it crashes after the broker ack and before `published_at` is set. That is why the consumer has an inbox. The pair is at-least-once, not exactly-once.

### Inbox, on the consumer

Each consumer group keeps a table:

```sql
CREATE TABLE inbox_messages (
  message_id      TEXT NOT NULL,           -- event_id from the header
  consumer        TEXT NOT NULL,           -- 'fraud-assessment-consumer'
  processed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (message_id, consumer)
);
```

The handler wraps the message in one transaction:

```
BEGIN
  INSERT INTO inbox_messages (message_id, consumer)
    VALUES ($1, $2)
    ON CONFLICT DO NOTHING
    RETURNING 1;
  -- empty RETURNING: already processed; COMMIT and return
  -- otherwise: apply the side effect (write fraud_history, publish the result through the outbox)
COMMIT
```

Messages that fail N times go to a separate dead-letter topic, `<topic>.dlq`.

## Alternatives considered

### A. A saga orchestrator (Temporal or Cadence)

It would cover atomicity and multi-step workflows. It is also another stateful service, and a steep curve for the MVP. Deferred until multi-step workflows are common. Outbox plus inbox covers one write and one event.

### B. Native Kafka transactions

Kafka can transactionally produce across topics. Tying that to a Postgres transaction means two-phase commit. XA has a poor operational record. The outbox is simpler and does not depend on a particular broker.

### C. Change data capture (Debezium)

Reading the Postgres WAL and publishing from it is attractive, and it couples the table shape to the event shape. It is also heavy to run (Kafka Connect, initial snapshots). The events that come out describe rows, not the ubiquitous language of the domain.

### D. A dedicated event store (EventStoreDB, Marten)

The stream becomes the source of truth. That is a good fit for Ledger and too much machinery for the other contexts. Revisit it for Ledger after the MVP. Everywhere else, Postgres plus the outbox.

## Consequences

### Positive

- State and event commit together, or neither does.
- Recovery is a restart. The relay drains whatever is still unpublished.
- Redelivery is safe.
- The pattern works with Kafka, Redpanda, NATS, or RabbitMQ.

### Negative

- Polling adds up to about 200 ms of delay. `LISTEN/NOTIFY` removes that wait.
- Each context gains a table, and each app gains a worker.
- Unpublished rows have to be watched. Alert when `count(*) where published_at IS NULL` crosses a threshold.

### Metrics to expose

- `outbox_unpublished_count` (gauge)
- `outbox_publish_latency_ms` (histogram)
- `inbox_dedupe_hits_total` (counter; a rising rate means redelivery is frequent)
- `dlq_messages_total` (counter, labeled by topic)
