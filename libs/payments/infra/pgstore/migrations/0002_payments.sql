-- 0002_payments.sql — Payments bounded context (Phase 3)
--
-- Tabelas:
--   payment_intents       — agregado principal
--   outbox_messages       — Outbox pattern (ADR-0002)
--   idempotency_records   — middleware HTTP de idempotência
--
-- FK em merchants exige que a migration 0001 de Identity já tenha rodado.

CREATE TABLE payment_intents (
    id            UUID        PRIMARY KEY,
    merchant_id   UUID        NOT NULL REFERENCES merchants (id) ON DELETE RESTRICT,
    amount_cents  BIGINT      NOT NULL CHECK (amount_cents > 0),
    currency      CHAR(3)     NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status        TEXT        NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending','authorized','captured','refunded','canceled','failed')),
    card_token    TEXT        NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    authorized_at TIMESTAMPTZ,
    captured_at   TIMESTAMPTZ
);

CREATE INDEX payment_intents_merchant_idx ON payment_intents (merchant_id);
CREATE INDEX payment_intents_status_idx   ON payment_intents (status);

-- Outbox pattern: o relay worker consome `WHERE published_at IS NULL` em ordem
-- e marca published_at quando confirma com o broker.
CREATE TABLE outbox_messages (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    topic        TEXT        NOT NULL,
    msg_key      TEXT,
    payload      JSONB       NOT NULL,
    headers      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX outbox_unpublished_idx ON outbox_messages (created_at)
  WHERE published_at IS NULL;

-- Idempotência por (scope, key) — scope distingue endpoints (ex.:
-- 'create_payment_intent') pra evitar colisão entre rotas que aceitam o mesmo
-- header.
CREATE TABLE idempotency_records (
    scope          TEXT        NOT NULL,
    key            TEXT        NOT NULL,
    request_hash   BYTEA       NOT NULL,
    response_status INT        NOT NULL,
    response_body  BYTEA       NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, key)
);

CREATE INDEX idempotency_records_created_idx ON idempotency_records (created_at);
