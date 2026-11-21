-- 0009_identity_audit_events.sql: the append-only audit log (phase 08, task
-- 08.2; spec 08 item 11; to-be sections 4.5 and 8.1).
--
-- Administrative actions are written here in the same transaction as the
-- change. Rows are chained through prev_hash and hash under a
-- transaction-scoped advisory lock, and id is a bigserial, so id order is chain
-- order. merchant_id is NULL for platform-wide actions such as staff changes.
-- Entity and actor ids have no foreign key: history must outlive what it
-- describes.
--
-- Append-only. The default privileges in 10-roles.sh give iris_app and
-- iris_worker SELECT, INSERT, UPDATE and DELETE on every table the migrator
-- creates, plus USAGE on its sequences (the id sequence included). This file
-- revokes UPDATE and DELETE from both; iris_platform keeps SELECT only.
-- BYPASSRLS skips row-level security, not table privileges, so it does not give
-- them back.
--
-- The revoke runs only when the service roles exist. A database prepared
-- without 10-roles.sh (the go-libs CI job, a bare test database) has neither
-- role, and the default privileges never grant a role created later anything
-- on an existing table. One role without the other is a broken setup, so the
-- migration fails.
--
-- No row-level security (phase 08 plan section 2, Option B): platform actions
-- write and read rows whose merchant_id is NULL.

BEGIN;

CREATE TABLE audit_events (
    id          BIGSERIAL   PRIMARY KEY,
    merchant_id UUID,
    action      TEXT        NOT NULL,
    entity_type TEXT        NOT NULL,
    entity_id   TEXT        NOT NULL,
    actor_type  TEXT        NOT NULL,
    actor_id    TEXT        NOT NULL,
    request_id  TEXT        NOT NULL,
    data        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    prev_hash   BYTEA       NOT NULL,
    hash        BYTEA       NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_merchant_occurred_idx
    ON audit_events (merchant_id, occurred_at DESC, id DESC);

DO $$
DECLARE
    service_roles INT;
BEGIN
    SELECT count(*) INTO service_roles
      FROM pg_roles
     WHERE rolname IN ('iris_app', 'iris_worker');

    IF service_roles = 2 THEN
        REVOKE UPDATE, DELETE ON audit_events FROM iris_app, iris_worker;
    ELSIF service_roles = 1 THEN
        RAISE EXCEPTION 'audit_events: iris_app and iris_worker must both exist (10-roles.sh) or both be absent';
    END IF;
END
$$;

COMMIT;
