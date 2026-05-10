'use client';

import { useActionState } from 'react';
import { Button } from '@/components/ui/button';
import { type ActionState, authorizeAction, captureAction } from './actions';

const initial: ActionState = { ok: false };

export function IntentActions({
  id,
  status,
}: {
  id: string;
  status: string;
}) {
  const [authState, authAction, authPending] = useActionState(authorizeAction, initial);
  const [capState, capAction, capPending] = useActionState(captureAction, initial);

  const canAuthorize = status === 'pending';
  const canCapture = status === 'authorized';

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
        <form action={authAction}>
          <input type="hidden" name="id" value={id} />
          <Button type="submit" disabled={!canAuthorize || authPending}>
            {authPending ? 'Autorizando…' : 'Autorizar'}
          </Button>
        </form>
        <form action={capAction}>
          <input type="hidden" name="id" value={id} />
          <Button type="submit" variant="secondary" disabled={!canCapture || capPending}>
            {capPending ? 'Capturando…' : 'Capturar'}
          </Button>
        </form>
      </div>
      {(authState.error ?? capState.error) && (
        <p
          role="alert"
          className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
        >
          {authState.error ?? capState.error}
        </p>
      )}
      {!canAuthorize && !canCapture && (
        <p className="text-xs text-muted-foreground">Status atual não permite mais transições.</p>
      )}
    </div>
  );
}
