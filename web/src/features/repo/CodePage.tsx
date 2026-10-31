import { Link, useNavigate, useOutletContext, useParams } from 'react-router-dom';
import { useBlob, useBranches, useTree } from '../../api/hooks/useRepo';
import type { Repo, TreeEntry } from '../../api/types';
import { Markdown } from '../../components/Markdown';
import { formatTime } from '../../components/formErrors';
import { formatSize, splitRefPath } from '../../lib/repoPaths';
import type { RepoContext } from './RepoLayout';

/** Repository home and directories: `/:owner/:repo` and `/:owner/:repo/tree/:ref/*`. */
export function CodePage() {
  const { repo } = useOutletContext<RepoContext>();
  const tail = useParams()['*'] ?? '';
  const branches = useBranches(repo.owner, repo.name, !repo.empty);
  if (repo.empty) return <EmptyRepo repo={repo} />;
  if (tail && !branches.data) return <p className="muted">Loading…</p>;
  const [ref, path] = splitRefPath(tail, branches.data?.map((b) => b.name) ?? [], repo.default_branch);
  return <TreeView repo={repo} gitRef={ref} path={path} branches={branches.data?.map((b) => b.name) ?? []} />;
}

function TreeView({ repo, gitRef, path, branches }: { repo: Repo; gitRef: string; path: string; branches: string[] }) {
  const tree = useTree(repo.owner, repo.name, gitRef, path);
  const readme = tree.data?.entries.find((e) => e.type === 'blob' && /^readme(\.md|\.markdown|\.txt)?$/i.test(e.name));
  const readmeBlob = useBlob(repo.owner, repo.name, gitRef, readme?.path ?? '', !!readme);
  const base = `/${repo.owner}/${repo.name}`;

  return (
    <>
      <div className="code-bar">
        <BranchSelect repo={repo} current={gitRef} branches={branches} path={path} />
        <Breadcrumbs repo={repo} gitRef={gitRef} path={path} />
      </div>
      {tree.isPending && <p className="muted">Loading…</p>}
      {tree.isError && <p className="error">{tree.error.message}</p>}
      {tree.data && (
        <div className="panel file-list">
          {tree.data.last_commit && (
            <div className="last-commit">
              <strong>{tree.data.last_commit.author.name}</strong>
              <span className="last-commit-msg">{tree.data.last_commit.subject}</span>
              <Link className="sha" to={`${base}/commits?ref=${encodeURIComponent(gitRef)}`}>{tree.data.last_commit.sha.slice(0, 7)}</Link>
              <span className="muted">{formatTime(tree.data.last_commit.committer.when)}</span>
            </div>
          )}
          <ul>
            {path && (
              <li><Link to={`${base}/tree/${gitRef}/${path.split('/').slice(0, -1).join('/')}`.replace(/\/$/, '')} className="file-link">..</Link></li>
            )}
            {tree.data.entries.map((e) => <EntryRow key={e.path} e={e} base={base} gitRef={gitRef} />)}
          </ul>
        </div>
      )}
      {readme && readmeBlob.data && (
        <section className="panel readme">
          <h2 className="readme-title">{readme.name}</h2>
          {/\.(md|markdown)$/i.test(readme.name) ? <Markdown source={readmeBlob.data.blob.content} /> : <pre className="plain">{readmeBlob.data.blob.content}</pre>}
        </section>
      )}
    </>
  );
}

function EntryRow({ e, base, gitRef }: { e: TreeEntry; base: string; gitRef: string }) {
  const kind = e.type === 'tree' ? 'tree' : 'blob';
  const icon = e.type === 'tree' ? '📁' : e.type === 'commit' ? '🔗' : '📄';
  return (
    <li>
      <span aria-hidden="true" className="file-icon">{icon}</span>
      {e.type === 'commit' ? (
        <span>{e.name} <span className="muted">(submodule)</span></span>
      ) : (
        <Link className="file-link" to={`${base}/${kind}/${gitRef}/${e.path}`}>{e.name}</Link>
      )}
      {e.type === 'blob' && <span className="muted file-size">{formatSize(e.size)}</span>}
    </li>
  );
}

/** A branch picker that keeps the current path when switching branches. */
export function BranchSelect({ repo, current, branches, path, kind = 'tree' }: { repo: Repo; current: string; branches: string[]; path: string; kind?: 'tree' | 'blob' }) {
  const navigate = useNavigate();
  const list = branches.includes(current) ? branches : [current, ...branches];
  return (
    <label className="branch-select">
      <span className="sr-only">Branch</span>
      <span aria-hidden="true">⑂</span>
      <select value={current} onChange={(e) => navigate(`/${repo.owner}/${repo.name}/${kind}/${e.target.value}${path ? '/' + path : ''}`)}>
        {list.map((b) => <option key={b} value={b}>{b}</option>)}
      </select>
    </label>
  );
}

/** owner/repo / dir / file breadcrumbs, each a link. */
export function Breadcrumbs({ repo, gitRef, path }: { repo: Repo; gitRef: string; path: string }) {
  if (!path) return null;
  const parts = path.split('/');
  const base = `/${repo.owner}/${repo.name}`;
  return (
    <nav className="crumbs" aria-label="Path">
      <Link to={base}>{repo.name}</Link>
      {parts.map((p, i) => (
        <span key={i}>
          <span className="muted"> / </span>
          {i === parts.length - 1 ? <strong>{p}</strong> : <Link to={`${base}/tree/${gitRef}/${parts.slice(0, i + 1).join('/')}`}>{p}</Link>}
        </span>
      ))}
    </nav>
  );
}

/** Instructions shown for a repository without commits. */
function EmptyRepo({ repo }: { repo: Repo }) {
  const canPush = repo.permission === 'write' || repo.permission === 'admin';
  return (
    <section className="panel">
      <h2>This repository is empty</h2>
      {canPush ? (
        <>
          <p className="muted">Push an existing project from your computer:</p>
          <pre className="plain">{`git remote add origin ${repo.clone_url}
git push -u origin ${repo.default_branch}`}</pre>
          <p className="muted">Or start a new one:</p>
          <pre className="plain">{`git clone ${repo.clone_url}
cd ${repo.name}
echo "# ${repo.name}" > README.md
git add README.md
git commit -m "First commit"
git push -u origin ${repo.default_branch}`}</pre>
          <p className="muted small">Use a <Link to="/settings/tokens">personal access token</Link> as the password.</p>
        </>
      ) : (
        <p className="muted">Nothing has been pushed yet.</p>
      )}
    </section>
  );
}
