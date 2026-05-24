'use client';

import { useTranslations } from 'next-intl';
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';
import type { Column } from '@tanstack/react-table';
import { cn } from '@/lib/utils';

interface DataTableColumnHeaderProps<TData, TValue> extends React.HTMLAttributes<HTMLDivElement> {
  column: Column<TData, TValue>;
  title: string;
  align?: 'left' | 'right' | 'center';
}

export function DataTableColumnHeader<TData, TValue>({
  column,
  title,
  align = 'left',
  className,
}: DataTableColumnHeaderProps<TData, TValue>) {
  const t = useTranslations('data_table.columns');

  if (!column.getCanSort()) {
    return (
      <div
        className={cn(
          'text-xs font-medium text-muted-foreground',
          align === 'right' && 'text-right',
          align === 'center' && 'text-center',
          className,
        )}
      >
        {title}
      </div>
    );
  }

  const sorted = column.getIsSorted();

  return (
    <button
      type="button"
      onClick={() => {
        if (sorted === false) column.toggleSorting(false);
        else if (sorted === 'asc') column.toggleSorting(true);
        else column.clearSorting();
      }}
      className={cn(
        'inline-flex h-8 items-center gap-1.5 rounded-md text-xs font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 cursor-pointer',
        align === 'right' && 'flex-row-reverse ml-auto',
        align === 'center' && 'mx-auto',
        className,
      )}
      aria-label={t('sort_by', { title })}
    >
      <span>{title}</span>
      {sorted === 'asc' ? (
        <ArrowUp className="size-3.5" aria-hidden />
      ) : sorted === 'desc' ? (
        <ArrowDown className="size-3.5" aria-hidden />
      ) : (
        <ArrowUpDown className="size-3.5 opacity-50" aria-hidden />
      )}
    </button>
  );
}
