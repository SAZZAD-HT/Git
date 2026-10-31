import { useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { User } from '../../api/types';
import { useSession } from '../../app/session';
import { Field } from '../../components/Field';
import { formErrors, formatTime, type FormErrors } from '../../components/formErrors';

/**
 * Site administration of users at `/admin/users` (FR-ADM-01). Query key `['admin-users']`,
 * invalidated after creating or changing a user.
 */
export function AdminUsersPage() {
  const { user: me } = useSession();
  const queryClient = useQueryClient();
  const list = useQuery({ queryKey: ['admin-users'], queryFn: () => api<User[]>('/api/v1/admin/users?per_page=100') });
  const [form, setForm] = useState({ username: '', email: '', full_name: '', password: '', is_admin: false });
  const [errors, setErrors] = useState<FormErrors>({ fields: {}, general: '' });
  const [actionError, setActionError] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  const create = useMutation({
    mutationFn: () => api<User>('/api/v1/admin/users', { method: 'POST', body: JSON.stringify(form) }),
    onSuccess: () => {
      setForm({ username: '', email: '', full_name: '', password: '', is_admin: false });
      setErrors({ fields: {}, general: '' });
      setShowCreate(false);
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
    },
    onError: (e) => setErrors(formErrors(e)),
  });

  const update = useMutation({
    mutationFn: ({ username, patch }: { username: string; patch: Partial<Pick<User, 'is_admin' | 'is_active'>> }) =>
      api<User>(`/api/v1/admin/users/${encodeURIComponent(username)}`, { method: 'PATCH', body: JSON.stringify(patch) }),
    onSuccess: () => {
      setActionError('');
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
    },
    onError: (e) => setActionError(formErrors(e).general),
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };

  return (
    <>
      <div className="page-head">
        <h1>Users</h1>
        <button type="button" className="button button-primary" onClick={() => setShowCreate((v) => !v)} aria-expanded={showCreate}>
          {showCreate ? 'Cancel' : 'New user'}
        </button>
      </div>

      {showCreate && (
        <section className="panel">
          <h2>Create user</h2>
          <form onSubmit={submit} noValidate className="form-grid">
            <Field label="Username" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} error={errors.fields.username} />
            <Field label="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} error={errors.fields.email} />
            <Field label="Full name" value={form.full_name} onChange={(e) => setForm({ ...form, full_name: e.target.value })} error={errors.fields.full_name} />
            <Field label="Initial password" type="password" autoComplete="new-password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} error={errors.fields.password} />
            <label className="check"><input type="checkbox" checked={form.is_admin} onChange={(e) => setForm({ ...form, is_admin: e.target.checked })} /> Site administrator</label>
            {errors.general && <p className="form-error" role="alert">{errors.general}</p>}
            <div><button type="submit" className="button button-primary" disabled={create.isPending}>Create user</button></div>
          </form>
        </section>
      )}

      {actionError && <p className="form-error" role="alert">{actionError}</p>}
      <section className="panel">
        {list.isPending && <p className="muted">Loading…</p>}
        {list.isError && <p className="error">{list.error.message}</p>}
        {list.data && (
          <div className="table-wrap">
            <table className="table">
              <thead><tr><th>User</th><th>Email</th><th>Role</th><th>Status</th><th>Joined</th><th><span className="sr-only">Actions</span></th></tr></thead>
              <tbody>
                {list.data.map((u) => (
                  <tr key={u.id}>
                    <td><strong>{u.username}</strong>{u.full_name && <div className="muted">{u.full_name}</div>}</td>
                    <td>{u.email}</td>
                    <td>{u.is_admin ? 'Admin' : 'User'}</td>
                    <td>{u.is_active ? <span className="badge badge-ok">✓ Active</span> : <span className="badge badge-bad">✕ Deactivated</span>}</td>
                    <td>{formatTime(u.created_at)}</td>
                    <td className="row-actions">
                      <button type="button" className="button" disabled={update.isPending || u.id === me?.id}
                        onClick={() => update.mutate({ username: u.username, patch: { is_admin: !u.is_admin } })}>
                        {u.is_admin ? 'Demote' : 'Make admin'}
                      </button>
                      <button type="button" className={u.is_active ? 'button button-danger' : 'button'} disabled={update.isPending || u.id === me?.id}
                        onClick={() => update.mutate({ username: u.username, patch: { is_active: !u.is_active } })}>
                        {u.is_active ? 'Deactivate' : 'Activate'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  );
}
