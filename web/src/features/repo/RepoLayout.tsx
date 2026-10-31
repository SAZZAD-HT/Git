import { useState } from 'react';
import { Link, NavLink, Outlet, useParams } from 'react-router-dom';
import { ApiError } from '../../api/client';
import { useRepo } from '../../api/hooks/useRepo';
import type { Repo } from '../../api/types';
import { NotFoundPage } from '../../components/NotFoundPage';

/** The context child routes receive through `useOutletContext`. */
export interface RepoContext {
  repo: Repo;
}

/**
 * The frame around every repository page at `/:owner/:repo/*`: name, visibility,
 * description, clone button and tabs. A 404 shows the shared Not Found page.
 */
export function RepoLayout() {
  const { owner = '', repo: name = '' } = useParams();
  const repo = useRepo(owner, name);

  if (repo.isPending) return <p className="muted">Loading…</p>;
  if (repo.isError) {
    if (repo.error instanceof ApiError && repo.error.status === 404) return <NotFoundPage />;
    return <p className="error">{repo.error.message}</p>;
  }
  const r = repo.data;
  const base = `/${r.owner}/${r.name}`;
  return (
    <>
      <div className="repo-head">
        <div>
          <h1 className="repo-title">
            <Link to={`/${r.owner}`}>{r.owner}</Link>
            <span className="muted"> / </span>
            <Link to={base}>{r.name}</Link>
            <span className={`badge ${r.private ? 'badge-private' : 'badge-public'}`}>{r.private ? 'Private' : 'Public'}</span>
          </h1>
          {r.description && <p className="repo-desc">{r.description}</p>}
        </div>
        <CloneButton url={r.clone_url} />
      </div>
      <nav className="tabs" aria-label="Repository">
        <NavLink to={base} end>Code</NavLink>
        <NavLink to={`${base}/commits`}>Commits</NavLink>
        {r.permission === 'admin' && <NavLink to={`${base}/settings`}>Settings</NavLink>}
      </nav>
      <Outlet context={{ repo: r } satisfies RepoContext} />
    </>
  );
}

/** Shows the HTTPS clone URL with a copy button and a reminder to use a token (docs/reference/09 §4). */
function CloneButton({ url }: { url: string }) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div className="menu">
      <button type="button" className="button button-primary" aria-expanded={open} onClick={() => setOpen((v) => !v)}>Clone ▾</button>
      {open && (
        <div className="menu-list clone-panel">
          <label htmlFor="clone-url" className="clone-label">Clone with HTTPS</label>
          <div className="token-row">
            <input id="clone-url" className="clone-input" readOnly value={url} onFocus={(e) => e.target.select()} />
            <button type="button" className="button" onClick={copy}>{copied ? 'Copied' : 'Copy'}</button>
          </div>
          <p className="muted small">When git asks for a password, use a <Link to="/settings/tokens">personal access token</Link>.</p>
        </div>
      )}
    </div>
  );
}
