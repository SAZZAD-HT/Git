import { Link, useNavigate, useOutletContext, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { encodePath, repoAPI, useBranches } from '../../api/hooks/useRepo';
import type { CompareView } from '../../api/types';
import { DiffView } from '../../components/DiffView';
import { formatTime } from '../../components/formErrors';
import type { RepoContext } from './RepoLayout';

/**
 * Compares two branches at `/:owner/:repo/compare/:base...:head` (the link `git push`
 * prints for a new branch). From P4 this page also opens the pull request.
 * Query key `['compare', owner, name, base, head]`.
 */
export function ComparePage() {
  const { repo } = useOutletContext<RepoContext>();
  const tail = useParams()['*'] ?? '';
  const navigate = useNavigate();
  const [base, head] = tail.includes('...') ? tail.split('...', 2) : [repo.default_branch, tail];
  const branches = useBranches(repo.owner, repo.name, !repo.empty);
  const cmp = useQuery({
    queryKey: ['compare', repo.owner, repo.name, base, head],
    queryFn: () => api<CompareView>(`${repoAPI(repo.owner, repo.name)}/compare/${encodePath(base)}...${encodePath(head)}`),
    enabled: !!base && !!head,
  });
  const go = (b: string, h: string) => navigate(`/${repo.owner}/${repo.name}/compare/${b}...${h}`);
  const names = branches.data?.map((b) => b.name) ?? [];
  const base_ = `/${repo.owner}/${repo.name}`;

  return (
    <>
      <h2>Compare changes</h2>
      <div className="code-bar">
        <label className="branch-select">base:
          <select value={base} onChange={(e) => go(e.target.value, head)}>
            {(names.includes(base) ? names : [base, ...names]).map((b) => <option key={b}>{b}</option>)}
          </select>
        </label>
        <span aria-hidden="true">←</span>
        <label className="branch-select">compare:
          <select value={head} onChange={(e) => go(base, e.target.value)}>
            {(names.includes(head) || !head ? names : [head, ...names]).map((b) => <option key={b}>{b}</option>)}
          </select>
        </label>
      </div>
      {!head && <p className="muted">Choose a branch to compare with {base}.</p>}
      {cmp.isPending && head && <p className="muted">Loading…</p>}
      {cmp.isError && <p className="error">{cmp.error.message}</p>}
      {cmp.data && (cmp.data.identical || cmp.data.ahead_by === 0 ? (
        <p className="panel muted">There is nothing to compare: <strong>{head}</strong> has no commits that are not on <strong>{base}</strong>.</p>
      ) : (
        <>
          <p className="muted">
            <strong>{cmp.data.ahead_by}</strong> commit{cmp.data.ahead_by === 1 ? '' : 's'} on {head} not on {base}
            {cmp.data.behind_base && ` · ${base} has also moved on since ${head} branched off`}
          </p>
          <ol className="panel commit-list">
            {cmp.data.commits.map((c) => (
              <li key={c.sha}>
                <div>
                  <Link className="commit-subject" to={`${base_}/commit/${c.sha}`}>{c.subject}</Link>
                  <div className="muted small"><strong>{c.author.name}</strong> committed {formatTime(c.committer.when)}</div>
                </div>
                <Link className="sha" to={`${base_}/commit/${c.sha}`}>{c.sha.slice(0, 7)}</Link>
              </li>
            ))}
          </ol>
          <DiffView diff={cmp.data.diff} />
        </>
      ))}
    </>
  );
}
