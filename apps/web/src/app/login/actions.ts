'use server';

import { redirect } from 'next/navigation';
import { getTranslations } from 'next-intl/server';
import { IrisApiError, IrisClient } from '@iris-gate/sdk-web';
import { env } from '@/lib/env';
import { getSession } from '@/lib/session';

export interface LoginFormState {
  ok: boolean;
  error?: string;
}

export async function loginAction(
  _prev: LoginFormState | undefined,
  formData: FormData,
): Promise<LoginFormState> {
  const t = await getTranslations('login.errors');
  const apiKey = String(formData.get('apiKey') ?? '').trim();
  if (!apiKey) {
    return { ok: false, error: t('missing') };
  }
  if (!apiKey.startsWith('irk_')) {
    return { ok: false, error: t('prefix') };
  }

  const client = new IrisClient({ baseUrl: env.apiBaseUrl, apiKey });
  try {
    const merchant = await client.me();
    const session = await getSession();
    session.apiKey = apiKey;
    session.merchant = {
      id: merchant.id,
      legalName: merchant.legal_name,
      status: merchant.status,
    };
    await session.save();
  } catch (err) {
    if (err instanceof IrisApiError) {
      if (err.status === 401 || err.status === 403) {
        return { ok: false, error: t('invalid') };
      }
      return { ok: false, error: err.message };
    }
    return { ok: false, error: t('unknown') };
  }

  redirect('/dashboard');
}

export async function logoutAction(): Promise<void> {
  const session = await getSession();
  await session.destroy();
  redirect('/login');
}
