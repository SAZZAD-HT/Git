import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import type { Repo } from '../api/types';
import { formatTime } from './formErrors';

/** A list of repositories with visibility badge, description and last update. */
export function RepoList({ repos, empty }: { repos: Repo[]; empty?: ReactNode }) {
  if (repos.length === 0) return <>{empty}</>;
  return (
    <ul className="repo-list">
      {repos.map((r) => (
        <li key={r.id} className="panel">
          <div className="repo-line">
            <Link to={`/${r.owner}/${r.name}`} className="repo-link">{r.full_name}</Link>
            <span className={`badge ${r.private ? 'badge-private' : 'badge-public'}`}>{r.private ? 'Private' : 'Public'}</span>
          </div>
          {r.description && <p className="muted">{r.description}</p>}
          <p className="muted small">{r.empty ? 'Empty' : `Updated ${formatTime(r.updated_at)}`}</p>
        </li>
      ))}
    </ul>
  );
}
