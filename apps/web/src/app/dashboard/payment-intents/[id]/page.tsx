import Link from 'next/link';
import { notFound } from 'next/navigation';
import { ChevronLeft } from 'lucide-react';
import { IrisApiError } from '@iris-gate/sdk-web';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { authenticatedClient } from '@/lib/api-client';
import { formatDateTime, formatMoney, statusVariant } from '@/lib/format';
import { IntentActions } from './intent-actions';

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function PaymentIntentDetailPage({ params }: PageProps) {
  const { id } = await params;
  const client = await authenticatedClient();

  let intent;
  try {
    intent = await client.getPaymentIntent(id);
  } catch (err) {
    if (err instanceof IrisApiError && err.status === 404) notFound();
    throw err;
  }

  return (
    <div className="container max-w-3xl space-y-6 px-6 py-8">
      <Button asChild variant="ghost" size="sm" className="w-fit">
        <Link href="/dashboard/payment-intents">
          <ChevronLeft className="size-4" />
          Voltar para a listagem
        </Link>
      </Button>

      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-wider text-muted-foreground">Payment intent</p>
          <h1 className="font-mono text-xl font-semibold">{intent.id}</h1>
        </div>
        <Badge variant={statusVariant(intent.status)} className="text-sm">
          {intent.status}
        </Badge>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Resumo</CardTitle>
          <CardDescription>Estado atual e timestamps das transições.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <Field label="Valor" value={formatMoney(intent.amount.amount_cents, intent.amount.currency)} />
          <Field label="Moeda" value={intent.amount.currency} />
          <Field label="Criado em" value={formatDateTime(intent.created_at)} />
          <Field label="Autorizado em" value={formatDateTime(intent.authorized_at)} />
          <Field label="Capturado em" value={formatDateTime(intent.captured_at)} />
          <Field label="Card token" value={intent.card_token ?? '—'} mono />
          {intent.description && (
            <Field label="Descrição" value={intent.description} className="sm:col-span-2" />
          )}
        </CardContent>
      </Card>

      <Separator />

      <Card>
        <CardHeader>
          <CardTitle>Transições</CardTitle>
          <CardDescription>
            authorize → capture. Idempotente: clicar duas vezes na mesma ação não duplica a transação.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <IntentActions id={intent.id} status={intent.status} />
        </CardContent>
      </Card>
    </div>
  );
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
      <dt className="text-xs uppercase tracking-wider text-muted-foreground">{label}</dt>
      <dd className={mono ? 'mt-1 break-all font-mono text-sm' : 'mt-1 text-sm'}>{value}</dd>
    </div>
  );
}
