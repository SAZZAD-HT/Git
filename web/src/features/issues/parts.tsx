import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { repoAPI } from '../../api/hooks/useRepo';
import type { Issue, IssueComment, Label, Pull, Repo } from '../../api/types';
import { Markdown } from '../../components/Markdown';
import { formErrors, formatTime } from '../../components/formErrors';
import { useSession } from '../../app/session';
import { labelTextColor } from '../../lib/issues';

/** A coloured label name. */
export function LabelChip({ label }: { label: Label }) {
  return <span className="label-chip" style={{ background: label.color, color: labelTextColor(label.color) }}>{label.name}</span>;
}

/** Open / Closed / Merged badge for an issue or pull request. */
export function StateBadge({ issue }: { issue: Issue | Pull }) {
  const merged = 'merged' in issue && issue.merged;
  const state = merged ? 'merged' : issue.state;
  const text = merged ? 'Merged' : issue.state === 'open' ? 'Open' : 'Closed';
  return <span className={`badge state-${state}`}>{text}</span>;
}

/**
 * A Markdown text area with Write and Preview tabs.
 * @param value - current text.
 * @param onChange - called with the new text.
 * @param label - accessible label for the text area.
 */
export function MarkdownEditor({ value, onChange, label, placeholder, rows = 6, autoFocus }: {
  value: string;
  onChange: (v: string) => void;
  label: string;
  placeholder?: string;
  rows?: number;
  autoFocus?: boolean;
}) {
  const [preview, setPreview] = useState(false);
  const id = useId();
  return (
    <div className="md-editor">
      <div className="md-tabs" role="tablist" aria-label={label}>
        <button type="button" role="tab" aria-selected={!preview} onClick={() => setPreview(false)}>Write</button>
        <button type="button" role="tab" aria-selected={preview} onClick={() => setPreview(true)}>Preview</button>
      </div>
      {preview ? (
        <div className="md-preview">{value.trim() ? <Markdown source={value} /> : <p className="muted">Nothing to preview.</p>}</div>
      ) : (
        <>
          <label htmlFor={id} className="sr-only">{label}</label>
          <textarea id={id} rows={rows} value={value} placeholder={placeholder ?? 'Leave a comment (Markdown supported)'} autoFocus={autoFocus} onChange={(e) => onChange(e.target.value)} />
        </>
      )}
    </div>
  );
}

/** Invalidates everything that shows an issue or pull request after a change. */
export function useRefreshIssue(repo: Repo, number: number) {
  const qc = useQueryClient();
  return () => {
    for (const k of ['issue', 'pull', 'comments']) qc.invalidateQueries({ queryKey: [k, repo.owner, repo.name, number] });
    qc.invalidateQueries({ queryKey: ['issues', repo.owner, repo.name] });
    qc.invalidateQueries({ queryKey: ['pulls', repo.owner, repo.name] });
  };
}

/** Turns a system event body ("closed", "reopened", "merged commit …") into display text. */
function systemText(body: string): string {
  return body === 'closed' || body === 'reopened' ? `${body} this` : body;
}

const VERB: Record<string, string> ={ approve: 'approved these changes', request_changes: 'requested changes', comment: 'reviewed' };

/**
 * The conversation: the opening post, then comments, reviews and system events in order.
 * Inline review comments are shown with the file and line they belong to.
 * @param footer - shown after the last entry, e.g. the merge box.
 */
export function Timeline({ repo, issue, comments, footer }: { repo: Repo; issue: Issue; comments: IssueComment[]; footer?: ReactNode }) {
  return (
    <ol className="timeline">
      <li><CommentCard repo={repo} issue={issue} author={issue.author} when={issue.created_at} body={issue.body} opening /></li>
      {comments.map((c) => (
        <li key={c.id}>
          {c.kind === 'system' ? (
            <p className="timeline-event">
              <span aria-hidden="true">●</span>
              {c.author && <Link to={`/${c.author}`}><strong>{c.author}</strong></Link>}
              <span>{systemText(c.body)}</span>
              <span className="muted small">{formatTime(c.created_at)}</span>
            </p>
          ) : c.kind === 'review' && !c.path ? (
            <div>
              <p className={`timeline-event review-${c.review_state ?? 'comment'}`}>
                <span aria-hidden="true">{c.review_state === 'approve' ? '✔' : c.review_state === 'request_changes' ? '✖' : '💬'}</span>{' '}
                <Link to={`/${c.author}`}><strong>{c.author}</strong></Link> {VERB[c.review_state ?? 'comment']} <span className="muted small">{formatTime(c.created_at)}</span>
              </p>
              {c.body && <CommentCard repo={repo} issue={issue} comment={c} author={c.author} when={c.created_at} body={c.body} />}
            </div>
          ) : (
            <CommentCard repo={repo} issue={issue} comment={c} author={c.author} when={c.created_at} body={c.body} />
          )}
        </li>
      ))}
      {footer && <li>{footer}</li>}
    </ol>
  );
}

