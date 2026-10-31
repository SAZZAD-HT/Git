import { useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate, useOutletContext, useParams } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import { repoAPI } from '../../api/hooks/useRepo';
import { useComments, useIssue, useLabels } from '../../api/hooks/useIssues';
import type { Issue } from '../../api/types';
import { Field } from '../../components/Field';
import { NotFoundPage } from '../../components/NotFoundPage';
import { formErrors, formatTime } from '../../components/formErrors';
import { useSession } from '../../app/session';
import type { RepoContext } from '../repo/RepoLayout';
import { CommentComposer, IssueTitle, LabelChip, LabelPicker, MarkdownEditor, Timeline } from './parts';

/**
 * One issue at `/:owner/:repo/issues/:number`: title, conversation, comment box and the
 * labels sidebar. A pull request number redirects to its pull request page.
 */
export function IssuePage() {
  const { repo } = useOutletContext<RepoContext>();
  const number = Number(useParams().number);
  const { user } = useSession();
  const issue = useIssue(repo.owner, repo.name, number);
  const comments = useComments(repo.owner, repo.name, number);
  const labels = useLabels(repo.owner, repo.name);

  if (issue.isPending) return <p className="muted">Loading…</p>;
  if (issue.isError) {
    if (issue.error instanceof ApiError && issue.error.status === 404) return <NotFoundPage />;
    return <p className="error">{issue.error.message}</p>;
  }
  const i = issue.data;
  if (i.is_pull) return <Navigate to={`/${repo.owner}/${repo.name}/pull/${i.number}`} replace />;
  const writer = repo.permission === 'write' || repo.permission === 'admin';
  const canClose = !!user && (writer || user.username === i.author);

  return (
    <>
      <IssueTitle repo={repo} issue={i}>
        <strong>{i.author}</strong> opened this issue {formatTime(i.created_at)} · {i.comments} comment{i.comments === 1 ? '' : 's'}
      </IssueTitle>
      <div className="issue-layout">
        <div>
          {comments.isError && <p className="error">{comments.error.message}</p>}
          <Timeline repo={repo} issue={i} comments={comments.data ?? []} />
          <CommentComposer repo={repo} issue={i} canClose={canClose} />
        </div>
        <aside>
          <LabelPicker repo={repo} issue={i} labels={labels.data ?? []} />
        </aside>
      </div>
    </>
  );
}

/**
 * The new-issue form at `/:owner/:repo/issues/new`. Writers can also pick labels.
 */
export function NewIssuePage() {
  const { repo } = useOutletContext<RepoContext>();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const labels = useLabels(repo.owner, repo.name);
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [picked, setPicked] = useState<number[]>([]);
  const writer = repo.permission === 'write' || repo.permission === 'admin';
  const create = useMutation({
    mutationFn: () => api<Issue>(`${repoAPI(repo.owner, repo.name)}/issues`, {
      method: 'POST',
      body: JSON.stringify({ title, body, ...(writer && picked.length ? { label_ids: picked } : {}) }),
    }),
    onSuccess: (i) => {
      qc.invalidateQueries({ queryKey: ['issues', repo.owner, repo.name] });
      navigate(`/${repo.owner}/${repo.name}/issues/${i.number}`);
    },
  });
  const errs = create.isError ? formErrors(create.error) : { fields: {} as Record<string, string>, general: '' };
  const submit = (e: FormEvent) => { e.preventDefault(); create.mutate(); };

  return (
    <form className="narrow" onSubmit={submit}>
      <h2>New issue</h2>
      <Field label="Title" value={title} maxLength={255} required autoFocus error={errs.fields.title} onChange={(e) => setTitle(e.target.value)} />
      <MarkdownEditor value={body} onChange={setBody} label="Description" placeholder="Describe the problem or idea (Markdown supported)" rows={10} />
      {errs.fields.body && <p className="field-error">{errs.fields.body}</p>}
      {writer && (labels.data?.length ?? 0) > 0 && (
        <fieldset className="fieldset label-fieldset">
          <legend>Labels</legend>
          <div className="label-row">
            {labels.data!.map((l) => (
              <label key={l.id} className="check">
                <input type="checkbox" checked={picked.includes(l.id)} onChange={(e) => setPicked((p) => e.target.checked ? [...p, l.id] : p.filter((x) => x !== l.id))} />
                <LabelChip label={l} />
              </label>
            ))}
          </div>
        </fieldset>
      )}
      {errs.general && <p className="form-error">{errs.general}</p>}
      <div className="form-actions">
        <button type="submit" className="button button-primary" disabled={!title.trim() || create.isPending}>Submit new issue</button>
        <Link to={`/${repo.owner}/${repo.name}/issues`}>Cancel</Link>
      </div>
    </form>
  );
}
