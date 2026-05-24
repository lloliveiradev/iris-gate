import { DEFAULT_THEME, THEME_COOKIE } from '@/lib/theme';

/**
 * Inline script that runs synchronously before paint — prevents the
 * flash-of-wrong-theme on initial load.
 *
 * Reads the `iris-theme` cookie first (so it stays in sync with the value the
 * server uses for SSR class names). Falls back to `localStorage` for legacy
 * clients that wrote there before the cookie migration. Defaults to `dark`.
 */
export function ThemeScript() {
  const code = `
    (function() {
      try {
        var cookies = document.cookie ? document.cookie.split('; ') : [];
        var fromCookie = null;
        for (var i = 0; i < cookies.length; i++) {
          var parts = cookies[i].split('=');
          if (parts[0] === '${THEME_COOKIE}') {
            fromCookie = parts[1];
            break;
          }
        }
        var fromStorage = null;
        try { fromStorage = localStorage.getItem('${THEME_COOKIE}'); } catch (_) {}
        var picked = fromCookie || fromStorage;
        var theme = picked === 'light' ? 'light' : '${DEFAULT_THEME}';
        var root = document.documentElement;
        root.classList.remove('dark', 'light');
        root.classList.add(theme);
      } catch (e) {
        document.documentElement.classList.add('${DEFAULT_THEME}');
      }
    })();
  `;
  return <script dangerouslySetInnerHTML={{ __html: code }} />;
}
