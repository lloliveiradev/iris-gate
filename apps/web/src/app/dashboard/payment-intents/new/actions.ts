'use server';

import { randomUUID } from 'node:crypto';
import { redirect } from 'next/navigation';
import { getTranslations } from 'next-intl/server';
import { IrisApiError } from '@iris-gate/sdk-web';
import { authenticatedClient } from '@/lib/api-client';

export interface CreateIntentState {
  ok: boolean;
  error?: string;
}

export async function createPaymentIntentAction(
  _prev: CreateIntentState | undefined,
  formData: FormData,
): Promise<CreateIntentState> {
  const t = await getTranslations('dashboard.payment_intents_new.errors');
  const amountStr = String(formData.get('amount') ?? '').replace(',', '.').trim();
  const currency = String(formData.get('currency') ?? 'BRL').toUpperCase();
  const cardToken = String(formData.get('cardToken') ?? '').trim();
  const description = String(formData.get('description') ?? '').trim();

  const amountReais = Number.parseFloat(amountStr);
  if (!Number.isFinite(amountReais) || amountReais <= 0) {
    return { ok: false, error: t('amount_invalid') };
  }
  if (!/^[A-Z]{3}$/.test(currency)) {
    return { ok: false, error: t('currency_invalid') };
  }
  if (cardToken.length < 8) {
    return { ok: false, error: t('card_token_short') };
  }

  const amountCents = Math.round(amountReais * 100);
  const idempotencyKey = randomUUID();

  let intentId: string;
  try {
    const client = await authenticatedClient();
    const created = await client.createPaymentIntent(
      {
        amount: { amount_cents: amountCents, currency },
        card_token: cardToken,
        ...(description ? { description } : {}),
      },
      idempotencyKey,
    );
    intentId = created.id;
  } catch (err) {
    if (err instanceof IrisApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: t('unknown') };
  }

  redirect(`/dashboard/payment-intents/${intentId}`);
}
