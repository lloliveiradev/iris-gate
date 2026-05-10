import Image from 'next/image';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

export function Hero() {
  return (
    <section className="relative overflow-hidden">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_top,_var(--accent),_transparent_70%)]" />
      <div className="container mx-auto grid gap-12 px-6 py-20 md:py-28 lg:grid-cols-2 lg:items-center">
        <div className="flex flex-col gap-6">
          <Badge variant="outline" className="w-fit">
            <span className="mr-2 size-1.5 rounded-full bg-emerald-500" />
            Now in private beta
          </Badge>
          <h1 className="text-balance text-4xl font-semibold tracking-tight md:text-5xl lg:text-6xl">
            Payment infrastructure that ships in production, not in slides.
          </h1>
          <p className="max-w-xl text-pretty text-base text-muted-foreground md:text-lg">
            IrisGate gives you idempotent payment intents, real-time fraud assessment,
            and a contract-first SDK so the gateway never drifts from your backend.
          </p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Button asChild size="lg">
              <Link href="/login">Start integrating</Link>
            </Button>
            <Button asChild variant="outline" size="lg">
              <Link href="#integration">See how it fits</Link>
            </Button>
          </div>
          <ul className="mt-2 flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground">
            <li>● PCI-DSS aligned</li>
            <li>● Idempotency on every mutation</li>
            <li>● OpenAPI + AsyncAPI as source of truth</li>
          </ul>
        </div>
        <div className="relative">
          <div className="absolute inset-0 -z-10 rounded-2xl bg-gradient-to-tr from-primary/10 via-accent to-background blur-2xl" />
          <div className="overflow-hidden rounded-2xl border bg-card shadow-2xl">
            <Image
              src="/hero.png"
              alt="IrisGate dashboard preview"
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
