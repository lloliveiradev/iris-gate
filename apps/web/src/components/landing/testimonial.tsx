import Image from 'next/image';
import { Card, CardContent } from '@/components/ui/card';

export function Testimonial() {
  return (
    <section id="testimonials" className="container mx-auto px-6 py-20 md:py-24">
      <Card className="overflow-hidden border-none bg-gradient-to-br from-primary/5 via-accent/40 to-background shadow-xl">
        <CardContent className="grid gap-8 p-8 md:grid-cols-[2fr_3fr] md:p-12">
          <div className="overflow-hidden rounded-xl">
            <Image
              src="/testimonial.png"
              alt="Customer using IrisGate"
              width={800}
              height={800}
              className="h-full w-full object-cover"
            />
          </div>
          <figure className="flex flex-col justify-center gap-6">
            <blockquote className="text-balance text-2xl font-medium leading-relaxed md:text-3xl">
              “We replaced three legacy services with IrisGate in six weeks. Idempotent
              authorization and the contract-first SDK alone paid for the migration.”
            </blockquote>
            <figcaption>
              <div className="font-semibold">Camila Reis</div>
              <div className="text-sm text-muted-foreground">
                Head of Payments · Lumen Marketplace
              </div>
            </figcaption>
            <dl className="grid grid-cols-3 gap-6 border-t pt-6">
              <Stat value="99.99%" label="Auth uptime" />
              <Stat value="74ms" label="p95 latency" />
              <Stat value="0" label="Drift incidents" />
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
