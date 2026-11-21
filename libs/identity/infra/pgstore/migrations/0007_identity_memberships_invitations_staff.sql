-- 0007_identity_memberships_invitations_staff.sql: merchant memberships,
-- invitations and platform staff (phase 08, task 08.2; to-be sections 4.3, 4.4
-- and 8.1).
--
-- A user belongs to many merchants through merchant_memberships, with one role
-- in each. A removed membership keeps its row, so re-inviting that user
-- reactivates it instead of inserting a second (merchant_id, user_id).
--
-- Exactly one active owner per merchant: merchant_memberships_one_owner_idx
-- makes Postgres reject a second one even when two requests race. A unique
-- index is checked row by row, not at commit, so an ownership transfer demotes
-- the current owner before it promotes the new one.
--
-- invitations.role accepts owner because the sales-led path invites the first
-- owner (POST /v1/platform/merchants); members invite the other roles only,
-- which the domain enforces. Invitation tokens are stored as hashes only.
--
-- No row-level security (phase 08 plan section 2, Option B): login lists a
-- user's memberships and accepting an invitation reads it by token before a
-- merchant is known. Every merchant-scoped query filters by merchant_id in the
-- repository.

BEGIN;

CREATE TABLE merchant_memberships (
    merchant_id UUID        NOT NULL REFERENCES merchants (id),
    user_id     UUID        NOT NULL REFERENCES users (id),
    role        TEXT        NOT NULL
                            CHECK (role IN ('owner', 'admin', 'developer', 'operations', 'risk_analyst', 'viewer')),
    status      TEXT        NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'removed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (merchant_id, user_id)
);

CREATE INDEX merchant_memberships_user_idx ON merchant_memberships (user_id);

CREATE UNIQUE INDEX merchant_memberships_one_owner_idx
    ON merchant_memberships (merchant_id)
    WHERE role = 'owner' AND status = 'active';

CREATE TABLE invitations (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id      UUID        NOT NULL REFERENCES merchants (id),
    email            CITEXT      NOT NULL,
    role             TEXT        NOT NULL
                                 CHECK (role IN ('owner', 'admin', 'developer', 'operations', 'risk_analyst', 'viewer')),
    token_hash       TEXT        NOT NULL UNIQUE,
    invited_by_actor TEXT        NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    accepted_at      TIMESTAMPTZ,
    revoked_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX invitations_merchant_idx ON invitations (merchant_id, created_at DESC);

CREATE TABLE platform_staff (
    user_id    UUID        PRIMARY KEY REFERENCES users (id),
    role       TEXT        NOT NULL
                           CHECK (role IN ('platform_admin', 'platform_support')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
