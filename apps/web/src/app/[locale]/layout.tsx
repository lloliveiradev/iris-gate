import type { Metadata, Viewport } from 'next';
import type { ReactNode } from 'react';
import { cookies } from 'next/headers';
import { notFound } from 'next/navigation';
import { GeistSans } from 'geist/font/sans';
import { GeistMono } from 'geist/font/mono';
import { Fraunces, Instrument_Serif } from 'next/font/google';
import { NextIntlClientProvider, hasLocale } from 'next-intl';
import { getMessages, getTranslations, setRequestLocale } from 'next-intl/server';
import { ThemeScript } from '@/components/theme/theme-script';
import { routing } from '@/i18n/routing';
import { htmlLang, locales, type Locale } from '@/i18n/config';
import { DEFAULT_THEME, THEME_COOKIE, isTheme } from '@/lib/theme';
import '../globals.css';

// Editorial serif for h2/h3 + card titles (variable, supports opsz + WONK + SOFT axes).
const fraunces = Fraunces({
  subsets: ['latin'],
  variable: '--font-fraunces',
  display: 'swap',
  axes: ['opsz', 'SOFT', 'WONK'],
});

// Display serif for hero h1 + marketing. Italic style gives the calligraphic feel.
const instrumentSerif = Instrument_Serif({
  subsets: ['latin'],
  weight: '400',
  style: ['normal', 'italic'],
  variable: '--font-instrument-serif',
  display: 'swap',
});

export function generateStaticParams() {
  return locales.map((locale) => ({ locale }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!hasLocale(routing.locales, locale)) return {};

  const t = await getTranslations({ locale, namespace: 'metadata.root' });
  const siteUrl = process.env.IRIS_SITE_URL ?? 'http://localhost:3000';

  // hreflang alternates so search engines surface the right locale variant.
  const languages = Object.fromEntries(
    routing.locales.map((loc) => [htmlLang[loc as Locale], `/${loc}`]),
  );

  return {
    metadataBase: new URL(siteUrl),
    title: {
      default: t('title_default'),
      template: t('title_template'),
    },
    description: t('description'),
    manifest: '/manifest.json',
    icons: {
      icon: [{ url: '/icon.png', type: 'image/png' }],
      apple: [{ url: '/apple-icon.png', type: 'image/png' }],
    },
    alternates: {
      canonical: `/${locale}`,
      languages: {
        ...languages,
        'x-default': `/${routing.defaultLocale}`,
      },
    },
    openGraph: {
      title: 'IrisGate',
      description: t('description'),
      type: 'website',
      locale: htmlLang[locale as Locale],
      images: [{ url: '/opengraph-image.png' }],
    },
    twitter: {
      card: 'summary_large_image',
      title: 'IrisGate',
      description: t('description'),
      images: ['/twitter-image.png'],
    },
  };
}

// Themed browser chrome — matches brand-ink (dark default) / brand-cream (light).
export const viewport: Viewport = {
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#EEE8B2' },
    { media: '(prefers-color-scheme: dark)', color: '#081B1B' },
  ],
  width: 'device-width',
  initialScale: 1,
};

export default async function LocaleLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!hasLocale(routing.locales, locale)) notFound();

  // Enables static rendering for this locale segment.
  setRequestLocale(locale);

  const messages = await getMessages();

  // Resolve theme from cookie so the SSR <html> class matches whatever the
  // user previously picked. Survives locale-switch re-renders that otherwise
  // would snap the class back to 'dark'.
  const cookieStore = await cookies();
  const themeCookie = cookieStore.get(THEME_COOKIE)?.value;
  const theme = isTheme(themeCookie) ? themeCookie : DEFAULT_THEME;

  return (
    <html
      lang={htmlLang[locale]}
      className={`${theme} ${GeistSans.variable} ${GeistMono.variable} ${fraunces.variable} ${instrumentSerif.variable}`}
      suppressHydrationWarning
    >
      <head>
        <ThemeScript />
      </head>
      <body className="min-h-screen bg-background text-foreground antialiased">
        <NextIntlClientProvider locale={locale} messages={messages}>
          {children}
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
