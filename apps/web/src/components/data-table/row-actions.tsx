'use client';

import { useTranslations } from 'next-intl';
import { Ellipsis } from 'lucide-react';
import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

interface DataTableRowActionsProps {
  children: ReactNode;
  label?: string;
  align?: 'start' | 'center' | 'end';
}

export function DataTableRowActions({ children, label, align = 'end' }: DataTableRowActionsProps) {
  const t = useTranslations('data_table');

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          aria-label={label ?? t('row_actions_label')}
        >
          <Ellipsis className="size-4" aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align={align} className="w-44">
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