/**
 * One comment box with author, time and, for the author or a repository admin, Edit and
 * Delete. The opening post edits the issue body instead of a comment.
 */
function CommentCard({ repo, issue, comment, author, when, body, opening }: {
  repo: Repo;
  issue: Issue;
  comment?: IssueComment;
  author: string;
  when: string;
  body: string;
  opening?: boolean;
}) {
  const { user } = useSession();
  const refresh = useRefreshIssue(repo, issue.number);
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(body);
  const mine = !!user && user.username === author;
  const canEdit = mine || repo.permission === 'admin' || (opening && repo.permission === 'write');
  const save = useMutation({
    mutationFn: () => opening
      ? api(`${repoAPI(repo.owner, repo.name)}/issues/${issue.number}`, { method: 'PATCH', body: JSON.stringify({ body: text }) })
      : api(`${repoAPI(repo.owner, repo.name)}/comments/${comment!.id}`, { method: 'PATCH', body: JSON.stringify({ body: text }) }),
    onSuccess: () => { setEditing(false); refresh(); },
  });
  const remove = useMutation({
    mutationFn: () => api(`${repoAPI(repo.owner, repo.name)}/comments/${comment!.id}`, { method: 'DELETE' }),
    onSuccess: refresh,
  });
  const submit = (e: FormEvent) => { e.preventDefault(); save.mutate(); };

  return (
    <article className="panel comment">
      <header className="comment-head">
        <span>
          <Link to={`/${author}`}><strong>{author}</strong></Link>
          <span className="muted"> {opening ? 'opened' : 'commented'} {formatTime(when)}</span>
          {comment?.path && <span className="muted"> on <code>{comment.path}</code> line {comment.line}</span>}
        </span>
        {canEdit && !editing && (
          <span className="row-actions">
            <button type="button" className="link-button" onClick={() => { setText(body); setEditing(true); }}>Edit</button>
            {!opening && comment && (
              <button type="button" className="link-button danger" disabled={remove.isPending} onClick={() => { if (confirm('Delete this comment?')) remove.mutate(); }}>Delete</button>
            )}
          </span>
        )}
      </header>
      {editing ? (
        <form className="comment-body" onSubmit={submit}>
          <MarkdownEditor value={text} onChange={setText} label="Edit comment" autoFocus />
          {save.isError && <p className="form-error">{formErrors(save.error).general || Object.values(formErrors(save.error).fields)[0]}</p>}
          <div className="form-actions">
            <button type="submit" className="button button-primary" disabled={save.isPending || (!opening && !text.trim())}>Save</button>
            <button type="button" className="button" onClick={() => setEditing(false)}>Cancel</button>
          </div>
        </form>
      ) : (
        <div className="comment-body">{body.trim() ? <Markdown source={body} /> : <p className="muted">No description provided.</p>}</div>
      )}
      {remove.isError && <p className="form-error comment-body">{remove.error.message}</p>}
    </article>
  );
}

/**
 * The "add a comment" box at the bottom of a conversation, with a Close/Reopen button for
 * people allowed to change the state. Signed-out visitors get a sign-in link.
 */
export function CommentComposer({ repo, issue, canClose, extraActions }: { repo: Repo; issue: Issue | Pull; canClose: boolean; extraActions?: ReactNode }) {
  const { user } = useSession();
  const refresh = useRefreshIssue(repo, issue.number);
  const [text, setText] = useState('');
  const base = repoAPI(repo.owner, repo.name);
  const comment = useMutation({
    mutationFn: () => api(`${base}/issues/${issue.number}/comments`, { method: 'POST', body: JSON.stringify({ body: text }) }),
    onSuccess: () => { setText(''); refresh(); },
  });
  const setState = useMutation({
    mutationFn: async (state: 'open' | 'closed') => {
      if (text.trim()) await api(`${base}/issues/${issue.number}/comments`, { method: 'POST', body: JSON.stringify({ body: text }) });
      await api(`${base}/issues/${issue.number}`, { method: 'PATCH', body: JSON.stringify({ state }) });
    },
    onSuccess: () => { setText(''); refresh(); },
  });
  if (!user) {
    return <p className="panel muted"><Link to={`/login?next=${encodeURIComponent(location.pathname)}`}>Sign in</Link> to join the conversation.</p>;
  }
  const merged = 'merged' in issue && issue.merged;
  const err = comment.error ?? setState.error;
  return (
    <form className="panel composer" onSubmit={(e) => { e.preventDefault(); comment.mutate(); }}>
      <MarkdownEditor value={text} onChange={setText} label="Add a comment" />
      {err && <p className="form-error">{formErrors(err).general || Object.values(formErrors(err).fields)[0]}</p>}
      <div className="form-actions composer-actions">
        {extraActions}
        {canClose && !merged && (
          <button type="button" className="button" disabled={setState.isPending} onClick={() => setState.mutate(issue.state === 'open' ? 'closed' : 'open')}>
            {issue.state === 'open' ? (text.trim() ? 'Close with comment' : issue.is_pull ? 'Close pull request' : 'Close issue') : issue.is_pull ? 'Reopen pull request' : 'Reopen issue'}
          </button>
        )}
        <button type="submit" className="button button-primary" disabled={!text.trim() || comment.isPending}>Comment</button>
      </div>
    </form>
  );
}

