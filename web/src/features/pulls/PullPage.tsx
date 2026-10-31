import { useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate, useOutletContext, useParams } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import { repoAPI } from '../../api/hooks/useRepo';
import { useComments, useLabels, usePull, usePullCompare } from '../../api/hooks/useIssues';
import type { DraftComment, IssueComment, Pull, Repo } from '../../api/types';
import { DiffView } from '../../components/DiffView';
import { Markdown } from '../../components/Markdown';
import { NotFoundPage } from '../../components/NotFoundPage';
import { formErrors, formatTime } from '../../components/formErrors';
import { useSession } from '../../app/session';
import { groupInline, lineKey, reviewVerdicts } from '../../lib/issues';
import { CommentComposer, IssueTitle, LabelPicker, MarkdownEditor, Timeline, useRefreshIssue } from '../issues/parts';
import type { RepoContext } from '../repo/RepoLayout';

type Tab = 'conversation' | 'commits' | 'files';

/**
 * One pull request at `/:owner/:repo/pull/:number`, with Conversation, Commits
 * (`/commits`) and Files (`/files`) tabs. Line comments written on the Files tab are kept as a draft review
 * until submitted, so switching tabs does not lose them.
 */
export function PullPage() {
  const { repo } = useOutletContext<RepoContext>();
  const params = useParams();
  const number = Number(params.number);
  const navigate = useNavigate();
  const tab: Tab = params['*'] === 'commits' ? 'commits' : params['*'] === 'files' ? 'files' : 'conversation';
  const pull = usePull(repo.owner, repo.name, number);
  const comments = useComments(repo.owner, repo.name, number);
  const labels = useLabels(repo.owner, repo.name);
  const compare = usePullCompare(repo.owner, repo.name, number, pull.data?.head_sha ?? '', !!pull.data && tab !== 'conversation');
  const [drafts, setDrafts] = useState<DraftComment[]>([]);
  const { user } = useSession();

  if (pull.isPending) return <p className="muted">Loading…</p>;
  if (pull.isError) {
    if (pull.error instanceof ApiError && pull.error.status === 404) return <NotFoundPage />;
    return <p className="error">{pull.error.message}</p>;
  }
  const p = pull.data;
  const writer = repo.permission === 'write' || repo.permission === 'admin';
  const canClose = !!user && (writer || user.username === p.author);
  const list = comments.data ?? [];
  const go = (t: Tab) => navigate(`/${repo.owner}/${repo.name}/pull/${number}${t === 'conversation' ? '' : '/' + t}`);
  const ahead = compare.data?.commits.length;

  return (
    <>
      <IssueTitle repo={repo} issue={p}>
        <strong>{p.author}</strong> {p.merged ? 'merged' : 'wants to merge'} {ahead !== undefined && `${ahead} commit${ahead === 1 ? '' : 's'} `}into{' '}
        <code className="branch-name">{p.base}</code> from <code className="branch-name">{p.head}</code>
      </IssueTitle>
      <nav className="tabs subtabs" aria-label="Pull request">
        {(['conversation', 'commits', 'files'] as const).map((t) => (
          <button key={t} type="button" className={tab === t ? 'active' : ''} aria-current={tab === t ? 'page' : undefined} onClick={() => go(t)}>
            {t === 'conversation' ? 'Conversation' : t === 'commits' ? 'Commits' : 'Files changed'}
            {t === 'files' && drafts.length > 0 && <span className="badge count-badge">{drafts.length}</span>}
          </button>
        ))}
      </nav>

      {tab === 'conversation' && (
        <div className="issue-layout">
          <div>
            {comments.isError && <p className="error">{comments.error.message}</p>}
            <Timeline repo={repo} issue={p} comments={list} footer={<MergeBox repo={repo} pull={p} writer={writer} comments={list} />} />
            <CommentComposer repo={repo} issue={p} canClose={canClose} />
          </div>
          <aside>
            <Reviewers comments={list} />
            <LabelPicker repo={repo} issue={p} labels={labels.data ?? []} />
          </aside>
        </div>
      )}

      {tab !== 'conversation' && compare.isPending && <p className="muted">Loading…</p>}
      {tab !== 'conversation' && compare.isError && <p className="error">{compare.error.message}</p>}

      {tab === 'commits' && compare.data && (
        compare.data.commits.length === 0 ? <p className="panel muted">No commits.</p> : (
          <ol className="panel commit-list">
            {compare.data.commits.map((c) => (
              <li key={c.sha}>
                <div>
                  <Link className="commit-subject" to={`/${repo.owner}/${repo.name}/commit/${c.sha}`}>{c.subject}</Link>
                  <div className="muted small"><strong>{c.author.name}</strong> committed {formatTime(c.committer.when)}</div>
                </div>
                <Link className="sha" to={`/${repo.owner}/${repo.name}/commit/${c.sha}`}>{c.sha.slice(0, 7)}</Link>
              </li>
            ))}
          </ol>
        )
      )}

      {tab === 'files' && compare.data && (
        <FilesTab repo={repo} pull={p} comments={list} drafts={drafts} setDrafts={setDrafts} diff={compare.data.diff} />
      )}
    </>
  );
}

