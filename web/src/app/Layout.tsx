import { useState } from 'react';
import { Link, NavLink, Outlet } from 'react-router-dom';
import { applyTheme, loadTheme, nextTheme, type Theme } from './theme';
import { ErrorBoundary } from './ErrorBoundary';

const THEME_LABEL: Record<Theme, string> = { system: 'System theme', light: 'Light theme', dark: 'Dark theme' };

/** The app shell: top bar with brand, navigation and theme switch, and the routed page below. */
export function Layout() {
  const [theme, setTheme] = useState<Theme>(loadTheme);

  const cycleTheme = () => {
    const t = nextTheme(theme);
    applyTheme(t);
    setTheme(t);
  };

  return (
    <div className="shell">
      <a className="skip-link" href="#main">Skip to content</a>
      <header className="topbar">
        <Link to="/" className="brand" aria-label="Anvil home">
          <svg viewBox="0 0 32 32" width="24" height="24" aria-hidden="true">
            <path d="M6 12h17c0 3-2 5-5 5h-1l2 6h3v3H10v-3h3l2-6c-5 0-9-2-9-5z" fill="currentColor" />
          </svg>
          <span>Anvil</span>
        </Link>
        <nav className="topnav" aria-label="Main">
          <NavLink to="/" end>Dashboard</NavLink>
          <NavLink to="/explorer">Explorer</NavLink>
        </nav>
        <div className="topbar-end">
          <button type="button" className="icon-button" onClick={cycleTheme} title={THEME_LABEL[theme]} aria-label={`${THEME_LABEL[theme]} (change)`}>
            {theme === 'dark' ? '☾' : theme === 'light' ? '☀' : '◐'}
          </button>
        </div>
      </header>
      <main id="main" className="page">
        <ErrorBoundary>
          <Outlet />
        </ErrorBoundary>
      </main>
    </div>
  );
}
