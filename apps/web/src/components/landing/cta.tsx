import Image from 'next/image';
import Link from 'next/link';
import { getTranslations } from 'next-intl/server';
import { Button } from '@/components/ui/button';

export async function Cta() {
  const t = await getTranslations('landing.cta');

  return (
    <section className="relative overflow-hidden">
      <div className="container mx-auto px-6 py-20 md:py-28">
        <div className="relative overflow-hidden rounded-3xl border bg-card shadow-xl">
          <div className="grid gap-8 lg:grid-cols-[1.4fr_1fr]">
            <div className="flex flex-col justify-center gap-6 p-10 md:p-14">
              <h2 className="text-balance text-3xl font-semibold tracking-tight md:text-4xl">
                {t('title')}
              </h2>
              <p className="text-pretty text-base text-muted-foreground md:text-lg">
                {t('body')}
              </p>
              <div className="flex flex-col gap-3 sm:flex-row">
                <Button asChild size="lg">
                  <Link href="/login">{t('primary')}</Link>
                </Button>
                <Button asChild size="lg" variant="outline">
                  <Link href="#integration">{t('secondary')}</Link>
                </Button>
              </div>
            </div>
            <div className="relative">
              <Image
                src="/cta.png"
                alt={t('image_alt')}
                width={900}
                height={900}
                className="h-full w-full object-cover"
              />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
