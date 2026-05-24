'use client';

import { Moon, Sun } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { DEFAULT_THEME, THEME_COOKIE, type Theme } from '@/lib/theme';

function writeThemeCookie(next: Theme) {
  document.cookie = `${THEME_COOKIE}=${next}; path=/; max-age=${60 * 60 * 24 * 365}; samesite=lax`;
}

export function ThemeToggle() {
  // `null` while we wait for hydration — avoids server/client mismatch.
  // The actual <html> class is correct (server SSR + ThemeScript pre-paint),
  // we just need to read it once on the client.
  const [theme, setTheme] = useState<Theme | null>(null);

  useEffect(() => {
    const current: Theme = document.documentElement.classList.contains('light')
      ? 'light'
      : 'dark';
    // Hydration sync from DOM (already set by the cookie-driven SSR class or
    // the pre-paint ThemeScript). Reading document during render would cause
    // SSR mismatch, so we read here.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTheme(current);
  }, []);

  function toggle() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark';
    const root = document.documentElement;
    root.classList.remove('dark', 'light');
    root.classList.add(next);
    writeThemeCookie(next);
    try {
      localStorage.setItem(THEME_COOKIE, next);
    } catch {
      /* localStorage blocked — cookie alone is enough */
    }
    setTheme(next);
  }

  // Render a stable placeholder during SSR / first paint.
  if (theme === null) {
    return (
      <Button
        variant="ghost"
        size="icon"
        aria-label="Toggle theme"
        className="size-9"
        disabled
      >
        <Sun className="size-4" aria-hidden />
      </Button>
    );
  }

  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={toggle}
      aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
      title={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
      className="size-9"
    >
      {theme === 'dark' ? <Sun className="size-4" aria-hidden /> : <Moon className="size-4" aria-hidden />}
    </Button>
  );
}

// Keep DEFAULT_THEME referenced so the import stays even if the toggle is
// extended later — also helps codemods detect intent.
export { DEFAULT_THEME };
