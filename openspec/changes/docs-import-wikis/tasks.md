# Tasks

## 1. Wiki.js export zip

- [x] 1.1 `docimport/htmlmd` wrapper with tests.
- [x] 1.2 Wiki.js export parser and mapping; `source` field on `POST /api/docs/import`; fixtures and tests.
- [x] 1.3 Source picker, entry point check, pt-BR `docs.import.*` strings with parity test; OpenAPI and client.

## 2. BookStack API pull

- [x] 2.1 Job manager, `Source` interface, pull routes, `docs.import.pull` elevation, audit.
- [x] 2.2 Guarded outbound client (no redirects, skip-TLS opt-in, caps, 429 handling).
- [x] 2.3 BookStack source and mapping with an `httptest` fake; credential-leak tests.
- [x] 2.4 Web: BookStack option, form, step-up, progress view with cancel.

## 3. Wiki.js API pull

- [ ] 3.1 Wiki.js GraphQL source writing the export layout; tests with a fake.
- [ ] 3.2 Web: Wiki.js option reusing the form and progress view.

## 4. Delivery

- [ ] 4.1 Full checks green; strict OpenSpec validation; docs updated.
- [ ] 4.2 Manual: a real BookStack pull, a real Wiki.js export zip and a real Wiki.js API pull.
