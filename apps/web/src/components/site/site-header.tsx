import Image from 'next/image';
import { getTranslations } from 'next-intl/server';
import { Link } from '@/i18n/routing';
import { Button } from '@/components/ui/button';
import { ThemeToggle } from '@/components/theme/theme-toggle';
import { LocaleSwitcher } from '@/components/i18n/locale-switcher';

export async function SiteHeader() {
  const t = await getTranslations('site.header');

  return (
    <header className="sticky top-0 z-40 w-full border-b border-border/40 bg-background/80 backdrop-blur">
      <div className="container mx-auto flex h-16 items-center justify-between px-6">
        <Link href="/" className="flex items-center gap-2.5 font-semibold tracking-tight">
          <Image
            src="/logo_horizontal.png"
            alt=""
            width={100}
            height={100}
            priority
            className="size-auto rounded-md object-contain"
          />
        </Link>
        <nav className="hidden items-center gap-6 text-sm text-muted-foreground md:flex">
          <a className="hover:text-foreground" href="#integration">{t('nav.integration')}</a>
          <a className="hover:text-foreground" href="#monitoring">{t('nav.monitoring')}</a>
          <a className="hover:text-foreground" href="#reporting">{t('nav.reporting')}</a>
          <a className="hover:text-foreground" href="#testimonials">{t('nav.customers')}</a>
        </nav>
        <div className="flex items-center gap-2">
          <ThemeToggle />
          <LocaleSwitcher />
          <Button asChild variant="ghost" size="sm">
            <Link href="/login">{t('sign_in')}</Link>
          </Button>
          <Button asChild size="sm">
            <Link href="/login">{t('get_started')}</Link>
          </Button>
        </div>
      </div>
    </header>
  );
}
