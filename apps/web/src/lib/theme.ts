export type Theme = 'dark' | 'light';

export const THEME_COOKIE = 'iris-theme';
export const DEFAULT_THEME: Theme = 'dark';

export function isTheme(value: unknown): value is Theme {
  return value === 'dark' || value === 'light';
}
