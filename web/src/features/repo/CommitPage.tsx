import { Link, useOutletContext, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { repoAPI } from '../../api/hooks/useRepo';
import type { CommitDetail } from '../../api/types';
import { DiffView } from '../../components/DiffView';
import { formatTime } from '../../components/formErrors';
import type { RepoContext } from './RepoLayout';

/** One commit and its changes at `/:owner/:repo/commit/:sha`. Query key `['commit', owner, name, sha]`. */
export function CommitPage() {
  const { repo } = useOutletContext<RepoContext>();
  const { sha = '' } = useParams();
  const commit = useQuery({
    queryKey: ['commit', repo.owner, repo.name, sha],
    queryFn: () => api<CommitDetail>(`${repoAPI(repo.owner, repo.name)}/commits/${encodeURIComponent(sha)}`),
    staleTime: Infinity, // a commit never changes
  });
  if (commit.isPending) return <p className="muted">Loading…</p>;
  if (commit.isError) return <p className="error">{commit.error.message}</p>;
  const c = commit.data;
  const base = `/${repo.owner}/${repo.name}`;
  return (
    <>
      <section className="panel commit-head">
        <h2 className="commit-title">{c.subject}</h2>
        {c.body && <pre className="commit-body">{c.body}</pre>}
        <div className="commit-meta">
          <span><strong>{c.author.name}</strong> committed {formatTime(c.committer.when)}</span>
          <span className="commit-ids">
            {c.parents.length > 0 && (
              <>parent{c.parents.length > 1 ? 's' : ''} {c.parents.map((p) => <Link key={p} className="sha" to={`${base}/commit/${p}`}>{p.slice(0, 7)}</Link>)} · </>
            )}
            commit <code className="sha">{c.sha.slice(0, 12)}</code>
          </span>
          <Link className="button" to={`${base}/tree/${c.sha}`}>Browse files</Link>
        </div>
      </section>
      {c.diff && <DiffView diff={c.diff} />}
    </>
  );
}
