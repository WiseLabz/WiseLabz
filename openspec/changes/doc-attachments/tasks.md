# Tasks

## 1. Storage and security

- [x] 1.1 Add paired migration, metadata CRUD and rollback/parity coverage.
- [x] 1.2 Implement bounded streaming blobstore, sniff allowlist, hashing, atomic publication and orphan GC with tests.
- [x] 1.3 Add upload/list/delete and signed raw routes, HKDF/HMAC expiry and hardened headers; verify ACL and signature tests.
- [x] 1.4 Include signed attachments in doc and scoped share responses; integrate GC with purge and verify share scoping.

## 2. Portability

- [x] 2.1 Implement v2 ZIP export/import with checksums, bounded streamed staging, v1 compatibility, verifier and backup CLI; verify round-trips and tamper limits.
- [x] 2.2 Export attachment files and rewrite links, include flag and Git size cap; verify marker stripping and export rewrite.

## 3. Web and contract

- [x] 3.1 Update OpenAPI and regenerate client, passing router contract check.
- [x] 3.2 Render attachment URLs/missing placeholders, image lightbox, PDF Open/Preview and queries refreshed before expiry.
- [x] 3.3 Add CodeMirror drop/paste placeholder upload and AttachmentsPanel list/insert/delete/unused; verify Vitest with MSW.

## 4. Delivery

- [x] 4.1 Update architecture/backup/export docs and PR3 roadmap status, run full Go/frontend/Postgres checks and strict OpenSpec validation.
- [x] 4.2 Refresh and commit graphify output, push conventional commits, open PR targeting main with Closes #519 and template; fix all CI checks until green.
