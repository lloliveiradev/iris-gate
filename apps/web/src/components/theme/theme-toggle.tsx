'use client';

import { Moon, Sun } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';

type Theme = 'dark' | 'light';

const STORAGE_KEY = 'iris-theme';

export function ThemeToggle() {
  // `null` while we wait for hydration — avoids server/client mismatch.
  // The actual <html> class is already correct (ThemeScript ran pre-paint),
  // we just need to read it once on the client.
  const [theme, setTheme] = useState<Theme | null>(null);

  useEffect(() => {
    const current = document.documentElement.classList.contains('light') ? 'light' : 'dark';
    // Hydration sync from DOM (already set by pre-paint ThemeScript). Reading
    // document during render would cause SSR mismatch, so we read here.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTheme(current);
  }, []);

  function toggle() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark';
    const root = document.documentElement;
    root.classList.remove('dark', 'light');
    root.classList.add(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* localStorage blocked — toggle remains in-memory until reload */
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
