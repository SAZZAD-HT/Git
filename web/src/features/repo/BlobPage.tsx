import { useOutletContext, useParams } from 'react-router-dom';
import { encodePath, repoAPI, useBlob, useBranches } from '../../api/hooks/useRepo';
import { Markdown } from '../../components/Markdown';
import { formatSize, splitRefPath } from '../../lib/repoPaths';
import { BranchSelect, Breadcrumbs } from './CodePage';
import type { RepoContext } from './RepoLayout';

/**
 * One file at `/:owner/:repo/blob/:ref/*`, with line numbers. Markdown can be shown
 * rendered or as source; binary and very large files offer a download instead.
 */
export function BlobPage() {
  const { repo } = useOutletContext<RepoContext>();
  const tail = useParams()['*'] ?? '';
  const branches = useBranches(repo.owner, repo.name);
  const names = branches.data?.map((b) => b.name) ?? [];
  const [ref, path] = splitRefPath(tail, names, repo.default_branch);
  const blob = useBlob(repo.owner, repo.name, ref, path, !!branches.data);
  const raw = `${repoAPI(repo.owner, repo.name)}/raw/${encodePath(ref)}/${encodePath(path)}`;
  const isMarkdown = /\.(md|markdown)$/i.test(path);

  return (
    <>
      <div className="code-bar">
        <BranchSelect repo={repo} current={ref} branches={names} path={path} kind="blob" />
        <Breadcrumbs repo={repo} gitRef={ref} path={path} />
      </div>
      {blob.isPending && <p className="muted">Loading…</p>}
      {blob.isError && <p className="error">{blob.error.message}</p>}
      {blob.data && (
        <section className="panel blob">
          <div className="blob-head">
            <span className="muted">{formatSize(blob.data.blob.size)}{!blob.data.blob.binary && !blob.data.blob.too_large && ` · ${countLines(blob.data.blob.content)} lines`}</span>
            <a className="button" href={raw}>Download</a>
          </div>
          {blob.data.blob.binary ? (
            <p className="muted blob-note">Binary file not shown.</p>
          ) : blob.data.blob.too_large ? (
            <p className="muted blob-note">File too large to display. Download it instead.</p>
          ) : isMarkdown ? (
            <div className="readme"><Markdown source={blob.data.blob.content} /></div>
          ) : (
            <CodeLines text={blob.data.blob.content} />
          )}
        </section>
      )}
    </>
  );
}

function countLines(s: string): number {
  if (!s) return 0;
  return s.endsWith('\n') ? s.split('\n').length - 1 : s.split('\n').length;
}

/** Plain text with line numbers; each line links to #L<n>. Syntax highlighting arrives later. */
function CodeLines({ text }: { text: string }) {
  const lines = text.replace(/\n$/, '').split('\n');
  return (
    <div className="code-view">
      <table>
        <tbody>
          {lines.map((l, i) => (
            <tr key={i} id={`L${i + 1}`}>
              <td className="ln"><a href={`#L${i + 1}`}>{i + 1}</a></td>
              <td className="lc"><code>{l || ' '}</code></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
