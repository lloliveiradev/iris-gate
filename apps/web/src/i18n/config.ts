import { routing, type Locale } from './routing';

export const locales = routing.locales;
export const defaultLocale = routing.defaultLocale;
export type { Locale };

/**
 * BCP-47 tag emitted on <html lang="…">. We keep the short ISO codes in URLs
 * (`/pt`) but render the SEO-friendly tag (`pt-BR`) on the document.
 */
export const htmlLang: Record<Locale, string> = {
  pt: 'pt-BR',
  en: 'en',
  es: 'es',
};

/**
 * Display name of each locale in its own language. No flags — neutral by design.
 */
export const localeLabels: Record<Locale, string> = {
  pt: 'Português',
  en: 'English',
  es: 'Español',
};

export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && (locales as readonly string[]).includes(value);
}
