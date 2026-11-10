'use client';

import * as React from 'react';
import {
  flexRender,
  useTable,
  type ColumnDef,
  type RowData,
  type TableState,
} from '@tanstack/react-table';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { dataTableFeatures, type DataTableFeatures } from './features';
import { DataTableEmptyState } from './empty-state';
import { DataTablePagination } from './pagination';
import { DataTableToolbar, type ToolbarConfig } from './toolbar';

export interface DataTableProps<TData extends RowData> {
  data: TData[];
  columns: ColumnDef<DataTableFeatures, TData>[];
  initialState?: Partial<TableState<DataTableFeatures>>;
  toolbar?: ToolbarConfig;
  isLoading?: boolean;
  /** Shown when `data` is empty AND no filters/search applied. */
  emptyState?: React.ReactNode;
  /** Shown when filtered rows is 0 but data is non-empty. */
  emptyResults?: React.ReactNode;
  /** Skeleton row count while loading (defaults to current page size). */
  skeletonRows?: number;
}

export function DataTable<TData extends RowData>({
  data,
  columns,
  initialState,
  toolbar,
  isLoading,
  emptyState,
  emptyResults,
  skeletonRows,
}: DataTableProps<TData>) {
  // The table owns its state; the default selector re-renders on every change.
  const table = useTable({
    features: dataTableFeatures,
    data,
    columns,
    initialState,
    enableRowSelection: true,
  });

  const filteredRows = table.getRowModel().rows;
  const totalDataRows = data.length;
  const hasActiveFilters = table.state.columnFilters.length > 0 || !!table.state.globalFilter;

  const skeletonCount = skeletonRows ?? table.state.pagination.pageSize;
  const visibleColumnCount = table.getVisibleLeafColumns().length;

  return (
    <div>
      {toolbar ? <DataTableToolbar table={table} config={toolbar} /> : null}

      <div className="border-t">
        <Table>
          <TableHeader className="bg-muted/30">
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="hover:bg-transparent">
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    style={{
                      width: header.column.columnDef.size
                        ? `${header.column.columnDef.size}px`
                        : undefined,
                    }}
                    className={cn(
                      'h-10 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground',
                      'first:pl-6 last:pr-6',
                    )}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: skeletonCount }).map((_, i) => (
                <TableRow key={`sk-${i}`} className="hover:bg-transparent">
                  {Array.from({ length: visibleColumnCount }).map((__, j) => (
                    <TableCell key={`sk-${i}-${j}`} className="px-3 first:pl-6 last:pr-6">
                      <Skeleton className="h-4 w-full max-w-[180px]" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : filteredRows.length > 0 ? (
              filteredRows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                  className="hover:bg-secondary/5"
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      className="px-3 py-3 first:pl-6 last:pr-6"
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={visibleColumnCount} className="p-0">
                  {totalDataRows === 0 || !hasActiveFilters
                    ? (emptyState ?? <DataTableEmptyState variant="no-data" />)
                    : (emptyResults ?? <DataTableEmptyState variant="no-results" />)}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <DataTablePagination table={table} />
    </div>
  );
}
