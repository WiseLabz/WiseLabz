## MODIFIED Requirements

### Requirement: Self-contained HTML
The system SHALL render GFM, a title-sorted hierarchical TOC grouped by Lab and connector, ID-safe anchors, embedded images, Mermaid without network access and print CSS; PDF and text attachments SHALL be listed by filename only. When printed, the HTML SHALL start with a cover page showing the title, the UTC export time, the exporter (the signed-in user's display name, or the report name for report attachments) and a scope line with the number of docs, followed by the linked table of contents on its own page, and SHALL keep each diagram on a single page. The embedded Mermaid script SHALL only draw diagrams that are still Mermaid source and SHALL leave diagrams that are already SVG untouched.

#### Scenario: Offline reading and printing
- **WHEN** the downloaded HTML is opened without network connectivity
- **THEN** diagrams render locally and printing yields readable documentation

#### Scenario: Printed cover and contents
- **WHEN** a user prints a Lab Book they downloaded
- **THEN** the first page names the book, the export time, that user and the doc count, and the next page lists the contents with links to each doc

#### Scenario: Report attachment cover
- **WHEN** a report attaches a Lab Book and its recipient prints it
- **THEN** the cover names the report as the exporter

## ADDED Requirements

### Requirement: Browser pre-rendered diagrams
When a user downloads the HTML Lab Book from the docs page, the web app SHALL replace every Mermaid diagram it can draw with inline SVG before saving, drawn with Mermaid's default light theme, strict security and no HTML labels regardless of the user's app theme, and sanitized so the SVG contains no scripts, event handlers, foreign objects or external references. The web app SHALL show progress and let the user cancel. A diagram that cannot be drawn SHALL stay as Mermaid source for the embedded script to draw when the file is opened, and if pre-rendering fails as a whole the web app SHALL save the server HTML unchanged. The download SHALL require no server renderer.

#### Scenario: Diagrams visible without JavaScript
- **WHEN** a downloaded HTML Lab Book is opened with JavaScript disabled and no network
- **THEN** every diagram that pre-rendered is visible and the page makes no network requests

#### Scenario: One diagram fails
- **WHEN** one diagram source is invalid and the others are valid
- **THEN** the download succeeds with SVG for the valid diagrams and Mermaid source for the invalid one

#### Scenario: Pre-rendering unavailable
- **WHEN** pre-rendering throws before producing a book
- **THEN** the user still receives the server HTML Lab Book

#### Scenario: User cancels
- **WHEN** the user cancels while diagrams are being drawn
- **THEN** no file is saved and the export control is ready for another export

#### Scenario: App theme does not leak
- **WHEN** a user with a dark app palette downloads the Lab Book
- **THEN** exported diagrams use the default light theme, and in-app diagrams keep the user's palette afterwards

### Requirement: In-app print to PDF
The docs page export control SHALL offer a PDF choice that builds the pre-rendered HTML Lab Book, loads it in a hidden frame that cannot run scripts, and opens the browser print dialog so the user can save it as PDF. Diagrams that could not be pre-rendered SHALL print as their Mermaid source. The PDF choice SHALL NOT call any server PDF or rendering endpoint.

#### Scenario: Print a pre-rendered book
- **WHEN** a user chooses PDF and confirms the print dialog
- **THEN** the printed pages contain the cover, contents and docs with diagrams drawn as SVG

#### Scenario: No scripts in the print frame
- **WHEN** the book is loaded for printing
- **THEN** the frame is sandboxed without script execution and the embedded Mermaid script does not run
