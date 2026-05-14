'use client';

import { Check, CirclePlus } from 'lucide-react';
import type { Column } from '@tanstack/react-table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';

export interface FacetedFilterOption {
  value: string;
  label: string;
}

interface DataTableFacetedFilterProps<TData, TValue> {
  column?: Column<TData, TValue>;
  title: string;
  options?: FacetedFilterOption[];
}

export function DataTableFacetedFilter<TData, TValue>({
  column,
  title,
  options,
}: DataTableFacetedFilterProps<TData, TValue>) {
  if (!column) return null;

  const facets = column.getFacetedUniqueValues();
  const selected = new Set((column.getFilterValue() as string[]) ?? []);

  // Build options: caller-provided, else derive from facets.
  const resolved: FacetedFilterOption[] =
    options ??
    Array.from(facets.keys())
      .filter((v): v is string => typeof v === 'string')
      .sort()
      .map((value) => ({ value, label: value }));

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="h-9 border-dashed">
          <CirclePlus className="size-4" aria-hidden />
          {title}
          {selected.size > 0 && (
            <>
              <Separator orientation="vertical" className="mx-1 h-4" />
              <Badge variant="secondary" className="rounded-sm px-1 font-normal lg:hidden">
                {selected.size}
              </Badge>
              <div className="hidden gap-1 lg:flex">
                {selected.size > 2 ? (
                  <Badge variant="secondary" className="rounded-sm px-1 font-normal">
                    {selected.size} selecionados
                  </Badge>
                ) : (
                  resolved
                    .filter((o) => selected.has(o.value))
                    .map((o) => (
                      <Badge
                        key={o.value}
                        variant="secondary"
                        className="rounded-sm px-1 font-normal"
                      >
                        {o.label}
                      </Badge>
                    ))
                )}
              </div>
            </>
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-48">
        <DropdownMenuLabel>{title}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {resolved.length === 0 ? (
          <div className="px-2 py-1.5 text-xs text-muted-foreground">Sem opções</div>
        ) : (
          resolved.map((option) => {
            const isSelected = selected.has(option.value);
            return (
              <DropdownMenuItem
                key={option.value}
                onSelect={(event) => {
                  event.preventDefault();
                  const next = new Set(selected);
                  if (isSelected) next.delete(option.value);
                  else next.add(option.value);
                  const arr = Array.from(next);
                  column.setFilterValue(arr.length ? arr : undefined);
                }}
                className="gap-2"
              >
                <span
                  className={cn(
                    'flex size-4 items-center justify-center rounded-sm border border-input',
                    isSelected
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'opacity-60',
                  )}
                >
                  {isSelected && <Check className="size-3.5" aria-hidden />}
                </span>
                <span className="capitalize">{option.label}</span>
                {facets.get(option.value) !== undefined && (
                  <span className="ml-auto font-mono text-xs tabular-nums text-muted-foreground">
                    {facets.get(option.value)}
                  </span>
                )}
              </DropdownMenuItem>
            );
          })
        )}
        {selected.size > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault();
                column.setFilterValue(undefined);
              }}
              className="justify-center text-xs text-muted-foreground"
            >
              Limpar filtros
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
