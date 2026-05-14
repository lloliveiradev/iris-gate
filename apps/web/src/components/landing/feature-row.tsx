import Image from 'next/image';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export interface FeatureRowProps {
  id?: string;
  eyebrow: string;
  title: string;
  /** Accepts rich nodes so callers can use `t.rich(...)` for inline markup. */
  body: ReactNode;
  bullets?: string[];
  image: string;
  imageAlt: string;
  reverse?: boolean;
}

export function FeatureRow({
  id,
  eyebrow,
  title,
  body,
  bullets,
  image,
  imageAlt,
  reverse,
}: FeatureRowProps) {
  return (
    <section id={id} className="container mx-auto px-6 py-20 md:py-28">
      <div
        className={cn(
          'grid items-center gap-12 lg:grid-cols-2',
          reverse && 'lg:[&>div:first-child]:order-2',
        )}
      >
        <div className="flex flex-col gap-4">
          <p className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
            {eyebrow}
          </p>
          <h2 className="text-balance font-serif text-3xl font-normal tracking-tight text-foreground md:text-4xl">
            {title}
          </h2>
          <p className="text-pretty text-base text-muted-foreground md:text-lg">{body}</p>
          {bullets && bullets.length > 0 && (
            <ul className="mt-2 flex flex-col gap-3 text-sm">
              {bullets.map((b) => (
                <li key={b} className="flex items-start gap-3">
                  <span className="mt-0.5 grid size-5 shrink-0 place-items-center rounded-full bg-secondary/15 ring-1 ring-secondary/30">
                    <span className="size-1.5 rounded-full bg-secondary" />
                  </span>
                  <span className="text-foreground/90">{b}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
        <div className="relative">
          <div className="absolute inset-0 -z-10 rounded-2xl bg-gradient-to-br from-primary/5 to-transparent blur-2xl" />
          <div className="overflow-hidden rounded-2xl border bg-card shadow-xl">
            <Image
              src={image}
              alt={imageAlt}
              width={1200}
              height={780}
              className="h-auto w-full"
              loading="eager"
            />
          </div>
        </div>
      </div>
    </section>
  );
}
