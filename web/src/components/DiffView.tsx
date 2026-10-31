import { useState, type ReactNode } from 'react';
import type { Diff, DiffLine, FileDiff } from '../api/types';

/**
 * Hooks for line comments in a diff (pull request reviews). Lines are addressed by side:
 * `old` for deleted lines (old line number), `new` for added and context lines.
 */
export interface DiffAnnotations {
  /** Content shown under a line, e.g. existing comments or a comment form; null for none. */
  below: (path: string, side: 'old' | 'new', line: number) => ReactNode;
  /** When set, each line gets a "+" button that calls it. */
  onAdd?: (path: string, side: 'old' | 'new', line: number) => void;
}

/** The side and line number a comment on this diff line attaches to. */
export function lineAnchor(l: DiffLine): { side: 'old' | 'new'; line: number } {
  return l.kind === 'del' ? { side: 'old', line: l.old } : { side: 'new', line: l.new };
}

/**
 * Shows a diff file by file: a summary of changed files, then each file's hunks in unified
 * form with old and new line numbers. Additions and deletions are marked with +/− as well
 * as colour (docs/reference/09 §6). Files can be collapsed.
 * @param diff - the parsed diff from the API.
 * @param annotations - optional line comments (see {@link DiffAnnotations}).
 */
export function DiffView({ diff, annotations }: { diff: Diff; annotations?: DiffAnnotations }) {
  if (diff.files.length === 0) return <p className="muted panel">No file changes.</p>;
  return (
    <div className="diff">
      <p className="diff-summary">
        Showing <strong>{diff.files.length} changed file{diff.files.length === 1 ? '' : 's'}</strong> with{' '}
        <span className="add-count">{diff.additions} addition{diff.additions === 1 ? '' : 's'}</span> and{' '}
        <span className="del-count">{diff.deletions} deletion{diff.deletions === 1 ? '' : 's'}</span>
        {diff.truncated && <span className="muted"> (the diff is too large to show completely)</span>}
      </p>
      {diff.files.map((f, i) => <FileBlock key={i} f={f} annotations={annotations} />)}
    </div>
  );
}

function FileBlock({ f, annotations }: { f: FileDiff; annotations?: DiffAnnotations }) {
  const [open, setOpen] = useState(true);
  const path = f.status === 'deleted' ? f.old_path : f.new_path;
  return (
    <section className="panel diff-file">
      <header className="diff-file-head">
        <button type="button" className="icon-button diff-toggle" aria-expanded={open} aria-label={open ? 'Collapse file' : 'Expand file'} onClick={() => setOpen((v) => !v)}>
          {open ? '▾' : '▸'}
        </button>
        <span className="diff-stat"><span className="add-count">+{f.additions}</span> <span className="del-count">−{f.deletions}</span></span>
        <code className="diff-path">{f.status === 'renamed' || f.status === 'copied' ? `${f.old_path} → ${f.new_path}` : path}</code>
        {f.status !== 'modified' && <span className={`badge diff-status diff-${f.status}`}>{f.status}</span>}
      </header>
      {open && (
        f.binary ? <p className="muted blob-note">Binary file changed.</p>
        : f.hunks.length === 0 ? <p className="muted blob-note">No content changes (mode or rename only).</p>
        : (
          <div className="code-view">
            <table className="diff-table">
              <tbody>
                {f.hunks.map((h, hi) => (
                  <HunkRows key={hi} path={path} header={h.header} lines={h.lines} annotations={annotations} />
                ))}
              </tbody>
            </table>
            {f.truncated && <p className="muted blob-note">This file's diff is too long to show completely.</p>}
          </div>
        )
      )}
    </section>
  );
}

function HunkRows({ path, header, lines, annotations }: { path: string; header: string; lines: DiffLine[]; annotations?: DiffAnnotations }) {
  return (
    <>
      <tr className="hunk-head"><td colSpan={3}><code>{header}</code></td></tr>
      {lines.map((l, i) => {
        const a = lineAnchor(l);
        const onAdd = annotations?.onAdd;
        return (
          <LineRows
            key={i}
            l={l}
            below={annotations?.below(path, a.side, a.line)}
            onAdd={onAdd && (() => onAdd(path, a.side, a.line))}
          />
        );
      })}
    </>
  );
}

function LineRows({ l, below, onAdd }: { l: DiffLine; below: ReactNode; onAdd?: () => void }) {
  return (
    <>
      <tr className={`dl dl-${l.kind}`}>
        <td className="ln">{l.old || ''}</td>
        <td className="ln">
          {onAdd && <button type="button" className="line-add" aria-label={`Comment on line ${l.kind === 'del' ? l.old : l.new}`} onClick={onAdd}>+</button>}
          {l.new || ''}
        </td>
        <td className="lc"><code><span className="dl-mark" aria-hidden="true">{l.kind === 'add' ? '+' : l.kind === 'del' ? '−' : ' '}</span>{l.text}</code></td>
      </tr>
      {below != null && below !== false && (
        <tr className="dl-notes"><td colSpan={3}>{below}</td></tr>
      )}
    </>
  );
}
