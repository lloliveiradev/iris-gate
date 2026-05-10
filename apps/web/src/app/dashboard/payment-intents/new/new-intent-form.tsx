'use client';

import { useActionState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { createPaymentIntentAction, type CreateIntentState } from './actions';

const initialState: CreateIntentState = { ok: false };

export function NewIntentForm() {
  const [state, formAction, pending] = useActionState(createPaymentIntentAction, initialState);

  return (
    <form action={formAction} className="grid gap-5">
      <div className="grid gap-2">
        <Label htmlFor="amount">Valor</Label>
        <Input
          id="amount"
          name="amount"
          inputMode="decimal"
          placeholder="129,90"
          required
          disabled={pending}
        />
        <p className="text-xs text-muted-foreground">
          Valor em reais (será convertido para centavos antes do envio).
        </p>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="currency">Moeda</Label>
        <Input
          id="currency"
          name="currency"
          defaultValue="BRL"
          maxLength={3}
          required
          disabled={pending}
        />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="cardToken">Card token</Label>
        <Input
          id="cardToken"
          name="cardToken"
          placeholder="tok_test_xxxxxxxxxxxxxxxx"
          required
          disabled={pending}
        />
        <p className="text-xs text-muted-foreground">
          Token retornado pela tokenização do adquirente. PAN nunca é enviado pelo painel.
        </p>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="description">Descrição (opcional)</Label>
        <Input id="description" name="description" maxLength={200} disabled={pending} />
      </div>
      {state.error && (
        <p
          role="alert"
          className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
        >
          {state.error}
        </p>
      )}
      <Button type="submit" disabled={pending}>
        {pending ? 'Criando…' : 'Criar payment intent'}
      </Button>
    </form>
  );
}
