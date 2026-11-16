-- 0004_payment_intent_version_and_failure.sql: persisted declines (D3) and
-- optimistic locking (D4) for payment_intents.
--
-- version: every update is conditional on (id, merchant_id, version) and
-- increments it, so two requests that loaded the same version cannot both
-- write. Existing rows start at 0.
--
-- failure_code: why an intent is failed (for example 'card_declined'). It may
-- only be set on a failed intent. Existing rows keep NULL. Not destructive.

ALTER TABLE payment_intents
    ADD COLUMN version INT NOT NULL DEFAULT 0 CHECK (version >= 0);

ALTER TABLE payment_intents
    ADD COLUMN failure_code TEXT;

ALTER TABLE payment_intents
    ADD CONSTRAINT payment_intents_failure_code_only_when_failed
    CHECK (failure_code IS NULL OR status = 'failed');
