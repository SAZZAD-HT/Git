# Anvil Handbook (documentation website)

A static HTML/CSS/JS website that presents everything in `docs/`: the formal IEEE documents, the reference annexes and all UML diagrams, with search, a zoomable diagram viewer and the build plan.

## Open it

Double-click `site/index.html`. No server is needed. Fonts come from Google Fonts and the annex sketch diagrams use Mermaid from cdnjs; offline, the site still works with fallback fonts and shows those sketches as text.

## Rebuild after changing the docs

```bash
npm --prefix site install    # once
npm --prefix site run build  # regenerates content.js and diagrams/
```

`build.mjs` converts every Markdown file to HTML, rewrites links between documents into site routes, copies the diagram SVGs, and builds the search index.

## Files

| File | Edited by hand? | Purpose |
|---|---|---|
| `index.html` | yes | Page shell: top bar, sidebar, search dialog |
| `styles.css` | yes | Design tokens (light and dark), layout, typography |
| `app.js` | yes | Routing, rendering, search, diagram viewer, build-plan timeline |
| `build.mjs` | yes | Build script |
| `content.js` | **no**, generated | All documents, diagram metadata and search index |
| `diagrams/` | **no**, generated | SVG copies of `docs/diagrams/svg` with explicit sizes |

## Features

- Every document with a drawing-style title block, "On this page" navigation and previous/next links.
- Requirement, test, use-case and diagram IDs (`FR-CI-08`, `TC-REPO-17`, `SD-03`) are clickable. Diagram IDs open the diagram; other IDs search for every mention.
- Tables with 12 or more rows get a filter box (useful in the SRS, STP and RTM).
- Full-text search: **Ctrl K** or **/**.
- Diagram catalogue filtered by UML type; the viewer supports drag, scroll-zoom, keyboard (+ − 0, arrows) and shows the Mermaid source and which documents use each diagram.
- Light, dark and system themes. Works at phone width.
