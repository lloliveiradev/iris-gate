import { existsSync } from 'node:fs';
import { defineConfig } from 'prisma/config';

// Prisma 7 no longer reads .env on its own. Load the local file when it exists;
// variables already set in the environment (Compose, CI) take precedence.
const LOCAL_ENV_FILE = '.env';
if (existsSync(LOCAL_ENV_FILE)) {
  process.loadEnvFile(LOCAL_ENV_FILE);
}

// The Prisma CLI (migrate, db execute) connects as antifraud_migrator, the
// owner of the schema. The Nest runtime reads DATABASE_URL (antifraud_app,
// DML only) instead. The default is the local Compose role from
// infra/docker/.env.example.
const LOCAL_MIGRATOR_DATABASE_URL =
  'postgresql://antifraud_migrator:antifraud_migrator@localhost:15432/antifraud?schema=public';

export default defineConfig({
  schema: 'prisma/schema.prisma',
  migrations: {
    path: 'prisma/migrations',
  },
  datasource: {
    // Read directly instead of env() so `prisma generate` works without a
    // database URL (Docker build, CI).
    url: process.env.ANTIFRAUD_MIGRATOR_DATABASE_URL ?? LOCAL_MIGRATOR_DATABASE_URL,
    // `prisma migrate dev` needs a shadow database, and the migrator cannot
    // create one; see apps/anti-fraud/README.md.
    shadowDatabaseUrl: process.env.ANTIFRAUD_SHADOW_DATABASE_URL,
  },
});
