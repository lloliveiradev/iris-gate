'use client';

import type { ReactNode } from 'react';
import { useTranslations } from 'next-intl';

interface DataTableEmptyStateProps {
  variant: 'no-data' | 'no-results';
  title?: string;
  description?: string;
  action?: ReactNode;
}

export function DataTableEmptyState({
  variant,
  title,
  description,
  action,
}: DataTableEmptyStateProps) {
  const t = useTranslations(
    variant === 'no-data' ? 'data_table.empty.no_data' : 'data_table.empty.no_results',
  );

  return (
    <div className="flex flex-col items-center gap-2 px-6 py-16 text-center">
      <h3 className="font-serif text-base text-foreground">{title ?? t('title')}</h3>
      <p className="max-w-prose text-sm text-muted-foreground">{description ?? t('description')}</p>
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
