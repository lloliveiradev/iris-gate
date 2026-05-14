'use client';

import { useTransition } from 'react';
import { useLocale, useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';
import { Languages } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { localeLabels, locales, LOCALE_COOKIE, type Locale } from '@/i18n/config';

/**
 * Side-effect-only helper. Lives outside the component so React Compiler's
 * immutability lint doesn't flag the `document.cookie` write as state-mutation
 * inside a render.
 */
function writeLocaleCookie(next: Locale) {
  document.cookie = `${LOCALE_COOKIE}=${next}; path=/; max-age=${60 * 60 * 24 * 365}; samesite=lax`;
}

/**
 * Cookie-based locale switcher (V1 — URL stays put).
 * On select, writes the NEXT_LOCALE cookie and triggers a server refresh so
 * server components re-render with the new locale's messages.
 */
export function LocaleSwitcher() {
  const router = useRouter();
  const active = useLocale() as Locale;
  const t = useTranslations('locale_switcher');
  const [pending, startTransition] = useTransition();

  function setLocale(next: Locale) {
    if (next === active) return;
    writeLocaleCookie(next);
    startTransition(() => {
      router.refresh();
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
