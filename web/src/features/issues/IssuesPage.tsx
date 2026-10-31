import { useState, type FormEvent } from 'react';
import { Link, useOutletContext, useSearchParams } from 'react-router-dom';
import { PER_PAGE, useIssues, usePulls } from '../../api/hooks/useIssues';
import type { Issue, Pull } from '../../api/types';
import { formatTime } from '../../components/formErrors';
import { useSession } from '../../app/session';
import type { RepoContext } from '../repo/RepoLayout';
import { LabelChip } from './parts';

type State = 'open' | 'closed' | 'all';

function readState(v: string | null): State {
  return v === 'closed' || v === 'all' ? v : 'open';
}

/**
 * The issue list at `/:owner/:repo/issues`, filtered by state, label, author and a
 * text search; the filters live in the URL so a filtered list can be shared.
 */
export function IssuesPage() {
  const { repo } = useOutletContext<RepoContext>();
  const { user } = useSession();
  const [sp, setSp] = useSearchParams();
  const state = readState(sp.get('state'));
  const page = Math.max(1, Number(sp.get('page')) || 1);
  const label = sp.get('label') ?? '';
  const author = sp.get('author') ?? '';
  const q = sp.get('q') ?? '';
  const [search, setSearch] = useState(q);
  const list = useIssues(repo.owner, repo.name, { state, label, author, q, page });
  const set = (k: string, v: string) => {
    const next = new URLSearchParams(sp);
    if (v) next.set(k, v);
    else next.delete(k);
    if (k !== 'page') next.delete('page');
    setSp(next);
  };
  const submit = (e: FormEvent) => { e.preventDefault(); set('q', search.trim()); };
  const base = `/${repo.owner}/${repo.name}`;

  return (
    <>
      <div className="list-bar">
        <form className="search-form" role="search" onSubmit={submit}>
          <label htmlFor="issue-search" className="sr-only">Search issues</label>
          <input id="issue-search" type="search" placeholder="Search issues" value={search} onChange={(e) => setSearch(e.target.value)} />
        </form>
        <Link className="button" to={`${base}/labels`}>Labels</Link>
        {user && <Link className="button button-primary" to={`${base}/issues/new`}>New issue</Link>}
      </div>
      {(label || author || q) && (
        <p className="muted small filter-note">
          Filtered by {[q && `“${q}”`, label && `label ${label}`, author && `author ${author}`].filter(Boolean).join(', ')} ·{' '}
          <button type="button" className="link-button" onClick={() => { setSearch(''); setSp(new URLSearchParams({ state })); }}>Clear filters</button>
        </p>
      )}
      <IssueList
        state={state}
        onState={(s) => set('state', s === 'open' ? '' : s)}
        items={list.data}
        error={list.error?.message}
        page={page}
        onPage={(p) => set('page', p === 1 ? '' : String(p))}
        linkFor={(i) => `${base}/issues/${i.number}`}
        onLabel={(name) => set('label', name)}
        empty={state === 'open' ? 'No open issues.' : 'No issues match.'}
      />
    </>
  );
}

/**
 * The pull request list at `/:owner/:repo/pulls`.
 */
export function PullsPage() {
  const { repo } = useOutletContext<RepoContext>();
  const [sp, setSp] = useSearchParams();
  const state = readState(sp.get('state'));
  const page = Math.max(1, Number(sp.get('page')) || 1);
  const list = usePulls(repo.owner, repo.name, state, page);
  const base = `/${repo.owner}/${repo.name}`;
  return (
    <>
      <div className="list-bar">
        <span className="list-bar-fill" />
        {!repo.empty && <Link className="button button-primary" to={`${base}/compare`}>New pull request</Link>}
      </div>
      <IssueList
        state={state}
        onState={(s) => setSp(s === 'open' ? {} : { state: s })}
        items={list.data}
        error={list.error?.message}
        page={page}
        onPage={(p) => setSp({ ...(state !== 'open' && { state }), ...(p > 1 && { page: String(p) }) })}
        linkFor={(i) => `${base}/pull/${i.number}`}
        empty={state === 'open' ? 'No open pull requests.' : 'No pull requests match.'}
      />
    </>
  );
}

/** A state switcher, the rows, and previous/next paging. Shared by issues and pull requests. */
function IssueList({ state, onState, items, error, page, onPage, linkFor, onLabel, empty }: {
  state: State;
  onState: (s: State) => void;
  items?: (Issue | Pull)[];
  error?: string;
  page: number;
  onPage: (p: number) => void;
  linkFor: (i: Issue) => string;
  onLabel?: (name: string) => void;
  empty: string;
}) {
  return (
    <section className="panel issue-list">
      <div className="issue-list-head" role="group" aria-label="State">
        {(['open', 'closed', 'all'] as const).map((s) => (
          <button key={s} type="button" className="link-button" aria-pressed={state === s} onClick={() => onState(s)}>
            {s === 'open' ? 'Open' : s === 'closed' ? 'Closed' : 'All'}
          </button>
        ))}
      </div>
      {error ? <p className="error issue-row">{error}</p>
        : !items ? <p className="muted issue-row">Loading…</p>
        : items.length === 0 ? <p className="muted issue-row empty-row">{empty}</p>
        : (
          <ul>
            {items.map((i) => (
              <li key={i.number} className="issue-row">
                <span className={`state-icon state-${'merged' in i && i.merged ? 'merged' : i.state}`} aria-label={'merged' in i && i.merged ? 'Merged' : i.state === 'open' ? 'Open' : 'Closed'} role="img">
                  {i.is_pull ? '⇄' : i.state === 'open' ? '◯' : '✓'}
                </span>
                <div className="issue-main">
                  <Link className="issue-link" to={linkFor(i)}>{i.title}</Link>
                  {i.labels.map((l) => onLabel
                    ? <button key={l.id} type="button" className="chip-button" onClick={() => onLabel(l.name)}><LabelChip label={l} /></button>
                    : <LabelChip key={l.id} label={l} />)}
                  <div className="muted small">
                    #{i.number} {i.state === 'open' ? 'opened' : 'closed'} {formatTime(i.state === 'open' ? i.created_at : i.closed_at ?? i.updated_at)} by {i.author}
                    {'base' in i && <> · <code>{i.head}</code> → <code>{i.base}</code></>}
                  </div>
                </div>
                {i.comments > 0 && <span className="muted small comment-count" aria-label={`${i.comments} comments`}>💬 {i.comments}</span>}
              </li>
            ))}
          </ul>
        )}
      {(page > 1 || (items?.length ?? 0) === PER_PAGE) && (
        <div className="pager">
          <button type="button" className="button" disabled={page <= 1} onClick={() => onPage(page - 1)}>Previous</button>
          <span className="muted small">Page {page}</span>
          <button type="button" className="button" disabled={(items?.length ?? 0) < PER_PAGE} onClick={() => onPage(page + 1)}>Next</button>
        </div>
      )}
    </section>
  );
}
