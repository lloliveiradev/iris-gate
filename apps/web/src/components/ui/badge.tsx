import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

/**
 * Status badges step outside the brand palette: states map to universal
 * semantic colors (success = emerald, warning = amber, destructive = rose,
 * info = sky) so the meaning reads instantly. Brand colors are reserved for
 * product-personality surfaces (buttons, accents).
 */
const badgeVariants = cva(
  'inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium tracking-tight transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
  {
    variants: {
      variant: {
        default:
          'border-secondary/30 bg-secondary/10 text-foreground',
        secondary:
          'border-slate-500/25 bg-slate-500/10 text-slate-800 dark:bg-slate-500/15 dark:text-slate-200',
        outline: 'border-border bg-transparent text-foreground',
        success:
          'border-emerald-500/30 bg-emerald-500/10 text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-300',
        warning:
          'border-amber-500/35 bg-amber-500/10 text-amber-800 dark:bg-amber-500/15 dark:text-amber-300',
        destructive:
          'border-rose-500/30 bg-rose-500/10 text-rose-800 dark:bg-rose-500/15 dark:text-rose-300',
        info: 'border-sky-500/30 bg-sky-500/10 text-sky-800 dark:bg-sky-500/15 dark:text-sky-300',
      },
    },
    defaultVariants: { variant: 'default' },
  },
);

type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>['variant']>;

const dotColor: Record<BadgeVariant, string> = {
  default: 'bg-secondary',
  secondary: 'bg-slate-500',
  outline: 'bg-foreground/60',
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  destructive: 'bg-rose-500',
  info: 'bg-sky-500',
};

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {
  /** Render a small filled status dot before children. Color tracks the variant. */
  dot?: boolean;
}

export function Badge({
  className,
  variant = 'default',
  dot,
  children,
  ...props
}: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props}>
      {dot && (
        <span
          aria-hidden
          className={cn(
            'size-1.5 shrink-0 rounded-full',
            dotColor[variant ?? 'default'],
          )}
        />
      )}
      {children}
    </div>
  );
}

export { badgeVariants };
