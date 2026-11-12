-- 0003_idempotency_per_merchant.sql: idempotency keys become per merchant (D2).
--
-- Before this migration the key was (scope, key), so two merchants sending the
-- same Idempotency-Key collided: one could replay or conflict with the other's
-- stored response. The key is now (merchant_id, scope, key).
--
-- DESTRUCTIVE: every existing idempotency_records row is deleted. Old rows
-- carry no merchant id and cannot be attributed to a tenant. The cost is that a
-- client retrying a request made before this migration is executed again
-- instead of replayed; the records only hold replay data, not payments.

DELETE FROM idempotency_records;

ALTER TABLE idempotency_records DROP CONSTRAINT idempotency_records_pkey;

ALTER TABLE idempotency_records
    ADD COLUMN merchant_id UUID NOT NULL REFERENCES merchants (id) ON DELETE RESTRICT;

ALTER TABLE idempotency_records
    ADD CONSTRAINT idempotency_records_pkey PRIMARY KEY (merchant_id, scope, key);
