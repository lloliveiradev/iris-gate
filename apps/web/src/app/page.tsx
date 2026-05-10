import { SiteFooter } from '@/components/site/site-footer';
import { SiteHeader } from '@/components/site/site-header';
import { Cta } from '@/components/landing/cta';
import { FeatureRow } from '@/components/landing/feature-row';
import { Hero } from '@/components/landing/hero';
import { Testimonial } from '@/components/landing/testimonial';

export default function HomePage() {
  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />
      <main className="flex-1">
        <Hero />

        <FeatureRow
          id="integration"
          eyebrow="Integration"
          title="Contract-first SDK that breaks builds, not production."
          body="Every endpoint and event is defined in OpenAPI / AsyncAPI. The TypeScript SDK is generated from the same source — rename a field on the backend and your frontend type-check fails before merge."
          bullets={[
            'OpenAPI 3.1 + AsyncAPI 2.6 published with the gateway',
            'Idempotency-Key on every mutation by default',
            'Typed clients for Web, Node and Go consumers',
          ]}
          image="/integration.png"
          imageAlt="Code editor showing the IrisGate TypeScript SDK"
        />

        <FeatureRow
          id="monitoring"
          eyebrow="Anti-fraud"
          title="Real-time risk scoring with rules you can read and tune."
          body="Suspicious accounts, unusual amounts and frequent high-value bursts are scored asynchronously through Kafka with at-least-once Inbox semantics. Decisions stream back as fraud.assessment.completed.v1 events you can route anywhere."
          bullets={[
            'Score-and-weight rules registered in Postgres (no code redeploy)',
            'Inbox dedupe + Outbox publishing for end-to-end exactly-once',
            'DLQ topic isolates terminal failures without blocking the stream',
          ]}
          image="/monitoring.png"
          imageAlt="Risk monitoring dashboard"
          reverse
        />

        <FeatureRow
          id="reporting"
          eyebrow="Reporting"
          title="Audit trails and reconciliation that your finance team trusts."
          body="Every payment intent, authorization, capture and refund is persisted with a full event history. Export reconciled balances or stream them to your ledger."
          bullets={[
            'Append-only event store with reproducible state',
            'CSV / JSON exports with idempotent re-runs',
            'Webhook delivery with HMAC and exponential retry',
          ]}
          image="/report.png"
          imageAlt="Reconciliation report"
        />

        <FeatureRow
          eyebrow="Support"
          title="Engineers on call, not a contact form."
          body="Founder-led support during onboarding. Production incidents get a Slack channel with the engineers who wrote the code, not a tier-1 queue."
          image="/support.png"
          imageAlt="Customer support team"
          reverse
        />

        <Testimonial />

        <Cta />
      </main>
      <SiteFooter />
    </div>
  );
}
