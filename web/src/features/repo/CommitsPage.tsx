import { Link, useOutletContext, useSearchParams } from 'react-router-dom';
import { useBranches, useCommits } from '../../api/hooks/useRepo';
import { formatTime } from '../../components/formErrors';
import type { RepoContext } from './RepoLayout';

/** Commit history at `/:owner/:repo/commits?ref=&page=`. */
export function CommitsPage() {
  const { repo } = useOutletContext<RepoContext>();
  const [params, setParams] = useSearchParams();
  const ref = params.get('ref') || repo.default_branch;
  const page = Math.max(1, Number(params.get('page')) || 1);
  const branches = useBranches(repo.owner, repo.name, !repo.empty);
  const commits = useCommits(repo.owner, repo.name, ref, '', page);

  if (repo.empty) return <p className="muted">No commits yet.</p>;
  const names = branches.data?.map((b) => b.name) ?? [ref];

  return (
    <>
      <div className="code-bar">
        <label className="branch-select">
          <span className="sr-only">Branch</span>
          <span aria-hidden="true">⑂</span>
          <select value={ref} onChange={(e) => setParams({ ref: e.target.value })}>
            {(names.includes(ref) ? names : [ref, ...names]).map((b) => <option key={b}>{b}</option>)}
          </select>
        </label>
      </div>
      {commits.isPending && <p className="muted">Loading…</p>}
      {commits.isError && <p className="error">{commits.error.message}</p>}
      {commits.data && (
        <ol className="panel commit-list">
          {commits.data.map((c) => (
            <li key={c.sha}>
              <div>
                <Link className="commit-subject" to={`/${repo.owner}/${repo.name}/commit/${c.sha}`}>{c.subject}</Link>
                <div className="muted small"><strong>{c.author.name}</strong> committed {formatTime(c.committer.when)}</div>
              </div>
              <Link className="sha" to={`/${repo.owner}/${repo.name}/commit/${c.sha}`}>{c.sha.slice(0, 7)}</Link>
            </li>
          ))}
        </ol>
      )}
      <div className="pager">
        <button type="button" className="button" disabled={page <= 1} onClick={() => setParams({ ref, page: String(page - 1) })}>Newer</button>
        <button type="button" className="button" disabled={(commits.data?.length ?? 0) < 30} onClick={() => setParams({ ref, page: String(page + 1) })}>Older</button>
      </div>
    </>
  );
}
