# Spec Delta

## ADDED Requirements

### Requirement: Doc-owned content-addressed storage
Attachments SHALL belong to one doc with cascading metadata deletion and inherit origin-aware lab or connector viewer/operator ACLs. Upload SHALL stream to disk with SHA256, enforce configurable 25 MiB default, sniff png/jpeg/gif/webp/pdf/text independent of extension, reject SVG, and publish atomically to hash-prefix directories.

#### Scenario: Spoofed extension
- **WHEN** a user uploads SVG named photo.png
- **THEN** the upload is rejected without metadata or published bytes

### Requirement: Signed raw serving
Raw attachment URLs SHALL use HKDF label wiselabz-attachment-url from auth.secret, HMAC over attachment ID and expiry, and at most fifteen-minute TTL. Serving SHALL require active owning docs and valid signatures, set nosniff, private caching and sandbox with frame-ancestors self, and use inline disposition only for images and PDFs.

#### Scenario: Expired or tampered URL
- **WHEN** the expiry or signature is invalid
- **THEN** raw bytes are denied

### Requirement: Doc and share attachment scope
Doc GET and share doc responses SHALL include attachment metadata and signed URLs only for attachments belonging to the authorized doc within the shared tree. Scheduled trash purge SHALL collect blobs with no referencing rows while retaining blobs referenced by other docs or trash.

#### Scenario: Shared doc references another attachment
- **WHEN** Markdown refers to an attachment of an out-of-scope doc
- **THEN** no URL for that attachment is issued

### Requirement: Portable backup archives
Backup export SHALL write v2 ZIP with bundle.json, deduplicated attachments/<sha> and manifest checksums. Import SHALL accept v1 JSON and v2 ZIP, stage streams to temporary disk within configurable 1 GiB default cap, validate hashes and references before import, and preserve attachment bytes and metadata. Verification and CLI restore SHALL support both formats.

#### Scenario: Backup round-trip
- **WHEN** a v2 backup with attachments is restored into a fresh database and blob directory
- **THEN** metadata and bytes round-trip and corrupted entries are rejected

### Requirement: Portable doc export
Doc export SHALL write attachments/<sha>.<ext> and rewrite doc-owned attachment links to relative paths while stripping generated markers. doc_export.include_attachments SHALL default true and Git mode SHALL skip files beyond doc_export.max_attachment_bytes.

#### Scenario: Export a photo
- **WHEN** a doc with an attached image is exported with inclusion enabled
- **THEN** its link resolves to the exported image file

### Requirement: Attachment authoring and preview
Web Markdown SHALL map attachment IDs to signed URLs, show placeholders for unknown IDs, provide image lightbox and PDF Open/inline iframe Preview. Queries SHALL refresh before signature expiry. Editor drop/paste SHALL upload files with replaceable placeholders and the panel SHALL list, insert, delete and mark unused attachments.

#### Scenario: Paste upload
- **WHEN** an operator pastes a file in the editor
- **THEN** a placeholder appears and successful upload replaces it with an attachment link

