# Tasks

## 1. Server print layout

- [x] 1.1 Add exporter label and export time to `labbook.Book` and render a print-only cover (title, UTC export time, exporter, doc count), the printed contents page and `figure.lab-book-diagram` page-break rules in `book.html.tmpl`; verify with labbook tests asserting cover text, doc count, print rules, and that the bootstrap still selects only `pre > code.language-mermaid`.
- [x] 1.2 Pass the user's display name (falling back to username) from the docs export handler and `Report: <definition name>` from the report Lab Book; verify with export handler and report Lab Book tests checking the cover label.

## 2. Shared Mermaid render helper

- [x] 2.1 Add `web/src/lib/mermaid.ts` that serializes renders and re-initializes Mermaid only when the requested config changes, and move `Mermaid.tsx` onto it; verify with vitest (mocked Mermaid) that interleaved renders each run under their own config and that the existing Mermaid component tests pass.

## 3. Pre-rendered HTML download

- [x] 3.1 Add DOMPurify as a direct web dependency and a Lab Book transform that replaces drawable Mermaid blocks with sanitized SVG figures (default theme, strict security, HTML labels off), reports progress, honours an abort signal and keeps failed diagrams as source; verify with vitest covering replacement, sanitizer stripping scripts, handlers, `foreignObject` and external hrefs, a failing diagram, abort, and the doctype, CSP and script being preserved.
- [x] 3.2 Update `ExportLabBook` so HTML downloads pre-render with a progress count and Cancel, falling back to the server HTML when the transform throws, with English and Brazilian Portuguese strings; verify with ExportLabBook tests for success, fallback, cancel and retry, plus the i18n parity test.

## 4. In-app print to PDF

- [x] 4.1 Add a PDF choice that builds the pre-rendered book and prints it from an off-screen `srcdoc` iframe sandboxed with `allow-same-origin allow-modals` only, removing it after printing; verify with ExportLabBook tests asserting the sandbox value, srcdoc content, the `print()` call, cleanup, and that no server PDF endpoint is called.

## 5. Docs, issue and verification

- [x] 5.1 Update DOC_EXPORT.md (pre-rendered diagrams, PDF via the print dialog, browser page-number footer, no server renderer needed) and REPORTS.md (report attachments keep Mermaid JS and gain the cover); verify the docs describe only shipped behaviour.
- [x] 5.2 Retitle and rewrite issue #619 for the client-side scope with the reason server PDF was dropped; verify with `gh issue view 619`.
- [x] 5.3 Run the backend and web checks, then by hand in Chromium and Firefox: open a downloaded book with JavaScript disabled and the network offline (diagrams visible, no requests), confirm generated topology diagrams render, print to PDF (cover, contents, unsplit diagrams), and confirm in-app diagrams keep the palette after an export; record results in verification.md.
