'use client';

import type { ColumnDef } from '@tanstack/react-table';
import type { PaymentIntent } from '@iris-gate/sdk-web';
import { Badge } from '@/components/ui/badge';
import { DataTableColumnHeader } from '@/components/data-table/column-header';
import { formatDateTime, formatMoney, statusVariant } from '@/lib/format';
import { PaymentIntentRowActions } from './row-actions';

export const paymentIntentColumns: ColumnDef<PaymentIntent>[] = [
  {
    accessorKey: 'id',
    meta: { label: 'ID' },
    header: ({ column }) => <DataTableColumnHeader column={column} title="ID" />,
    cell: ({ row }) => (
      <span className="font-mono text-xs text-muted-foreground">{row.original.id}</span>
    ),
    enableGlobalFilter: true,
  },
  {
    id: 'amount',
    meta: { label: 'Valor' },
    accessorFn: (row) => row.amount.amount_cents,
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title="Valor" align="right" />
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
    meta: { label: 'Status' },
    header: ({ column }) => <DataTableColumnHeader column={column} title="Status" />,
    cell: ({ row }) => (
      <Badge variant={statusVariant(row.original.status)} dot>
        {row.original.status}
      </Badge>
    ),
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
    meta: { label: 'Criado' },
    header: ({ column }) => <DataTableColumnHeader column={column} title="Criado" />,
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
    header: () => <span className="sr-only">Ações</span>,
    cell: ({ row }) => (
      <div className="flex justify-end">
        <PaymentIntentRowActions intent={row.original} />
      </div>
    ),
    size: 56,
  },
];
