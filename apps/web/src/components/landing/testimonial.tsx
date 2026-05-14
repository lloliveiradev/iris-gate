import Image from 'next/image';
import { getTranslations } from 'next-intl/server';
import { Card, CardContent } from '@/components/ui/card';

export async function Testimonial() {
  const t = await getTranslations('landing.testimonial');

  return (
    <section id="testimonials" className="container mx-auto px-6 py-20 md:py-24">
      <Card className="overflow-hidden border-none bg-gradient-to-br from-primary/5 via-accent/40 to-background shadow-xl">
        <CardContent className="grid gap-8 p-8 md:grid-cols-[2fr_3fr] md:p-12">
          <div className="overflow-hidden rounded-xl">
            <Image
              src="/testimonial.png"
              alt={t('image_alt')}
              width={800}
              height={800}
              className="h-full w-full object-cover"
              loading="eager"
            />
          </div>
          <figure className="flex flex-col justify-center gap-6">
            <blockquote className="text-balance text-2xl font-medium leading-relaxed md:text-3xl">
              “{t('quote')}”
            </blockquote>
            <figcaption>
              <div className="font-semibold">{t('author_name')}</div>
              <div className="text-sm text-muted-foreground">{t('author_role')}</div>
            </figcaption>
            <dl className="grid grid-cols-3 gap-6 border-t pt-6">
              <Stat value={t('stats.uptime.value')} label={t('stats.uptime.label')} />
              <Stat value={t('stats.latency.value')} label={t('stats.latency.label')} />
              <Stat value={t('stats.drift.value')} label={t('stats.drift.label')} />
            </dl>
          </figure>
        </CardContent>
      </Card>
    </section>
  );
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div>
      <dt className="text-2xl font-semibold tracking-tight">{value}</dt>
      <dd className="text-xs uppercase tracking-wider text-muted-foreground">{label}</dd>
    </div>
  );
}
