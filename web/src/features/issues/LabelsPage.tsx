import { useState, type FormEvent } from 'react';
import { Link, useOutletContext } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { repoAPI } from '../../api/hooks/useRepo';
import { useLabels } from '../../api/hooks/useIssues';
import type { Label } from '../../api/types';
import { formErrors } from '../../components/formErrors';
import type { RepoContext } from '../repo/RepoLayout';
import { LabelChip } from './parts';

const PALETTE = ['#1d76db', '#0e8a16', '#d93f0b', '#fbca04', '#5319e7', '#b60205', '#006b75', '#6b7280'];

/**
 * Lists a repository's labels at `/:owner/:repo/labels`. Writers can create, rename,
 * recolour and delete them; deleting removes the label from every issue.
 */
export function LabelsPage() {
  const { repo } = useOutletContext<RepoContext>();
  const labels = useLabels(repo.owner, repo.name);
  const writer = repo.permission === 'write' || repo.permission === 'admin';
  const [adding, setAdding] = useState(false);
  const base = `/${repo.owner}/${repo.name}`;

  return (
    <>
      <div className="list-bar">
        <h2 className="list-bar-fill">Labels</h2>
        <Link className="button" to={`${base}/issues`}>Back to issues</Link>
        {writer && !adding && <button type="button" className="button button-primary" onClick={() => setAdding(true)}>New label</button>}
      </div>
      {adding && <LabelForm repoOwner={repo.owner} repoName={repo.name} onDone={() => setAdding(false)} />}
      <section className="panel issue-list">
        {labels.isPending ? <p className="muted issue-row">Loading…</p>
          : labels.isError ? <p className="error issue-row">{labels.error.message}</p>
          : labels.data.length === 0 ? <p className="muted issue-row empty-row">No labels yet.</p>
          : <ul>{labels.data.map((l) => <LabelRow key={l.id} label={l} repoOwner={repo.owner} repoName={repo.name} writer={writer} />)}</ul>}
      </section>
    </>
  );
}

function LabelRow({ label, repoOwner, repoName, writer }: { label: Label; repoOwner: string; repoName: string; writer: boolean }) {
  const [editing, setEditing] = useState(false);
  const qc = useQueryClient();
  const remove = useMutation({
    mutationFn: () => api(`${repoAPI(repoOwner, repoName)}/labels/${label.id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['labels', repoOwner, repoName] }),
  });
  if (editing) return <li className="issue-row"><LabelForm repoOwner={repoOwner} repoName={repoName} label={label} onDone={() => setEditing(false)} /></li>;
  return (
    <li className="issue-row">
      <div className="issue-main">
        <Link to={`/${repoOwner}/${repoName}/issues?label=${encodeURIComponent(label.name)}`} className="chip-link"><LabelChip label={label} /></Link>
      </div>
      {writer && (
        <span className="row-actions">
          <button type="button" className="link-button" onClick={() => setEditing(true)}>Edit</button>
          <button type="button" className="link-button danger" disabled={remove.isPending}
            onClick={() => { if (confirm(`Delete the label "${label.name}"? It will be removed from every issue.`)) remove.mutate(); }}>Delete</button>
        </span>
      )}
      {remove.isError && <p className="form-error">{remove.error.message}</p>}
    </li>
  );
}

function LabelForm({ repoOwner, repoName, label, onDone }: { repoOwner: string; repoName: string; label?: Label; onDone: () => void }) {
  const qc = useQueryClient();
  const [name, setName] = useState(label?.name ?? '');
  const [color, setColor] = useState(label?.color ?? PALETTE[Math.floor(Math.random() * PALETTE.length)]);
  const save = useMutation({
    mutationFn: () => api(`${repoAPI(repoOwner, repoName)}/labels${label ? '/' + label.id : ''}`, {
      method: label ? 'PATCH' : 'POST',
      body: JSON.stringify({ name, color }),
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['labels', repoOwner, repoName] });
      qc.invalidateQueries({ queryKey: ['issues', repoOwner, repoName] });
      onDone();
    },
  });
  const errs = save.isError ? formErrors(save.error) : null;
  const submit = (e: FormEvent) => { e.preventDefault(); save.mutate(); };
  return (
    <form className="panel label-form" onSubmit={submit}>
      <LabelChip label={{ id: 0, name: name || 'Label preview', color }} />
      <div className="label-form-row">
        <label className="field">
          <span>Name</span>
          <input value={name} maxLength={50} required autoFocus onChange={(e) => setName(e.target.value)} aria-invalid={errs?.fields.name ? true : undefined} />
        </label>
        <label className="field">
          <span>Colour</span>
          <input type="color" value={color} onChange={(e) => setColor(e.target.value)} />
        </label>
        <div className="form-actions">
          <button type="submit" className="button button-primary" disabled={!name.trim() || save.isPending}>{label ? 'Save' : 'Create label'}</button>
          <button type="button" className="button" onClick={onDone}>Cancel</button>
        </div>
      </div>
      {errs && <p className="form-error">{errs.general || Object.values(errs.fields)[0]}</p>}
    </form>
  );
}
