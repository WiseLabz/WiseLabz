## ADDED Requirements

### Requirement: Permission-scoped offline download
The system SHALL let any signed-in user download viewable active docs through GET /api/docs/export as html or md.zip, with a dated attachment filename, without exporting hidden connector docs or generated lab docs the caller cannot view.

#### Scenario: Viewer downloads docs
- **WHEN** a viewer downloads either supported format
- **THEN** only that viewer's permitted connector docs and human lab notes are included

#### Scenario: Invalid format
- **WHEN** a caller requests an unsupported format
- **THEN** the endpoint returns a validation error

### Requirement: Self-contained HTML
The system SHALL render GFM, a title-sorted hierarchical TOC grouped by Lab and connector, ID-safe anchors, embedded images, Mermaid without network access and print CSS; PDF and text attachments SHALL be listed by filename only.

#### Scenario: Offline reading and printing
- **WHEN** the downloaded HTML is opened without network connectivity
- **THEN** diagrams render locally and printing yields readable documentation

### Requirement: Importable Markdown archive
The system SHALL export hierarchy folders, readable titles, scoped front matter, doc-relative links and attachments in a zip readable by docimport.OpenArchive and Analyze.

#### Scenario: Archive round trip
- **WHEN** a downloaded archive is analyzed by the Markdown importer
- **THEN** hierarchy, content, links and attachments resolve

### Requirement: Optional report Lab Book
The system SHALL persist an attach_lab_book flag and expose attachLabBook in the report API and form; enabled reports SHALL attach HTML containing selected connector docs plus lab-wide docs, or all docs for an unfiltered report.

#### Scenario: Filtered report
- **WHEN** an attachment-enabled report selects one connector
- **THEN** its Lab Book includes that connector and lab-wide docs and excludes other connector docs

### Requirement: Report file delivery
The system SHALL offer email as a report channel, send email attachments as multipart MIME and Discord files as multipart uploads, keep Slack and webhooks text-only, and fall back to text with a note when a file exceeds the channel limit.

#### Scenario: Oversized attachment
- **WHEN** an enabled report attachment exceeds the channel limit
- **THEN** the channel receives the text report and an attachment omission note

### Requirement: Stable report filenames
The system SHALL download reports using report-<slug>-<date>.<format>, with the slug from the immutable report snapshot and the UTC period-end date; legacy snapshots without a valid slug SHALL fall back to their report ID.

#### Scenario: Definition deleted
- **WHEN** the originating definition has been deleted
- **THEN** a report download retains the original snapshot slug in its filename
