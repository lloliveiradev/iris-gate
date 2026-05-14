import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { authenticatedClient } from '@/lib/api-client';
import { PaymentIntentsTable } from './payment-intents-table';

export default async function PaymentIntentsList() {
  const client = await authenticatedClient();
  const initialData = await client.listPaymentIntents();

  return (
    <div className="container max-w-6xl space-y-6 px-6 py-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-1">
          <p className="text-sm font-medium uppercase tracking-wider text-secondary">
            Histórico de transações
          </p>
          <h1 className="font-serif text-3xl font-normal tracking-tight text-foreground">
            Payment intents
          </h1>
          <p className="text-sm text-muted-foreground">
            Listagem das últimas{' '}
            <span className="font-mono tabular-nums text-foreground">{initialData.length}</span>{' '}
            intents do merchant.
          </p>
        </div>
        <Button asChild size="lg">
          <Link href="/dashboard/payment-intents/new">
            Nova intent
            <ArrowRight className="size-4" />
          </Link>
        </Button>
      </header>

      <PaymentIntentsTable initialData={initialData} />
    </div>
  );
}
