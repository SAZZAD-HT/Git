// build.mjs — turns docs/**/*.md and docs/diagrams/* into the static website in this folder.
//
// Output:
//   content.js   window.ANVIL = { docs, diagrams, search }  (loaded by index.html)
//   diagrams/    copies of docs/diagrams/svg/*.svg
//
// Run from the repository root or from site/:  npm --prefix site run build
// The site itself has no runtime dependencies: open site/index.html in a browser.

import { Marked } from 'marked';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SITE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(SITE, '..');
const DOCS = path.join(ROOT, 'docs');

// Formal documents, in register order. Metadata feeds the title block on each page.
const FORMAL = [
  { id: 'srs', file: '01-SRS-Software-Requirements-Specification.md', short: 'SRS', docId: 'ANV-SRS-001', standard: 'ISO/IEC/IEEE 29148:2018', summary: 'What the system must do: requirements, use cases, quality targets.' },
  { id: 'sdd', file: '02-SDD-Software-Design-Description.md', short: 'SDD', docId: 'ANV-SDD-001', standard: 'IEEE 1016-2009', summary: 'How it is built: design viewpoints with the UML diagrams.' },
  { id: 'stp', file: '03-STP-Software-Test-Plan.md', short: 'STP', docId: 'ANV-STP-001', standard: 'IEEE 829-2008', summary: 'How every requirement is tested, on Windows and Linux.' },
  { id: 'spmp', file: '04-SPMP-Software-Project-Management-Plan.md', short: 'SPMP', docId: 'ANV-SPMP-001', standard: 'ISO/IEC/IEEE 16326', summary: 'Phases, schedule, work breakdown and risks.' },
  { id: 'rtm', file: '05-RTM-Requirements-Traceability-Matrix.md', short: 'RTM', docId: 'ANV-RTM-001', standard: 'Traceability', summary: 'Every requirement linked to design and tests.' },
];

const DIAGRAM_TYPES = {
  CX: 'Context', UC: 'Use case', CP: 'Component', CD: 'Class', ER: 'Entity-relationship',
  SD: 'Sequence', AD: 'Activity', ST: 'State machine', DP: 'Deployment', PM: 'Project',
};

// ---------- collect documents ----------

