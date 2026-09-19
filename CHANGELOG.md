# Changelog

## 0.4.0 (2026-09-19)

## What's Changed
* Fix release workflow SBOM upload by setting explicit repository context by @gsaraiva2109 with @Copilot in https://github.com/WiseLabz/WiseLabz/pull/247
* feat: "ask your lab" retrieval-augmented chat (#238, piece 1/3) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/246
* feat: on-demand anomaly narration (#238, piece 2/3) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/249
* feat(ai): add provider fallback routing (#238, piece 3/3) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/250
* feat: cross-service linking and topology diagrams in generated docs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/251
* feat(connectors): implement service.restart per ADR 0001 (PR1 of #236) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/252
* feat(connectors): start/stop + config-push (PR2 of #236) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/253
* docs: explain CodeQL cookie-secure false positive in session.go by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/254
* feat(connectors): maintenance windows (PR3 of #236) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/255
* feat(connectors): add bulk sync/reauth/restart (PR4 of #236) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/256
* feat: per-connector permissions (PR1 of #240) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/257
* feat(docs): read-only doc share links (#240 PR2) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/258
* feat(connectors): secret rotation reminders and finding notifications (#239 PR1) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/259
* feat(connectors): structured entity attributes and attribute schema (#239 PR2) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/260
* feat(compliance): user-defined compliance rules on snapshot data (#239 PR3) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/261
* Notification digests + custom alert rules (#237) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/262
* connectors: add Netbird and Cloudflare (#241, #245) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/263
* fix(security): tighten authorization and outbound-request hardening by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/311
* chore: update Go to 1.27.1 by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/312
* fix: harden webhook delivery and session/credential lifecycle by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/313
* fix(security): harden outbound requests, connector inputs and auth edge cases by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/314
* fix: security hardening follow-ups (CSP, OIDC redirect host, sync logs) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/315
* fix(docker): dial ssh connection per request and close it with the transport by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/316
* fix(store): run sqlite migrations with foreign_keys off to avoid cascade data loss by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/317
* fix(sync): prevent overlapping connector syncs and bound sync duration by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/318
* perf(store): batch retention deletes and purge unbounded tables by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/319
* perf(sync): persist snapshot, changes and alerts atomically in batches by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/321
* perf(store): enable SQLite WAL, busy_timeout and synchronous NORMAL by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/320
* fix(connectors): apply per-user permission filter before pagination by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/323
* ci: check sqlite/postgres migration parity and run store tests on Postgres by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/322

## New Contributors
* @gsaraiva2109 with @Copilot made their first contribution in https://github.com/WiseLabz/WiseLabz/pull/247

**Full Changelog**: https://github.com/WiseLabz/WiseLabz/compare/v0.3.0...v0.4.0

## 0.3.0 (2026-09-14)

## What's Changed
* chore(release): publish release assets by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/188
* fix(ci): use lowercase GHCR image reference by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/189
* fix(release): generate notes from merged pull requests by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/191
* feat(settings): backup ops, API keys, and delivery history by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/192
* feat: runbook management UI and diagnostics bundle download by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/193
* Connector health check UI + expanded backup regression coverage by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/194
* fix: wire AIRegistry into router config and guard nil connector config map by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/213
* fix: apply configured HTTP server read/write timeouts by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/214
* fix: reject disabled users' API keys and revoke on disable by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/215
* fix: bind OIDC state/nonce to browser, harden backup perms, unify cookie Secure derivation by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/216
* fix: safe shutdown, safe config seeding, and bulk backup queries by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/217
* test: cover auth handlers, 13 zero-coverage api packages, and crypto.DecodeKey by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/218
* fix: sync.complete on all exit paths, deterministic diff order, resilient stale check by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/219
* fix: dedupe doc scans, bound sync concurrency, and generic pagination/decode helpers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/220
* test: cover retention error paths and diagnostics bundle by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/221
* test: cover notification delivery retry and dispatch paths by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/229
* test: cover auth/session/API-key/OIDC error paths by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/230
* test: cover backup redaction and restore failure paths by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/231
* test: cover connector error handling and timeout paths by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/232
* chore: switch frontend package management from npm to Bun by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/233
* test: add coverage for config, docs, and application-layer handlers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/234


**Full Changelog**: https://github.com/WiseLabz/WiseLabz/compare/v0.2.0...v0.3.0

## [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12)


### Features

* **services:** preview restart impact ([7a668ad](https://github.com/WiseLabz/WiseLabz/commit/7a668ad762d7466fcc5d4b5fc378e06f1181b06e))
* **services:** preview restart impact ([a89c1e4](https://github.com/WiseLabz/WiseLabz/commit/a89c1e42da2e189d4748f1c0a15c9abf8bccdb63))


### Bug Fixes

* **ci:** correct mistyped SHA pins for docker and codeql actions ([ae26a9c](https://github.com/WiseLabz/WiseLabz/commit/ae26a9c750322ae291874493425caf9dde5594f6))
* **ci:** correct mistyped SHA pins for docker and codeql actions ([8cb6acc](https://github.com/WiseLabz/WiseLabz/commit/8cb6acc1016f1c427562bd942e18a651a815d691))
* **release:** configure root package ([9ed1178](https://github.com/WiseLabz/WiseLabz/commit/9ed1178b8f31af3f11005f37c5e8b97b40725180))
* **release:** configure root package ([afd9da2](https://github.com/WiseLabz/WiseLabz/commit/afd9da2128715868b5a0d18439a47bcb532bb410))
* **release:** establish release please baseline ([28b18c4](https://github.com/WiseLabz/WiseLabz/commit/28b18c4256b43363511112dd1b00ea774f3d7825))
* **release:** establish release please baseline ([54105d6](https://github.com/WiseLabz/WiseLabz/commit/54105d6258c9e7190248a55d3c5b389b2be462f2))

## Changelog

All notable changes to this project are documented in this file, generated by
[release-please](https://github.com/googleapis/release-please) from Conventional Commits.
