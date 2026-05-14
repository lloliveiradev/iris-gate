import type { Metadata, Viewport } from 'next';
import type { ReactNode } from 'react';
import { GeistSans } from 'geist/font/sans';
import { GeistMono } from 'geist/font/mono';
import { Fraunces, Instrument_Serif } from 'next/font/google';
import { NextIntlClientProvider } from 'next-intl';
import { getLocale, getMessages } from 'next-intl/server';
import { ThemeScript } from '@/components/theme/theme-script';
import './globals.css';

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

export const metadata: Metadata = {
  metadataBase: new URL(process.env.IRIS_SITE_URL ?? 'http://localhost:3000'),
  title: {
    default: 'IrisGate — Payment infrastructure built for Brazil',
    template: '%s · IrisGate',
  },
  description:
    'Payment gateway with first-class fraud detection, idempotent APIs, and a contract-first architecture.',
  manifest: '/manifest.json',
  icons: {
    icon: [{ url: '/icon.png', type: 'image/png' }],
    apple: [{ url: '/apple-icon.png', type: 'image/png' }],
  },
  openGraph: {
    title: 'IrisGate',
    description: 'Modern payment infrastructure for Brazilian merchants.',
    type: 'website',
    images: [{ url: '/opengraph-image.png' }],
  },
  twitter: {
    card: 'summary_large_image',
    title: 'IrisGate',
    description: 'Modern payment infrastructure for Brazilian merchants.',
    images: ['/twitter-image.png'],
  },
};

// Themed browser chrome — matches brand-ink (dark default) / brand-cream (light).
export const viewport: Viewport = {
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#EEE8B2' },
    { media: '(prefers-color-scheme: dark)', color: '#081B1B' },
  ],
  width: 'device-width',
  initialScale: 1,
};

// Map our short locale codes to a more SEO-friendly BCP-47 tag.
const htmlLang: Record<string, string> = {
  pt: 'pt-BR',
  en: 'en',
  es: 'es',
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  const locale = await getLocale();
  const messages = await getMessages();

  return (
    <html
      lang={htmlLang[locale] ?? locale}
      className={`dark ${GeistSans.variable} ${GeistMono.variable} ${fraunces.variable} ${instrumentSerif.variable}`}
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
