-- 0008_identity_api_keys_kinds.sql: API key kinds, names and expiry (phase 08,
-- task 08.2; spec 08 item 8; to-be section 8.1).
--
-- kind is secret (irk_, the existing format), restricted (irr_, scopes from
-- the permission list) or publishable (ipk_, public). A publishable key is
-- stored in clear, so it has no hash; api_keys_hash_matches_kind_check makes
-- hash NULL for publishable keys and only for them.
--
-- Backfill: every existing key was issued by irisctl as an irk_ secret key
-- without a name, so it becomes kind 'secret', name 'legacy'. Nothing is
-- deleted.
--
-- The column defaults keep the phase 07 insert path (pgstore CreateAPIKey,
-- which irisctl issue-api-key uses) working until it writes kind and name
-- itself (task 08.10): until then the key it issues is a secret key named
-- 'legacy'.
--
-- expires_at is set when a key is rolled. created_by_actor is NULL for keys
-- issued before this migration, which recorded no actor.

BEGIN;

ALTER TABLE api_keys
    ADD COLUMN kind             TEXT,
    ADD COLUMN name             TEXT,
    ADD COLUMN created_by_actor TEXT,
    ADD COLUMN expires_at       TIMESTAMPTZ;

UPDATE api_keys SET kind = 'secret', name = 'legacy' WHERE kind IS NULL;

ALTER TABLE api_keys
    ALTER COLUMN kind SET DEFAULT 'secret',
    ALTER COLUMN kind SET NOT NULL,
    ALTER COLUMN name SET DEFAULT 'legacy',
    ALTER COLUMN name SET NOT NULL,
    ALTER COLUMN hash DROP NOT NULL,
    ADD CONSTRAINT api_keys_kind_check
        CHECK (kind IN ('secret', 'publishable', 'restricted')),
    ADD CONSTRAINT api_keys_hash_matches_kind_check
        CHECK ((kind = 'publishable' AND hash IS NULL)
            OR (kind IN ('secret', 'restricted') AND hash IS NOT NULL));

COMMIT;
