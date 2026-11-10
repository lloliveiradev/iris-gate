'use client';

import { useActionState } from 'react';
import { ArrowRight, LoaderCircle } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { createPaymentIntentAction, type CreateIntentState } from './actions';

const initialState: CreateIntentState = { ok: false };

export function NewIntentForm() {
  const t = useTranslations('dashboard.payment_intents_new');
  const [state, formAction, pending] = useActionState(createPaymentIntentAction, initialState);

  return (
    <form action={formAction} className="grid gap-5">
      <div className="grid gap-5 sm:grid-cols-[2fr_1fr]">
        <div className="grid gap-2">
          <Label htmlFor="amount">{t('fields.amount.label')}</Label>
          <Input
            id="amount"
            name="amount"
            inputMode="decimal"
            placeholder={t('fields.amount.placeholder')}
            required
            disabled={pending}
            className="h-11 font-mono tabular-nums"
          />
          <p className="text-xs text-muted-foreground">{t('fields.amount.hint')}</p>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="currency">{t('fields.currency.label')}</Label>
          <Input
            id="currency"
            name="currency"
            defaultValue="BRL"
            maxLength={3}
            required
            disabled={pending}
            className="h-11 font-mono uppercase tracking-wider"
          />
        </div>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="cardToken">{t('fields.card_token.label')}</Label>
        <Input
          id="cardToken"
          name="cardToken"
          placeholder={t('fields.card_token.placeholder')}
          required
          disabled={pending}
          className="h-11 font-mono"
        />
        <p className="text-xs text-muted-foreground">{t('fields.card_token.hint')}</p>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="description">{t('fields.description.label')}</Label>
        <Input
          id="description"
          name="description"
          maxLength={200}
          disabled={pending}
          className="h-11"
        />
      </div>
      {state.error && (
        <p
          role="alert"
          className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
        >
          {state.error}
        </p>
      )}
      <Button type="submit" size="lg" disabled={pending} className="w-full sm:w-auto sm:self-end">
        {pending ? (
          <>
            <LoaderCircle className="size-4 animate-spin" />
            {t('submit_pending')}
          </>
        ) : (
          <>
            {t('submit')}
            <ArrowRight className="size-4" />
          </>
        )}
      </Button>
    </form>
  );
}
