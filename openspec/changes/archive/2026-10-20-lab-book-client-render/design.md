# Design

## Context

The server builds the HTML Lab Book in `internal/labbook` (Goldmark, `book.html.tmpl`, vendored Mermaid 12 inlined in a script) and serves it from `GET /api/docs/export`. Its bootstrap script turns `pre > code.language-mermaid` into Mermaid nodes on open. The final image is `distroless/static` for amd64 and arm64, with no browser or renderer. The web app bundles the same Mermaid major version and draws in-app diagrams in `components/docs/Mermaid.tsx`, which calls the global `mermaid.initialize` with the user's palette. Scheduled reports build the same HTML on the server with no browser involved. See proposal.md for why server rendering was dropped.

## Goals / Non-Goals

**Goals:** browser-only pre-rendering and printing, with the server change limited to template and cover data; exported diagrams identical across users; in-app diagrams unaffected by exports.

**Non-Goals:** server PDF or any server renderer, a job queue, pre-rendered SVG in report attachments, SVG files in `md.zip`, printed page numbers in the contents (no Paged.js), Playwright or other browser automation in CI.

## Decisions

- **Server-side HTML stays the single source of the book.** The web app fetches `GET /api/docs/export?format=html` and transforms it, instead of assembling a book from the docs API. Scoping, attachments, anchors and the CSP stay in one place. Alternative: a client-built book, which would duplicate the Go builder.
- **Transform with `DOMParser`, replace `pre` with `figure.lab-book-diagram`.** Each `pre > code.language-mermaid` that draws becomes a figure holding the SVG. The existing bootstrap selector then naturally skips it, so the embedded script only draws what is left. The serialized result keeps the doctype, the CSP meta and the inline Mermaid script. Diagrams are drawn one at a time, yielding between diagrams so progress paints and cancel is checked.
- **One shared render helper serializes Mermaid use.** `web/src/lib/mermaid.ts` loads Mermaid lazily and chains renders on one promise. Each render passes its config, and the helper calls `mermaid.initialize` only when that config differs from the last one applied. `Mermaid.tsx` passes its themed config, and the export passes `theme: 'default'`, `securityLevel: 'strict'` and HTML labels off. Alternative: re-initializing around the export, which races with in-app renders because `initialize` is global.
- **Sanitize with DOMPurify's SVG profile, plus a hardening pass.** Scripts, `foreignObject`, event handlers and any `href` or `xlink:href` that is not a fragment are removed. Turning HTML labels off keeps labels as SVG text, which also prints better. DOMPurify becomes a direct dependency at the version Mermaid already resolves.
- **Print through a sandboxed `srcdoc` iframe.** The frame is created with `sandbox="allow-same-origin allow-modals"`: same-origin lets the parent call `print()`, there is no `allow-scripts` so the book's script never runs, and `allow-modals` lets the print dialog open. The frame is placed off-screen with zero size (not `display: none`, which prints blank in some browsers) and removed after `afterprint` or a fallback timeout.
- **Print layout lives in the server template.** A print-only cover (`header.cover`: title, UTC export time, exporter, doc count), the contents `nav` printed on its own page instead of hidden, and `figure.lab-book-diagram { break-inside: avoid }`. The book receives the exporter label from its caller: the export handler uses the user's display name (falling back to the username), and the report path uses `Report: <definition name>`.
- **Failure handling.** A diagram that throws keeps its source. If the transform throws, the untouched server HTML is saved, and for PDF it is printed with source blocks in place of diagrams. Cancel aborts the in-flight request and skips the save.

## Risks / Trade-offs

- [Large books keep the main thread busy while diagrams draw] → Draw one diagram at a time, yield between them, show progress and allow cancel.
- [The served HTML is about 5 MB plus images, and it is held as text, a parsed document and a serialized copy] → Acceptable for documentation-sized books; each copy is dropped once the next exists.
- [Browser print differs between engines (margins, footer page numbers)] → Print CSS stays conservative; manual verification covers Chromium and Firefox.
- [Mermaid output with HTML labels off may lay out slightly differently from in-app diagrams] → The export is internally consistent (SVG and JS fallback use the same theme); visual parity with the app is not a goal.
- [jsdom cannot lay out Mermaid, so unit tests mock the render helper] → Real rendering, no-JS viewing and print fidelity are checked by hand and recorded in verification.md.

## Migration Plan

No data or configuration migration. Rollback is a revert; previously downloaded books stay valid because the bootstrap behaviour for Mermaid source is unchanged. Issue #619 is retitled and rewritten to this scope, with the reason for dropping server PDF.
