import { ArrowRight } from 'lucide-react';
import type { Metadata } from 'next';
import { getTranslations } from 'next-intl/server';
import { Button } from '@/components/ui/button';
import { Link } from '@/i18n/routing';
import { authenticatedClient } from '@/lib/api-client';
import { PaymentIntentsTable } from './payment-intents-table';

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations('dashboard.payment_intents');
  return { title: t('title') };
}

export default async function PaymentIntentsList() {
  const t = await getTranslations('dashboard.payment_intents');
  const client = await authenticatedClient();
  const initialData = await client.listPaymentIntents();

  return (
    <div className="container max-w-6xl space-y-6 px-6 py-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <p className="text-sm font-medium uppercase tracking-wider text-secondary">
            {t('eyebrow')}
          </p>
          <h1 className="font-serif text-3xl font-normal tracking-tight text-foreground">
            {t('title')}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t.rich('description', {
              count: initialData.length,
              n: (chunks) => (
                <span className="font-mono tabular-nums text-foreground">{chunks}</span>
              ),
            })}
          </p>
        </div>
        <Button asChild size="lg">
          <Link href="/dashboard/payment-intents/new">
            {t('new_cta')}
            <ArrowRight className="size-4" />
          </Link>
        </Button>
      </header>

      <PaymentIntentsTable initialData={initialData} />
    </div>
  );
}
