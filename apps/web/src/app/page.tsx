import { getTranslations } from 'next-intl/server';
import { SiteFooter } from '@/components/site/site-footer';
import { SiteHeader } from '@/components/site/site-header';
import { Cta } from '@/components/landing/cta';
import { FeatureRow } from '@/components/landing/feature-row';
import { Hero } from '@/components/landing/hero';
import { Testimonial } from '@/components/landing/testimonial';

export default async function HomePage() {
  const t = await getTranslations('landing.features');

  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />
      <main className="flex-1">
        <Hero />

        <FeatureRow
          id="integration"
          eyebrow={t('integration.eyebrow')}
          title={t('integration.title')}
          body={t('integration.body')}
          bullets={[
            t('integration.bullets.openapi'),
            t('integration.bullets.idempotency'),
            t('integration.bullets.clients'),
          ]}
          image="/integration.png"
          imageAlt={t('integration.image_alt')}
        />

        <FeatureRow
          id="monitoring"
          eyebrow={t('monitoring.eyebrow')}
          title={t('monitoring.title')}
          body={t.rich('monitoring.body', {
            code: (chunks) => (
              <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-sm text-foreground">
                {chunks}
              </code>
            ),
          })}
          bullets={[
            t('monitoring.bullets.rules'),
            t('monitoring.bullets.exactly_once'),
            t('monitoring.bullets.dlq'),
          ]}
          image="/monitoring.png"
          imageAlt={t('monitoring.image_alt')}
          reverse
        />

        <FeatureRow
          id="reporting"
          eyebrow={t('reporting.eyebrow')}
          title={t('reporting.title')}
          body={t('reporting.body')}
          bullets={[
            t('reporting.bullets.event_store'),
            t('reporting.bullets.exports'),
            t('reporting.bullets.webhooks'),
          ]}
          image="/report.png"
          imageAlt={t('reporting.image_alt')}
        />

        <FeatureRow
          eyebrow={t('support.eyebrow')}
          title={t('support.title')}
          body={t('support.body')}
          image="/support.png"
          imageAlt={t('support.image_alt')}
          reverse
        />

        <Testimonial />

        <Cta />
      </main>
      <SiteFooter />
    </div>
  );
}