/**
 * Sidebar label picker. Writers tick labels in a menu; everyone else sees the list.
 */
export function LabelPicker({ repo, issue, labels }: { repo: Repo; issue: Issue; labels: Label[] }) {
  const refresh = useRefreshIssue(repo, issue.number);
  const [open, setOpen] = useState(false);
  const canEdit = repo.permission === 'write' || repo.permission === 'admin';
  const current = new Set(issue.labels.map((l) => l.id));
  const save = useMutation({
    mutationFn: (ids: number[]) => api(`${repoAPI(repo.owner, repo.name)}/issues/${issue.number}`, { method: 'PATCH', body: JSON.stringify({ label_ids: ids }) }),
    onSuccess: refresh,
  });
  const toggle = (id: number) => {
    const next = new Set(current);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    save.mutate([...next]);
  };
  return (
    <section className="side-section">
      <div className="side-head">
        <h3>Labels</h3>
        {canEdit && <button type="button" className="link-button" aria-expanded={open} onClick={() => setOpen((v) => !v)}>{open ? 'Done' : 'Edit'}</button>}
      </div>
      {open ? (
        labels.length === 0 ? (
          <p className="muted small">No labels yet. <Link to={`/${repo.owner}/${repo.name}/labels`}>Create labels</Link></p>
        ) : (
          <ul className="label-options">
            {labels.map((l) => (
              <li key={l.id}>
                <label className="check">
                  <input type="checkbox" checked={current.has(l.id)} disabled={save.isPending} onChange={() => toggle(l.id)} />
                  <LabelChip label={l} />
                </label>
              </li>
            ))}
          </ul>
        )
      ) : issue.labels.length ? (
        <div className="label-row">{issue.labels.map((l) => <LabelChip key={l.id} label={l} />)}</div>
      ) : (
        <p className="muted small">None yet</p>
      )}
      {save.isError && <p className="form-error">{save.error.message}</p>}
    </section>
  );
}

/**
 * Editable title line for an issue or pull request. The author and writers can rename.
 */
export function IssueTitle({ repo, issue, children }: { repo: Repo; issue: Issue; children?: ReactNode }) {
  const { user } = useSession();
  const refresh = useRefreshIssue(repo, issue.number);
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(issue.title);
  const canEdit = !!user && (user.username === issue.author || repo.permission === 'write' || repo.permission === 'admin');
  const save = useMutation({
    mutationFn: () => api(`${repoAPI(repo.owner, repo.name)}/issues/${issue.number}`, { method: 'PATCH', body: JSON.stringify({ title }) }),
    onSuccess: () => { setEditing(false); refresh(); },
  });
  return (
    <div className="issue-head">
      {editing ? (
        <form className="title-edit" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
          <label className="sr-only" htmlFor="issue-title">Title</label>
          <input id="issue-title" value={title} maxLength={255} autoFocus onChange={(e) => setTitle(e.target.value)} />
          <button type="submit" className="button button-primary" disabled={!title.trim() || save.isPending}>Save</button>
          <button type="button" className="button" onClick={() => setEditing(false)}>Cancel</button>
          {save.isError && <p className="form-error">{formErrors(save.error).general || Object.values(formErrors(save.error).fields)[0]}</p>}
        </form>
      ) : (
        <div className="title-row">
          <h2 className="issue-title">{issue.title} <span className="muted">#{issue.number}</span></h2>
          {canEdit && <button type="button" className="button" onClick={() => { setTitle(issue.title); setEditing(true); }}>Edit</button>}
        </div>
      )}
      <p className="issue-meta"><StateBadge issue={issue} /> {children}</p>
    </div>
  );
}
