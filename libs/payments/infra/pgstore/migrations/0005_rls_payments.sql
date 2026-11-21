-- 0005_rls_payments.sql: row-level security on merchant-scoped payments tables
-- (phase 07, item 3).
--
-- A statement sees only rows whose merchant_id equals the transaction-local
-- setting app.merchant_id, which pgtenant.WithMerchant sets. With no setting,
-- current_setting(..., true) is NULL, the policy is not true, and no row is
-- visible or writable (NULLIF covers the empty string a connection keeps after a
-- transaction-local set_config ends). FORCE applies the policy to the table owner too.
--
-- iris_worker and iris_platform keep BYPASSRLS; iris_app does not. Role
-- attributes are not changed here.
--
-- outbox_messages has no merchant_id and is deliberately left without RLS: the
-- relay (iris_worker) drains every tenant's rows, and the intent and its
-- outbox row are inserted in one tenant-scoped transaction.

BEGIN;

ALTER TABLE payment_intents ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_intents FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_intents_merchant_isolation ON payment_intents
    USING (merchant_id = NULLIF(current_setting('app.merchant_id', true), '')::uuid)
    WITH CHECK (merchant_id = NULLIF(current_setting('app.merchant_id', true), '')::uuid);

ALTER TABLE idempotency_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_records FORCE ROW LEVEL SECURITY;
CREATE POLICY idempotency_records_merchant_isolation ON idempotency_records
    USING (merchant_id = NULLIF(current_setting('app.merchant_id', true), '')::uuid)
    WITH CHECK (merchant_id = NULLIF(current_setting('app.merchant_id', true), '')::uuid);

COMMIT;
