import Link from 'next/link';
import { ArrowRight, Activity, AlertTriangle, DollarSign } from 'lucide-react';
import { getLocale, getTranslations } from 'next-intl/server';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { authenticatedClient } from '@/lib/api-client';
import {
  formatDateTimeServer,
  formatMoneyServer,
  statusVariant,
} from '@/lib/format';
import { requireSession } from '@/lib/session';

export default async function DashboardOverview() {
  const session = await requireSession();
  const locale = await getLocale();
  const t = await getTranslations('dashboard.overview');
  const tStatus = await getTranslations('dashboard.payment_intents.statuses');
  const client = await authenticatedClient();
  const recent = await client.listPaymentIntents().catch(() => []);

  const totalCents = recent.reduce((sum, p) => sum + p.amount.amount_cents, 0);
  const captured = recent.filter((p) => p.status === 'captured').length;
  const flagged = recent.filter((p) => p.status === 'failed' || p.status === 'canceled').length;
  const currency = recent[0]?.amount.currency ?? 'BRL';

  return (
    <div className="container max-w-6xl space-y-8 px-6 py-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <p className="text-sm text-muted-foreground">
            {t.rich('welcome', {
              merchantName: session.merchant.legalName,
              strong: (chunks) => (
                <strong className="font-medium text-foreground">{chunks}</strong>
              ),
            })}
          </p>
          <h1 className="font-serif text-3xl font-normal tracking-tight text-foreground">
            {t('title')}
          </h1>
        </div>
        <Button asChild size="lg">
          <Link href="/dashboard/payment-intents/new">
            {t('new_cta')}
            <ArrowRight className="size-4" />
          </Link>
        </Button>
      </header>

      <section className="grid gap-4 md:grid-cols-3">
        <StatCard
          icon={<Activity className="size-4 text-secondary" />}
          label={t('cards.recent_intents')}
          value={String(recent.length)}
          hint={t('cards.recent_intents_hint', { count: 50 })}
        />
        <StatCard
          icon={<DollarSign className="size-4 text-secondary" />}
          label={t('cards.volume')}
          value={formatMoneyServer(locale, totalCents, currency)}
          hint={t('cards.volume_hint', { captured })}
          mono
        />
        <StatCard
          icon={<AlertTriangle className="size-4 text-secondary" />}
          label={t('cards.flagged')}
          value={String(flagged)}
          hint={t('cards.flagged_hint')}
        />
      </section>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-4">
            <div>
              <CardTitle>{t('recent.title')}</CardTitle>
              <CardDescription>{t('recent.description')}</CardDescription>
            </div>
            <Button asChild variant="outline" size="sm">
              <Link href="/dashboard/payment-intents">{t('recent.view_all')}</Link>
            </Button>
          </div>
        </CardHeader>
        <CardContent className="px-0 pb-0">
          {recent.length === 0 ? (
            <EmptyState />
          ) : (
            <ul className="divide-y border-t">
              {recent.slice(0, 6).map((p) => {
                const statusLabel = translateStatus(tStatus, p.status);
                return (
                  <li
                    key={p.id}
                    className="flex items-center justify-between gap-4 px-6 py-4 transition-colors hover:bg-secondary/5"
                  >
                    <div className="min-w-0">
                      <Link
                        href={`/dashboard/payment-intents/${p.id}`}
                        className="block truncate font-mono text-base font-medium tabular-nums text-foreground hover:underline"
                      >
                        {formatMoneyServer(locale, p.amount.amount_cents, p.amount.currency)}
                      </Link>
                      <p className="truncate text-xs text-muted-foreground">
                        <span className="font-mono">{p.id}</span> ·{' '}
                        {formatDateTimeServer(locale, p.created_at)}
                      </p>
                    </div>
                    <Badge variant={statusVariant(p.status)} dot className="shrink-0">
                      {statusLabel}
                    </Badge>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
  hint,
  mono,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  hint?: string;
  mono?: boolean;
}) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2 pb-2">
        <CardDescription className="text-sm">{label}</CardDescription>
        <span className="grid size-8 place-items-center rounded-md bg-secondary/10 ring-1 ring-secondary/25">
          {icon}
        </span>
      </CardHeader>
      <CardContent>
        <p
          className={
            mono
              ? 'font-mono text-3xl font-semibold tabular-nums tracking-tight text-foreground'
              : 'text-3xl font-semibold tracking-tight text-foreground'
          }
        >
          {value}
        </p>
        {hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}
      </CardContent>
    </Card>
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

async function EmptyState() {
  const t = await getTranslations('dashboard.overview.empty');
  return (
    <div className="border-t p-12 text-center">
      <p className="text-sm text-muted-foreground">{t('title')}</p>
      <Button asChild className="mt-4">
        <Link href="/dashboard/payment-intents/new">{t('cta')}</Link>
      </Button>
    </div>
  );
}
