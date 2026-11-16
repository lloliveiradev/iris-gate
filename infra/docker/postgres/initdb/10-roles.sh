#!/usr/bin/env bash
# Least-privilege database roles (phase 07, docs/design/to-be.md section 9.2).
#
# The Postgres image runs the files in this directory once, in name order, when
# the data volume is empty (`make nuke` resets it), so this runs after
# 00-extensions.sql. The image sources a non-executable script in its own bash
# with `set -e`; a single psql call with ON_ERROR_STOP keeps the result the same
# when the file is executed instead.
#
# Passwords come from the container environment (infra/docker/.env through
# docker-compose.yml). psql reads them with \getenv, so they never appear on a
# command line.
#
#   iris       iris_migrator (owner, DDL), iris_app (request handlers, DML),
#              iris_worker (jobs, DML, BYPASSRLS), iris_platform (SELECT, BYPASSRLS)
#   antifraud  antifraud_migrator (owner, DDL), antifraud_app (Nest runtime, DML)

: "${POSTGRES_USER:?POSTGRES_USER is not set}"
: "${POSTGRES_DB:?POSTGRES_DB is not set}"
: "${ANTI_FRAUD_DB:?ANTI_FRAUD_DB is not set}"
: "${IRIS_MIGRATOR_PASSWORD:?IRIS_MIGRATOR_PASSWORD is not set (infra/docker/.env)}"
: "${IRIS_APP_PASSWORD:?IRIS_APP_PASSWORD is not set (infra/docker/.env)}"
: "${IRIS_WORKER_PASSWORD:?IRIS_WORKER_PASSWORD is not set (infra/docker/.env)}"
: "${IRIS_PLATFORM_PASSWORD:?IRIS_PLATFORM_PASSWORD is not set (infra/docker/.env)}"
: "${ANTIFRAUD_MIGRATOR_PASSWORD:?ANTIFRAUD_MIGRATOR_PASSWORD is not set (infra/docker/.env)}"
: "${ANTIFRAUD_APP_PASSWORD:?ANTIFRAUD_APP_PASSWORD is not set (infra/docker/.env)}"

psql -v ON_ERROR_STOP=1 --no-psqlrc --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
\getenv iris_db POSTGRES_DB
\getenv antifraud_db ANTI_FRAUD_DB
\getenv iris_migrator_password IRIS_MIGRATOR_PASSWORD
\getenv iris_app_password IRIS_APP_PASSWORD
\getenv iris_worker_password IRIS_WORKER_PASSWORD
\getenv iris_platform_password IRIS_PLATFORM_PASSWORD
\getenv antifraud_migrator_password ANTIFRAUD_MIGRATOR_PASSWORD
\getenv antifraud_app_password ANTIFRAUD_APP_PASSWORD

-- Roles are cluster-wide. None is a superuser or may create roles or
-- databases. BYPASSRLS is set here because only a superuser can grant it: the
-- worker and platform roles read across merchants once row-level security
-- lands (phase 07.2); iris_app never does.
CREATE ROLE iris_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS
    PASSWORD :'iris_migrator_password';
CREATE ROLE iris_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS
    PASSWORD :'iris_app_password';
CREATE ROLE iris_worker LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION BYPASSRLS
    PASSWORD :'iris_worker_password';
CREATE ROLE iris_platform LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION BYPASSRLS
    PASSWORD :'iris_platform_password';
CREATE ROLE antifraud_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS
    PASSWORD :'antifraud_migrator_password';
CREATE ROLE antifraud_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS
    PASSWORD :'antifraud_app_password';

-- ---------------------------------------------------------------------------
-- Database iris (apps/api). The image created it from POSTGRES_DB, and this
-- session is connected to it.
-- ---------------------------------------------------------------------------

-- The migrator owns the database and, through pg_database_owner, the public
-- schema: it runs every DDL statement. Only the iris roles may connect, and no
-- one else gets TEMPORARY.
ALTER DATABASE :"iris_db" OWNER TO iris_migrator;
REVOKE ALL ON DATABASE :"iris_db" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"iris_db" TO iris_app, iris_worker, iris_platform;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO iris_app, iris_worker, iris_platform;

-- Migration bookkeeping, created before the default privileges below so that
-- no service role gets a grant on it: a role that could write it could mark a
-- migration (the RLS one, say) as applied. Same definition as MigrateEmbedded
-- in libs/identity and libs/payments, whose CREATE TABLE IF NOT EXISTS finds it.
CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());
ALTER TABLE schema_migrations OWNER TO iris_migrator;

-- Every table and sequence the migrator creates from now on: request handlers
-- and jobs get DML, platform handlers read. TRUNCATE, REFERENCES and TRIGGER
-- stay with the owner.
ALTER DEFAULT PRIVILEGES FOR ROLE iris_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO iris_app, iris_worker;
ALTER DEFAULT PRIVILEGES FOR ROLE iris_migrator IN SCHEMA public
    GRANT SELECT ON TABLES TO iris_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE iris_migrator IN SCHEMA public
    GRANT USAGE ON SEQUENCES TO iris_app, iris_worker;

-- ---------------------------------------------------------------------------
-- Database antifraud (apps/anti-fraud). `make migrate` runs
-- `prisma migrate deploy` as antifraud_migrator.
-- ---------------------------------------------------------------------------

CREATE DATABASE :"antifraud_db" OWNER antifraud_migrator;
REVOKE ALL ON DATABASE :"antifraud_db" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"antifraud_db" TO antifraud_app;

\connect :antifraud_db

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO antifraud_app;

-- Covers every Prisma migration, including the phase 10 tables. Prisma's own
-- _prisma_migrations table gets these grants too; `make -C apps/anti-fraud
-- prisma-deploy` revokes them after each deploy.
ALTER DEFAULT PRIVILEGES FOR ROLE antifraud_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO antifraud_app;
ALTER DEFAULT PRIVILEGES FOR ROLE antifraud_migrator IN SCHEMA public
    GRANT USAGE ON SEQUENCES TO antifraud_app;
SQL
