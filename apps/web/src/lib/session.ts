import 'server-only';
import { cookies } from 'next/headers';
import { getIronSession, type SessionOptions } from 'iron-session';
import { env } from './env';

export interface IrisSession {
  apiKey?: string;
  merchant?: {
    id: string;
    legalName: string;
    status: string;
  };
}

const sessionOptions: SessionOptions = {
  password: env.sessionPassword,
  cookieName: env.sessionCookieName,
  cookieOptions: {
    secure: process.env.NODE_ENV === 'production',
    httpOnly: true,
    sameSite: 'lax',
    path: '/',
  },
};

export type IrisSessionHandle = Awaited<ReturnType<typeof getIronSession<IrisSession>>>;

export async function getSession(): Promise<IrisSessionHandle> {
  const store = await cookies();
  return getIronSession<IrisSession>(store, sessionOptions);
}

export async function requireSession(): Promise<
  IrisSessionHandle & { apiKey: string; merchant: NonNullable<IrisSession['merchant']> }
> {
  const session = await getSession();
  if (!session.apiKey || !session.merchant) {
    throw new Error('unauthenticated');
  }
  return session as IrisSessionHandle & {
    apiKey: string;
    merchant: NonNullable<IrisSession['merchant']>;
  };
}
