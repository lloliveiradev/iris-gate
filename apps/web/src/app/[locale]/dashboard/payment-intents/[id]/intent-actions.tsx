'use client';

import { useActionState } from 'react';
import { CircleDollarSign, LoaderCircle, ShieldCheck } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
  const t = useTranslations('dashboard.payment_intents.actions');
  const tDetail = useTranslations('dashboard.payment_intents_detail.transitions');
  const [authState, authAction, authPending] = useActionState(authorizeAction, initial);
  const [capState, capAction, capPending] = useActionState(captureAction, initial);

  const canAuthorize = status === 'pending';
  const canCapture = status === 'authorized';

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-3">
        <form action={authAction}>
          <input type="hidden" name="id" value={id} />
          <Button type="submit" size="lg" disabled={!canAuthorize || authPending}>
            {authPending ? (
              <LoaderCircle className="size-4 animate-spin" />
            ) : (
              <ShieldCheck className="size-4" />
            )}
            {authPending ? t('authorize_pending') : t('authorize')}
          </Button>
        </form>
        <form action={capAction}>
          <input type="hidden" name="id" value={id} />
          <Button
            type="submit"
            size="lg"
            variant="secondary"
            disabled={!canCapture || capPending}
          >
            {capPending ? (
              <LoaderCircle className="size-4 animate-spin" />
            ) : (
              <CircleDollarSign className="size-4" />
            )}
            {capPending ? t('capture_pending') : t('capture')}
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
        <p className="text-xs text-muted-foreground">{tDetail('no_more')}</p>
      )}
    </div>
  );
}
