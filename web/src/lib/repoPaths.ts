/**
 * Splits the URL tail after /tree/ or /blob/ into ref and path. Branch names may contain
 * slashes, so the longest known branch that is a prefix wins (same rule as the server's
 * repos.SplitRefPath); otherwise the first segment is the ref (a commit sha).
 * @param tail - the part of the URL after /tree/ or /blob/.
 * @param branches - known branch names.
 * @param fallback - the ref to use when tail is empty (the default branch).
 */
export function splitRefPath(tail: string, branches: string[], fallback: string): [string, string] {
  if (!tail) return [fallback, ''];
  let best = '';
  for (const b of branches) if ((tail === b || tail.startsWith(b + '/')) && b.length > best.length) best = b;
  if (best) return [best, tail.slice(best.length + 1)];
  const i = tail.indexOf('/');
  return i < 0 ? [tail, ''] : [tail.slice(0, i), tail.slice(i + 1)];
}

/** Formats a byte count as B, KiB or MiB. */
export function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
  return `${(n / 1024 / 1024).toFixed(1)} MiB`;
}
