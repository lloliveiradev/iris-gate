'use client';

import * as React from 'react';
import {
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type ColumnFiltersState,
  type InitialTableState,
  type SortingState,
  type VisibilityState,
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
import { DataTableEmptyState } from './empty-state';
import { DataTablePagination } from './pagination';
import { DataTableToolbar, type ToolbarConfig } from './toolbar';

export interface DataTableProps<TData, TValue> {
  data: TData[];
  columns: ColumnDef<TData, TValue>[];
  initialState?: InitialTableState;
  toolbar?: ToolbarConfig;
  isLoading?: boolean;
  /** Shown when `data` is empty AND no filters/search applied. */
  emptyState?: React.ReactNode;
  /** Shown when filtered rows is 0 but data is non-empty. */
  emptyResults?: React.ReactNode;
  /** Skeleton row count while loading (defaults to current page size). */
  skeletonRows?: number;
}

export function DataTable<TData, TValue>({
  data,
  columns,
  initialState,
  toolbar,
  isLoading,
  emptyState,
  emptyResults,
  skeletonRows,
}: DataTableProps<TData, TValue>) {
  const [sorting, setSorting] = React.useState<SortingState>(
    initialState?.sorting ?? [],
  );
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>(
    initialState?.columnFilters ?? [],
  );
  const [columnVisibility, setColumnVisibility] = React.useState<VisibilityState>(
    initialState?.columnVisibility ?? {},
  );
  const [globalFilter, setGlobalFilter] = React.useState<string>(
    (initialState?.globalFilter as string) ?? '',
  );
  const [rowSelection, setRowSelection] = React.useState(
    initialState?.rowSelection ?? {},
  );

  // TanStack Table internally returns non-memoizable functions; the React
  // Compiler `incompatible-library` rule fires for any call to `useReactTable`.
  // Known interaction — the library's owner is tracking it. Safe to silence.
  // eslint-disable-next-line react-hooks/incompatible-library
  const table = useReactTable({
    data,
    columns,
    initialState,
    state: {
      sorting,
      columnFilters,
      columnVisibility,
      globalFilter,
      rowSelection,
    },
    enableRowSelection: true,
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onColumnVisibilityChange: setColumnVisibility,
    onGlobalFilterChange: setGlobalFilter,
    onRowSelectionChange: setRowSelection,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
  });

  const filteredRows = table.getRowModel().rows;
  const totalDataRows = data.length;
  const hasActiveFilters =
    table.getState().columnFilters.length > 0 || !!table.getState().globalFilter;

  const skeletonCount = skeletonRows ?? table.getState().pagination.pageSize;
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
