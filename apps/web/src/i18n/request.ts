import { cookies, headers } from 'next/headers';
import { getRequestConfig } from 'next-intl/server';
import {
  defaultLocale,
  isLocale,
  LOCALE_COOKIE,
  pickLocaleFromAcceptLanguage,
  type Locale,
} from './config';

/**
 * Resolve the active locale for a request.
 *
 * Order (V1 cookie-based; URL routing is a future refactor):
 * 1. NEXT_LOCALE cookie (sticky from a prior switch).
 * 2. Accept-Language header (browser preference).
 * 3. defaultLocale ('pt').
 */
async function resolveLocale(): Promise<Locale> {
  const cookieStore = await cookies();
  const fromCookie = cookieStore.get(LOCALE_COOKIE)?.value;
  if (isLocale(fromCookie)) return fromCookie;

  const headerStore = await headers();
  return pickLocaleFromAcceptLanguage(headerStore.get('accept-language'));
}

export default getRequestConfig(async () => {
  let locale: Locale;
  try {
    locale = await resolveLocale();
  } catch {
    locale = defaultLocale;
  }

  const messages = (await import(`../../messages/${locale}.json`)).default;
  return { locale, messages };
});
