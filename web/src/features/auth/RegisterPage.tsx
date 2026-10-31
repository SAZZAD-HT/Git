import { useState, type ChangeEvent, type FormEvent } from 'react';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../../api/client';
import type { SessionInfo } from '../../api/types';
import { safeNext, useSession } from '../../app/session';
import { Field } from '../../components/Field';
import { formErrors, type FormErrors } from '../../components/formErrors';
import { useMeta } from './LoginPage';

/** The registration page at `/register`. Shown only when the server allows self-registration. */
export function RegisterPage() {
  const { user, signedIn } = useSession();
  const meta = useMeta();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [form, setForm] = useState({ username: '', email: '', full_name: '', password: '' });
  const [errors, setErrors] = useState<FormErrors>({ fields: {}, general: '' });
  const [busy, setBusy] = useState(false);
  const next = safeNext(params.get('next'));

  if (user) return <Navigate to={next} replace />;
  if (meta.data && !meta.data.allow_registration) {
    return (
      <div className="auth-card panel">
        <h1>Registration is closed</h1>
        <p className="muted">Ask an administrator of this server to create an account for you.</p>
        <p><Link to="/login">Sign in</Link></p>
      </div>
    );
  }

  const set = (k: keyof typeof form) => (e: ChangeEvent<HTMLInputElement>) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setErrors({ fields: {}, general: '' });
    try {
      const info = await api<SessionInfo>('/api/v1/auth/register', { method: 'POST', body: JSON.stringify(form) });
      signedIn(info);
      navigate(next, { replace: true });
    } catch (err) {
      setErrors(formErrors(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="auth-card panel">
      <h1>Create your account</h1>
      <form onSubmit={submit} noValidate>
        <Field label="Username" autoComplete="username" required autoFocus value={form.username} onChange={set('username')} error={errors.fields.username} hint="Letters, digits and hyphens. Appears in your repository URLs." />
        <Field label="Email" type="email" autoComplete="email" required value={form.email} onChange={set('email')} error={errors.fields.email} />
        <Field label="Full name (optional)" autoComplete="name" value={form.full_name} onChange={set('full_name')} error={errors.fields.full_name} />
        <Field label="Password" type="password" autoComplete="new-password" required value={form.password} onChange={set('password')} error={errors.fields.password} hint="At least 10 characters, not a common password." />
        {errors.general && <p className="form-error" role="alert">{errors.general}</p>}
        <button type="submit" className="button button-primary button-block" disabled={busy}>{busy ? 'Creating…' : 'Create account'}</button>
      </form>
      <p className="auth-alt">Already have an account? <Link to="/login">Sign in</Link></p>
    </div>
  );
}
