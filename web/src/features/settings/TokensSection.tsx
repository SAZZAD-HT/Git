import { useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { AccessToken } from '../../api/types';
import { Field } from '../../components/Field';
import { formErrors, formatTime, type FormErrors } from '../../components/formErrors';

const SCOPES: { id: string; label: string }[] = [
  { id: 'repo:read', label: 'repo:read — clone and read repositories' },
  { id: 'repo:write', label: 'repo:write — push and change repositories' },
  { id: 'package:write', label: 'package:write — publish packages' },
  { id: 'admin', label: 'admin — site administration (admins only)' },
];

const EXPIRY: { days: number; label: string }[] = [
  { days: 30, label: '30 days' },
  { days: 90, label: '90 days' },
  { days: 365, label: '1 year' },
  { days: 0, label: 'Never' },
];

/**
 * Personal access tokens at `/settings/tokens` (FR-AUTH-04). Query key `['user-tokens']`,
 * invalidated after creating or revoking a token. A new token's value is shown once.
 */
export function TokensSection() {
  const queryClient = useQueryClient();
  const tokens = useQuery({ queryKey: ['user-tokens'], queryFn: () => api<AccessToken[]>('/api/v1/user/tokens') });
  const [name, setName] = useState('');
  const [scopes, setScopes] = useState<string[]>(['repo:read']);
  const [days, setDays] = useState(90);
  const [errors, setErrors] = useState<FormErrors>({ fields: {}, general: '' });
  const [created, setCreated] = useState<AccessToken | null>(null);
  const [copied, setCopied] = useState(false);

  const create = useMutation({
    mutationFn: () =>
      api<AccessToken>('/api/v1/user/tokens', {
        method: 'POST',
        body: JSON.stringify({ name, scopes, expires_at: days ? new Date(Date.now() + days * 864e5).toISOString() : null }),
      }),
    onSuccess: (tok) => {
      setCreated(tok);
      setCopied(false);
      setName('');
      setErrors({ fields: {}, general: '' });
      queryClient.invalidateQueries({ queryKey: ['user-tokens'] });
    },
    onError: (e) => setErrors(formErrors(e)),
  });

  const revoke = useMutation({
    mutationFn: (id: number) => api(`/api/v1/user/tokens/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['user-tokens'] }),
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };

  const toggle = (id: string) => setScopes((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  const copy = async () => {
    if (!created?.token) return;
    try {
      await navigator.clipboard.writeText(created.token);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return (
    <>
      <section className="panel">
        <h2>New access token</h2>
        <p className="muted">Use a token instead of your password for the API and for <code>git</code> over HTTPS (enter it as the password).</p>
        {created?.token && (
          <div className="token-reveal" role="status">
            <p><strong>Copy your new token now.</strong> You will not be able to see it again.</p>
            <div className="token-row">
              <code className="token-value">{created.token}</code>
              <button type="button" className="button" onClick={copy}>{copied ? 'Copied' : 'Copy'}</button>
            </div>
          </div>
        )}
        <form onSubmit={submit} noValidate>
          <Field label="Name" placeholder="e.g. laptop git" value={name} onChange={(e) => setName(e.target.value)} error={errors.fields.name} />
          <fieldset className="fieldset">
            <legend>Scopes</legend>
            {SCOPES.map((s) => (
              <label key={s.id} className="check">
                <input type="checkbox" checked={scopes.includes(s.id)} onChange={() => toggle(s.id)} /> {s.label}
              </label>
            ))}
            {errors.fields.scopes && <p className="field-error">{errors.fields.scopes}</p>}
          </fieldset>
          <div className="field">
            <label htmlFor="token-expiry">Expires</label>
            <select id="token-expiry" value={days} onChange={(e) => setDays(Number(e.target.value))}>
              {EXPIRY.map((x) => <option key={x.days} value={x.days}>{x.label}</option>)}
            </select>
          </div>
          {errors.general && <p className="form-error" role="alert">{errors.general}</p>}
          <button type="submit" className="button button-primary" disabled={create.isPending}>Create token</button>
        </form>
      </section>

      <section className="panel">
        <h2>Your tokens</h2>
        {tokens.isPending && <p className="muted">Loading…</p>}
        {tokens.isError && <p className="error">{tokens.error.message}</p>}
        {tokens.data?.length === 0 && <p className="muted">No tokens yet.</p>}
        {!!tokens.data?.length && (
          <div className="table-wrap">
            <table className="table">
              <thead><tr><th>Name</th><th>Token</th><th>Scopes</th><th>Last used</th><th>Expires</th><th><span className="sr-only">Actions</span></th></tr></thead>
              <tbody>
                {tokens.data.map((t) => (
                  <tr key={t.id}>
                    <td>{t.name}</td>
                    <td><code>{t.token_prefix}…</code></td>
                    <td>{t.scopes.join(', ')}</td>
                    <td>{formatTime(t.last_used_at)}</td>
                    <td>{formatTime(t.expires_at)}</td>
                    <td><button type="button" className="button button-danger" disabled={revoke.isPending} onClick={() => revoke.mutate(t.id)}>Revoke</button></td>
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
