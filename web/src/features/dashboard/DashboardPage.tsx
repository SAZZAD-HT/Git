import { useReadiness, useVersion } from '../../api/hooks/useServerStatus';
import { StatusBadge } from '../../components/StatusBadge';

/**
 * The dashboard at `/`. In P1 it shows the server's health; from P3 it lists your
 * repositories and recent activity (docs/reference/09 §2).
 */
export function DashboardPage() {
  const version = useVersion();
  const ready = useReadiness();

  return (
    <>
      <h1>Welcome to Anvil</h1>
      <p className="lead">Your self-hosted forge is running. Repositories, pull requests and pipelines arrive in the next phases.</p>

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
      </section>

      <section className="panel" aria-labelledby="version-title">
        <h2 id="version-title">Build</h2>
        {version.isPending && <p className="muted">Loading…</p>}
        {version.isError && <p className="error">Cannot load version: {version.error.message}</p>}
        {version.data && (
          <dl className="facts">
            <dt>Anvil</dt><dd>{version.data.version}</dd>
            <dt>Commit</dt><dd><code>{version.data.commit.slice(0, 12) || 'unknown'}</code></dd>
            <dt>Built</dt><dd>{version.data.build_date || 'unknown'}</dd>
            <dt>Go</dt><dd>{version.data.go_version}</dd>
            <dt>Git</dt><dd>{version.data.git_version}</dd>
          </dl>
        )}
      </section>
    </>
  );
}
