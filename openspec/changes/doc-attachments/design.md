# Design

## Context

PR2 supplies active-doc filtering, origin-aware lab permissions, connector grants and scheduled purge. Bytes must be portable without requiring bearer headers in images or PDF iframes.

## Goals / Non-Goals

Implement roadmap PR3 completely using existing store/handler/editor seams and standard library streaming I/O. No Markdown/Obsidian import or PR4 work.

## Decisions

- Migration 000052 adds doc_attachments with cascading doc FK and doc/hash indexes. Store methods preserve creator and timestamps.
- Blobstore streams through a bounded temp file, sniffs the first 512 bytes independent of filenames, rejects SVG and unsupported content, hashes then atomically renames into two hash-prefix directories. Defaults: /data/attachments and 25 MiB.
- Metadata belongs to one doc. Viewer checks precede reads and operator checks precede writes. Raw serving uses an HKDF-separated HMAC key derived from auth.secret, attachment ID and expiry, with a fifteen-minute maximum lifetime. Active-doc lookup prevents deleted docs being served; security headers allow same-origin PDF embedding only.
- Doc and share serialization signs only the owning doc's attachments after access/scope validation. No traversal of arbitrary attachment links in Markdown.
- GC checks all referencing metadata, including trash, before deleting orphan blobs; serialize publication and collection within the process.
- ZIP contains bundle.json and one attachments/<sha> entry per distinct blob, with manifest checksums. Import streams to a bounded temporary archive, verifies paths, content hashes and metadata before database writes, and supports v1 JSON. Default import cap 1 GiB.
- Export strips generated markers, writes safe MIME-derived extensions and rewrites only doc-owned attachment links. Attachment inclusion defaults on; Git exports skip attachments beyond the configured cap.
- React maps attachment links using signed metadata, displays missing placeholders, uses existing Dialog for images and PDF previews, and a CodeMirror extension for drop/paste placeholder replacement. Refetch before the fifteen-minute signature expiry.

## Risks / Trade-offs

Content-addressed deduplication requires reference-aware collection; aborted writes leave collectible blobs. ZIP inputs need bounded decompression and checksums before importing rows. Signed URLs grant temporary bearer access until expiry; deleting metadata or soft-deleting the doc invalidates serving immediately.

## Migration Plan

Apply sqlite and postgres migration 000052, update both rollback passes and Postgres rollback tests, regenerate OpenAPI client after frozen install, then verify ACL/security/portability and UI workflows.
