import type { IssueComment } from '../api/types';

/**
 * Picks black or white text for a label background so the name stays readable
 * (WCAG relative luminance). Falls back to white for malformed colours.
 * @param hex - background colour such as `#1d76db`.
 */
export function labelTextColor(hex: string): string {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) return '#ffffff';
  const n = parseInt(m[1], 16);
  const lin = (c: number) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  const l = 0.2126 * lin((n >> 16) & 255) + 0.7152 * lin((n >> 8) & 255) + 0.0722 * lin(n & 255);
  return l > 0.179 ? '#000000' : '#ffffff';
}

/** Key that ties an inline comment to a diff line: `path\u0000side\u0000line`. */
export function lineKey(path: string, side: 'old' | 'new', line: number): string {
  return `${path}\u0000${side}\u0000${line}`;
}

/** Groups inline review comments (those with a path) by {@link lineKey}. */
export function groupInline(comments: IssueComment[]): Map<string, IssueComment[]> {
  const m = new Map<string, IssueComment[]>();
  for (const c of comments) {
    if (!c.path || !c.line || !c.side) continue;
    const k = lineKey(c.path, c.side, c.line);
    const list = m.get(k);
    if (list) list.push(c);
    else m.set(k, [c]);
  }
  return m;
}

/** A reviewer's latest decision on a pull request. */
export interface ReviewVerdict {
  author: string;
  state: 'approve' | 'request_changes';
}

/**
 * Works out each reviewer's current verdict: their latest approve or request-changes
 * review. Plain "comment" reviews don't change a verdict. Order follows the first review.
 */
export function reviewVerdicts(comments: IssueComment[]): ReviewVerdict[] {
  const latest = new Map<string, ReviewVerdict['state']>();
  for (const c of comments) {
    if (c.kind !== 'review' || c.path) continue;
    if (c.review_state === 'approve' || c.review_state === 'request_changes') latest.set(c.author, c.review_state);
  }
  return [...latest].map(([author, state]) => ({ author, state }));
}
