'use client';

import { Search, X } from 'lucide-react';
import type { Table } from '@tanstack/react-table';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DataTableFacetedFilter, type FacetedFilterOption } from './faceted-filter';
import { DataTableViewOptions } from './view-options';

export interface ToolbarFilterConfig {
  /** Column id whose filterValue this control manipulates. */
  columnId: string;
  /** Display title on the trigger. */
  title: string;
  /** Optional pre-defined options. Falls back to faceted unique values. */
  options?: FacetedFilterOption[];
}

export interface ToolbarConfig {
  /** Render a global-search input. */
  search?: boolean;
  /** Placeholder for the search input. */
  searchPlaceholder?: string;
  /** Faceted filter controls. */
  filters?: ToolbarFilterConfig[];
  /** Render the column-visibility toggle. */
  viewOptions?: boolean;
}

interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  config: ToolbarConfig;
}

export function DataTableToolbar<TData>({ table, config }: DataTableToolbarProps<TData>) {
  const isFiltered =
    table.getState().columnFilters.length > 0 || !!table.getState().globalFilter;

  const showSearch = config.search;
  const filters = config.filters ?? [];
  const showViewOptions = config.viewOptions;

  if (!showSearch && filters.length === 0 && !showViewOptions) return null;

  return (
    <div className="flex flex-wrap items-center gap-2 px-6 py-4">
      {showSearch && (
        <div className="relative w-full max-w-xs">
          <Search
            aria-hidden
            className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            value={(table.getState().globalFilter as string) ?? ''}
            onChange={(e) => table.setGlobalFilter(e.target.value)}
            placeholder={config.searchPlaceholder ?? 'Buscar…'}
            className="h-9 pl-9"
            aria-label="Buscar"
          />
        </div>
      )}

      {filters.map((f) => (
        <DataTableFacetedFilter
          key={f.columnId}
          column={table.getColumn(f.columnId)}
          title={f.title}
          options={f.options}
        />
      ))}

      {isFiltered && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            table.resetColumnFilters();
            table.setGlobalFilter('');
          }}
          className="h-9 px-2"
        >
          Limpar
          <X className="size-4" aria-hidden />
        </Button>
      )}

      {showViewOptions && <DataTableViewOptions table={table} />}
    </div>
  );
}
