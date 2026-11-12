import createMiddleware from 'next-intl/middleware';
import { routing } from './i18n/routing';

export default createMiddleware(routing);

export const config = {
  // Skip API routes, _next assets, _vercel internals, and any path with a dot
  // (so static files at /icon.png, /favicon.ico, /opengraph-image.png pass through).
  matcher: ['/((?!api|_next|_vercel|.*\\..*).*)'],
};