/** The Files tab: the diff with line comments, and the review form above it. */
function FilesTab({ repo, pull, comments, drafts, setDrafts, diff }: {
  repo: Repo;
  pull: Pull;
  comments: IssueComment[];
  drafts: DraftComment[];
  setDrafts: (f: (d: DraftComment[]) => DraftComment[]) => void;
  diff: Parameters<typeof DiffView>[0]['diff'];
}) {
  const { user } = useSession();
  const [composing, setComposing] = useState<{ path: string; side: 'old' | 'new'; line: number } | null>(null);
  // Only comments made on the commit being shown line up with this diff; older ones stay in the conversation.
  const inline = useMemo(() => groupInline(comments.filter((c) => !c.commit_sha || c.commit_sha === pull.head_sha)), [comments, pull.head_sha]);
  const canReview = !!user && pull.state === 'open';

  const below = (path: string, side: 'old' | 'new', line: number) => {
    const k = lineKey(path, side, line);
    const existing = inline.get(k) ?? [];
    const mine = drafts.filter((d) => lineKey(d.path, d.side, d.line) === k);
    const open = composing && lineKey(composing.path, composing.side, composing.line) === k;
    if (!existing.length && !mine.length && !open) return null;
    return (
      <div className="line-notes">
        {existing.map((c) => (
          <div key={c.id} className="line-note">
            <p className="small"><Link to={`/${c.author}`}><strong>{c.author}</strong></Link> <span className="muted">{formatTime(c.created_at)}</span></p>
            <Markdown source={c.body} />
          </div>
        ))}
        {mine.map((d, i) => (
          <div key={i} className="line-note draft">
            <p className="small"><span className="badge">Pending</span>{' '}
              <button type="button" className="link-button danger" onClick={() => setDrafts((all) => all.filter((x) => x !== d))}>Remove</button>
            </p>
            <Markdown source={d.body} />
          </div>
        ))}
        {open && <LineComposer onCancel={() => setComposing(null)} onSave={(body) => { setDrafts((all) => [...all, { path, side, line, body }]); setComposing(null); }} />}
      </div>
    );
  };

  return (
    <>
      {canReview && <ReviewForm repo={repo} pull={pull} drafts={drafts} onSubmitted={() => setDrafts(() => [])} />}
      {pull.merged && <p className="muted small">Showing the changes as they were merged.</p>}
      <DiffView diff={diff} annotations={{ below, onAdd: canReview ? (path, side, line) => setComposing({ path, side, line }) : undefined }} />
    </>
  );
}

function LineComposer({ onSave, onCancel }: { onSave: (body: string) => void; onCancel: () => void }) {
  const [text, setText] = useState('');
  return (
    <form className="line-composer" onSubmit={(e) => { e.preventDefault(); if (text.trim()) onSave(text); }}>
      <MarkdownEditor value={text} onChange={setText} label="Line comment" rows={3} autoFocus />
      <div className="form-actions">
        <button type="submit" className="button button-primary" disabled={!text.trim()}>Add to review</button>
        <button type="button" className="button" onClick={onCancel}>Cancel</button>
      </div>
    </form>
  );
}

