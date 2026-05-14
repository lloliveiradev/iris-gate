import { NextResponse, type NextRequest } from 'next/server';
import { LOCALE_COOKIE, pickLocaleFromAcceptLanguage } from './i18n/config';

/**
 * Minimal locale-detection middleware (V1 — cookie-only, no URL routing).
 *
 * On first request, if the NEXT_LOCALE cookie is missing, parse Accept-Language,
 * pick the first supported locale (falling back to 'pt'), and set the cookie so
 * subsequent requests / the LocaleSwitcher stay in sync.
 *
 * URL-segment routing (/pt, /en, /es) is a future refactor — see docs/plans/i18n.md.
 */
export default function middleware(req: NextRequest) {
  const res = NextResponse.next();

  if (!req.cookies.get(LOCALE_COOKIE)) {
    const detected = pickLocaleFromAcceptLanguage(req.headers.get('accept-language'));
    res.cookies.set(LOCALE_COOKIE, detected, {
      path: '/',
      maxAge: 60 * 60 * 24 * 365,
      sameSite: 'lax',
    });
  }

  return res;
}

export const config = {
  matcher: ['/((?!api|_next|_vercel|.*\\..*).*)'],
};
