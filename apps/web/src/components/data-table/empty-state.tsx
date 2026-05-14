'use client';

import type { ReactNode } from 'react';

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
  const defaultTitle =
    variant === 'no-data' ? 'Nenhuma intent ainda.' : 'Nenhum resultado para sua busca.';
  const defaultDescription =
    variant === 'no-data'
      ? 'Crie sua primeira intent para começar a coletar autorizações e capturas.'
      : 'Tente ajustar os filtros ou limpar a busca.';

  return (
    <div className="flex flex-col items-center gap-2 px-6 py-16 text-center">
      <h3 className="font-serif text-base text-foreground">{title ?? defaultTitle}</h3>
      <p className="max-w-prose text-sm text-muted-foreground">
        {description ?? defaultDescription}
      </p>
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
