import { useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { Meta, SessionInfo } from '../../api/types';
import { safeNext, useSession } from '../../app/session';
import { Field } from '../../components/Field';
import { formErrors } from '../../components/formErrors';

/** Returns the public instance settings. Query key `['meta']`; never invalidated. */
export function useMeta() {
  return useQuery({ queryKey: ['meta'], queryFn: () => api<Meta>('/api/v1/meta'), staleTime: Infinity });
}

/** The sign-in page at `/login`. Redirects to `?next=` (a safe local path) afterwards. */
export function LoginPage() {
  const { user, signedIn } = useSession();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const meta = useMeta();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const next = safeNext(params.get('next'));

  if (user) return <Navigate to={next} replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const info = await api<SessionInfo>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) });
      signedIn(info);
      navigate(next, { replace: true });
    } catch (err) {
      setError(formErrors(err).general);
      setPassword('');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="auth-card panel">
      <h1>Sign in to Anvil</h1>
      <form onSubmit={submit} noValidate>
        <Field label="Username" name="username" autoComplete="username" required autoFocus value={username} onChange={(e) => setUsername(e.target.value)} />
        <Field label="Password" name="password" type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
        {error && <p className="form-error" role="alert">{error}</p>}
        <button type="submit" className="button button-primary button-block" disabled={busy || !username || !password}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
      {meta.data?.allow_registration && (
        <p className="auth-alt">New here? <Link to={`/register${next !== '/' ? `?next=${encodeURIComponent(next)}` : ''}`}>Create an account</Link></p>
      )}
    </div>
  );
}
