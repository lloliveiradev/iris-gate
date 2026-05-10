export function formatMoney(amountCents: number, currency: string): string {
  try {
    return new Intl.NumberFormat('pt-BR', {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
    }).format(amountCents / 100);
  } catch {
    return `${(amountCents / 100).toFixed(2)} ${currency}`;
  }
}

export function formatDateTime(value: string | Date | undefined | null): string {
  if (!value) return '—';
  const d = typeof value === 'string' ? new Date(value) : value;
  if (Number.isNaN(d.getTime())) return '—';
  return new Intl.DateTimeFormat('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(d);
}

const STATUS_VARIANT: Record<string, 'default' | 'success' | 'warning' | 'destructive' | 'secondary'> = {
  pending: 'secondary',
  authorized: 'warning',
  captured: 'success',
  refunded: 'secondary',
  canceled: 'secondary',
  failed: 'destructive',
};

export function statusVariant(status: string): 'default' | 'success' | 'warning' | 'destructive' | 'secondary' {
  return STATUS_VARIANT[status] ?? 'default';
}
