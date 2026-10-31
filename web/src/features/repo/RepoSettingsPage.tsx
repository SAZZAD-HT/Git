import { useState, type FormEvent } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { repoAPI } from '../../api/hooks/useRepo';
import type { Collaborator, ProtectionRule, Repo } from '../../api/types';
import { Field } from '../../components/Field';
import { formErrors, type FormErrors } from '../../components/formErrors';
import type { RepoContext } from './RepoLayout';

/**
 * Repository settings at `/:owner/:repo/settings` (admins): general settings, collaborators,
 * branch protection and deletion.
 */
export function RepoSettingsPage() {
  const { repo } = useOutletContext<RepoContext>();
  if (repo.permission !== 'admin') return <p className="muted">You need admin access to change settings.</p>;
  return (
    <>
      <GeneralSection repo={repo} />
      <CollaboratorsSection repo={repo} />
      <ProtectionSection repo={repo} />
      <DangerSection repo={repo} />
    </>
  );
}

function GeneralSection({ repo }: { repo: Repo }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [form, setForm] = useState({ name: repo.name, description: repo.description, private: repo.private, default_branch: repo.default_branch });
  const [errors, setErrors] = useState<FormErrors>({ fields: {}, general: '' });
  const [saved, setSaved] = useState(false);
  const save = useMutation({
    mutationFn: () => api<Repo>(repoAPI(repo.owner, repo.name), { method: 'PATCH', body: JSON.stringify(form) }),
    onSuccess: (r) => {
      setErrors({ fields: {}, general: '' });
      setSaved(true);
      queryClient.invalidateQueries({ queryKey: ['repo', repo.owner, repo.name] });
      if (r.name !== repo.name) navigate(`/${r.owner}/${r.name}/settings`, { replace: true });
    },
    onError: (e) => { setSaved(false); setErrors(formErrors(e)); },
  });
  const submit = (e: FormEvent) => { e.preventDefault(); save.mutate(); };
  return (
    <section className="panel">
      <h2>General</h2>
      <form onSubmit={submit} noValidate>
        <Field label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} error={errors.fields.name} hint="Renaming changes the clone URL." />
        <Field label="Description" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} error={errors.fields.description} />
        <Field label="Default branch" value={form.default_branch} onChange={(e) => setForm({ ...form, default_branch: e.target.value })} error={errors.fields.default_branch} />
        <label className="check"><input type="checkbox" checked={form.private} onChange={(e) => setForm({ ...form, private: e.target.checked })} /> Private repository</label>
        {errors.general && <p className="form-error" role="alert">{errors.general}</p>}
        <div className="form-actions">
          <button type="submit" className="button button-primary" disabled={save.isPending}>Save</button>
          {saved && <span className="form-ok" role="status">Saved.</span>}
        </div>
      </form>
    </section>
  );
}

