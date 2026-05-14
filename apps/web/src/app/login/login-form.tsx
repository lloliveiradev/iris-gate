'use client';

import { useActionState } from 'react';
import { ArrowRight, KeyRound, Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { loginAction, type LoginFormState } from './actions';

const initialState: LoginFormState = { ok: false };

export function LoginForm() {
  const t = useTranslations('login.form');
  const [state, formAction, pending] = useActionState(loginAction, initialState);

  return (
    <form action={formAction} className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <Label htmlFor="apiKey" className="text-sm">
          {t('api_key_label')}
        </Label>
        <div className="relative">
          <KeyRound
            aria-hidden
            className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            id="apiKey"
            name="apiKey"
            type="password"
            placeholder={t('api_key_placeholder')}
            autoComplete="off"
            autoCapitalize="off"
            spellCheck={false}
            required
            disabled={pending}
            aria-invalid={!!state.error}
            aria-describedby={state.error ? 'apiKey-error' : 'apiKey-hint'}
            className="h-11 pl-9 font-mono"
          />
        </div>
        <p id="apiKey-hint" className="text-xs text-muted-foreground">
          {t.rich('hint', {
            code: (chunks) => <code className="font-mono">{chunks}</code>,
          })}
        </p>
      </div>

      {state.error && (
        <p
          id="apiKey-error"
          role="alert"
          className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
        >
          {state.error}
        </p>
      )}

      <Button type="submit" size="lg" disabled={pending} className="w-full">
        {pending ? (
          <>
            <Loader2 className="size-4 animate-spin" />
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
