import Image from 'next/image';
import { cn } from '@/lib/utils';

export interface FeatureRowProps {
  id?: string;
  eyebrow: string;
  title: string;
  body: string;
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
          <h2 className="text-balance text-3xl font-semibold tracking-tight md:text-4xl">
            {title}
          </h2>
          <p className="text-pretty text-base text-muted-foreground md:text-lg">{body}</p>
          {bullets && bullets.length > 0 && (
            <ul className="mt-2 flex flex-col gap-2 text-sm">
              {bullets.map((b) => (
                <li key={b} className="flex items-start gap-2">
                  <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-primary" />
                  <span>{b}</span>
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
            />
          </div>
        </div>
      </div>
    </section>
  );
}
