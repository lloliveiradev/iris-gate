export const locales = ['pt', 'en', 'es'] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = 'pt';

export const LOCALE_COOKIE = 'NEXT_LOCALE';

/**
 * Display name of each locale in its own language. No flags — neutral by design.
 */
export const localeLabels: Record<Locale, string> = {
  pt: 'Português',
  en: 'English',
  es: 'Español',
};

/**
 * Pure helper: type-guards a candidate string into a Locale.
 */
export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && (locales as readonly string[]).includes(value);
}

/**
 * Pure helper: pick the first locale in our list that matches the given
 * Accept-Language header, by primary subtag. Falls back to defaultLocale.
 *
 * Implementation note: kept minimal on purpose — RFC 7231 quality-value
 * sorting + a primary-subtag match. We avoid pulling negotiator here to
 * keep the middleware bundle small.
 */
export function pickLocaleFromAcceptLanguage(header: string | null | undefined): Locale {
  if (!header) return defaultLocale;
  const candidates = header
    .split(',')
    .map((raw) => {
      const [tag, ...params] = raw.trim().split(';');
      let q = 1;
      for (const p of params) {
        const [k, v] = p.trim().split('=');
        if (k === 'q') {
          const parsed = Number.parseFloat(v ?? '');
          if (Number.isFinite(parsed)) q = parsed;
        }
      }
      return { tag: tag.trim().toLowerCase(), q };
    })
    .filter((c) => c.tag.length > 0 && c.q > 0)
    .sort((a, b) => b.q - a.q);

  for (const { tag } of candidates) {
    const primary = tag.split('-')[0];
    if (isLocale(primary)) return primary;
  }
  return defaultLocale;
}
