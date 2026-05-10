function required(key: string, fallback?: string): string {
  const v = process.env[key] ?? fallback;
  if (!v) throw new Error(`missing required env var: ${key}`);
  return v;
}

export const env = {
  apiBaseUrl: required('IRIS_API_BASE_URL', 'http://localhost:8080'),
  sessionPassword: required(
    'IRIS_SESSION_PASSWORD',
    'dev-only-do-not-use-in-prod-32+chars-iris-gate-secret',
  ),
  sessionCookieName: process.env.IRIS_SESSION_COOKIE ?? 'iris_session',
};