/** Returns the list of documents to publish, with stable ids and repo-relative paths. */
function collectDocs() {
  const docs = FORMAL.map((d) => ({ ...d, group: 'formal', rel: d.file }));
  docs.push({ id: 'ref-index', group: 'reference', rel: 'reference/README.md', short: 'Index' });
  for (const f of fs.readdirSync(path.join(DOCS, 'reference')).sort()) {
    const m = f.match(/^(\d\d)-(.+)\.md$/);
    if (m) docs.push({ id: `ref-${m[1]}`, group: 'reference', rel: `reference/${f}`, short: m[1] });
  }
  for (const d of docs) {
    d.md = fs.readFileSync(path.join(DOCS, d.rel), 'utf8').replace(/\r\n/g, '\n');
    d.title = (d.md.match(/^#\s+(.+)$/m) || [, d.rel])[1].replace(/^\d\d\s+—\s+/, '').trim();
  }
  return docs;
}

/** Reads diagram titles from the Mermaid front matter in docs/diagrams/src. */
function collectDiagrams() {
  const src = path.join(DOCS, 'diagrams', 'src');
  return fs.readdirSync(src).filter((f) => f.endsWith('.mmd')).sort().map((f) => {
    const name = f.replace(/\.mmd$/, '');
    const text = fs.readFileSync(path.join(src, f), 'utf8');
    const full = (text.match(/^title:\s*"(.*)"\s*$/m) || [, name])[1];
    const code = name.slice(0, 5);                                      // e.g. "SD-03"
    const title = full.replace(/^[A-Z]{2}-\d\d\s+/, '').replace(/\s*\((UML |C4 )?[^)]*(Diagram|Chart|Level 1)\)\s*$/, '').trim();
    return { name, code, kind: DIAGRAM_TYPES[name.slice(0, 2)] || 'Other', title, full, source: text, usedIn: [] };
  });
}

// ---------- markdown rendering ----------

/** GitHub-compatible heading slug, so links like #4-functional-requirements keep working. */
function slugify(raw) {
  const plain = raw.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/<[^>]+>/g, '').replace(/[`*_]/g, '');
  return plain.toLowerCase().trim().replace(/[^\p{L}\p{N}\s-]/gu, '').replace(/\s/g, '-');
}

const escapeHtml = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

/**
 * Renders one document to HTML. Links between docs become hash routes (#/doc/<id>/<anchor>),
 * diagram images become figures pointing at site/diagrams/*.svg, Mermaid blocks are kept for
 * client-side rendering, and h2/h3 sections are returned for the search index.
 */
function renderDoc(doc, byPath, diagramsByName) {
  const dir = path.posix.dirname(doc.rel);
  const seen = new Map();
  const headings = [];

  const resolveHref = (href) => {
    if (/^(https?:|mailto:)/.test(href)) return { href, external: true };
    if (href.startsWith('#')) return { href: `#/doc/${doc.id}/${href.slice(1)}` };
    const [p, anchor] = href.split('#');
    const target = path.posix.normalize(path.posix.join(dir, p));
    const dia = target.match(/^diagrams\/(?:png|svg|src)\/([^/]+)\.(?:png|svg|mmd)$/);
    if (dia) return { href: `#/diagram/${dia[1]}`, diagram: dia[1] };
    if (/^diagrams\/?(src\/?|png\/?|svg\/?|README\.md)?$/.test(target)) return { href: '#/diagrams' };
    if (/^reference\/?$/.test(target)) return { href: '#/doc/ref-index' };
    const hit = byPath.get(target);
    if (hit) return { href: `#/doc/${hit.id}${anchor ? '/' + anchor : ''}` };
    return { href: null }; // points at code that does not exist yet (e.g. examples/) — render as text
  };

  const renderer = {
    heading({ tokens, depth, text }) {
      let slug = slugify(text);
      const n = seen.get(slug) || 0;
      seen.set(slug, n + 1);
      if (n) slug = `${slug}-${n}`;
      const html = this.parser.parseInline(tokens);
      if (depth >= 2 && depth <= 3) headings.push({ depth, slug, text: html.replace(/<[^>]+>/g, '') });
      const anchor = depth > 1 ? `<a class="h-anchor" href="#/doc/${doc.id}/${slug}" aria-label="Link to this section">#</a>` : '';
      return `<h${depth} id="${slug}">${html}${anchor}</h${depth}>\n`;
    },
    link({ href, title, tokens }) {
      const inner = this.parser.parseInline(tokens);
      const r = resolveHref(href);
      if (!r.href) return `<span class="dead-link" title="Planned path: ${escapeHtml(href)}">${inner}</span>`;
      if (r.diagram && diagramsByName.has(r.diagram)) diagramsByName.get(r.diagram).usedIn.push(doc.id);
      const t = title ? ` title="${escapeHtml(title)}"` : '';
      return r.external
        ? `<a href="${escapeHtml(r.href)}"${t} target="_blank" rel="noopener">${inner}</a>`
        : `<a href="${r.href}"${t}>${inner}</a>`;
    },
    image({ href, text }) {
      const r = resolveHref(href);
      if (r.diagram && diagramsByName.has(r.diagram)) {
        const d = diagramsByName.get(r.diagram);
        d.usedIn.push(doc.id);
        return `<a class="diagram-figure" href="#/diagram/${d.name}"><img src="diagrams/${d.name}.svg" alt="${escapeHtml(d.full)}" loading="lazy"><span class="diagram-tag">${d.code} · ${d.kind}</span></a>`;
      }
      return `<img src="${escapeHtml(href)}" alt="${escapeHtml(text)}">`;
    },
    code({ text, lang }) {
      if (lang === 'mermaid') return `<pre class="mermaid">${escapeHtml(text)}</pre>\n`;
      const cls = lang ? ` class="language-${escapeHtml(lang)}"` : '';
      return `<div class="code-wrap"><pre><code${cls}>${escapeHtml(text)}</code></pre></div>\n`;
    },
  };

  const md = doc.md.replace(/^#\s+.+\n/, ''); // the H1 is shown in the title block instead
  const local = new Marked({ gfm: true, renderer });
  let html = local.parse(md);
  html = html.replace(/<table>/g, '<div class="table-wrap"><table>').replace(/<\/table>/g, '</table></div>');
  return { html, headings };
}

/** Splits rendered HTML at h2/h3 boundaries into searchable sections. */
function sectionsFor(doc) {
  const parts = doc.html.split(/(?=<h[23] id=")/);
  return parts.map((part) => {
    const m = part.match(/^<h[23] id="([^"]+)">(.*?)<a class="h-anchor"/s);
    const text = part.replace(/<a class="h-anchor"[^>]*>#<\/a>/g, '').replace(/<[^>]+>/g, ' ')
      .replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&#39;/g, "'")
      .replace(/\s+/g, ' ').trim();
    return { doc: doc.id, anchor: m ? m[1] : '', heading: m ? m[2].replace(/<[^>]+>/g, '') : doc.title, text };
  }).filter((s) => s.text.length > 0);
}

// ---------- build ----------

const docs = collectDocs();
const diagrams = collectDiagrams();
const byPath = new Map(docs.map((d) => [d.rel, d]));
const diagramsByName = new Map(diagrams.map((d) => [d.name, d]));

for (const doc of docs) Object.assign(doc, renderDoc(doc, byPath, diagramsByName));
for (const d of diagrams) d.usedIn = [...new Set(d.usedIn)];

const search = docs.flatMap(sectionsFor);

// Mermaid writes width="100%" and a max-width style on the root <svg>, which gives <img> no
// intrinsic size. Replace them with the viewBox dimensions so images size predictably and the
// zoom viewer knows the real drawing size.
fs.mkdirSync(path.join(SITE, 'diagrams'), { recursive: true });
for (const d of diagrams) {
  let svg = fs.readFileSync(path.join(DOCS, 'diagrams', 'svg', `${d.name}.svg`), 'utf8');
  const vb = svg.match(/<svg[^>]*\bviewBox="([-\d.]+)[ ,]+([-\d.]+)[ ,]+([\d.]+)[ ,]+([\d.]+)"/);
  if (vb) {
    d.width = Math.ceil(Number(vb[3]));
    d.height = Math.ceil(Number(vb[4]));
    svg = svg.replace(/<svg\b[^>]*>/, (tag) => tag
      .replace(/\swidth="[^"]*"/, '').replace(/\sheight="[^"]*"/, '').replace(/\sstyle="[^"]*"/, '')
      .replace(/^<svg/, `<svg width="${d.width}" height="${d.height}"`));
  }
  fs.writeFileSync(path.join(SITE, 'diagrams', `${d.name}.svg`), svg);
}

const payload = {
  builtAt: new Date().toISOString(),
  docs: docs.map(({ md, ...d }) => d),
  diagrams,
  search,
};
fs.writeFileSync(path.join(SITE, 'content.js'),
  '// GENERATED by site/build.mjs from docs/ — do not edit. Run: npm --prefix site run build\n' +
  `window.ANVIL = ${JSON.stringify(payload)};\n`);

console.log(`Built ${docs.length} documents, ${diagrams.length} diagrams, ${search.length} search sections.`);
