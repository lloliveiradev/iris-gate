import Image from 'next/image';
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { ShieldCheck, Workflow, Zap } from 'lucide-react';
import { getTranslations } from 'next-intl/server';
import { ThemeToggle } from '@/components/theme/theme-toggle';
import { LocaleSwitcher } from '@/components/i18n/locale-switcher';
import { getSession } from '@/lib/session';
import { LoginForm } from './login-form';

export const dynamic = 'force-dynamic';

export default async function LoginPage() {
  const session = await getSession();
  if (session.apiKey) redirect('/dashboard');

  const tBrand = await getTranslations('login.brand_panel');
  const tForm = await getTranslations('login.form');

  return (
    <div className="grid min-h-screen lg:grid-cols-[1.1fr_1fr]">
      {/* Brand panel — always dark for consistent identity */}
      <aside className="dark relative hidden flex-col justify-between overflow-hidden bg-background p-12 text-foreground lg:flex">
        <div
          aria-hidden
          className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_top_left,_oklch(0.32_0.022_174)_0%,_oklch(0.18_0.012_195)_55%,_oklch(0.18_0.012_195)_100%)]"
        />
        <div
          aria-hidden
          className="absolute -right-32 -top-32 -z-10 size-[480px] rounded-full bg-secondary/10 blur-3xl"
        />
        <div
          aria-hidden
          className="absolute -bottom-40 -left-20 -z-10 size-[420px] rounded-full bg-primary/15 blur-3xl"
        />

        <Link
          href="/"
          className="flex items-center gap-2.5 text-base font-semibold tracking-tight"
        >
          <Image
            src="/icon.png"
            alt=""
            width={64}
            height={64}
            className="size-9 rounded-md object-contain"
          />
          <span>IrisGate</span>
        </Link>

        <div className="flex flex-col gap-10">
          <div className="flex flex-col gap-6">
            <span className="inline-flex w-fit items-center gap-2 rounded-full border border-secondary/40 bg-secondary/10 px-3 py-1 text-xs font-medium uppercase tracking-wider text-secondary">
              <span className="size-1.5 rounded-full bg-secondary" />
              {tBrand('console_badge')}
            </span>
            <h1 className="text-balance text-4xl tracking-tight xl:text-5xl">
              <span className="font-display italic">{tBrand('headline_pre')}</span>{' '}
              <span className="font-serif font-normal">{tBrand('headline_mid')}</span>{' '}
              <span className="font-display italic text-brand-copper">
                {tBrand('headline_post')}
              </span>
            </h1>
            <p className="max-w-md text-pretty text-base text-muted-foreground">
              {tBrand('body')}
            </p>
          </div>

          <ul className="grid gap-4 text-sm">
            {[
              { icon: Zap, label: tBrand('bullets.latency') },
              { icon: ShieldCheck, label: tBrand('bullets.compliance') },
              { icon: Workflow, label: tBrand('bullets.idempotency') },
            ].map(({ icon: Icon, label }) => (
              <li key={label} className="flex items-center gap-3 text-foreground/90">
                <span className="grid size-8 place-items-center rounded-md bg-secondary/15 ring-1 ring-secondary/30">
                  <Icon className="size-4 text-secondary" />
                </span>
                {label}
              </li>
            ))}
          </ul>
        </div>

        <p className="text-xs text-muted-foreground">
          {tBrand('copyright', { year: new Date().getFullYear() })}
        </p>
      </aside>

      {/* Form panel */}
      <main className="relative flex flex-col bg-background">
        <div className="flex items-center justify-between px-6 py-6 lg:px-10">
          <Link
            href="/"
            className="flex items-center gap-2 text-sm font-semibold tracking-tight lg:invisible"
          >
            <Image
              src="/icon.png"
              alt=""
              width={56}
              height={56}
              className="size-7 rounded object-contain"
            />
            <span>IrisGate</span>
          </Link>
          <div className="flex items-center gap-1">
            <LocaleSwitcher />
            <ThemeToggle />
          </div>
        </div>

        <div className="flex flex-1 items-center justify-center px-6 pb-12 lg:px-10">
          <div className="w-full max-w-md space-y-8">
            <div className="space-y-2">
              <p className="text-sm font-medium uppercase tracking-wider text-secondary">
                {tForm('eyebrow')}
              </p>
              <h2 className="font-serif text-3xl font-normal tracking-tight text-foreground">
                {tForm('title')}
              </h2>
              <p className="text-sm text-muted-foreground">
                {tForm.rich('description', {
                  code: (chunks) => (
                    <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground">
                      {chunks}
                    </code>
                  ),
                })}
              </p>
            </div>

            <LoginForm />

            <p className="text-xs text-muted-foreground">
              {tForm('footer_no_key')}{' '}
              <Link
                href="/"
                className="font-medium text-foreground underline-offset-4 hover:underline"
              >
                {tForm('footer_back_link')}
              </Link>{' '}
              {tForm('footer_back_suffix')}
            </p>
          </div>
        </div>
      </main>
    </div>
  );
}
