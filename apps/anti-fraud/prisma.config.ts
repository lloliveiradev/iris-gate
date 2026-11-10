import { existsSync } from 'node:fs';
import { defineConfig } from 'prisma/config';

// Prisma 7 no longer reads .env on its own. Load the local file when it exists;
// variables already set in the environment (Compose, CI) take precedence.
const LOCAL_ENV_FILE = '.env';
if (existsSync(LOCAL_ENV_FILE)) {
  process.loadEnvFile(LOCAL_ENV_FILE);
}

export default defineConfig({
  schema: 'prisma/schema.prisma',
  migrations: {
    path: 'prisma/migrations',
  },
  datasource: {
    // Read directly instead of env() so `prisma generate` works without a
    // database URL (Docker build, CI). Migrate commands still require it.
    url: process.env.DATABASE_URL,
  },
});
