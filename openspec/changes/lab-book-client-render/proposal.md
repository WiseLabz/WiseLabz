# Proposal

## Why

Issue #619 asked for server PDF export and pre-rendered Mermaid SVG for the offline Lab Book, but either needs a headless browser beside the distroless image, adding hundreds of megabytes and real RAM to every install that turns it on. The signed-in browser already ships Mermaid 12, so it can draw the diagrams and print the book itself, giving no-JavaScript diagrams and a PDF path without making the image or hardware requirements heavier.

## What Changes

- Downloading the HTML Lab Book from the docs page now pre-renders every Mermaid diagram in the browser into sanitized inline SVG, with progress and cancel; diagrams that fail keep their source and the embedded Mermaid JS draws them on open. If pre-rendering fails as a whole, the plain server HTML is saved.
- Exported diagrams always use Mermaid's default light theme, strict security and no HTML labels, independent of the user's app palette.
- A new "PDF" export choice builds the same pre-rendered book, loads it in a hidden sandboxed frame with scripts disabled and opens the browser print dialog.
- The server HTML gains a print-only cover page (title, export time, exporter, scope line), a printed linked table of contents, and keeps diagrams on one page. Page numbers come from the browser's print footer.
- No server renderer, sidecar, job queue, configuration or new endpoint. Server PDF output from #619 is dropped by decision, recorded in design.md.
- `md.zip` and scheduled report attachments (HTML with embedded Mermaid JS) are unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `lab-book-export`: the self-contained HTML requirement gains the print cover and printed contents; new requirements cover browser pre-rendered diagrams and in-app print to PDF.

## Impact

Lab Book HTML template and the docs export handler (exporter name on the cover), report Lab Book cover label, the docs page export control, a shared Mermaid render helper used by the in-app diagram component, English and Brazilian Portuguese strings, DOMPurify as a direct web dependency (already present transitively through Mermaid), DOC_EXPORT.md and REPORTS.md. Issue #619 is retitled and rewritten to the client-side scope.
