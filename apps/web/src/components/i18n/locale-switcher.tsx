'use client';

import { useTransition } from 'react';
import { useLocale, useTranslations } from 'next-intl';
import { useParams } from 'next/navigation';
import { Languages } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { usePathname, useRouter } from '@/i18n/routing';
import { localeLabels, locales, type Locale } from '@/i18n/config';

/**
 * URL-segment locale switcher. On select, replaces the locale segment in the
 * current pathname and pushes the new URL. next-intl persists the choice via
 * the NEXT_LOCALE cookie during middleware processing.
 */
export function LocaleSwitcher() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useParams();
  const active = useLocale() as Locale;
  const t = useTranslations('locale_switcher');
  const [pending, startTransition] = useTransition();

  function setLocale(next: Locale) {
    if (next === active) return;
    startTransition(() => {
      // params are forwarded so dynamic segments like /payment-intents/[id] survive the swap.
      router.replace(
        // @ts-expect-error -- generic Pathname narrowing on dynamic params
        { pathname, params },
        { locale: next },
      );
    });
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t('label')}
          title={t('current', { locale: localeLabels[active] })}
          disabled={pending}
          className="size-9"
        >
          <Languages className="size-4" aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-[10rem]">
        {locales.map((loc) => (
          <DropdownMenuItem
            key={loc}
            onSelect={() => setLocale(loc)}
            data-active={loc === active ? 'true' : undefined}
            className="data-[active=true]:text-foreground"
          >
            <span className="flex-1">{localeLabels[loc]}</span>
            {loc === active && (
              <span aria-hidden className="ml-2 size-1.5 rounded-full bg-primary" />
            )}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
