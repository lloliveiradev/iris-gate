-- 0006_identity_users_and_tokens.sql: users, their single-use tokens and
-- refresh-token families (phase 08, task 08.2; to-be section 8.1).
--
-- merchants gains how it was onboarded and when it was approved. Every
-- existing merchant was created by irisctl, the sales-led path, so the default
-- fits it; approved_at stays NULL because no approval was ever recorded.
--
-- users.name and users.password_hash are nullable: irisctl bootstrap-admin
-- creates the first platform admin from an email only, and that user sets a
-- password through a password_setup token. Users are disabled, never deleted.
--
-- Tokens are stored as hashes only; the plaintext reaches the user once.
--
-- No row-level security (phase 08 plan section 2, Option B): login, refresh and
-- token checks look these rows up before a merchant is known.

BEGIN;

ALTER TABLE merchants
    ADD COLUMN onboarding TEXT NOT NULL DEFAULT 'sales_led'
        CHECK (onboarding IN ('sales_led', 'self_signup')),
    ADD COLUMN approved_at TIMESTAMPTZ;

CREATE TABLE users (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email               CITEXT      NOT NULL UNIQUE,
    name                TEXT,
    password_hash       TEXT,
    status              TEXT        NOT NULL DEFAULT 'active'
                                    CHECK (status IN ('active', 'disabled')),
    email_verified_at   TIMESTAMPTZ,
    password_changed_at TIMESTAMPTZ,
    last_login_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id),
    family_id   UUID        NOT NULL,
    audience    TEXT        NOT NULL
                            CHECK (audience IN ('iris-dashboard', 'iris-platform')),
    token_hash  TEXT        NOT NULL UNIQUE,
    replaced_by UUID        REFERENCES refresh_tokens (id),
    rotated_at  TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_idx   ON refresh_tokens (user_id);

CREATE TABLE user_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id),
    purpose    TEXT        NOT NULL
                           CHECK (purpose IN ('email_verification', 'password_reset', 'password_setup')),
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX user_tokens_user_idx ON user_tokens (user_id);

COMMIT;
