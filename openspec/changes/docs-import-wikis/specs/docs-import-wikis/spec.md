# Spec Delta

## ADDED Requirements

### Requirement: Wiki.js export zip import
The import SHALL accept a Wiki.js 2.x storage-export zip when `source=wikijs`, mapping pages to docs under the existing limits, converting HTML pages to Markdown, skipping `.adoc` pages with a report, warning on unpublished pages, and rewriting absolute site links and assets to doc links and attachments.

#### Scenario: Page beside folder
- **WHEN** an export holds `a.md` and `a/b.md`
- **THEN** the preview shows `b` as a child of `a`

### Requirement: Remote pull jobs
An instance admin SHALL be able to start one background pull at a time from BookStack (URL and API token) or Wiki.js (URL and API key), poll its status and cancel it. Starting a pull SHALL require elevation for `docs.import.pull`. A finished pull SHALL produce the same preview and commit as a zip upload. Only archives produced by a server-started BookStack or Wiki.js pull SHALL bypass the per-entry compression-ratio limit. Pull archives SHALL continue to enforce entry count, safe paths, duplicate paths, expanded-size limits while validating and reading, note size, and attachment size. Missing or unrecognized server pull-origin markers SHALL use the strict upload archive policy. A request source field or an entry inside an uploaded ZIP SHALL NOT establish pull origin.

#### Scenario: Second pull
- **WHEN** a pull is running and another is started
- **THEN** the request is rejected as conflicting

#### Scenario: Not elevated
- **WHEN** an admin starts a pull without a valid elevation token
- **THEN** the request is refused

#### Scenario: Highly compressible pulled export
- **WHEN** a server-originated BookStack or Wiki.js pull contains an entry over the upload compression-ratio limit
- **THEN** preview and commit succeed when all other archive limits pass

#### Scenario: Uploaded archive claims pull origin
- **WHEN** an uploaded archive has `source=wikijs` or contains a `source` entry naming a pull provider and exceeds the compression-ratio limit
- **THEN** preview rejects it under the strict upload archive policy

#### Scenario: Missing or invalid staged pull marker
- **WHEN** staged analysis or commit sees no recognized server-written pull-origin marker
- **THEN** it applies the strict upload archive policy

### Requirement: Safe outbound access
Pulls SHALL refuse loopback, link-local and metadata addresses, follow no redirects, verify TLS unless `skipTlsVerify` is set (recorded in the audit entry), cap response sizes and honour `Retry-After`. Credentials SHALL be held in memory for the job only and SHALL NOT appear in storage, logs, audit rows or error text.

#### Scenario: Loopback target
- **WHEN** the URL resolves to a loopback address
- **THEN** the job fails without a connection

### Requirement: BookStack mapping
Shelves, books, chapters and pages SHALL become nested docs; a book in several shelves SHALL sit under the first and be listed in the preview; `[[bsexport:...]]` references SHALL become doc links or attachments.

#### Scenario: Old BookStack
- **WHEN** the book export endpoint answers 404
- **THEN** the job fails naming the minimum BookStack version
