import 'server-only';
import { IrisClient } from '@iris-gate/sdk-web';
import { env } from './env';
import { getSession } from './session';

export function publicClient(): IrisClient {
  return new IrisClient({ baseUrl: env.apiBaseUrl });
}

export async function authenticatedClient(): Promise<IrisClient> {
  const session = await getSession();
  if (!session.apiKey) {
    throw new Error('unauthenticated');
  }
  return new IrisClient({ baseUrl: env.apiBaseUrl, apiKey: session.apiKey });
}
