import { useState, type FormEvent } from 'react';
import { api } from '../../api/client';
import { useSession } from '../../app/session';
import { Field } from '../../components/Field';
import { formErrors, type FormErrors } from '../../components/formErrors';

/** Profile settings at `/settings`: display name and email (PATCH /api/v1/user). */
export function ProfileSection() {
  const { user, refresh } = useSession();
  const [fullName, setFullName] = useState(user?.full_name ?? '');
  const [email, setEmail] = useState(user?.email ?? '');
  const [errors, setErrors] = useState<FormErrors>({ fields: {}, general: '' });
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setSaved(false);
    setErrors({ fields: {}, general: '' });
    try {
      await api('/api/v1/user', { method: 'PATCH', body: JSON.stringify({ full_name: fullName, email }) });
      await refresh();
      setSaved(true);
    } catch (err) {
      setErrors(formErrors(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel">
      <h2>Profile</h2>
      <form onSubmit={submit} noValidate>
        <Field label="Username" value={user?.username ?? ''} disabled hint="Usernames cannot be changed yet." />
        <Field label="Full name" autoComplete="name" value={fullName} onChange={(e) => setFullName(e.target.value)} error={errors.fields.full_name} />
        <Field label="Email" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} error={errors.fields.email} />
        {errors.general && <p className="form-error" role="alert">{errors.general}</p>}
        <div className="form-actions">
          <button type="submit" className="button button-primary" disabled={busy}>Save profile</button>
          {saved && <span className="form-ok" role="status">Saved.</span>}
        </div>
      </form>
    </section>
  );
}
