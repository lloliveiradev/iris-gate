-- 0001_init.sql — Identity bounded context (Phase 2)
--
-- Tabelas: merchants, api_keys.
-- Pré-requisitos: extensões pgcrypto, citext (criadas em infra/docker/postgres/initdb/00-extensions.sql).

CREATE TABLE merchants (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_name    TEXT        NOT NULL CHECK (length(legal_name) BETWEEN 1 AND 200),
    document      VARCHAR(14) NOT NULL UNIQUE CHECK (document ~ '^[0-9]{14}$'),
    contact_email CITEXT      NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'active', 'suspended')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX merchants_status_idx ON merchants (status);

CREATE TABLE api_keys (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id  UUID        NOT NULL REFERENCES merchants (id) ON DELETE CASCADE,
    prefix       VARCHAR(32) NOT NULL UNIQUE,
    hash         TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active', 'revoked')),
    scopes       TEXT[]      NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX api_keys_merchant_idx ON api_keys (merchant_id);
CREATE INDEX api_keys_status_idx   ON api_keys (status);
