'use server';

import { revalidatePath } from 'next/cache';
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
  const trimmed = id.trim();
  if (!trimmed) return { ok: false, error: 'id ausente.' };
  try {
    await fn(trimmed);
    revalidatePath(`/dashboard/payment-intents/${trimmed}`);
    revalidatePath('/dashboard/payment-intents');
    revalidatePath('/dashboard');
    return { ok: true };
  } catch (err) {
    if (err instanceof IrisApiError) return { ok: false, error: err.message };
    return { ok: false, error: 'Falha ao executar ação.' };
  }
}
