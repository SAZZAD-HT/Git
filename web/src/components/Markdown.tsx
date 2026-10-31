import { useMemo } from 'react';
import { Marked } from 'marked';
import DOMPurify from 'dompurify';

const marked = new Marked({ gfm: true, breaks: false });

// Every link opened from user content gets rel="noopener nofollow" (docs/reference/09 §4).
// DOMPurify needs a DOM; outside a browser (unit tests in Node) it is unsupported.
if (DOMPurify.isSupported) {
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (node.tagName === 'A' && node.getAttribute('href')) {
      node.setAttribute('rel', 'noopener nofollow');
      if (/^https?:/i.test(node.getAttribute('href') ?? '')) node.setAttribute('target', '_blank');
    }
  });
}

/**
 * Renders Markdown from repositories, issues and comments. The HTML from `marked` always
 * passes through DOMPurify before it reaches the DOM, so scripts and event handlers in
 * user content never run (docs/reference/10 §3).
 * @param source - the Markdown text.
 */
export function Markdown({ source }: { source: string }) {
  const html = useMemo(() => DOMPurify.sanitize(marked.parse(source, { async: false }) as string), [source]);
  return <div className="markdown" dangerouslySetInnerHTML={{ __html: html }} />;
}
