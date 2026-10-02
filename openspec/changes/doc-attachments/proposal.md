# Proposal

## Why

Issue #519 makes images, PDFs and text part of maintained docs, building on human docs in PR #600. The docs roadmap fixes storage, security, backup and editor behavior.

## What Changes

- Content-addressed attachment storage and doc-owned metadata, ACL inheritance and fifteen-minute signed serving.
- Attachments in doc and scoped share responses, orphan collection alongside trash purge.
- ZIP v2 backups with checksums and v1 JSON import compatibility; portable Markdown export with rewritten links.
- Editor upload on drop/paste, attachment management, image lightbox and inline PDF preview.

## Capabilities

### New Capabilities

- `doc-attachments`: storage, serving, portability and authoring of doc attachments.

### Modified Capabilities

None.

## Impact

Paired migration 000052; blobstore, docs API, backup, retention, doc export, config, OpenAPI/generated client and React docs UI. This builds on merged PR #600 and targets main; Markdown import (#514) is out of scope.
