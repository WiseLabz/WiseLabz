# Changelog

## Unreleased

### ⚠ BREAKING CHANGES

* **api:** `PATCH /connectors/{id}` has been removed. It was an undocumented
  alias of `PUT /connectors/{id}`, kept for clients predating the OpenAPI
  contract; use `PUT` instead. The generated frontend client has always been
  PUT-only, so no shipped client is affected.
* **api:** the `details` field of the `Error` envelope is now an array of
  `{ field, msg }` objects instead of a free-form object. Nothing had ever
  populated it, so no client can have depended on the previous shape.

### Features

* **discovery:** instance admins can scan a private /24 for Proxmox VE, Proxmox Backup Server, Home Assistant, Portainer, UniFi, AdGuard Home, Traefik, Docker, Caddy, Nginx Proxy Manager, pfSense, OPNsense, TrueNAS and Pi-hole during onboarding and from the add-connector page, and connect what is found. See [docs/NETWORK_DISCOVERY.md](docs/NETWORK_DISCOVERY.md).

## 2.0.0 (2026-10-10)

## What's Changed
* Add the make targets CONTRIBUTING.md already documents by @wufangyong973 in https://github.com/WiseLabz/WiseLabz/pull/551
* chore(agents): track shared tooling and prune unused skills by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/554
* fix(backup): disable age pruning when maxAgeHours <= 0 by @ChangedRuby in https://github.com/WiseLabz/WiseLabz/pull/555
* fix(docs): require viewer access for doc history and lock reads by @ChangedRuby in https://github.com/WiseLabz/WiseLabz/pull/556
* fix(dashboard): scope the overview to connectors the caller can view by @ChangedRuby in https://github.com/WiseLabz/WiseLabz/pull/558
* fix(auth): require step-up to start MFA enrollment and block API keys by @ChangedRuby in https://github.com/WiseLabz/WiseLabz/pull/559
* chore(dx): skip unaffected checks in lefthook using the CI change classifier by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/562
* chore(ci): harden agent-only change skips and exclude tooling from build contexts by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/563
* fix(web): surface mutation errors, single-flight token refresh, and handle WS events by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/564
* fix(connector): Pi-hole token leak and sessions, Proxmox mem decode and drift by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/565
* fix(backend): store, docs and scheduling correctness fixes by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/566
* fix: real test notifications, lab docs in tree, a11y for dialogs/palette/notifications by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/567
* fix(auth): enforce MFA lockout and auth settings, stop leaking connector config by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/568
* fix(security): enforce connector grants in notifications, runbooks, docs and attention cache by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/569
* fix(security): sanitize errors in test-notification logs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/570
* fix(web): repair HTTP copying, chat retries, and OIDC step-up by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/571
* fix(api): close connector authorization gaps by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/572
* fix(backend): correct notification delivery, reports and audit exports by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/573
* fix(backend): preserve docs and correct snapshot and report persistence by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/574
* fix(docs): restore sync and document editing workflows by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/575
* fix(sync): preserve baselines and drain bounded sync jobs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/576
* test: add command-line entrypoint tests for issues #489 and #493 by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/579
* test: comprehensive coverage for reports, compliance, and auth handlers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/580
* test(web): add unit tests for lib helpers and auth store by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/577
* test: add comprehensive backup scheduling, authz and AI encryption tests by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/581
* test: add concurrent cache tests and postgres testing for backup/retention by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/578
* docs: fix README config section and remove dead store/sqlc by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/583
* perf: add HTTP compression, cache headers, and optimize doc editor rendering by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/582
* fix(security): add placeholder secret validation and PKCE to OIDC by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/586
* chore: small backend cleanups and request-scoped logging foundation by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/585
* chore(web): replace leftover API clients and i18n reports by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/584
* refactor(backend): move AI config SQL into store, decouple mcp/chat, consolidate ai HTTP helpers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/587
* perf(backend): sync/linking snapshot reuse and SQLite read pool by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/588
* feat(auth): step-up on admin/auth-policy actions, single-use bound elevation tokens; web deps cleanup by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/591
* refactor(backend): share connector helpers and split oversized functions by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/589
* perf: coalesce WS event bursts and trim hot-path queries by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/590
* feat(compliance): Tailscale connector and recommended rule pack by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/592
* feat: connector lifecycle actions and UI language picker by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/593
* feat: Prometheus /metrics endpoint and more notification channels by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/594
* fix(security): purpose-bound keys and AAD for stored secrets (#531) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/595
* feat(health): scheduled connector health checks and uptime reporting by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/596
* feat(mcp): FTS search, topology path, runbook tools and doc edit proposals by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/597
* feat(docs): section ownership — sync merges generated blocks instead of overwriting human edits (#478) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/598
* chore(openspec): archive doc-section-ownership and sync its spec by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/599
* feat(docs): human-written docs — create, delete, nest and trash (#494) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/600
* chore(openspec): archive human-docs and sync its spec by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/604
* feat(docs): doc attachments — upload, signed serving, backups and export (#519) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/605
* feat(docs): import Markdown/Obsidian vaults (#514) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/610
* fix(docs): attachment review follow-ups (#519) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/609
* feat(connectors): declare connectors in config.yaml (#500) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/616
* feat(journal): add unified lab timeline and manual entries (#501) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/623
* feat(docs): add offline Lab Book exports and report attachments (#499) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/621
* chore(graphify): refresh graph report and register graph.json merge driver by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/624
* feat(search): add lab-wide search across docs and entities (#495) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/625
* feat(connector): add Nginx Proxy Manager support (#497) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/626
* feat(compliance): add cross-connector related-entity rule clauses (#627) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/628
* feat(connector): add Proxmox Backup Server support and backup rules (#496) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/629
* feat(connector): add Caddy connector by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/630
* feat(identities): persist entity identities from sync snapshots by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/631
* feat(topology): add typed edges and traversal APIs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/633
* feat(entities): add entity detail pages by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/635
* feat(topology): add live topology graph UI by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/636
* fix(entities): show far-end neighbours, dedupe edges, name identities from visible members by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/637
* fix(connector): allow Caddy pasted-JSON creation and name Pi-hole DNS records by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/639
* fix(topology): follow-ups from the topology end-to-end run by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/638
* fix(topology): address remaining entity and graph follow-ups by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/643
* fix(connector): give firewall entities stable external IDs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/644
* fix(ci): fail CI Status when detection or selected jobs do not succeed by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/645
* feat(entities): store, reconcile and back up identity overrides by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/646
* feat(entities): add entity override admin API, audit and OpenAPI by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/647
* feat(entities): add admin entity identity overrides web workflow and list by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/648
* docs(openspec): propose runbook-runs and custom-rest-recipes changes by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/654
* refactor(connectors): extract reusable lifecycle cores by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/655
* feat(store): add runbook runs, step kinds and retention store by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/656
* feat(runbooks): validate step kinds and reject non-lifecycle single-step execution by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/657
* feat(retention): expire open runs, prune history and add execution ADR by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/659
* feat(runbooks): add step editor for new step kinds and retention settings (#510) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/660
* feat(runbooks): add the server-side run executor by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/658
* feat(runbooks): expose run API and read-only MCP history by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/661
* feat(web): add runbook run controls and history by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/662
* test(runbooks): complete run execution integration checks by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/663
* feat(connectors): add connector categories (#513) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/664
* feat(connectors): add custom REST recipes and run safeguards by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/667
* feat(connectors): add bounded REST recipe pagination by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/668
* feat(connectors): add recipe previews and web editor by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/669
* test(connectors): verify custom REST recipe integration by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/670
* docs(openspec): archive runbook-runs and custom-rest-recipes by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/671
* docs(openspec): plan runbook-step-kinds and cert-expiry-watch by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/672
* feat(store): persist config-push and entity-wait runbook steps by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/673
* feat(connector): share verified config pushes with runbook runs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/674
* feat(compliance): add days-left operators and normalize NPM expiry by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/676
* feat(connector): supply stored snapshot inputs during sync by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/675
* feat(runbookrun): execute config_push and wait_for_entity steps by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/678
* feat(connector): add TLS probe connector with Traefik host import by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/679
* feat(runbooks): author config-push and entity-wait runbook steps by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/680
* feat(compliance): add certificate expiry pack and pack installed state by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/682
* feat(runbooks): add preview, history, MCP, and docs for config-push and entity-wait by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/683
* feat: add certificate expiry views and runbook configuration steps by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/684
* docs(openspec): complete integration task 7.1 for runbook-step-kinds and cert-expiry-watch by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/685
* docs(openspec): archive runbook-step-kinds and cert-expiry-watch by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/686
* fix(docs): stage attachment uploads before acquiring publication lock by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/687
* fix(docs): sweep stale attachment upload temp files by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/688
* fix(docimport): strip trailing image embed size from aliases by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/689
* feat(discovery): network discovery during onboarding by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/690
* feat(connectors): allow declaring a TLS probe connector in config.yaml by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/692
* fix(connectors): correct elevation response contract, opnsense savepoint flow, and proxmox memory reader by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/693
* feat(connectors): recipe-defined actions for custom REST connectors by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/698
* feat(web): recipe form builder for custom connectors by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/694
* fix(deps): bump Go to 1.27.2 and golang.org/x/net to v0.60.0 for govulncheck findings by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/701
* fix(web): escape hyphens in recipe field ids to avoid collisions by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/703
* fix(runbooks): refuse lifecycle and config-push steps on orphaned connectors by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/704
* feat(docs): translate attachment UI strings to pt-BR (#608) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/705
* feat(journal): show grant-scoped lab actions to non-admin members by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/707
* feat(journal): narrate a selected timeline window with AI by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/706
* feat(runbooks): optional second approver for runbook runs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/708
* feat(docs): import a Wiki.js storage export by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/709
* test(opnsense): bound waits and add a go test timeout in CI by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/711
* fix(auth): always require elevation to approve runbook runs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/712
* feat(docs): wikilinks and backlinks between docs and entities by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/710
* feat(docs): import from BookStack via API pull by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/713
* feat(docs): import from Wiki.js via API pull by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/714
* fix: handle unverified config writes and preserve API pull content by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/717
* fix(opnsense): make every firewall rule addressable by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/718

## New Contributors
* @wufangyong973 made their first contribution in https://github.com/WiseLabz/WiseLabz/pull/551

**Full Changelog**: https://github.com/WiseLabz/WiseLabz/compare/v1.0.0...v2.0.0

## 1.0.0 (2026-10-01)

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
* fix(docker): honor context cancellation when dialing ssh connectors by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/325
* perf(auth): cache user status and throttle API-key activity writes by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/326
* perf(dashboard): single-query attention queue and brief overview cache by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/327
* fix(chat): embed before deleting old doc embeddings by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/328
* perf(ws): make hub broadcast non-blocking and evict slow clients by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/324
* perf(docs): stop loading full doc content and connector config in list endpoints by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/329
* perf(store): make Postgres pool configurable and cache placeholder rewrites by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/330
* perf(quality): reuse compliance snapshots across rules by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/331
* fix(store): make doc search portable across SQLite and Postgres by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/332
* fix(api): handle ignored errors and bound detached contexts in handlers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/336
* chore(deps): bump outdated Go dependencies by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/334
* ci: add govulncheck and staticcheck jobs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/335
* chore(api): mount /api/v1 alias and add OpenAPI-vs-router contract test by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/339
* perf(store): add missing indexes for hot queries by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/333
* test(api): raise coverage for middleware, findings, chat and alerts by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/337
* feat(config): add config validate/print subcommands and HandleStoreError helper by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/338
* perf(chat): cache decoded embedding vectors across questions by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/340
* refactor(api): split connectors handlers by concern and extract helpers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/341
* refactor(api,store): split docs, settings and doc store by concern by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/342
* feat(config): publish config schema with WISELABZ env names by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/344
* refactor: split store connector, sync engine, docker and proxmox by concern by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/343
* test(api): raise coverage for templates and connectors handlers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/345
* perf(store): index share-link expiry and revocation for retention cleanup by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/346
* test(api): raise connectors handler coverage and add server embed test by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/347
* feat(api): field-level error details, spec response assertions, drop PATCH alias by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/349
* perf(store): index sessions(last_seen_at) for the stale-session retention sweep by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/348
* refactor(auth): extract helpers from OIDCCallback and UpdateUser by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/352
* refactor(connectors): extract Update and ConfigPush helpers (#309) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/353
* refactor(api): split NewRouter into per-domain mount helpers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/354
* refactor(sync): split runSyncFields into helpers by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/355
* refactor: extract helpers from notifyAlert, importBundle, UpdateAIConfig by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/356
* chore(lint): add funlen ratchet at current maximum by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/357
* feat(api): add liveness and readiness probes by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/358
* feat(connector): add Traefik connector by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/360
* feat(api): roll out {field,msg} validation details envelope by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/359
* feat(connector): extend Pi-hole with lists, groups, clients and v5 support by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/362
* feat(connector): add AdGuard Home connector by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/363
* feat(connector): add Portainer connector by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/364
* feat(connector): add UniFi connector by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/365
* feat(connector): add Home Assistant connector by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/366
* refactor(api): unify pagination and add opt-in keyset cursors by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/367
* test(notifications): wait on dispatch quiescence instead of fixed sleeps by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/368
* feat(connector): add TrueNAS connector by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/369
* fix(api): declare Connector timestamps as optional-or-empty strings by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/370
* test(compliance): pin exact details for every rule validation branch by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/371
* feat(backup): add manifest/checksum, scheduled restore verification, and backup CLI by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/373
* refactor(server): errgroup-based lifecycle manager with ordered shutdown by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/374
* feat(notifications): add ntfy, Telegram, SMTP channels and Discord embeds by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/375
* feat(docs): scheduled Markdown export to a local directory by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/378
* feat(connectors): track uptime/SLO history and expose availability/MTTR by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/376
* feat(quality): configuration drift detection against a pinned golden snapshot by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/379
* feat(httpx): shared hardened outbound HTTP client by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/381
* feat(docexport): Git remote target for scheduled doc export by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/382
* feat(platform): persist scheduled job health (#384) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/385
* feat(web): add scheduled reports frontend by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/386
* feat: API-key scopes (#278) and shared connector HTTP client (#265) by @ChangedRuby in https://github.com/WiseLabz/WiseLabz/pull/387
* feat(mcp): read-only MCP server over connectors/docs/findings/changes/attention by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/388
* feat(auth): OIDC group→connector roles and IdP step-up (#279 part 3) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/389
* feat(auth): TOTP two-factor authentication (#279 part 1) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/390
* refactor(auth): share OIDC flow cookie helper (#279 follow-up) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/391
* feat(auth): WebAuthn second factor (#279 part 2) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/392
* feat(snapshots): browser and time-travel diff (#276) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/393
* feat: runbook execution with approvals by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/394
* feat(docexport): replay document revisions as Git commits by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/395
* feat(server): bound sync fan-out and elect PostgreSQL leader by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/396
* refactor: expose connector capabilities and add shared contract tests by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/397
* fix(security): sanitize user-controlled values in logs by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/398
* fix(store): release postgres migration connections by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/399
* ci: shard race, coverage and postgres tests and run build in parallel by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/402
* ci: speed up release and compose smoke and reclaim cache quota by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/403
* test: profile backend test time and apply safe speedups by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/404
* test: reuse SQLite fixtures, stabilize lifecycle tests, and expand race CI by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/412
* test: make scheduled export coverage deterministic by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/413
* ci: retain an on-demand shuffled race stress gate by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/414
* docs(ha): record cross-replica WebSocket relay decision (ADR 0005) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/423
* feat(ws): add id and ts to the WebSocket envelope by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/424
* feat(web): refetch volatile queries after WebSocket reconnect and on system.resync by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/425
* feat(ws): filter broadcast events by per-connector access by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/426
* docs: reconcile audit and deferred-feature documentation with merged implementations by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/427
* chore: ignore local openspec directory by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/428
* ci: skip CI jobs for non-code changes by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/430
* ci: add Dependabot, SHA-pin checkout and move actions to Node 24 (#429) by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/440
* chore(deps): bump the go-minor-patch group in /backend with 2 updates by @dependabot[bot] in https://github.com/WiseLabz/WiseLabz/pull/441
* chore(deps): bump @types/diff from 7.0.2 to 8.0.0 in /web by @dependabot[bot] in https://github.com/WiseLabz/WiseLabz/pull/444
* chore(deps): bump motion from 12.42.0 to 13.4.4 in /web by @dependabot[bot] in https://github.com/WiseLabz/WiseLabz/pull/445
* chore(deps): bump mermaid from 11.17.2 to 12.0.0 in /web by @dependabot[bot] in https://github.com/WiseLabz/WiseLabz/pull/446
* ci: bump anchore/sbom-action from 0.24.0 to 0.24.2 in the actions group across 1 directory by @dependabot[bot] in https://github.com/WiseLabz/WiseLabz/pull/447
* ci: cut Dependabot and release-please CI load, and extend Dependabot automation by @gsaraiva2109 in https://github.com/WiseLabz/WiseLabz/pull/448
* chore(deps): bump the web-minor-patch group across 1 directory with 41 updates by @dependabot[bot] in https://github.com/WiseLabz/WiseLabz/pull/442

## New Contributors
* @gsaraiva2109 with @Copilot made their first contribution in https://github.com/WiseLabz/WiseLabz/pull/247
* @ChangedRuby made their first contribution in https://github.com/WiseLabz/WiseLabz/pull/387
* @dependabot[bot] made their first contribution in https://github.com/WiseLabz/WiseLabz/pull/441

**Full Changelog**: https://github.com/WiseLabz/WiseLabz/compare/v0.3.0...v1.0.0

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
