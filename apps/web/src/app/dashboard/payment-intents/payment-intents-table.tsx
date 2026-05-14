'use client';

import Link from 'next/link';
import { ArrowRight, Loader2 } from 'lucide-react';
import type { PaymentIntent, PaymentIntentStatus } from '@iris-gate/sdk-web';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { DataTable } from '@/components/data-table/data-table';
import { DataTableEmptyState } from '@/components/data-table/empty-state';
import { useListPaymentIntents } from '@/lib/queries/payment-intents';
import { paymentIntentColumns } from './columns';

const STATUS_OPTIONS: { value: PaymentIntentStatus; label: string }[] = [
  { value: 'pending', label: 'pending' },
  { value: 'authorized', label: 'authorized' },
  { value: 'captured', label: 'captured' },
  { value: 'refunded', label: 'refunded' },
  { value: 'canceled', label: 'canceled' },
  { value: 'failed', label: 'failed' },
];

interface PaymentIntentsTableProps {
  initialData?: PaymentIntent[];
}

export function PaymentIntentsTable({ initialData }: PaymentIntentsTableProps) {
  const { data, isFetching, isLoading } = useListPaymentIntents({ initialData });
  const rows = data ?? [];

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0">
        <div className="space-y-1.5">
          <CardTitle>Histórico</CardTitle>
          <CardDescription>
            Mais recentes primeiro · use o menu de ações em cada linha.
          </CardDescription>
        </div>
        {isFetching && !isLoading ? (
          <Loader2
            className="size-4 animate-spin text-muted-foreground"
            aria-label="Atualizando"
          />
        ) : null}
      </CardHeader>
      <CardContent className="px-0 pb-0">
        <DataTable
          data={rows}
          columns={paymentIntentColumns}
          isLoading={isLoading}
          initialState={{
            pagination: { pageIndex: 0, pageSize: 25 },
            sorting: [{ id: 'created_at', desc: true }],
          }}
          toolbar={{
            search: true,
            searchPlaceholder: 'Buscar por ID…',
            filters: [
              {
                columnId: 'status',
                title: 'Status',
                options: STATUS_OPTIONS,
              },
            ],
            viewOptions: true,
          }}
          emptyState={
            <DataTableEmptyState
              variant="no-data"
              action={
                <Button asChild size="sm">
                  <Link href="/dashboard/payment-intents/new">
                    Nova intent
                    <ArrowRight className="size-4" />
                  </Link>
                </Button>
              }
            />
          }
        />
      </CardContent>
    </Card>
  );
}
