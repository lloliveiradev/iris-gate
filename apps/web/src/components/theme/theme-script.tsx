/**
 * Inline script that runs synchronously before paint — prevents the
 * flash-of-wrong-theme on initial load.
 *
 * Reads `localStorage.theme` ('dark' | 'light'). When absent, defaults
 * to 'dark' (the brand-first stance documented in TOKENS.md §2).
 * Applies the result as the class on `<html>` before React hydrates.
 */
export function ThemeScript() {
  const code = `
    (function() {
      try {
        var stored = localStorage.getItem('iris-theme');
        var theme = stored === 'light' ? 'light' : 'dark';
        var root = document.documentElement;
        root.classList.remove('dark', 'light');
        root.classList.add(theme);
      } catch (e) {
        document.documentElement.classList.add('dark');
      }
    })();
  `;
  return <script dangerouslySetInnerHTML={{ __html: code }} />;
}
