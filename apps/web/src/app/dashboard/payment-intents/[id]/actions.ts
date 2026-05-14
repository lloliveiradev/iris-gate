'use server';

import { revalidatePath } from 'next/cache';
import { getTranslations } from 'next-intl/server';
import { IrisApiError } from '@iris-gate/sdk-web';
import { authenticatedClient } from '@/lib/api-client';

export interface ActionState {
  ok: boolean;
  error?: string;
}

export async function authorizeAction(
  _prev: ActionState | undefined,
  formData: FormData,
): Promise<ActionState> {
  return runMutation(formData, async (id) => {
    const c = await authenticatedClient();
    await c.authorizePaymentIntent(id);
  });
}

export async function captureAction(
  _prev: ActionState | undefined,
  formData: FormData,
): Promise<ActionState> {
  return runMutation(formData, async (id) => {
    const c = await authenticatedClient();
    await c.capturePaymentIntent(id);
  });
}

async function runMutation(
  formData: FormData,
  fn: (id: string) => Promise<void>,
): Promise<ActionState> {
  const t = await getTranslations('dashboard.payment_intents_detail.errors');
  const id = String(formData.get('id') ?? '').trim();
  if (!id) return { ok: false, error: t('missing_id') };
  try {
    await fn(id);
    revalidatePath(`/dashboard/payment-intents/${id}`);
    revalidatePath('/dashboard/payment-intents');
    revalidatePath('/dashboard');
    return { ok: true };
  } catch (err) {
    if (err instanceof IrisApiError) return { ok: false, error: err.message };
    return { ok: false, error: t('unknown') };
  }
}
