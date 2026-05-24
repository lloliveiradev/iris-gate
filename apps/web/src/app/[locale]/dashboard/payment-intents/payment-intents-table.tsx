'use client';

import { useMemo } from 'react';
import { useTranslations } from 'next-intl';
import { ArrowRight, Loader2 } from 'lucide-react';
import type { PaymentIntent, PaymentIntentStatus } from '@iris-gate/sdk-web';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { DataTable } from '@/components/data-table/data-table';
import { DataTableEmptyState } from '@/components/data-table/empty-state';
import { Link } from '@/i18n/routing';
import { useListPaymentIntents } from '@/lib/queries/payment-intents';
import { usePaymentIntentColumns } from './columns';

interface PaymentIntentsTableProps {
  initialData?: PaymentIntent[];
}

export function PaymentIntentsTable({ initialData }: PaymentIntentsTableProps) {
  const t = useTranslations('dashboard.payment_intents');
  const tStatus = useTranslations('dashboard.payment_intents.statuses');
  const tTable = useTranslations('data_table');
  const columns = usePaymentIntentColumns();
  const { data, isFetching, isLoading } = useListPaymentIntents({ initialData });
  const rows = data ?? [];

  const statusOptions = useMemo<{ value: PaymentIntentStatus; label: string }[]>(
    () => [
      { value: 'pending', label: tStatus('pending') },
      { value: 'authorized', label: tStatus('authorized') },
      { value: 'captured', label: tStatus('captured') },
      { value: 'refunded', label: tStatus('refunded') },
      { value: 'canceled', label: tStatus('canceled') },
      { value: 'failed', label: tStatus('failed') },
    ],
    [tStatus],
  );

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0">
        <div className="space-y-1.5">
          <CardTitle>{t('card_title')}</CardTitle>
          <CardDescription>{t('card_description')}</CardDescription>
        </div>
        {isFetching && !isLoading ? (
          <Loader2
            className="size-4 animate-spin text-muted-foreground"
            aria-label={tTable('refreshing')}
          />
        ) : null}
      </CardHeader>
      <CardContent className="px-0 pb-0">
        <DataTable
          data={rows}
          columns={columns}
          isLoading={isLoading}
          initialState={{
            pagination: { pageIndex: 0, pageSize: 25 },
            sorting: [{ id: 'created_at', desc: true }],
          }}
          toolbar={{
            search: true,
            searchPlaceholder: t('search_placeholder'),
            filters: [
              {
                columnId: 'status',
                title: t('columns.status'),
                options: statusOptions,
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
                    {t('new_cta')}
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
