import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Repo } from '../../api/types';
import { useSession } from '../../app/session';
import { Field } from '../../components/Field';
import { formErrors, type FormErrors } from '../../components/formErrors';

/** Create a repository at `/new` (FR-REPO-01). */
export function NewRepoPage() {
  const { user } = useSession();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form, setForm] = useState({ name: '', description: '', private: false, init_readme: true, default_branch: 'main' });
  const [errors, setErrors] = useState<FormErrors>({ fields: {}, general: '' });
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErrors({ fields: {}, general: '' });
    try {
      const repo = await api<Repo>('/api/v1/repos', { method: 'POST', body: JSON.stringify(form) });
      queryClient.invalidateQueries({ queryKey: ['my-repos'] });
      navigate(`/${repo.owner}/${repo.name}`);
    } catch (err) {
      setErrors(formErrors(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="narrow">
      <h1>Create a repository</h1>
      <p className="lead">A repository holds your project's files and their full history.</p>
      <form onSubmit={submit} noValidate className="panel">
        <div className="name-row">
          <div className="field">
            <label>Owner</label>
            <input value={user?.username ?? ''} disabled />
          </div>
          <span className="name-slash" aria-hidden="true">/</span>
          <Field label="Repository name" required autoFocus value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} error={errors.fields.name} />
        </div>
        <Field label="Description (optional)" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} error={errors.fields.description} />
        <fieldset className="fieldset">
          <legend>Visibility</legend>
          <label className="check radio-card">
            <input type="radio" name="vis" checked={!form.private} onChange={() => setForm({ ...form, private: false })} />
            <span><strong>Public</strong><br /><span className="muted">Anyone who can reach this server can see it. You choose who can push.</span></span>
          </label>
          <label className="check radio-card">
            <input type="radio" name="vis" checked={form.private} onChange={() => setForm({ ...form, private: true })} />
            <span><strong>Private</strong><br /><span className="muted">Only you and the collaborators you add can see it.</span></span>
          </label>
        </fieldset>
        <label className="check"><input type="checkbox" checked={form.init_readme} onChange={(e) => setForm({ ...form, init_readme: e.target.checked })} /> Add a README file</label>
        <Field label="Default branch" value={form.default_branch} onChange={(e) => setForm({ ...form, default_branch: e.target.value })} error={errors.fields.default_branch} />
        {errors.general && <p className="form-error" role="alert">{errors.general}</p>}
        <button type="submit" className="button button-primary" disabled={busy || !form.name}>{busy ? 'Creating…' : 'Create repository'}</button>
      </form>
    </div>
  );
}