/**
 * Submits a review: an overall comment, approval or change request, plus any pending line
 * comments. Authors can only comment on their own pull request.
 */
function ReviewForm({ repo, pull, drafts, onSubmitted }: { repo: Repo; pull: Pull; drafts: DraftComment[]; onSubmitted: () => void }) {
  const { user } = useSession();
  const refresh = useRefreshIssue(repo, pull.number);
  const own = user?.username === pull.author;
  const [event, setEvent] = useState<'comment' | 'approve' | 'request_changes'>('comment');
  const [body, setBody] = useState('');
  const [open, setOpen] = useState(false);
  const submit = useMutation({
    mutationFn: () => api(`${repoAPI(repo.owner, repo.name)}/pulls/${pull.number}/reviews`, {
      method: 'POST',
      body: JSON.stringify({ event, body, comments: drafts }),
    }),
    onSuccess: () => { setBody(''); setEvent('comment'); setOpen(false); onSubmitted(); refresh(); },
  });
  const empty = event === 'comment' && !body.trim() && drafts.length === 0;
  const onSubmit = (e: FormEvent) => { e.preventDefault(); submit.mutate(); };
  return (
    <div className="review-bar">
      <span className="muted small">{drafts.length ? `${drafts.length} pending line comment${drafts.length === 1 ? '' : 's'}` : 'Click + on a line to comment on it.'}</span>
      <div className="menu">
        <button type="button" className="button button-primary" aria-expanded={open} onClick={() => setOpen((v) => !v)}>Review changes ▾</button>
        {open && (
          <form className="menu-list review-panel" onSubmit={onSubmit}>
            <MarkdownEditor value={body} onChange={setBody} label="Review summary" placeholder="Leave a summary (optional)" rows={4} />
            <fieldset className="fieldset">
              <legend className="sr-only">Review outcome</legend>
              <label className="check"><input type="radio" name="event" checked={event === 'comment'} onChange={() => setEvent('comment')} /> Comment</label>
              <label className="check"><input type="radio" name="event" checked={event === 'approve'} disabled={own} onChange={() => setEvent('approve')} /> Approve</label>
              <label className="check"><input type="radio" name="event" checked={event === 'request_changes'} disabled={own} onChange={() => setEvent('request_changes')} /> Request changes</label>
              {own && <p className="muted small">You can't approve or request changes on your own pull request.</p>}
            </fieldset>
            {submit.isError && <p className="form-error">{formErrors(submit.error).general || Object.values(formErrors(submit.error).fields)[0]}</p>}
            <button type="submit" className="button button-primary" disabled={empty || submit.isPending}>Submit review</button>
          </form>
        )}
      </div>
    </div>
  );
}

/** Sidebar list of reviewers and their current verdict. */
function Reviewers({ comments }: { comments: IssueComment[] }) {
  const v = reviewVerdicts(comments);
  return (
    <section className="side-section">
      <div className="side-head"><h3>Reviews</h3></div>
      {v.length === 0 ? <p className="muted small">No reviews yet</p> : (
        <ul className="reviewer-list">
          {v.map((r) => (
            <li key={r.author}>
              <Link to={`/${r.author}`}>{r.author}</Link>{' '}
              <span className={r.state === 'approve' ? 'form-ok' : 'error'}>{r.state === 'approve' ? '✔ approved' : '✖ changes requested'}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * The box under the conversation showing whether the pull request can be merged, and the
 * merge button for writers. The merge sends the head commit the page showed, so a push
 * that lands meanwhile makes the server refuse instead of merging unseen commits.
 */
function MergeBox({ repo, pull, writer, comments }: { repo: Repo; pull: Pull; writer: boolean; comments: IssueComment[] }) {
  const qc = useQueryClient();
  const refresh = useRefreshIssue(repo, pull.number);
  const [strategy, setStrategy] = useState<'merge' | 'squash'>('merge');
  const [message, setMessage] = useState('');
  const [confirming, setConfirming] = useState(false);
  const merge = useMutation({
    mutationFn: () => api<Pull>(`${repoAPI(repo.owner, repo.name)}/pulls/${pull.number}/merge`, {
      method: 'POST',
      body: JSON.stringify({ strategy, message: message.trim(), expected_head_sha: pull.head_sha }),
    }),
    onSuccess: () => {
      setConfirming(false);
      refresh();
      for (const k of ['branches', 'tree', 'commits', 'repo']) qc.invalidateQueries({ queryKey: [k, repo.owner, repo.name] });
    },
    onError: () => refresh(),
  });
  const changes = reviewVerdicts(comments).filter((v) => v.state === 'request_changes');

  if (pull.merged) {
    return (
      <div className="panel merge-box merge-merged">
        <strong>Merged</strong> {formatTime(pull.merged_at)}
        {pull.merge_commit_sha && <> as <Link className="sha" to={`/${repo.owner}/${repo.name}/commit/${pull.merge_commit_sha}`}>{pull.merge_commit_sha.slice(0, 7)}</Link></>}
      </div>
    );
  }
  if (pull.state === 'closed') return <div className="panel merge-box muted">This pull request was closed without merging.</div>;

  const status = {
    unknown: { cls: 'merge-checking', text: 'Checking whether this can be merged…' },
    checking: { cls: 'merge-checking', text: 'Checking whether this can be merged…' },
    clean: { cls: 'merge-clean', text: `This branch has no conflicts with ${pull.base}.` },
    conflict: { cls: 'merge-conflict', text: `This branch has conflicts with ${pull.base} that must be resolved.` },
  }[pull.merge_status];

  return (
    <div className={`panel merge-box ${status.cls}`}>
      <p className="merge-status" role="status"><strong>{status.text}</strong></p>
      {pull.merge_status === 'conflict' && pull.conflict_files.length > 0 && (
        <>
          <p className="muted small">Conflicting files:</p>
          <ul className="conflict-files">{pull.conflict_files.map((f) => <li key={f}><code>{f}</code></li>)}</ul>
          <p className="muted small">Merge <code>{pull.base}</code> into <code>{pull.head}</code> locally, resolve the conflicts, and push.</p>
        </>
      )}
      {changes.length > 0 && <p className="error small">Changes requested by {changes.map((c) => c.author).join(', ')}.</p>}
      {writer ? (
        confirming ? (
          <form className="merge-form" onSubmit={(e) => { e.preventDefault(); merge.mutate(); }}>
            <label className="field">
              <span>Commit message <span className="muted">(optional)</span></span>
              <textarea rows={3} value={message} placeholder={strategy === 'squash' ? `${pull.title} (#${pull.number})` : `Merge pull request #${pull.number} from ${pull.head}`} onChange={(e) => setMessage(e.target.value)} />
            </label>
            <div className="form-actions">
              <button type="submit" className="button button-primary" disabled={merge.isPending}>{merge.isPending ? 'Merging…' : strategy === 'squash' ? 'Confirm squash and merge' : 'Confirm merge'}</button>
              <button type="button" className="button" onClick={() => setConfirming(false)}>Cancel</button>
            </div>
          </form>
        ) : (
          <div className="form-actions">
            <label className="branch-select">Method:
              <select value={strategy} onChange={(e) => setStrategy(e.target.value as 'merge' | 'squash')}>
                <option value="merge">Create a merge commit</option>
                <option value="squash">Squash and merge</option>
              </select>
            </label>
            <button type="button" className="button button-primary" disabled={pull.merge_status !== 'clean'} onClick={() => setConfirming(true)}>
              {strategy === 'squash' ? 'Squash and merge' : 'Merge pull request'}
            </button>
          </div>
        )
      ) : (
        <p className="muted small">Only people with write access can merge.</p>
      )}
      {merge.isError && (
        <p className="form-error">
          {merge.error instanceof ApiError && merge.error.status === 409 ? `${merge.error.message} The page has been refreshed; check the new commits and try again.` : merge.error.message}
        </p>
      )}
    </div>
  );
}
