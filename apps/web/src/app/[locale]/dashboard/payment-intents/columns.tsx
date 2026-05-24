'use client';

import { useMemo } from 'react';
import { useTranslations } from 'next-intl';
import type { ColumnDef } from '@tanstack/react-table';
import type { PaymentIntent } from '@iris-gate/sdk-web';
import { Badge } from '@/components/ui/badge';
import { DataTableColumnHeader } from '@/components/data-table/column-header';
import { statusVariant, useFormatDateTime, useFormatMoney } from '@/lib/format';
import { PaymentIntentRowActions } from './row-actions';

type StatusKey = PaymentIntent['status'];

const KNOWN_STATUSES: readonly StatusKey[] = [
  'pending',
  'authorized',
  'captured',
  'refunded',
  'canceled',
  'failed',
];

function isKnownStatus(value: string): value is StatusKey {
  return (KNOWN_STATUSES as readonly string[]).includes(value);
}

export function usePaymentIntentColumns(): ColumnDef<PaymentIntent>[] {
  const t = useTranslations('dashboard.payment_intents.columns');
  const tStatus = useTranslations('dashboard.payment_intents.statuses');
  const formatMoney = useFormatMoney();
  const formatDateTime = useFormatDateTime();

  return useMemo<ColumnDef<PaymentIntent>[]>(
    () => [
      {
        accessorKey: 'id',
        meta: { label: t('id') },
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('id')} />,
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">{row.original.id}</span>
        ),
        enableGlobalFilter: true,
      },
      {
        id: 'amount',
        meta: { label: t('amount') },
        accessorFn: (row) => row.amount.amount_cents,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('amount')} align="right" />
        ),
        cell: ({ row }) => (
          <div className="text-right font-mono font-medium tabular-nums text-foreground">
            {formatMoney(row.original.amount.amount_cents, row.original.amount.currency)}
          </div>
        ),
        sortingFn: 'basic',
        enableGlobalFilter: false,
      },
      {
        accessorKey: 'status',
        meta: { label: t('status') },
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('status')} />,
        cell: ({ row }) => {
          const raw = row.original.status;
          const label = isKnownStatus(raw) ? tStatus(raw) : raw;
          return (
            <Badge variant={statusVariant(raw)} dot>
              {label}
            </Badge>
          );
        },
        filterFn: (row, columnId, filterValue) => {
          if (!filterValue) return true;
          const list = Array.isArray(filterValue) ? filterValue : [filterValue];
          if (list.length === 0) return true;
          const cellValue = row.getValue<string>(columnId);
          return list.includes(cellValue);
        },
        enableGlobalFilter: false,
      },
      {
        accessorKey: 'created_at',
        meta: { label: t('created') },
        header: ({ column }) => <DataTableColumnHeader column={column} title={t('created')} />,
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {formatDateTime(row.original.created_at)}
          </span>
        ),
        sortingFn: (a, b) => {
          const da = new Date(a.original.created_at).getTime();
          const db = new Date(b.original.created_at).getTime();
          return da - db;
        },
        enableGlobalFilter: false,
      },
      {
        id: 'actions',
        enableHiding: false,
        enableSorting: false,
        enableGlobalFilter: false,
        header: () => <span className="sr-only">{t('actions')}</span>,
        cell: ({ row }) => (
          <div className="flex justify-end">
            <PaymentIntentRowActions intent={row.original} />
          </div>
        ),
        size: 56,
      },
    ],
    [t, tStatus, formatMoney, formatDateTime],
  );
}
