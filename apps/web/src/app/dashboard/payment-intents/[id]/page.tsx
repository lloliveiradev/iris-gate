import Link from 'next/link';
import { notFound } from 'next/navigation';
import { ChevronLeft } from 'lucide-react';
import { getLocale, getTranslations } from 'next-intl/server';
import { IrisApiError } from '@iris-gate/sdk-web';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { authenticatedClient } from '@/lib/api-client';
import {
  formatDateTimeServer,
  formatMoneyServer,
  statusVariant,
} from '@/lib/format';
import { IntentActions } from './intent-actions';

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function PaymentIntentDetailPage({ params }: PageProps) {
  const { id } = await params;
  const client = await authenticatedClient();
  const locale = await getLocale();
  const t = await getTranslations('dashboard.payment_intents_detail');
  const tStatus = await getTranslations('dashboard.payment_intents.statuses');

  let intent;
  try {
    intent = await client.getPaymentIntent(id);
  } catch (err) {
    if (err instanceof IrisApiError && err.status === 404) notFound();
    throw err;
  }

  const statusLabel = translateStatus(tStatus, intent.status);

  return (
    <div className="container max-w-3xl space-y-6 px-6 py-8">
      <Button asChild variant="ghost" size="sm" className="w-fit">
        <Link href="/dashboard/payment-intents">
          <ChevronLeft className="size-4" />
          {t('back')}
        </Link>
      </Button>

      <header className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-1">
            <p className="text-sm font-medium uppercase tracking-wider text-secondary">
              {t('eyebrow')}
            </p>
            <p className="break-all font-mono text-sm text-muted-foreground">{intent.id}</p>
          </div>
          <Badge variant={statusVariant(intent.status)} dot className="text-sm">
            {statusLabel}
          </Badge>
        </div>
        <h1 className="font-mono text-5xl font-semibold tabular-nums tracking-tight text-foreground">
          {formatMoneyServer(locale, intent.amount.amount_cents, intent.amount.currency)}
        </h1>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>{t('summary.title')}</CardTitle>
          <CardDescription>{t('summary.description')}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <Field
            label={t('summary.fields.amount')}
            value={formatMoneyServer(locale, intent.amount.amount_cents, intent.amount.currency)}
            mono
          />
          <Field label={t('summary.fields.currency')} value={intent.amount.currency} mono />
          <Field
            label={t('summary.fields.created_at')}
            value={formatDateTimeServer(locale, intent.created_at)}
          />
          <Field
            label={t('summary.fields.authorized_at')}
            value={formatDateTimeServer(locale, intent.authorized_at)}
          />
          <Field
            label={t('summary.fields.captured_at')}
            value={formatDateTimeServer(locale, intent.captured_at)}
          />
          <Field
            label={t('summary.fields.card_token')}
            value={intent.card_token ?? '—'}
            mono
          />
          {intent.description && (
            <Field
              label={t('summary.fields.description')}
              value={intent.description}
              className="sm:col-span-2"
            />
          )}
        </CardContent>
      </Card>

      <Separator />

      <Card>
        <CardHeader>
          <CardTitle>{t('transitions.title')}</CardTitle>
          <CardDescription>{t('transitions.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <IntentActions id={intent.id} status={intent.status} />
        </CardContent>
      </Card>
    </div>
  );
}

const KNOWN_STATUSES = [
  'pending',
  'authorized',
  'captured',
  'refunded',
  'canceled',
  'failed',
] as const;
type KnownStatus = (typeof KNOWN_STATUSES)[number];

function translateStatus(
  t: (key: KnownStatus) => string,
  status: string,
): string {
  return (KNOWN_STATUSES as readonly string[]).includes(status)
    ? t(status as KnownStatus)
    : status;
}

function Field({
  label,
  value,
  mono,
  className,
}: {
  label: string;
  value: string;
  mono?: boolean;
  className?: string;
}) {
  return (
    <div className={className}>
      <dt className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </dt>
      <dd
        className={
          mono
            ? 'mt-1 break-all font-mono text-sm tabular-nums text-foreground'
            : 'mt-1 text-sm text-foreground'
        }
      >
        {value}
      </dd>
    </div>
  );
}
