import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useSession } from './session';

/**
 * Renders children only for signed-in users (and, with `admin`, site admins). Signed-out
 * visitors are sent to /login?next=<current url> (docs/reference/09 §7).
 */
export function RequireAuth({ children, admin = false }: { children: ReactNode; admin?: boolean }) {
  const { user } = useSession();
  const location = useLocation();
  if (user === undefined) return <p className="muted">Loading…</p>;
  if (user === null) {
    const next = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?next=${next}`} replace />;
  }
  if (admin && !user.is_admin) {
    return (
      <div className="panel">
        <h1>Administrators only</h1>
        <p className="muted">You need site administrator rights to see this page.</p>
      </div>
    );
  }
  return <>{children}</>;
}
