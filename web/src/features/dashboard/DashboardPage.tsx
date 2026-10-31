import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { useReadiness, useVersion } from '../../api/hooks/useServerStatus';
import type { Repo } from '../../api/types';
import { useSession } from '../../app/session';
import { RepoList } from '../../components/RepoList';
import { StatusBadge } from '../../components/StatusBadge';

/**
 * The dashboard at `/`: your repositories when signed in, plus server status.
 * Query key `['my-repos']`, invalidated when you create or delete a repository.
 */
export function DashboardPage() {
  const { user } = useSession();
  const mine = useQuery({ queryKey: ['my-repos'], queryFn: () => api<Repo[]>('/api/v1/user/repos'), enabled: !!user });

  return (
    <div className="dashboard">
      <div>
        {user ? (
          <>
            <div className="page-head">
              <h1>Your repositories</h1>
              <Link to="/new" className="button button-primary">New repository</Link>
            </div>
            {mine.isPending && <p className="muted">Loading…</p>}
            {mine.data && (
              <RepoList repos={mine.data} empty={
                <div className="panel empty-state">
                  <h2>Create your first repository</h2>
                  <p className="muted">Repositories hold your code and its history. Push to them with git, review changes with pull requests, and run pipelines.</p>
                  <Link to="/new" className="button button-primary">New repository</Link>
                </div>
              } />
            )}
          </>
        ) : (
          <>
            <h1>Welcome to Anvil</h1>
            <p className="lead">A self-hosted forge: Git hosting, pull requests and CI/CD, on Windows and Linux. <Link to="/login">Sign in</Link> to see your repositories.</p>
          </>
        )}
      </div>
      <ServerStatus />
    </div>
  );
}

/** Server readiness and build information (P1). */
function ServerStatus() {
  const version = useVersion();
  const ready = useReadiness();
  return (
    <aside>
      <section className="panel" aria-labelledby="status-title">
        <div className="panel-head">
          <h2 id="status-title">Server status</h2>
          {ready.data && <StatusBadge ok={ready.data.status === 'ready'} label={ready.data.status === 'ready' ? 'Ready' : 'Not ready'} />}
        </div>
        {ready.isError && <p className="error">Cannot load status: {ready.error.message}</p>}
        {ready.data && (
          <ul className="checks">
            {Object.entries(ready.data.checks).map(([name, result]) => (
              <li key={name}>
                <StatusBadge ok={result === 'ok'} label={result === 'ok' ? 'OK' : 'Failing'} />
                <span className="check-name">{name}</span>
                {result !== 'ok' && <span className="muted">{result}</span>}
              </li>
            ))}
          </ul>
        )}
        {version.data && (
          <dl className="facts small">
            <dt>Anvil</dt><dd>{version.data.version} <code>{version.data.commit.slice(0, 7)}</code></dd>
            <dt>Git</dt><dd>{version.data.git_version}</dd>
            <dt>Go</dt><dd>{version.data.go_version}</dd>
          </dl>
        )}
      </section>
    </aside>
  );
}
