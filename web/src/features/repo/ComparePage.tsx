import { useState, type FormEvent } from 'react';
import { Link, useNavigate, useOutletContext, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { encodePath, repoAPI, useBranches } from '../../api/hooks/useRepo';
import type { CompareView, Pull, Repo } from '../../api/types';
import { DiffView } from '../../components/DiffView';
import { Field } from '../../components/Field';
import { formErrors, formatTime } from '../../components/formErrors';
import { useSession } from '../../app/session';
import { MarkdownEditor } from '../issues/parts';
import type { RepoContext } from './RepoLayout';

/**
 * Compares two branches at `/:owner/:repo/compare/:base...:head` (the link `git push`
 * prints for a new branch). Signed-in users can open a pull request from here.
 * Query key `['compare', owner, name, base, head]`.
 */
export function ComparePage() {
  const { repo } = useOutletContext<RepoContext>();
  const tail = useParams()['*'] ?? '';
  const navigate = useNavigate();
  const { user } = useSession();
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
            {!head && <option value="">choose a branch</option>}
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
          {user && (
            <OpenPullForm
              key={`${base}...${head}`}
              repo={repo}
              base={base}
              head={head}
              defaultTitle={cmp.data.commits.length === 1 ? cmp.data.commits[0].subject : head}
            />
          )}
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

/**
 * The "Open a pull request" form shown above a comparison. The title defaults to the only
 * commit's subject, or the branch name. If a pull request for the same branches is
 * already open, the server refuses and the message says so.
 */
function OpenPullForm({ repo, base, head, defaultTitle }: { repo: Repo; base: string; head: string; defaultTitle: string }) {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState(defaultTitle);
  const [body, setBody] = useState('');
  const create = useMutation({
    mutationFn: () => api<Pull>(`${repoAPI(repo.owner, repo.name)}/pulls`, { method: 'POST', body: JSON.stringify({ title, body, base, head }) }),
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['pulls', repo.owner, repo.name] });
      navigate(`/${repo.owner}/${repo.name}/pull/${p.number}`);
    },
  });
  const errs = create.isError ? formErrors(create.error) : { fields: {} as Record<string, string>, general: '' };
  const submit = (e: FormEvent) => { e.preventDefault(); create.mutate(); };
  if (!open) {
    return (
      <div className="panel open-pull-bar">
        <span>Ready to propose these changes?</span>
        <button type="button" className="button button-primary" onClick={() => setOpen(true)}>Create pull request</button>
      </div>
    );
  }
  const other = errs.general || errs.fields.head || errs.fields.base || errs.fields.body;
  return (
    <form className="panel open-pull-form" onSubmit={submit}>
      <h3>Open a pull request</h3>
      <Field label="Title" value={title} maxLength={255} required autoFocus error={errs.fields.title} onChange={(e) => setTitle(e.target.value)} />
      <MarkdownEditor value={body} onChange={setBody} label="Description" placeholder="Describe the change (Markdown supported)" rows={6} />
      {other && <p className="form-error">{other}</p>}
      <div className="form-actions">
        <button type="submit" className="button button-primary" disabled={!title.trim() || create.isPending}>Create pull request</button>
        <button type="button" className="button" onClick={() => setOpen(false)}>Cancel</button>
      </div>
    </form>
  );
}
