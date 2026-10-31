/** A theme choice. `system` follows the operating system's light/dark setting. */
export type Theme = 'system' | 'light' | 'dark';

const KEY = 'anvil.theme';
const ORDER: Theme[] = ['system', 'light', 'dark'];

/** Returns the saved theme, or `system` when none is saved or storage is unavailable. */
export function loadTheme(): Theme {
  try {
    const t = localStorage.getItem(KEY) as Theme | null;
    return t && ORDER.includes(t) ? t : 'system';
  } catch {
    return 'system';
  }
}

/**
 * Applies theme to the document by setting `data-theme` on <html> (absent for `system`,
 * so the CSS `prefers-color-scheme` rules apply) and saves it.
 */
export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  if (theme === 'system') root.removeAttribute('data-theme');
  else root.setAttribute('data-theme', theme);
  try {
    localStorage.setItem(KEY, theme);
  } catch {
    // Private windows may block storage; the theme still applies for this page view.
  }
}

/** Returns the theme after `t` in the cycle system → light → dark → system. */
export function nextTheme(t: Theme): Theme {
  return ORDER[(ORDER.indexOf(t) + 1) % ORDER.length];
}
