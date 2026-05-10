import type { Metadata, Viewport } from 'next';
import type { ReactNode } from 'react';
import './globals.css';

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

export const viewport: Viewport = {
  themeColor: '#ffffff',
  width: 'device-width',
  initialScale: 1,
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="pt-BR" suppressHydrationWarning>
      <body className="min-h-screen bg-background text-foreground antialiased">{children}</body>
    </html>
  );
}