function CollaboratorsSection({ repo }: { repo: Repo }) {
  const queryClient = useQueryClient();
  const key = ['collaborators', repo.owner, repo.name];
  const list = useQuery({ queryKey: key, queryFn: () => api<Collaborator[]>(`${repoAPI(repo.owner, repo.name)}/collaborators`) });
  const [username, setUsername] = useState('');
  const [role, setRole] = useState<Collaborator['role']>('write');
  const [error, setError] = useState('');
  const set = useMutation({
    mutationFn: (c: Collaborator) => api(`${repoAPI(repo.owner, repo.name)}/collaborators/${encodeURIComponent(c.username)}`, { method: 'PUT', body: JSON.stringify({ role: c.role }) }),
    onSuccess: () => { setUsername(''); setError(''); queryClient.invalidateQueries({ queryKey: key }); },
    onError: (e) => { const fe = formErrors(e); setError(fe.general || Object.values(fe.fields).join(' ')); },
  });
  const remove = useMutation({
    mutationFn: (u: string) => api(`${repoAPI(repo.owner, repo.name)}/collaborators/${encodeURIComponent(u)}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: key }),
  });
  return (
    <section className="panel">
      <h2>Collaborators</h2>
      <p className="muted">People who can see{repo.private ? '' : ' and change'} this repository besides you.</p>
      <form className="inline-form" onSubmit={(e) => { e.preventDefault(); set.mutate({ username, role }); }}>
        <input aria-label="Username" placeholder="username" value={username} onChange={(e) => setUsername(e.target.value)} />
        <select aria-label="Role" value={role} onChange={(e) => setRole(e.target.value as Collaborator['role'])}>
          <option value="read">Read</option><option value="write">Write</option><option value="admin">Admin</option>
        </select>
        <button type="submit" className="button" disabled={!username || set.isPending}>Add</button>
      </form>
      {error && <p className="form-error" role="alert">{error}</p>}
      <ul className="simple-list">
        {list.data?.map((c) => (
          <li key={c.username}>
            <strong>{c.username}</strong>
            <select aria-label={`Role of ${c.username}`} value={c.role} onChange={(e) => set.mutate({ username: c.username, role: e.target.value as Collaborator['role'] })}>
              <option value="read">Read</option><option value="write">Write</option><option value="admin">Admin</option>
            </select>
            <button type="button" className="button button-danger" onClick={() => remove.mutate(c.username)}>Remove</button>
          </li>
        ))}
        {list.data?.length === 0 && <li className="muted">No collaborators yet.</li>}
      </ul>
    </section>
  );
}

function ProtectionSection({ repo }: { repo: Repo }) {
  const queryClient = useQueryClient();
  const key = ['protection', repo.owner, repo.name];
  const list = useQuery({ queryKey: key, queryFn: () => api<ProtectionRule[]>(`${repoAPI(repo.owner, repo.name)}/protected-branches`) });
  const blank = { pattern: '', allow_force_push: false, allow_delete: false, require_pull_request: false };
  const [rule, setRule] = useState(blank);
  const [error, setError] = useState('');
  const create = useMutation({
    mutationFn: () => api(`${repoAPI(repo.owner, repo.name)}/protected-branches`, { method: 'POST', body: JSON.stringify(rule) }),
    onSuccess: () => { setRule(blank); setError(''); queryClient.invalidateQueries({ queryKey: key }); },
    onError: (e) => { const fe = formErrors(e); setError(fe.general || Object.values(fe.fields).join(' ')); },
  });
  const remove = useMutation({
    mutationFn: (id: number) => api(`${repoAPI(repo.owner, repo.name)}/protected-branches/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: key }),
  });
  return (
    <section className="panel">
      <h2>Branch protection</h2>
      <p className="muted">Protected branches reject force-pushes and deletion. Patterns may use globs, such as <code>release/*</code>.</p>
      <ul className="simple-list">
        {list.data?.map((p) => (
          <li key={p.id}>
            <code>{p.pattern}</code>
            <span className="muted small">
              {[p.require_pull_request && 'pull request required', p.allow_force_push && 'force-push allowed', p.allow_delete && 'deletion allowed'].filter(Boolean).join(' · ') || 'no force-push, no deletion'}
            </span>
            <button type="button" className="button button-danger" onClick={() => remove.mutate(p.id)}>Remove</button>
          </li>
        ))}
      </ul>
      <form onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <div className="inline-form">
          <input aria-label="Branch pattern" placeholder="main or release/*" value={rule.pattern} onChange={(e) => setRule({ ...rule, pattern: e.target.value })} />
          <button type="submit" className="button" disabled={!rule.pattern || create.isPending}>Add rule</button>
        </div>
        <div className="check-row">
          <label className="check"><input type="checkbox" checked={rule.require_pull_request} onChange={(e) => setRule({ ...rule, require_pull_request: e.target.checked })} /> Require a pull request</label>
          <label className="check"><input type="checkbox" checked={rule.allow_force_push} onChange={(e) => setRule({ ...rule, allow_force_push: e.target.checked })} /> Allow force-push</label>
          <label className="check"><input type="checkbox" checked={rule.allow_delete} onChange={(e) => setRule({ ...rule, allow_delete: e.target.checked })} /> Allow deletion</label>
        </div>
        {error && <p className="form-error" role="alert">{error}</p>}
      </form>
    </section>
  );
}

function DangerSection({ repo }: { repo: Repo }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const del = useMutation({
    mutationFn: () => api(repoAPI(repo.owner, repo.name), { method: 'DELETE', body: JSON.stringify({ confirm }) }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['my-repos'] }); navigate(`/${repo.owner}`); },
    onError: (e) => { const fe = formErrors(e); setError(fe.general || Object.values(fe.fields).join(' ')); },
  });
  return (
    <section className="panel danger">
      <h2>Delete this repository</h2>
      <p className="muted">This permanently deletes the repository, its history and its settings. It cannot be undone.</p>
      <form className="inline-form" onSubmit={(e) => { e.preventDefault(); del.mutate(); }}>
        <input aria-label={`Type ${repo.full_name} to confirm`} placeholder={repo.full_name} value={confirm} onChange={(e) => setConfirm(e.target.value)} />
        <button type="submit" className="button button-danger" disabled={confirm.toLowerCase() !== repo.full_name.toLowerCase() || del.isPending}>Delete repository</button>
      </form>
      {error && <p className="form-error" role="alert">{error}</p>}
    </section>
  );
}
