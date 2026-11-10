'use client';

import { useTranslations } from 'next-intl';
import { Search, X } from 'lucide-react';
import type { ReactTable, RowData } from '@tanstack/react-table';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DataTableFacetedFilter, type FacetedFilterOption } from './faceted-filter';
import type { DataTableFeatures } from './features';
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
  /** Placeholder for the search input. Defaults to the localized data_table label. */
  searchPlaceholder?: string;
  /** Faceted filter controls. */
  filters?: ToolbarFilterConfig[];
  /** Render the column-visibility toggle. */
  viewOptions?: boolean;
}

interface DataTableToolbarProps<TData extends RowData> {
  table: ReactTable<DataTableFeatures, TData>;
  config: ToolbarConfig;
}

export function DataTableToolbar<TData extends RowData>({
  table,
  config,
}: DataTableToolbarProps<TData>) {
  const t = useTranslations('data_table');

  const isFiltered = table.state.columnFilters.length > 0 || !!table.state.globalFilter;

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
            value={(table.state.globalFilter as string) ?? ''}
            onChange={(e) => table.setGlobalFilter(e.target.value)}
            placeholder={config.searchPlaceholder ?? t('search_placeholder')}
            className="h-9 pl-9"
            aria-label={t('search_label')}
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
          {t('clear_filters')}
          <X className="size-4" aria-hidden />
        </Button>
      )}

      {showViewOptions && <DataTableViewOptions table={table} />}
    </div>
  );
}
