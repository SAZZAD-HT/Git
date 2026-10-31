/**
 * A small status pill. Uses an icon and text as well as colour, so status is never
 * conveyed by colour alone (docs/reference/09 §6).
 * @param ok - true for a good state (green check), false for a bad one (red cross).
 * @param label - the text shown next to the icon.
 */
export function StatusBadge({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span className={`badge ${ok ? 'badge-ok' : 'badge-bad'}`}>
      <span aria-hidden="true">{ok ? '✓' : '✕'}</span> {label}
    </span>
  );
}
