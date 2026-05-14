import Image from 'next/image';
import Link from 'next/link';
import { getTranslations } from 'next-intl/server';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

export async function Hero() {
  const t = await getTranslations('landing.hero');

  const trust = [
    t('trust.pci'),
    t('trust.idempotency'),
    t('trust.contracts'),
  ];

  return (
    <section className="relative overflow-hidden">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_top,_var(--accent),_transparent_70%)]" />
      <div className="container mx-auto grid gap-12 px-6 py-20 md:py-28 lg:grid-cols-2 lg:items-center">
        <div className="flex flex-col gap-6">
          <Badge
            variant="outline"
            className="w-fit gap-2 border-secondary/40 bg-secondary/10 text-foreground"
          >
            <span className="size-1.5 rounded-full bg-secondary shadow-[0_0_0_3px_oklch(0.67_0.105_65/0.22)]" />
            {t('badge')}
          </Badge>
          <h1 className="text-balance text-5xl tracking-tight md:text-6xl lg:text-7xl">
            <span className="font-display italic text-foreground">{t('title_pre')}</span>{' '}
            <span className="font-serif font-normal text-foreground">{t('title_mid')}</span>{' '}
            <span className="font-display italic text-brand-copper">{t('title_post')}</span>
          </h1>
          <p className="max-w-xl text-pretty font-sans text-base text-muted-foreground md:text-lg">
            {t.rich('body', {
              code: (chunks) => <span className="font-mono text-foreground">{chunks}</span>,
            })}
          </p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Button asChild size="lg">
              <Link href="/login">{t('cta_primary')}</Link>
            </Button>
            <Button asChild variant="outline" size="lg">
              <Link href="#integration">{t('cta_secondary')}</Link>
            </Button>
          </div>
          <ul className="mt-2 flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground">
            {trust.map((item) => (
              <li key={item} className="flex items-center gap-2">
                <span className="size-1.5 shrink-0 rounded-full bg-secondary" />
                {item}
              </li>
            ))}
          </ul>
        </div>
        <div className="relative">
          <div className="absolute inset-0 -z-10 rounded-2xl bg-gradient-to-tr from-primary/10 via-accent to-background blur-2xl" />
          <div className="overflow-hidden rounded-2xl border bg-card shadow-2xl">
            <Image
              src="/hero.png"
              alt={t('image_alt')}
              width={1280}
              height={800}
              priority
              className="h-auto w-full"
            />
          </div>
        </div>
      </div>
    </section>
  );
}
