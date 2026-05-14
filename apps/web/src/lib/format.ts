import { useFormatter } from 'next-intl';
import type { Locale } from '@/i18n/config';

/**
 * Map our short locale codes to BCP-47 tags that Intl APIs understand precisely.
 * 'pt' → 'pt-BR' so currency/date defaults match the Brazilian conventions the
 * product was originally tuned to. Other locales pass through.
 */
const intlLocale: Record<string, string> = {
  pt: 'pt-BR',
  en: 'en-US',
  es: 'es-MX',
};

function resolveIntlLocale(locale: string | undefined): string {
  if (!locale) return 'pt-BR';
  return intlLocale[locale] ?? locale;
}

// ---------------------------------------------------------------------------
// Legacy helpers (kept for back-compat with existing call sites).
//
// They default to pt-BR — matching the pre-i18n behavior — so any caller that
// has not yet been migrated keeps rendering correctly. Migrate call sites to
// the locale-aware variants below at your own pace.
// ---------------------------------------------------------------------------

export function formatMoney(amountCents: number, currency: string): string {
  return formatMoneyServer('pt', amountCents, currency);
}

export function formatDateTime(value: string | Date | undefined | null): string {
  return formatDateTimeServer('pt', value);
}

// ---------------------------------------------------------------------------
// Server helpers — accept an explicit locale. Use in server components /
// server actions when the locale has already been resolved (e.g. via
// `getLocale()` from 'next-intl/server').
// ---------------------------------------------------------------------------

export function formatMoneyServer(
  locale: Locale | string | undefined,
  amountCents: number,
  currency: string,
): string {
  try {
    return new Intl.NumberFormat(resolveIntlLocale(locale), {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
    }).format(amountCents / 100);
  } catch {
    return `${(amountCents / 100).toFixed(2)} ${currency}`;
  }
}

export function formatDateTimeServer(
  locale: Locale | string | undefined,
  value: string | Date | undefined | null,
): string {
  if (!value) return '—';
  const d = typeof value === 'string' ? new Date(value) : value;
  if (Number.isNaN(d.getTime())) return '—';
  return new Intl.DateTimeFormat(resolveIntlLocale(locale), {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(d);
}

// ---------------------------------------------------------------------------
// Client hooks — wrap next-intl's useFormatter so call sites stay terse and
// readers don't have to remember the Intl option spelling.
// ---------------------------------------------------------------------------

export function useFormatMoney() {
  const f = useFormatter();
  return (amountCents: number, currency: string): string => {
    try {
      return f.number(amountCents / 100, {
        style: 'currency',
        currency,
        minimumFractionDigits: 2,
      });
    } catch {
      return `${(amountCents / 100).toFixed(2)} ${currency}`;
    }
  };
}

export function useFormatDateTime() {
  const f = useFormatter();
  return (value: string | Date | undefined | null): string => {
    if (!value) return '—';
    const d = typeof value === 'string' ? new Date(value) : value;
    if (Number.isNaN(d.getTime())) return '—';
    return f.dateTime(d, { dateStyle: 'short', timeStyle: 'short' });
  };
}

// ---------------------------------------------------------------------------
// Status variant — unchanged. Style, not language.
// ---------------------------------------------------------------------------

export type StatusVariant =
  | 'default'
  | 'success'
  | 'warning'
  | 'destructive'
  | 'secondary'
  | 'info';

const STATUS_VARIANT: Record<string, StatusVariant> = {
  pending: 'info',
  authorized: 'warning',
  captured: 'success',
  refunded: 'info',
  canceled: 'secondary',
  failed: 'destructive',
};

export function statusVariant(status: string): StatusVariant {
  return STATUS_VARIANT[status] ?? 'default';
}
