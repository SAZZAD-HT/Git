import { useId, type InputHTMLAttributes } from 'react';

/**
 * A labelled text input with an optional hint and error message, wired up for screen
 * readers (aria-describedby, aria-invalid).
 * @param label - visible label text.
 * @param error - validation message shown under the input.
 * @param hint - help text shown under the input when there is no error.
 */
export function Field({ label, error, hint, ...input }: InputHTMLAttributes<HTMLInputElement> & { label: string; error?: string; hint?: string }) {
  const id = useId();
  const describe = error ? `${id}-err` : hint ? `${id}-hint` : undefined;
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input id={id} aria-invalid={error ? true : undefined} aria-describedby={describe} {...input} />
      {error ? <p id={`${id}-err`} className="field-error">{error}</p> : hint ? <p id={`${id}-hint`} className="field-hint">{hint}</p> : null}
    </div>
  );
}
