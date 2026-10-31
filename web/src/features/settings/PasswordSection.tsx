import { useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import { Field } from '../../components/Field';
import { formErrors, type FormErrors } from '../../components/formErrors';

/** Password change at `/settings/password` (PUT /api/v1/user/password). Ends your other sessions. */
export function PasswordSection() {
  const queryClient = useQueryClient();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [errors, setErrors] = useState<FormErrors>({ fields: {}, general: '' });
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setDone(false);
    if (next !== confirm) {
      setErrors({ fields: { confirm: 'does not match the new password' }, general: '' });
      return;
    }
    setBusy(true);
    setErrors({ fields: {}, general: '' });
    try {
      await api('/api/v1/user/password', { method: 'PUT', body: JSON.stringify({ current, new: next }) });
      setCurrent('');
      setNext('');
      setConfirm('');
      setDone(true);
      queryClient.invalidateQueries({ queryKey: ['user-sessions'] });
    } catch (err) {
      const fe = formErrors(err);
      if (fe.fields.password) fe.fields.new = fe.fields.password; // server names the policy field "password"
      setErrors(fe);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="panel">
      <h2>Change password</h2>
      <p className="muted">Changing your password signs you out everywhere except this browser.</p>
      <form onSubmit={submit} noValidate>
        <Field label="Current password" type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} error={errors.fields.current} />
        <Field label="New password" type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} error={errors.fields.new} hint="At least 10 characters, not a common password." />
        <Field label="Confirm new password" type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} error={errors.fields.confirm} />
        {errors.general && <p className="form-error" role="alert">{errors.general}</p>}
        <div className="form-actions">
          <button type="submit" className="button button-primary" disabled={busy || !current || !next}>Change password</button>
          {done && <span className="form-ok" role="status">Password changed.</span>}
        </div>
      </form>
    </section>
  );
}
