# lab-book-export Specification

## Purpose
Lets users and scheduled reports take a permission-scoped copy of the documentation offline, as a self-contained HTML Lab Book or an importable Markdown archive.

## Requirements

### Requirement: Permission-scoped offline download
The system SHALL let any signed-in user download viewable active docs through GET /api/docs/export as html or md.zip, with a dated attachment filename, without exporting hidden connector docs or generated lab docs the caller cannot view.

#### Scenario: Viewer downloads docs
- **WHEN** a viewer downloads either supported format
- **THEN** only that viewer's permitted connector docs and human lab notes are included

#### Scenario: Invalid format
- **WHEN** a caller requests an unsupported format
- **THEN** the endpoint returns a validation error

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
The system SHALL offer email as a report channel and deliver an enabled report's Lab Book on every report channel: email as a multipart MIME attachment, Discord as a multipart upload, Slack through a file upload when the Slack channel has a bot token and channel ID, and generic webhooks inside the JSON body. Each channel SHALL enforce a fixed limit on the file's raw size (Discord 8 MiB, email 10 MiB, generic webhook 10 MiB, Slack 50 MiB) and fall back to the text report with an omission note when the file exceeds it. Only report deliveries SHALL carry files; alert, digest, finding, runbook and system notifications SHALL stay text-only on every channel.

#### Scenario: Oversized attachment
- **WHEN** an enabled report attachment exceeds the channel limit
- **THEN** the channel receives the text report and an attachment omission note

#### Scenario: Alerts stay text-only
- **WHEN** an alert or digest is delivered to a Slack, webhook or Discord channel that would accept a file
- **THEN** the delivery carries no file and its body is unchanged from text-only behavior

### Requirement: Stable report filenames
The system SHALL download reports using report-<slug>-<date>.<format>, with the slug from the immutable report snapshot and the UTC period-end date; legacy snapshots without a valid slug SHALL fall back to their report ID.

#### Scenario: Definition deleted
- **WHEN** the originating definition has been deleted
- **THEN** a report download retains the original snapshot slug in its filename

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

### Requirement: Slack file upload configuration
The Slack notification channel SHALL accept an optional bot token and channel ID alongside its required incoming-webhook URL. The bot token SHALL be write-only: stored encrypted, never returned by the API, reported only as a set flag, kept when omitted on save and cleared when saved empty. Saving a Slack channel with only one of bot token or channel ID SHALL fail validation. With both set, a report Lab Book SHALL be uploaded to that channel as one message whose comment is the report summary, and the bot token SHALL be sent only to the Slack API, never to the returned upload URL. Without both, the summary SHALL go through the incoming webhook with a note that file upload needs a bot token and channel ID. If the upload fails, the summary SHALL go through the incoming webhook with a note that the upload failed. Sending a Slack test SHALL, when a bot token is set, also verify the token and report an invalid one as a test failure.

#### Scenario: Upload with bot token
- **WHEN** a report with a Lab Book is delivered to a Slack channel with a bot token and channel ID
- **THEN** one Slack message in that channel carries the file with the summary as its comment, and no incoming-webhook message is sent

#### Scenario: No bot token
- **WHEN** a report with a Lab Book is delivered to a Slack channel without a bot token
- **THEN** the incoming webhook receives the summary with a note that file upload needs a bot token and channel ID

#### Scenario: Upload rejected
- **WHEN** Slack rejects the upload (for example a missing files:write scope or the bot is not in the channel)
- **THEN** the incoming webhook receives the summary with an upload-failed note and the delivery counts as sent

#### Scenario: Token never exposed
- **WHEN** an admin reads the notification configuration after saving a bot token
- **THEN** the response reports the token as set and contains neither the token nor its ciphertext

#### Scenario: Incomplete pair
- **WHEN** an admin saves a Slack channel with a channel ID but no bot token, or a bot token but no channel ID
- **THEN** the save is rejected with a validation error

### Requirement: Generic webhook file payload
A generic webhook report delivery that carries a Lab Book SHALL send the usual `title` and `message` JSON fields plus an `attachments` array with one entry holding `filename`, `contentType`, `size` (raw bytes), `sha256` (lowercase hex of the raw bytes) and `data` (standard base64). Deliveries without a file SHALL keep the exact existing body. The signature, when a signing secret is set, SHALL cover the whole body.

#### Scenario: Webhook receives the file
- **WHEN** a report with a Lab Book within 10 MiB is delivered to a generic webhook
- **THEN** the body's attachment decodes to bytes whose length and SHA-256 match `size` and `sha256`, and the signature verifies over the raw body

#### Scenario: Text-only body unchanged
- **WHEN** a notification without a file is delivered to a generic webhook
- **THEN** the body contains only `title` and `message`

### Requirement: Signed file uploads
When a channel has a signing secret, file-carrying Discord and generic webhook deliveries SHALL carry the same timestamp and HMAC-SHA256 signature headers as text deliveries, computed over the exact request body sent.

#### Scenario: Signed Discord upload
- **WHEN** a report Lab Book is uploaded to a Discord channel with a signing secret
- **THEN** the multipart request carries timestamp and signature headers that verify over its raw body

### Requirement: Report file retries
When a file-carrying report delivery fails and the file is within that channel's limit, the system SHALL keep the file and retry the delivery with the same file on the normal retry schedule. The kept file SHALL be released once no delivery for that report notification is still pending, and SHALL be removed with the notification. If the kept file is no longer available, the retry SHALL send the text report with an omission note.

#### Scenario: Retry resends the file
- **WHEN** a generic webhook report delivery with a Lab Book fails and the endpoint accepts the retry
- **THEN** the retried request carries the same file bytes as the first attempt

#### Scenario: File released
- **WHEN** every delivery of a report notification has been sent or has exhausted its retries
- **THEN** the kept file is no longer referenced and the next blob sweep removes it

#### Scenario: Kept file missing
- **WHEN** a retry finds the kept file unavailable
- **THEN** the channel receives the text report with an omission note
