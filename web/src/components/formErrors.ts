import { ApiError } from '../api/client';

/** Field messages and a general message extracted from an API error, for forms. */
export interface FormErrors {
  fields: Record<string, string>;
  general: string;
}

/**
 * Splits an error into per-field messages (from a `validation_failed` response's
 * `details.fields`) and one general message for everything else.
 */
export function formErrors(e: unknown): FormErrors {
  if (e instanceof ApiError) {
    const fields = (e.details.fields as Record<string, string> | undefined) ?? {};
    const general = Object.keys(fields).length ? '' : e.message;
    return { fields, general };
  }
  return { fields: {}, general: e instanceof Error ? e.message : String(e) };
}

/** Formats an RFC 3339 timestamp for display in the viewer's locale, or "never" when empty. */
export function formatTime(iso?: string): string {
  if (!iso) return 'never';
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}
