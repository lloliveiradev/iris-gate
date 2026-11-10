export interface DatabaseConfig {
  url: string;
}

/** Reads the Postgres connection URL. Fails fast: there is no safe default database. */
export function loadDatabaseConfig(): DatabaseConfig {
  const url = process.env.DATABASE_URL;
  if (!url) {
    throw new Error('DATABASE_URL is not set');
  }
  return { url };
}

export const DATABASE_CONFIG = Symbol('DATABASE_CONFIG');
