'use server';

import { revalidatePath } from 'next/cache';
import { getLocale, getTranslations } from 'next-intl/server';
import { IrisApiError, type PaymentIntent } from '@iris-gate/sdk-web';
import { authenticatedClient } from '@/lib/api-client';

export async function listPaymentIntentsAction(): Promise<PaymentIntent[]> {
  const client = await authenticatedClient();
  return client.listPaymentIntents();
}

export interface MutationResult {
  ok: boolean;
  error?: string;
}

export async function authorizePaymentIntentAction(input: {
  id: string;
}): Promise<MutationResult> {
  return runMutation(input.id, async (id) => {
    const client = await authenticatedClient();
    await client.authorizePaymentIntent(id);
  });
}

export async function capturePaymentIntentAction(input: {
  id: string;
}): Promise<MutationResult> {
  return runMutation(input.id, async (id) => {
    const client = await authenticatedClient();
    await client.capturePaymentIntent(id);
  });
}

async function runMutation(
  id: string,
  fn: (id: string) => Promise<void>,
): Promise<MutationResult> {
  const t = await getTranslations('dashboard.payment_intents_detail.errors');
  const locale = await getLocale();
  const trimmed = id.trim();
  if (!trimmed) return { ok: false, error: t('missing_id') };
  try {
    await fn(trimmed);
    revalidatePath(`/${locale}/dashboard/payment-intents/${trimmed}`);
    revalidatePath(`/${locale}/dashboard/payment-intents`);
    revalidatePath(`/${locale}/dashboard`);
    return { ok: true };
  } catch (err) {
    if (err instanceof IrisApiError) return { ok: false, error: err.message };
    return { ok: false, error: t('unknown') };
  }
}
