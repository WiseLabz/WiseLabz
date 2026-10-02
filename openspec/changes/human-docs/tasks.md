# Tasks

## 1. Store and lifecycle

- [x] 1.1 Add paired migration 000051, metadata scanners, hierarchy validation and atomic lifecycle writes; verify SQLite/Postgres migration rollback and scope/cycle/depth/subtree tests.
- [x] 1.2 Filter active docs across lists, versions, search, embeddings, MCP, chat, shares, export and quality; verify visibility regression tests.
- [x] 1.3 Preserve hierarchy/creator/deletion metadata in v1 backups with order-independent import; verify backup round-trip tests.
- [x] 1.4 Integrate configurable thirty-day deleted-doc purge into scheduled retention; verify purge and cascade tests and document configuration.

## 2. API

- [x] 2.1 Add creation, metadata PATCH, subtree DELETE, administrator trash and restore endpoints and origin-aware ACL/tree; verify authz matrix and nested tree tests.
- [x] 2.2 Update OpenAPI and regenerate client after frozen dependency installation; verify TestOpenAPIMatchesRouter and client build.
- [x] 2.3 Document human doc lifecycle and ACL in ARCHITECTURE.md and update PR2 roadmap status; verify docs match final behavior.

## 3. Web

- [x] 3.1 Add creation dialog and palette action with permitted scopes and parent selection; verify frontend tests and lint.
- [x] 3.2 Add nested tree and drag re-parenting, editor delete/metadata actions, administrator trash page and restore; verify frontend tests/build and create/nest/delete/restore workflow.

## 4. Integration

- [x] 4.1 Run full backend/frontend checks and strict OpenSpec validation, refresh and commit graphify output; verify all tasks and requirements with fresh evidence.
- [ ] 4.2 Push conventional commits, open PR with template and Closes #494, and fix CI until green; verify PR URL and gh pr checks.
