import { useEffect, useRef, useState } from 'react';
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { applyTheme, loadTheme, nextTheme, type Theme } from './theme';
import { ErrorBoundary } from './ErrorBoundary';
import { useSession } from './session';

const THEME_LABEL: Record<Theme, string> = { system: 'System theme', light: 'Light theme', dark: 'Dark theme' };

/** The app shell: top bar with brand, navigation, theme switch and user menu, and the routed page below. */
export function Layout() {
  const [theme, setTheme] = useState<Theme>(loadTheme);
  const { user } = useSession();
  const location = useLocation();

  const cycleTheme = () => {
    const t = nextTheme(theme);
    applyTheme(t);
    setTheme(t);
  };

  const onAuthPage = location.pathname === '/login' || location.pathname === '/register';

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
          {user?.is_admin && <NavLink to="/admin/users">Admin</NavLink>}
        </nav>
        <div className="topbar-end">
          <button type="button" className="icon-button" onClick={cycleTheme} title={THEME_LABEL[theme]} aria-label={`${THEME_LABEL[theme]} (change)`}>
            {theme === 'dark' ? '☾' : theme === 'light' ? '☀' : '◐'}
          </button>
          {user ? <UserMenu username={user.username} /> : user === null && !onAuthPage && (
            <Link className="button button-primary" to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`}>Sign in</Link>
          )}
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

/** The signed-in user's menu: settings and sign out. Closes on Escape or outside click. */
function UserMenu({ username }: { username: string }) {
  const { signOut } = useSession();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => { if (!ref.current?.contains(e.target as Node)) setOpen(false); };
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', onDoc);
    document.addEventListener('keydown', onKey);
    return () => { document.removeEventListener('mousedown', onDoc); document.removeEventListener('keydown', onKey); };
  }, [open]);

  const doSignOut = async () => {
    setOpen(false);
    await signOut();
    navigate('/');
  };

  return (
    <div className="menu" ref={ref}>
      <button type="button" className="menu-button" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <span className="avatar" aria-hidden="true">{username.slice(0, 1).toUpperCase()}</span>
        <span className="menu-name">{username}</span>
      </button>
      {open && (
        <div className="menu-list" role="menu">
          <Link role="menuitem" to="/settings" onClick={() => setOpen(false)}>Settings</Link>
          <Link role="menuitem" to="/settings/tokens" onClick={() => setOpen(false)}>Access tokens</Link>
          <button role="menuitem" type="button" onClick={doSignOut}>Sign out</button>
        </div>
      )}
    </div>
  );
}
