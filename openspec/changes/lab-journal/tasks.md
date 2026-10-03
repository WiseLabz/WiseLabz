# Tasks

## 1. Store and durability

- [x] 1.1 Add paired journal migration and CRUD; verify persistence and SET NULL survival tests.
- [x] 1.2 Add SQL timeline union, filters and composite cursor; verify precision order, paging, grants, keys and admin audit tests.
- [x] 1.3 Include notes in backup import/export and exclude from retention; verify backup round-trip and document architecture.

## 2. API

- [x] 2.1 Add timeline and journal handlers, authorization and audit; verify handler authorization matrix and backdated entity scenario.
- [x] 2.2 Register routes and OpenAPI schemas; regenerate API client and verify router/pagination contracts.

## 3. Web

- [x] 3.1 Add JournalPage, URL filters, infinite paging, source links and entry dialog with Markdown, datetime, scope, entity and doc; verify feature vitest.
- [x] 3.2 Add route, nav, palette, translations and user documentation; verify typecheck and lint.
- [x] 3.3 Return serviceName from Changes/Alerts lists and send Changes/Alerts severity filters to the server; verify handler and page tests.

## 4. Delivery

- [x] 4.1 Run strict OpenSpec validation, full Go tests/lint and web tests/lint/typecheck sequentially with low-memory settings; record evidence.
- [ ] 4.2 Refresh graphify, commit, file two follow-ups and open labeled/assigned PR closing #501; verify PR and CI started.
