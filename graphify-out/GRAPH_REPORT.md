# Graph Report - fix-bugs-I  (2026-10-01)

## Corpus Check
- 964 files · ~663,437 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 24 file(s) not represented in the graph (top: (none) 13, .toml 2, .tmpl 2)

## Summary
- 7334 nodes · 22065 edges · 278 communities (250 shown, 28 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1754 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7af43d24`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestApp
- testing.T
- web_src_api_model_index
- context.Context
- react
- @tanstack/react-query
- Store
- DashboardPage.tsx
- newTestHandler
- App.tsx
- MapTransportError
- go_pkg_encoding_json
- go_pkg_context
- SystemPage.tsx
- go_pkg_testing
- ServiceSnapshot
- go_pkg_github_com_wiselabz_wiselabz_internal_store
- go_pkg_strconv
- Errorf
- NewMalformedResponseError
- icons.tsx
- ServiceDetailPage.tsx
- Hub
- rowScanner
- ProfilePage.tsx
- DecodeKey
- truenas/tables.go
- useRole.ts
- compliance/engine.go
- NewEngine
- TemplateEditorPage.tsx
- package.json
- Compare
- net/http.Request
- RunMigrations
- ExportToFile
- NewStore
- fixtures.ts
- dispatcher_test.go
- newDocTestStore
- newTestHandler
- traefik_test.go
- SuggestWithFallback
- NewUser
- dependencies
- home_assistant/tables.go
- ADDED Requirements
- Error Creation
- response.go
- ConnectorRecord
- ADDED Requirements
- New
- home_assistant_test.go
- portainer/tables.go
- Store
- adguardhome_test.go
- Connector
- adguardhome/tables.go
- User
- Manager
- WebSocketProvider.tsx
- NewService
- main
- nilToStr
- traefik/tables.go
- routerDeps
- unifi/tables.go
- connector/connector.go
- Common Go Bugs
- NewEngine
- settings.mock.ts
- Connector
- rewritePlaceholders
- runbooks_test.go
- .Fetch
- newHandler
- api/docs_test.go
- net/http.Client
- git.go
- DocRecord
- all_test.go
- SnapshotEntity
- handlers.ts
- IsSecureRequest
- unifi_test.go
- ShareLinkPage.tsx
- ws.ts
- Go Code Style
- provider_test.go
- dashboard_test.go
- Config
- ContextWithUser
- AppearancePage.tsx
- devDependencies
- Codebase Design
- router.go
- WiseLabz — Design Contract
- Register
- net/http.Handler
- portainer_test.go
- Checker
- templates.fixtures.ts
- connector_permission.go
- NewRegistry
- NewHTTPClient
- Deps
- New
- HTML Report Format
- net/http.ResponseWriter
- lifecycleManager
- Sanitize
- WiseLabz — Architecture & Technical Decisions
- Configuration & Documentation Backup (Export/Import)
- ADDED Requirements
- Decisions
- Requirements
- api/changes_test.go
- connectors_health_test.go
- Handler
- Connector
- Backend test performance
- EventRoutingTable.tsx
- Handler
- truenas_test.go
- diagnostics/diagnostics.go
- retention/retention_test.go
- Decision
- compilerOptions
- time.Duration
- handlers_actions_test.go
- sshStdioConn
- RunVerifyOnce
- walkCursorPages
- RunbookRecord
- docdiffmodel.ts
- .GetConnectorUptime
- transform_test.go
- stubEmbedder
- all.go
- ADDED Requirements
- compilerOptions
- HashPassword
- handlers_contract_test.go
- changes/handlers_test.go
- chat/chat.go
- Handler
- httpx/retry_test.go
- time.Time
- pagination_contract_test.go
- createUser
- digestDue
- render_test.go
- Contributing to WiseLabz
- Backup Recovery: What Comes Back, and What Doesn't
- Go Database Best Practices
- golang-troubleshooting/SKILL.md
- Delve Debugger
- General Debugging Methodology
- Production Debugging
- The Golden Rules
- newTestHandler
- notifications/handlers_test.go
- VerifyBundleFile
- Changelog
- WiseLabz
- ARCHITECTURE.md
- Decisions
- scripts
- .agents/skills/openspec-explore/SKILL.md
- config/validate_test.go
- templatefuncs.go
- .render
- Store
- Store
- .claude/skills/openspec-explore/SKILL.md
- Connector
- Contributor Covenant Code of Conduct
- Decision
- Decision
- WiseLabz Connector Guide
- log/slog.Logger
- runbooks/handlers_test.go
- vectorCache
- docker_test.go
- responseWriter
- keyset_test.go
- explore.md
- Decision
- done
- .call
- Handler
- handlers_bulk_test.go
- Engine
- connectors_maintenance_test.go
- .call
- Elector
- ReportData
- ErrorWithDetails
- diagram.go
- Tasks
- Tasks
- Product
- changes.sh
- test-shards.sh
- mockServiceWorker.js
- pprof Reference
- store/backup_test.go
- ComplianceRuleRecord
- routerOperations
- RateLimit
- middleware.go
- release-please-config.json
- Database Performance
- Transactions, Isolation Levels, and Locking
- Testing Database Code
- dashboard/handlers_test.go
- ComputeWindow
- Cache
- Audit Trail
- Step by step
- Proposal
- Proposal
- Proposal
- Proposal
- RequireConnectorRole
- ShareLink
- .enrollmentResult
- webAuthnUser
- scanMaintenanceWindow
- computeNextRun
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- TestComplianceRuleValidation
- areas.sh script
- Struct Scanning and NULLable Columns
- Test-Driven Debugging
- Design
- Tasks
- Design
- Tasks
- Security Policy
- Batch Processing
- Compilation Issues
- Concurrency Debugging
- Web Interface Guidelines
- golden_snapshot_test.go
- Authentication design
- Development workflow
- compose-smoke.sh
- Indexing Strategy
- lifecycle_test.go
- ClassifyHealth
- timeoutError
- RetentionSettings
- Saved Views
- Error Handling
- Parameterized Queries
- JSON Pitfalls
- WiseLabz — v2 Backlog
- coverage-parity.sh
- vite-env.d.ts
- tsconfig.json
- AGENTS.md
- CLAUDE.md
- setup-env.sh
- CHANGE_PROVENANCE.md
- coverpkg.sh
- github.com/WiseLabz/wiselabz
- fields_test.go

## God Nodes (most connected - your core abstractions)
1. `newTestApp()` - 240 edges
2. `Errorf()` - 188 edges
3. `newDocTestStore()` - 146 edges
4. `Store` - 144 edges
5. `UserIDFromContext()` - 88 edges
6. `SnapshotEntity` - 77 edges
7. `react` - 77 edges
8. `NewStore()` - 71 edges
9. `cn()` - 69 edges
10. `@tanstack/react-query` - 64 edges

## Surprising Connections (you probably didn't know these)
- `Panel (`Panel.tsx`)` --references--> `Panel()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `Radii — rounded but tight. Soft-dark, not pill-everything.` --references--> `Panel()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `Surfaces — depth from lightness steps + shadow, never borders alone` --references--> `Panel()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `4. Typography` --references--> `PanelHeader()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `9. Anti-slop bans` --references--> `PanelHeader()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx

## Import Cycles
- None detected.

## Communities (278 total, 28 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.02
Nodes (168): templateBody, testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow() (+160 more)

### Community 1 - "testing.T"
Cohesion: 0.02
Nodes (161): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens() (+153 more)

### Community 2 - "web_src_api_model_index"
Cohesion: 0.06
Nodes (52): Frontend, motion, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve, web_src_api_generated_alerts_alerts_postalertsalertidsnooze, web_src_api_generated_alerts_alerts_postalertsbulksnooze, web_src_api_generated_attention_attention_getgetattentionquerykey, web_src_api_generated_changes_changes_getgetchangeschangeidquerykey (+44 more)

### Community 3 - "context.Context"
Cohesion: 0.03
Nodes (31): fakeConnectorRoleChecker, sanitizeSessions(), Connector, Connector, Connector, SendTest(), Store, SnapshotRecord (+23 more)

### Community 4 - "react"
Cohesion: 0.03
Nodes (167): react, react-i18next, web_src_api_generated_auth_auth_postauthloginmfawebauthnbegin, web_src_api_generated_auth_auth_usegetauthproviders, web_src_api_generated_chat_chat, web_src_api_generated_chat_chat_getgetchatconversationsidquerykey, web_src_api_generated_chat_chat_getgetchatconversationsquerykey, web_src_api_generated_chat_chat_postchatconversations (+159 more)

### Community 5 - "@tanstack/react-query"
Cohesion: 0.04
Nodes (54): msw, react-router-dom, @tanstack/react-query, @testing-library/jest-dom, @testing-library/react, vitest, web_src_api_generated_connectors_connectors_deleteconnectorsconnectorid, web_src_api_generated_connectors_connectors_usegetconnectorsconnectoridremovalimpact (+46 more)

### Community 6 - "Store"
Cohesion: 0.06
Nodes (30): Config, Handler, Registry, NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler() (+22 more)

### Community 7 - "DashboardPage.tsx"
Cohesion: 0.04
Nodes (77): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress`, 3. `sync.complete`, 4. `change.detected` (+69 more)

### Community 8 - "newTestHandler"
Cohesion: 0.06
Nodes (78): mockElevateOIDCServer, secondFactorInput, virtualAuthenticator, testHandler, TestElevateFailuresCountTowardLockout(), TestLocalLoginDisabledIsEnforced(), TestLoginMFARejectsLockedAccountEvenWithCorrectCode(), TestTokenTTLsAndStepUpComeFromSettings() (+70 more)

### Community 9 - "App.tsx"
Cohesion: 0.04
Nodes (69): setAccessToken(), setMfaEnrollmentRequiredHandler(), web_src_api_generated_auth_auth, web_src_api_generated_auth_auth_postauthelevateoidcbegin, web_src_api_generated_auth_auth_postauthelevateoidccomplete, web_src_api_generated_auth_auth_postauthlogin, web_src_api_generated_auth_auth_postauthloginmfa, web_src_api_generated_auth_auth_postauthlogout (+61 more)

### Community 10 - "MapTransportError"
Cohesion: 0.06
Nodes (20): Connector, setHeaders(), TestValidateCustomURL(), tryParseEntities(), validateCustomURL(), Connector, ReadBody(), TestReadBodyLimit() (+12 more)

### Community 11 - "go_pkg_encoding_json"
Cohesion: 0.04
Nodes (54): isSafeMethod(), TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), TestBuildPeerTableAttributes(), TestBuildPolicyTableAttributes(), buildPeerTable() (+46 more)

### Community 12 - "go_pkg_context"
Cohesion: 0.07
Nodes (19): contains(), searchString(), shareLinkContextKey, go_pkg_context, go_pkg_database_sql, go_pkg_errors, go_pkg_fmt, go_pkg_github_com_google_uuid (+11 more)

### Community 13 - "SystemPage.tsx"
Cohesion: 0.06
Nodes (34): web_src_api_generated_connectors_connectors_deleteconnectorsconnectoridgoldensnapshot, web_src_api_generated_connectors_connectors_getgetconnectorsconnectoridgoldensnapshotquerykey, web_src_api_generated_connectors_connectors_postconnectorsconnectoridgoldensnapshot, web_src_api_generated_connectors_connectors_usegetconnectorsconnectoridgoldensnapshot, web_src_api_generated_connectors_connectors_usegetconnectorsconnectoridsnapshotsdiff, web_src_api_generated_system_system_getgetsystembackuprunsquerykey, web_src_api_generated_system_system_getgetsystembackupschedulequerykey, web_src_api_generated_system_system_getsystembackupschedule (+26 more)

### Community 14 - "go_pkg_testing"
Cohesion: 0.07
Nodes (17): Schema(), schemaFor(), TestSchemaMatchesConfig(), IsTimeout(), TestIsTimeout(), go_pkg_github_com_gorilla_websocket, go_pkg_github_com_wiselabz_wiselabz_internal_api_apitest, go_pkg_github_com_wiselabz_wiselabz_internal_web (+9 more)

### Community 15 - "ServiceSnapshot"
Cohesion: 0.03
Nodes (19): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, init(), RegisterTransformer(), runTransformers(), TestRunTransformersAppliesInOrderAndStopsOnError(), TestRunTransformersUnknownCategoryIsNoop() (+11 more)

### Community 16 - "go_pkg_github_com_wiselabz_wiselabz_internal_store"
Cohesion: 0.05
Nodes (46): bulkSnoozeItemResult, bulkSnoozeRequest, changePromptData(), stripPromptTags(), truncateUTF8(), bulkResolveItemResult, bulkResolveRequest, go_pkg_flag (+38 more)

### Community 17 - "go_pkg_strconv"
Cohesion: 0.15
Nodes (7): IPKey(), TestIPKey(), versionSections(), TemplateVersionSection, go_pkg_golang_org_x_time_rate, go_pkg_net_netip, go_pkg_strconv

### Community 18 - "Errorf"
Cohesion: 0.05
Nodes (33): Handler, PermissionChecker, newToken(), sanitize(), Handler, diffToSpec(), Handler, Handler (+25 more)

### Community 19 - "NewMalformedResponseError"
Cohesion: 0.12
Nodes (36): NewMalformedResponseError(), buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP(), groupNames() (+28 more)

### Community 20 - "icons.tsx"
Cohesion: 0.05
Nodes (71): 1. `service.status`, match-sorter, @radix-ui/react-popover, web_src_api_generated_attention_attention, web_src_api_generated_attention_attention_usegetattention, web_src_api_generated_connectors_connectors_deleteconnectorsconnectoridmaintenancewindow, web_src_api_generated_connectors_connectors_getgetconnectorsmaintenancewindowsquerykey, web_src_api_generated_connectors_connectors_postconnectorsbulkreauth (+63 more)

### Community 21 - "ServiceDetailPage.tsx"
Cohesion: 0.05
Nodes (49): ADR-0001, ADR-0003, web_src_api_generated_changes_changes_usegetchanges, web_src_api_generated_connectors_connectors_postconnectorsconnectoridconfigpush, web_src_api_generated_connectors_connectors_postconnectorsconnectoridhealth, web_src_api_generated_connectors_connectors_postconnectorsconnectoridrestart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstop (+41 more)

### Community 22 - "Hub"
Cohesion: 0.07
Nodes (43): TestBroadcastDocEventScoping(), Envelope, Hub, newEnvelope(), newHeartbeat(), NewHub(), normalizeOrigin(), assertEnvelope() (+35 more)

### Community 23 - "rowScanner"
Cohesion: 0.06
Nodes (30): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), BackupRun, Store, scanBackupRun() (+22 more)

### Community 24 - "ProfilePage.tsx"
Cohesion: 0.04
Nodes (48): qrcode, @simplewebauthn/browser, web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_postauthelevate, web_src_api_generated_auth_auth_postauthelevatewebauthnbegin, web_src_api_generated_auth_auth_usegetauthapikeys (+40 more)

### Community 25 - "DecodeKey"
Cohesion: 0.11
Nodes (21): testHandler, Handler, Handler, Handler, Handler, DecodeKey(), Decrypt(), DeriveKey() (+13 more)

### Community 26 - "truenas/tables.go"
Cohesion: 0.13
Nodes (40): buildDatasets(), buildDisks(), buildInterfaces(), buildNFSShares(), buildPools(), buildReplicationTasks(), buildServices(), buildSMBShares() (+32 more)

### Community 27 - "useRole.ts"
Cohesion: 0.07
Nodes (33): Frontend shell & theme (decided 2026-06), i18next, react-error-boundary, sonner, web_src_api_generated_me_me, web_src_api_generated_me_me_usegetme, AppShell, Command (+25 more)

### Community 28 - "compliance/engine.go"
Cohesion: 0.11
Nodes (30): catalog(), contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity (+22 more)

### Community 29 - "NewEngine"
Cohesion: 0.19
Nodes (28): TestTreeEmpty(), TestVersion(), NewEngine(), newEngineTestStore(), seedEngineConnector(), seedEngineTemplate(), TestGenerateFromSnapshotIncludesDependencies(), TestGenerateFromTemplateReturnsVersionPersistenceError() (+20 more)

### Community 30 - "TemplateEditorPage.tsx"
Cohesion: 0.04
Nodes (46): zustand, web_src_api_generated_templates_templates, web_src_api_generated_templates_templates_getgettemplatesquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidversionsquerykey, web_src_api_generated_templates_templates_posttemplatestemplateidpreview, web_src_api_generated_templates_templates_posttemplatestemplateidversionsrevrestore, web_src_api_generated_templates_templates_puttemplatestemplateid (+38 more)

### Community 31 - "package.json"
Cohesion: 0.03
Nodes (102): clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks (+94 more)

### Community 32 - "Compare"
Cohesion: 0.07
Nodes (45): configPushLanded(), SafeCell(), driftDescription(), Checker, highestDriftSeverity(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges() (+37 more)

### Community 33 - "net/http.Request"
Cohesion: 0.07
Nodes (36): oidcElevateFlow, webAuthnFlow, sanitizeUser(), setRefreshCookie(), errLocalLoginDisabled(), Handler, userLocked(), clearFlowCookie() (+28 more)

### Community 34 - "RunMigrations"
Cohesion: 0.11
Nodes (37): main(), OpenDB(), newPostgresTestStore(), GetMigrationStatus(), newMigrator(), collectColumns(), postgresSchemaColumns(), sqliteSchemaColumns() (+29 more)

### Community 35 - "ExportToFile"
Cohesion: 0.15
Nodes (28): Export(), ExportToFile(), Import(), newTestStore(), TestExportIncludesRecordsBeyondAPage(), TestExportRedactsConnectorSecrets(), TestExportToFile(), TestExportToFileCreatesDirectory() (+20 more)

### Community 36 - "NewStore"
Cohesion: 0.11
Nodes (34): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+26 more)

### Community 37 - "fixtures.ts"
Cohesion: 0.06
Nodes (44): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+36 more)

### Community 38 - "dispatcher_test.go"
Cohesion: 0.19
Nodes (50): TestExternalChannelsOncePerEvent(), TestRetryPoisonRowsLeaveDueQueue(), NewDispatcher(), deliveriesFor(), findDelivery(), Dispatcher, grantReader(), newTestStore() (+42 more)

### Community 39 - "newDocTestStore"
Cohesion: 0.03
Nodes (141): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+133 more)

### Community 40 - "newTestHandler"
Cohesion: 0.14
Nodes (24): createDNSResolverConnector(), Handler, itoa(), TestConfigFieldsHandler(), TestConfigPushHandler(), TestStartStopHandler(), Handler, newTestHandler() (+16 more)

### Community 41 - "traefik_test.go"
Cohesion: 0.09
Nodes (40): TestRegisteredSchema(), TestSchemaConfigValidation(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestSchemaExposesAPIVersion(), TestAPIKeyIsStoredAsPassword(), TestRegisteredSchema() (+32 more)

### Community 42 - "SuggestWithFallback"
Cohesion: 0.09
Nodes (20): claudeProvider, Provider, StatusError, StubProvider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError() (+12 more)

### Community 43 - "NewUser"
Cohesion: 0.17
Nodes (41): TestExpireAlertsOnceNotifiesViaDispatcher(), GrantConnectorRole(), instanceAdminRole(), NewUser(), Handler, newTestHandler(), TestAISuggestInvalidJSON(), TestGetLockOfUnknownDoc() (+33 more)

### Community 44 - "dependencies"
Cohesion: 0.05
Nodes (42): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+34 more)

### Community 45 - "home_assistant/tables.go"
Cohesion: 0.09
Nodes (39): jsonType(), TestAttributeCatalogCoversEmittedKeys(), unavailable(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations() (+31 more)

### Community 46 - "ADDED Requirements"
Cohesion: 0.05
Nodes (40): ADDED Requirements, Purpose, Requirement: Classification is explained and self-tested, Requirement: Code paths map to their CI areas, Requirement: CodeQL scans only on relevant changes, Requirement: Draft pull requests defer heavy jobs, Requirement: Non-code changes skip all heavy CI jobs, Requirement: Postgres tests run only when their dependencies change (+32 more)

### Community 47 - "Error Creation"
Cohesion: 0.05
Nodes (36): Creating Errors, Custom Error Types, Custom types that wrap other errors, Decision table: which error strategy to use, Error Creation, Error String Conventions, Errors as Values, `errors.New` — static error messages (+28 more)

### Community 48 - "response.go"
Cohesion: 0.07
Nodes (27): decodeStoredSnapshot(), Handler, snapshotStoreError(), Handler, Cursor(), DecodeCursor(), EncodeCursor(), T (+19 more)

### Community 49 - "ConnectorRecord"
Cohesion: 0.09
Nodes (46): NewChecker(), createComplianceRule(), createComplianceSnapshot(), createConnector(), findings(), newTestStore(), TestCheckEmptyDetectsAndAutoResolves(), TestCheckFailingDetectsAndAutoResolves() (+38 more)

### Community 50 - "ADDED Requirements"
Cohesion: 0.05
Nodes (39): ADDED Requirements, Purpose, Requirement: Container images are pinned and updated, Requirement: Every action reference is pinned to a commit SHA, Requirement: GitHub Actions are updated monthly in one grouped PR, Requirement: Go modules and web packages are updated monthly, Requirement: Merging several PRs does not re-run CI on every open PR, Requirement: Minor and patch Dependabot PRs merge without further action (+31 more)

### Community 51 - "New"
Cohesion: 0.09
Nodes (28): confirm(), formatCounts(), main(), runRestore(), runVerify(), newSeededStore(), TestRunRestoreImportsIntoConfiguredDatabase(), TestRunRestoreRejectsCorruptedBundle() (+20 more)

### Community 52 - "home_assistant_test.go"
Cohesion: 0.10
Nodes (37): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+29 more)

### Community 53 - "portainer/tables.go"
Cohesion: 0.12
Nodes (33): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEnvironmentTable(), buildStackTable(), cell(), containerRows(), environmentNames(), environmentTypeName() (+25 more)

### Community 54 - "Store"
Cohesion: 0.07
Nodes (17): changeServiceIDs(), Store, placeholders(), changeFilterClause(), AlertRecord, ChangeRecord, Store, idArgs() (+9 more)

### Community 55 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 56 - "Connector"
Cohesion: 0.07
Nodes (11): init(), ConfigField, Connector, buildRouteTable(), buildGatewayTable(), primaryGatewayName(), wanInterfaceName(), PathSegment() (+3 more)

### Community 57 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (32): statusInfo, unavailable(), upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo() (+24 more)

### Community 58 - "User"
Cohesion: 0.11
Nodes (16): discordPayload(), sendDiscordChannel(), sendGenericWebhookChannel(), sendSlackChannel(), slackPayload(), webhookPayload(), findChannel(), findRoute() (+8 more)

### Community 59 - "Manager"
Cohesion: 0.08
Nodes (13): cron.EntryID, Manager, Scheduler, LogPartial(), cron.EntryID, Store, Store, ReportDefinitionRecord (+5 more)

### Community 60 - "WebSocketProvider.tsx"
Cohesion: 0.05
Nodes (51): RFC-3339, 15. `system.resync`, Client dispatch model, Delivery, Envelope, Mock emitter (frontend-first), Naming convention, Reconnect behavior (+43 more)

### Community 61 - "NewService"
Cohesion: 0.07
Nodes (43): APIKeyChecker, testAuditCall, testAuditRecorder, UserStatusChecker, TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed() (+35 more)

### Community 62 - "main"
Cohesion: 0.10
Nodes (26): authSettingsSource(), main(), runHealthcheck(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder() (+18 more)

### Community 63 - "nilToStr"
Cohesion: 0.07
Nodes (26): connectorIDs(), docIDs(), exportDocs(), exportTemplates(), exportWithin(), importBundle(), importConnectors(), importDocVersions() (+18 more)

### Community 64 - "traefik/tables.go"
Cohesion: 0.14
Nodes (30): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable(), cell() (+22 more)

### Community 65 - "routerDeps"
Cohesion: 0.09
Nodes (38): routerDeps, AuditRecorder, MFAChecker, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router (+30 more)

### Community 66 - "unifi/tables.go"
Cohesion: 0.17
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+21 more)

### Community 67 - "connector/connector.go"
Cohesion: 0.06
Nodes (21): TimeoutError, NewAuthError(), NewServiceUnavailableError(), NewTimeoutError(), TestTypedErrorsAreDistinguishableByType(), TestTypedErrorsWrapAndUnwrap(), apiMessage(), controllerName() (+13 more)

### Community 68 - "Common Go Bugs"
Cohesion: 0.06
Nodes (31): `break` in `select`/`switch` Inside `for` Loop, Closed Channel in `select` Causes Busy Loop, Common Go Bugs, Concurrent Map Read/Write (Fatal), Context Misuse, Copying sync Types, Defer Gotchas, Enum Zero Value with `iota` (+23 more)

### Community 69 - "NewEngine"
Cohesion: 0.12
Nodes (31): RequestedFields(), TestBaseContext(), TestRunDueSyncsRespectsLimits(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector(), TestRunSyncFieldsPassesHintToConnector() (+23 more)

### Community 70 - "settings.mock.ts"
Cohesion: 0.08
Nodes (27): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role, web_src_api_model_index_session (+19 more)

### Community 71 - "Connector"
Cohesion: 0.22
Nodes (4): SnapshotSection, Connector, unavailable(), session

### Community 72 - "rewritePlaceholders"
Cohesion: 0.10
Nodes (14): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), database/sql.Result, database/sql.Row, database/sql.Rows (+6 more)

### Community 73 - "runbooks_test.go"
Cohesion: 0.14
Nodes (26): runbookResp, runbookStepResp, TestAttentionRunbookLinkForAlert(), TestAttentionRunbookLinkForFinding(), createRunbookWithStep(), testApp, seedProxmoxConnector(), seedRunbook() (+18 more)

### Community 74 - ".Fetch"
Cohesion: 0.10
Nodes (12): ServiceDependency, WantsField(), environmentDependencies(), putMetadata(), unavailable(), agentEnabled(), Connector, poolDependencies() (+4 more)

### Community 75 - "newHandler"
Cohesion: 0.21
Nodes (16): TestEmbeddedSPAWithoutFrontendBuild(), NewHandler(), TestCreate(), TestList(), TestRevoke(), JWTService(), Token(), WithAuth() (+8 more)

### Community 76 - "api/docs_test.go"
Cohesion: 0.10
Nodes (31): createKey(), testApp, newConnector(), TestAPIKeyCreateValidation(), TestAPIKeyDefaultsToFullScope(), TestConnectorRestrictedAPIKey(), TestReadOnlyAPIKey(), TestReadOnlyAPIKeyCapsConnectorRoleAtViewer() (+23 more)

### Community 77 - "net/http.Client"
Cohesion: 0.10
Nodes (7): ollamaEmbedder, openAICompatibleProvider, openAIEmbedder, Connector, LimitedBody(), Connector, net/http.Client

### Community 78 - "git.go"
Cohesion: 0.06
Nodes (33): gitAuth(), installHTTPS(), TestCommitMessage(), TestGitAuthHTTPSNoToken(), TestGitAuthHTTPSToken(), TestGitAuthSSH(), writeTestKey(), keys() (+25 more)

### Community 79 - "DocRecord"
Cohesion: 0.17
Nodes (5): existingIDs(), docSearchWhere(), escapeLike(), DocRecord, Store

### Community 80 - "all_test.go"
Cohesion: 0.40
Nodes (5): countLifecycle(), TestAllConnectorImplementationsRegister(), TestConnectorCapabilitiesMatchOptionalInterfaces(), TestConnectorFailureContract(), go_pkg_github_com_wiselabz_wiselabz_internal_connector_connectortest

### Community 81 - "SnapshotEntity"
Cohesion: 0.13
Nodes (17): SnapshotEntity, TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), TestBuildHostOverrideTableMalformedCases(), TestBuildHostOverrideTableValidOverrides(), TestBuildContainerTableAttributes(), buildContainerTable() (+9 more)

### Community 82 - "handlers.ts"
Cohesion: 0.06
Nodes (30): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+22 more)

### Community 83 - "IsSecureRequest"
Cohesion: 0.27
Nodes (10): ClientIP(), hostOnly(), IsSecureRequest(), isTrustedProxy(), TestClientIPRejectsNonIPForwardedFor(), TestClientIPTrustedPeerUsesForwardedFor(), TestClientIPUntrustedPeerIgnoresHeaders(), TestIsSecureRequestTLS() (+2 more)

### Community 84 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 85 - "ShareLinkPage.tsx"
Cohesion: 0.06
Nodes (45): axios, AXIOS_INSTANCE, BodyType, customInstance(), ErrorType, getAccessToken(), MfaEnrollmentRequiredFn, RefreshFn (+37 more)

### Community 86 - "ws.ts"
Cohesion: 0.07
Nodes (35): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+27 more)

### Community 87 - "Go Code Style"
Cohesion: 0.08
Nodes (23): Code Style Details, Extract Complex Conditions, Value vs Pointer Arguments, Code Organization Within Files, Complex Conditions & Init Scope, Composite Literals, Control Flow, Cross-References (+15 more)

### Community 88 - "provider_test.go"
Cohesion: 0.29
Nodes (6): testProvider, TestRegistryGet(), TestRegistryList(), TestStubProviderName(), TestStubProviderSuggest(), TestStubProviderSuggestStream()

### Community 89 - "dashboard_test.go"
Cohesion: 0.19
Nodes (16): dashboardLayout, scopedOverview, assertOnlyConnector(), dashboardConnector(), getOverview(), testApp, TestDashboardAdminDefaultPermissionGate(), TestDashboardLayoutPerUserIsolation() (+8 more)

### Community 90 - "Config"
Cohesion: 0.10
Nodes (23): NewWebAuthnService(), TestWebAuthnRPConfig(), WebAuthnRPConfig(), Config, LogSettings, IsSSHRemote(), AISettings, AuthSettings (+15 more)

### Community 91 - "ContextWithUser"
Cohesion: 0.15
Nodes (15): TestListCacheSeparatesRestrictedKeys(), TestConnectorStoreErrorPaths(), TestGetRequiresGrantEvenForInstanceAdmin(), TestListFiltersGrantsBeforePagination(), TestByServiceNoDocsYet(), TestGenerate(), TestGetUnknownIDFallsBackToServicePlaceholder(), TestListHidesLabWideDocsAndTotalsFromNonAdmins() (+7 more)

### Community 92 - "AppearancePage.tsx"
Cohesion: 0.15
Nodes (20): MotionProvider(), AppearancePage(), ChoiceGroup(), AppearanceState, apply(), Contrast, css(), DEFAULTS (+12 more)

### Community 93 - "devDependencies"
Cohesion: 0.08
Nodes (25): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+17 more)

### Community 94 - "Codebase Design"
Cohesion: 0.09
Nodes (21): 1. In-process, 2. Local-substitutable, 3. Remote but owned (Ports & Adapters), 4. True external (Mock), Deepening, Dependency categories, Seam discipline, Testing strategy: replace, don't layer (+13 more)

### Community 95 - "router.go"
Cohesion: 0.14
Nodes (22): go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts, go_pkg_github_com_wiselabz_wiselabz_internal_api_apikeys, go_pkg_github_com_wiselabz_wiselabz_internal_api_attention, go_pkg_github_com_wiselabz_wiselabz_internal_api_auth, go_pkg_github_com_wiselabz_wiselabz_internal_api_changes, go_pkg_github_com_wiselabz_wiselabz_internal_api_chat, go_pkg_github_com_wiselabz_wiselabz_internal_api_compliance, go_pkg_github_com_wiselabz_wiselabz_internal_api_connectors (+14 more)

### Community 96 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 97 - "Register"
Cohesion: 0.10
Nodes (30): init(), init(), Capabilities(), CapabilityDescriptor, supportedLifecycleVerbs(), init(), init(), init() (+22 more)

### Community 98 - "net/http.Handler"
Cohesion: 0.09
Nodes (27): CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders(), Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken() (+19 more)

### Community 99 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 100 - "Checker"
Cohesion: 0.19
Nodes (8): Snapshot, complianceRule(), Checker, RunStaleSweepOnce(), QualityFindingRecord, scanQualityFinding(), FindingNotifier, RotationConfig

### Community 101 - "templates.fixtures.ts"
Cohesion: 0.18
Nodes (12): web_src_api_model_index_docversion, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, renderTemplate(), resolveToken() (+4 more)

### Community 102 - "connector_permission.go"
Cohesion: 0.15
Nodes (11): APIKeyRestriction, auditConnectorGrantDiffJSON(), ClampConnectorRole(), getConnectorGrant(), ConnectorGrantDiff, Store, highestConnectorRole(), listOIDCConnectorGrants() (+3 more)

### Community 103 - "NewRegistry"
Cohesion: 0.45
Nodes (12): NewRegistry(), NewHandler(), TestAIConfigRoundTrip(), testConfig(), TestGetAuthConfig(), TestGetDecryptedAPIKeyNoKeyStored(), TestNotificationsConfigRoundTrip(), TestNotificationsConfigSigningSecret() (+4 more)

### Community 104 - "NewHTTPClient"
Cohesion: 0.12
Nodes (16): newConnector(), Connector, newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), intConfig(), newConnector(), NewHTTPClient() (+8 more)

### Community 105 - "Deps"
Cohesion: 0.22
Nodes (19): registerListAttentionItems(), registerListChanges(), jsonResult(), registerListConnectors(), registerSearchDocs(), connectorAllowSet(), findingConnectorIDs(), registerListFindings() (+11 more)

### Community 106 - "New"
Cohesion: 0.05
Nodes (63): fetchAllDocs(), fileName(), Exporter, IsGeneratedName(), NewExporter(), pruneStale(), RunExportOnce(), slugify() (+55 more)

### Community 107 - "HTML Report Format"
Cohesion: 0.10
Nodes (18): Call-graph collapse, Candidate card, Cross-section (good for layered shallowness), Diagram patterns, Hand-built boxes-and-arrows (when Mermaid's layout fights you), Header, HTML Report Format, Mass diagram (good for "interface as wide as implementation") (+10 more)

### Community 108 - "net/http.ResponseWriter"
Cohesion: 0.07
Nodes (22): Handler, validateConfigPushRequest(), capitalize(), Handler, decodeBulkRequest(), Handler, APIKeyRestrictionFromContext(), RejectRestrictedAPIKey() (+14 more)

### Community 109 - "lifecycleManager"
Cohesion: 0.12
Nodes (9): newLifecycleManager(), Election, context.CancelFunc, golang.org/x/sync/errgroup.Group, net/http.Server, gatedStopScheduler, lifecycleDeps, lifecycleManager (+1 more)

### Community 110 - "Sanitize"
Cohesion: 0.09
Nodes (16): Handler, isWritableField(), loggablePath(), loggableQuery(), ConfigPusher, Err(), Sanitize(), TestErr() (+8 more)

### Community 111 - "WiseLabz — Architecture & Technical Decisions"
Cohesion: 0.10
Nodes (20): ADR index, AI module, API design, Backend, Build pipeline, Changes / diff contract (decided 2026-06), Connector interface, Connector management via UI (decided 2026-06-27) (+12 more)

### Community 112 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.10
Nodes (17): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, Manifest, checksum, and verification, What's excluded, and why, What's included, Bundle format (+9 more)

### Community 113 - "ADDED Requirements"
Cohesion: 0.10
Nodes (19): ADDED Requirements, Purpose, Requirement: Checks follow CI change classification, Requirement: Fail-safe to full checks, Requirement: Full-run escape hatch, Requirement: Narrowed scope within an area, Requirement: Workflow linting, Scenario: Backend-only commit (+11 more)

### Community 114 - "Decisions"
Cohesion: 0.10
Nodes (19): Context, D10. Postgres job gated on the Go dependency closure, D11. `govulncheck` gating and nightly workflow, D12. "CodeQL – Code Quality" (GitHub's built-in Code Quality scan): check before acting, D1. Keep `dorny/paths-filter` to list files; classify in `scripts/ci/changes.sh`, D2. Rules are an ordered Bash `case` table; first match wins, D3. `web/package.json` version-only rule, pull requests only, D4. Fixture table and `check` mode (+11 more)

### Community 115 - "Requirements"
Cohesion: 0.10
Nodes (19): pre-commit-hooks Specification, Purpose, Requirement: Checks follow CI change classification, Requirement: Fail-safe to full checks, Requirement: Full-run escape hatch, Requirement: Narrowed scope within an area, Requirement: Workflow linting, Requirements (+11 more)

### Community 116 - "api/changes_test.go"
Cohesion: 0.26
Nodes (12): testApp, seedChange(), seedChangeWithSeverity(), TestChangesAcknowledgeRoleBoundary(), TestChangesAcknowledgeSuccess(), TestChangesBulkResolveEmptyIDs(), TestChangesBulkResolveInvalidStatus(), TestChangesBulkResolvePartialFailure() (+4 more)

### Community 117 - "connectors_health_test.go"
Cohesion: 0.32
Nodes (12): testApp, registerHealthFakeType(), seedHealthTestConnector(), TestConnectorsHealthDegraded(), TestConnectorsHealthDoesNotCreateSnapshot(), TestConnectorsHealthOffline(), TestConnectorsHealthOnline(), TestConnectorsHealthRecordsTimeSeriesRow() (+4 more)

### Community 118 - "Handler"
Cohesion: 0.19
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 120 - "Backend test performance"
Cohesion: 0.11
Nodes (19): Backend test performance, CI job times, CI measurements, Coverage strategy, Dependency updates and action pinning, Deterministic scheduled exports (#408), Fixture reuse and lifecycle tests (#405–#407), Follow-ups (+11 more)

### Community 121 - "EventRoutingTable.tsx"
Cohesion: 0.24
Nodes (9): web_src_api_generated_connectors_connectors_usegetconnectors, web_src_api_model_index_connectorcategory, web_src_api_model_index_notificationchanneltype, web_src_api_model_index_notificationconfig, CHANNEL_LABELS, eventLabel(), EventRoutingTable(), KNOWN_EVENT_TYPES (+1 more)

### Community 122 - "Handler"
Cohesion: 0.24
Nodes (7): NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), Handler, RuleEvaluator

### Community 123 - "truenas_test.go"
Cohesion: 0.11
Nodes (32): entityKinds(), sectionByTitle(), TestConfigPushV5(), TestFetchAuthFailureReturnsError(), TestFetchBothVersions(), TestFetchDegradesPerSection(), TestFetchReleasesV6Session(), TestRestartUnsupportedOnV5() (+24 more)

### Community 124 - "diagnostics/diagnostics.go"
Cohesion: 0.10
Nodes (33): AIConfigSummary, LoadAIConfigSummary(), CheckHealth(), Collect(), collectVersions(), newTestStore(), TestCheckHealthReportsDegradedOnClosedDB(), TestCollectIncludesHealthVersionsAndSchedule() (+25 more)

### Community 125 - "retention/retention_test.go"
Cohesion: 0.61
Nodes (8): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupPrunesOldHealthChecks(), TestRunCleanupSkipsDisabledCategories()

### Community 126 - "Decision"
Cohesion: 0.12
Nodes (14): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+6 more)

### Community 127 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 128 - "time.Duration"
Cohesion: 0.10
Nodes (17): Claims, ElevationClaims, ElevationToken, IssuePairOptions, MFAClaims, MFATicket, RuntimeSettings, Service (+9 more)

### Community 129 - "handlers_actions_test.go"
Cohesion: 0.32
Nodes (14): actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews(), TestActionMaintenanceLifecycle(), TestActionPermissions(), TestActionStoreFailures() (+6 more)

### Community 130 - "sshStdioConn"
Cohesion: 0.12
Nodes (10): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), sshStdioConn, golang.org/x/crypto/ssh.Client, golang.org/x/crypto/ssh.ClientConfig, golang.org/x/crypto/ssh.Session, io.Closer (+2 more)

### Community 131 - "RunVerifyOnce"
Cohesion: 0.28
Nodes (9): failVerification(), LatestBundle(), RunVerifyOnce(), ListVerifications(), RecordVerification(), TestLatestBundleNoBundles(), TestListVerificationsNewestFirstAndLimit(), TestRunVerifyOnceNoBundles() (+1 more)

### Community 132 - "walkCursorPages"
Cohesion: 0.40
Nodes (6): cursorPage, decodeCursorPage(), testApp, TestAuditCursorPaginationTraversal(), TestChangesCursorPaginationTraversal(), walkCursorPages()

### Community 133 - "RunbookRecord"
Cohesion: 0.29
Nodes (5): RunbookRecord, RunbookStepRecord, Store, scanRunbook(), scanRunbookStep()

### Community 134 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 135 - ".GetConnectorUptime"
Cohesion: 0.33
Nodes (3): Store, HealthCheckRecord, UptimeStats

### Community 136 - "transform_test.go"
Cohesion: 0.50
Nodes (3): normalizeEnabledColumn(), normalizeFirewallRules(), TestNormalizeFirewallRulesRewritesEnabledColumn()

### Community 138 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 140 - "ADDED Requirements"
Cohesion: 0.12
Nodes (15): ADDED Requirements, Purpose, Requirement: Agent-only changes select no application jobs, Requirement: Agent tooling is not in the Docker build context, Requirement: Application checks stay scoped to application sources, Requirement: Mixed and unknown changes keep application checks, Scenario: Agent files plus application code, Scenario: Context contents (+7 more)

### Community 141 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 142 - "HashPassword"
Cohesion: 0.12
Nodes (17): mustHashDummyPassword(), instanceAdminRoleFor(), testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty(), TestBackupRoutesRequireOperatorRole(), TestBackupScheduleGetDefaults() (+9 more)

### Community 144 - "handlers_contract_test.go"
Cohesion: 0.28
Nodes (14): AssertMatchesSpec(), loadSpec(), specPath(), createForSpec(), decodeEnvelope(), fieldMsgs(), Handler, TestConnectorSuccessPayloadsMatchSpec() (+6 more)

### Community 145 - "changes/handlers_test.go"
Cohesion: 0.32
Nodes (13): Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound(), TestDismissSuccess() (+5 more)

### Community 146 - "chat/chat.go"
Cohesion: 0.16
Nodes (13): buildPrompt(), TestBuildPrompt(), cosineSimilarity(), Match, packVector(), SplitSections(), TestCosineSimilarityRanksClosestVectorHighest(), TestPackUnpackVectorRoundTrips() (+5 more)

### Community 147 - "Handler"
Cohesion: 0.20
Nodes (8): definition(), NewHandler(), record(), reportJSON(), valid(), JobName(), Handler, input

### Community 148 - "httpx/retry_test.go"
Cohesion: 0.20
Nodes (20): retryable(), RetryTransport(), sleep(), do(), fail(), status(), TestRetryTransportDisabled(), TestRetryTransportDoesNotRetry() (+12 more)

### Community 149 - "time.Time"
Cohesion: 0.25
Nodes (16): time.Time, ChangeEntry, ComplianceSection, ConnectorDrift, DocChangeEntry, DocsSection, DriftSection, FindingSummary (+8 more)

### Community 153 - "pagination_contract_test.go"
Cohesion: 0.21
Nodes (13): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_go_ast (+5 more)

### Community 154 - "createUser"
Cohesion: 0.23
Nodes (15): ContextWithAPIKeyRestriction(), TestClampConnectorRole(), seedAlert(), TestListAttentionItems(), seedChange(), TestListChanges(), TestListConnectors(), enableFakeEmbedding() (+7 more)

### Community 155 - "digestDue"
Cohesion: 0.40
Nodes (5): calendarDaysBetween(), digestDue(), TestDigestDue(), TestDigestDueWeeklyAcrossSpringDST(), time.Location

### Community 156 - "render_test.go"
Cohesion: 0.31
Nodes (13): RenderHTML(), RenderMarkdown(), sampleData(), TestRenderHTML_EscapesDocTitles(), TestRenderHTML_SectionUnavailable(), TestRenderHTML_Truncated(), TestRenderMarkdown_Golden(), TestRenderMarkdown_SectionUnavailable() (+5 more)

### Community 157 - "Contributing to WiseLabz"
Cohesion: 0.14
Nodes (14): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+6 more)

### Community 158 - "Backup Recovery: What Comes Back, and What Doesn't"
Cohesion: 0.14
Nodes (12): 1. Every export gets a manifest and a checksum, 2. Verifying a backup actually restores, 3. Restoring for real, 4. What a restore does *not* bring back, Backup Recovery: What Comes Back, and What Doesn't, Recovery runbook (suggested order), Backups, PostgreSQL support (+4 more)

### Community 159 - "Go Database Best Practices"
Cohesion: 0.15
Nodes (13): Avoid Hidden SQL Features, Best Practices Summary, Connection Pool, Context Propagation, Cross-References, Deep Dives, Go Database Best Practices, Library Choice (+5 more)

### Community 160 - "golang-troubleshooting/SKILL.md"
Cohesion: 0.19
Nodes (5): Code Review Red Flags, CPU Profiling, Lock Contention, Memory Profiling, Performance Troubleshooting

### Community 161 - "Delve Debugger"
Cohesion: 0.15
Nodes (13): Advanced Analysis, Basic Usage, Common Commands, Delve Debugger, Diagnostic Tools, GC Tracing, Go documentation command, GOTRACEBACK (+5 more)

### Community 162 - "General Debugging Methodology"
Cohesion: 0.15
Nodes (13): General Debugging Methodology, Step 10: Defense-in-Depth, Step 1: Understand Expected vs Actual, Step 2: Get the Full Error, Step 3: Isolate the Problem, Step 4: Check External Dependencies, Step 5: Check Observability Tools, Step 6: Compare with Working Code (+5 more)

### Community 163 - "Production Debugging"
Cohesion: 0.15
Nodes (12): HTTP Client Issues, Logging & Observability, Network & HTTP Debugging, Production Debugging, Production Debugging Checklist, Request ID Tracing, Step 1: Capture Immediately (don't restart!), Step 2: System Metrics (+4 more)

### Community 164 - "The Golden Rules"
Cohesion: 0.15
Nodes (13): 1. Read the Error Message First, 2. Reproduce Before You Fix, 3. If You Don't Measure It, You're Guessing, 4. One Hypothesis at a Time, 5. Find the Root Cause — No Workarounds, 6. Research the Codebase, Not Just the Diff, 7. Start Simple, Cross-References (+5 more)

### Community 166 - "newTestHandler"
Cohesion: 0.15
Nodes (17): spaHandler(), templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), Handler, newTestHandler() (+9 more)

### Community 167 - "notifications/handlers_test.go"
Cohesion: 0.28
Nodes (15): AuthedUser(), NewHandler(), decodePaginated(), jsonHasEmptyArrayItems(), newTestStore(), seedDelivery(), TestListDeliveriesEmpty(), TestListDeliveriesNoFilter() (+7 more)

### Community 169 - "VerifyBundleFile"
Cohesion: 0.19
Nodes (16): exists(), Handler, seedBackupRuns(), TestPruneBackupsAgeLimit(), ImportFromFile(), BuildManifest(), BundleCounts(), ChecksumBytes() (+8 more)

### Community 170 - "Changelog"
Cohesion: 0.15
Nodes (12): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), 1.0.0 (2026-10-01), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features (+4 more)

### Community 171 - "WiseLabz"
Cohesion: 0.15
Nodes (8): Code of Conduct, Configuration, Contributing, Features, License, Quick start, Supported services, WiseLabz

### Community 172 - "ARCHITECTURE.md"
Cohesion: 0.17
Nodes (8): 0004 — PostgreSQL leader election for background workers, Consequences, Context, Decision, Adding a channel type, Channel reference, Notification Channels, Webhook signing (HMAC-SHA256)

### Community 173 - "Decisions"
Cohesion: 0.15
Nodes (12): Checkout at v7.0.1 rather than v6.1.0, Context, Decisions, Design, `directories` with a glob for github-actions, not `directory: "/"`, `.github/dependabot.yml` is ignored (`-`), not `w`, Goals / Non-Goals, Grouping and commit conventions (+4 more)

### Community 174 - "scripts"
Cohesion: 0.15
Nodes (13): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+5 more)

### Community 175 - ".agents/skills/openspec-explore/SKILL.md"
Cohesion: 0.17
Nodes (11): Check for context, Ending Discovery, Guardrails, Handling Different Entry Points, OpenSpec Awareness, Planning a Change, The Stance, What You Don't Have To Do (+3 more)

### Community 176 - "config/validate_test.go"
Cohesion: 0.17
Nodes (13): Config, mask(), redactDSN(), redactKVPassword(), Config, TestEveryKeyEnvOverridable(), TestRedactDSN(), TestRedacted() (+5 more)

### Community 177 - "templatefuncs.go"
Cohesion: 0.23
Nodes (10): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+2 more)

### Community 178 - ".render"
Cohesion: 0.24
Nodes (4): TemplateFuncs(), GenerateResult, renderResult, text/template.FuncMap

### Community 179 - "Store"
Cohesion: 0.13
Nodes (8): fakeStatusChecker, testAPIKeyChecker, APIKeyClaims, ValidAPIKey(), decodeConnectorIDs(), APIKey, Store, scanAPIKey()

### Community 180 - "Store"
Cohesion: 0.23
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 181 - ".claude/skills/openspec-explore/SKILL.md"
Cohesion: 0.17
Nodes (11): Check for context, Ending Discovery, Guardrails, Handling Different Entry Points, OpenSpec Awareness, Planning a Change, The Stance, What You Don't Have To Do (+3 more)

### Community 183 - "Contributor Covenant Code of Conduct"
Cohesion: 0.17
Nodes (12): 1. Correction, 2. Warning, 3. Temporary Ban, 4. Permanent Ban, Attribution, Contributor Covenant Code of Conduct, Enforcement, Enforcement Guidelines (+4 more)

### Community 184 - "Decision"
Cohesion: 0.17
Nodes (12): 0001 — Lab-mutating operation boundaries, Addendum (#282): runbook steps are an additional entry point, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision (+4 more)

### Community 185 - "Decision"
Cohesion: 0.17
Nodes (12): 0005 — Cross-replica WebSocket event relay for active/active, Authorization and secrets, Consequences, Context, Decision, Duplicate suppression and ordering, Event ownership: local first, then relay, Mechanism: PostgreSQL LISTEN/NOTIFY (+4 more)

### Community 186 - "WiseLabz Connector Guide"
Cohesion: 0.17
Nodes (12): Conventions, Dependencies, Getting your connector merged, Health checks vs. sync, Keeping snapshots stable, Session-based and multi-flavour APIs, Sync flow, Testing without a real instance (+4 more)

### Community 188 - "log/slog.Logger"
Cohesion: 0.10
Nodes (16): expireAlertsOnce(), newLogger(), formatDigest(), Dispatcher, Dispatcher, Dispatcher, RunDeliveryRetries(), Store (+8 more)

### Community 189 - "runbooks/handlers_test.go"
Cohesion: 0.31
Nodes (12): Handler, newTestHandler(), seedProxmoxConnector(), TestCreate(), TestCreateStepsValidation(), TestExecuteStepForbiddenWithoutOperatorGrant(), TestExecuteStepNotFound(), TestGetNotFound() (+4 more)

### Community 190 - "vectorCache"
Cohesion: 0.17
Nodes (11): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), TestVectorCachePutExistingKeyUpdatesInPlace(), vectorCache, vectorEntry, vectorKey (+3 more)

### Community 191 - "docker_test.go"
Cohesion: 0.06
Nodes (48): GuardedDialer(), IsDangerousIP(), buildDockerTLSConfig(), newDockerClient(), newTCPDockerClient(), generateSelfSignedCert(), generateSSHHostKey(), serveOneHTTPExchange() (+40 more)

### Community 193 - "keyset_test.go"
Cohesion: 0.19
Nodes (16): Store, seedConnectorForChanges(), TestChangeRelatedServiceIDsAndPatternIDRoundTrip(), TestChangeRelatedServiceIDsDefaultsToEmptyArray(), TestCountRecentChangePatterns(), TestCountRecentChangesByPattern(), assertSameSet(), Store (+8 more)

### Community 194 - "explore.md"
Cohesion: 0.18
Nodes (10): Check for context, Ending Discovery, Guardrails, OpenSpec Awareness, Planning a Change, The Stance, What You Don't Have To Do, What You Might Do (+2 more)

### Community 195 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 197 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 198 - "Handler"
Cohesion: 0.17
Nodes (6): updateUserRequest, Handler, writeUserWriteError(), OIDCProvider, github.com/coreos/go-oidc/v3/oidc.Provider, golang.org/x/oauth2.Config

### Community 199 - "handlers_bulk_test.go"
Cohesion: 0.47
Nodes (9): bulkReq(), bulkResults(), createBulkFakeConnector(), Handler, registerBulkFakeConnector(), TestBulkReauth(), TestBulkRestart(), TestBulkSync() (+1 more)

### Community 200 - "Engine"
Cohesion: 0.22
Nodes (3): Engine, sync.Map, DocRegenerator

### Community 201 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 202 - ".call"
Cohesion: 0.44
Nodes (6): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture

### Community 203 - "Elector"
Cohesion: 0.24
Nodes (6): New(), postgresDB(), TestSecondElectorWaitsThenTakesOver(), TestWatchReportsTerminatedSession(), database/sql.Conn, Elector

### Community 204 - "ReportData"
Cohesion: 0.47
Nodes (4): connectorFilter(), DefinitionSummary, Generator, ReportData

### Community 207 - "ErrorWithDetails"
Cohesion: 0.09
Nodes (17): applyConnectorScalarUpdates(), configRequestField(), Handler, parseScheduleUpdates(), validateConnectorConfig(), validateRotationFields(), viewOf(), writeConfigRejection() (+9 more)

### Community 208 - "diagram.go"
Cohesion: 0.26
Nodes (12): entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks(), relatedEntities(), EntityLink (+4 more)

### Community 209 - "Tasks"
Cohesion: 0.20
Nodes (9): 1. Pin actions/checkout, 2. Upgrade Node 20 actions to Node 24, 3. Dependabot configuration, 4. Change classification, 5. Documentation, 6. Reduce Dependabot CI load, 7. Release job and Dependabot automation, 8. Integration checks (+1 more)

### Community 210 - "Tasks"
Cohesion: 0.20
Nodes (9): 1. Classifier script and fixtures, 2. Wire the classifier into `ci.yml`, 3. Composite Bun action, 4. CodeQL workflow, 5. Nightly vulnerability scan, 6. "CodeQL – Code Quality" check, 7. Documentation, 8. End-to-end verification (+1 more)

### Community 211 - "Product"
Cohesion: 0.20
Nodes (10): Accessibility & Inclusion, Anti-references, Brand Personality, Design Principles, Locked frontend direction (planning session, 2026-06; revised 2026-09), Product, Product decisions (pre-planning, v1), Product Purpose (+2 more)

### Community 212 - "changes.sh"
Cohesion: 0.40
Nodes (9): classify_path(), cmd_check(), cmd_classify(), cmd_go_closure(), describe_flags(), die(), go_package_dirs(), package_version_only() (+1 more)

### Community 213 - "test-shards.sh"
Cohesion: 0.40
Nodes (8): cmd_check(), cmd_matrix(), cmd_profile(), cmd_run(), cmd_timings(), die(), test-shards.sh script, shard_json()

### Community 214 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 215 - "pprof Reference"
Cohesion: 0.22
Nodes (9): Analyzing and Interpreting Profiles, Capturing Profiles, Enable pprof HTTP Server, pprof Reference, Profile Types, Quick Setup (Development), Remote Profiling (Production), Secure Setup (Production) (+1 more)

### Community 216 - "store/backup_test.go"
Cohesion: 0.11
Nodes (25): newBackupTestStore(), TestCreateBackupRun(), TestGetBackupScheduleWhenNotExists(), TestListBackupRunsPaginated(), TestPruneBackupRunsByAge(), TestPruneBackupRunsByCount(), TestPruneBackupRunsCombinedLimits(), TestPruneBackupRunsNegativeMaxBackups() (+17 more)

### Community 217 - "ComplianceRuleRecord"
Cohesion: 0.36
Nodes (4): changedFields(), ComplianceRuleRecord, Store, scanComplianceRule()

### Community 218 - "routerOperations"
Cohesion: 0.50
Nodes (5): normalizeParams(), routerOperations(), specOperations(), TestOpenAPIMatchesRouter(), chi.Routes

### Community 219 - "RateLimit"
Cohesion: 0.31
Nodes (6): TestRateLimit(), RateLimit(), golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 220 - "middleware.go"
Cohesion: 0.09
Nodes (16): contextKey, elevationError, TestEmailDomainAllowed(), TestOIDCConnectorRolesForGroups(), TestOIDCRoleForGroups(), connectorRoleLess(), emailDomainAllowed(), oidcConnectorRolesForGroups() (+8 more)

### Community 221 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 222 - "Database Performance"
Cohesion: 0.25
Nodes (7): Configuration, Connection Pool Sizing, Database Performance, Monitoring, Prometheus Metrics, Query Performance Tips, Table of Contents

### Community 223 - "Transactions, Isolation Levels, and Locking"
Cohesion: 0.25
Nodes (5): Basic transaction pattern, Custom isolation level, Locking variants, SELECT FOR UPDATE — prevent race conditions, Transactions, Isolation Levels, and Locking

### Community 224 - "Testing Database Code"
Cohesion: 0.25
Nodes (8): Integration Tests, Mock for service-layer tests, sqlmock for Query-Level Testing, Table of Contents, Test database with testcontainers-go, Testing Database Code, Unit Tests with Mocks, What to Test

### Community 225 - "dashboard/handlers_test.go"
Cohesion: 0.43
Nodes (7): Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout()

### Community 226 - "ComputeWindow"
Cohesion: 0.39
Nodes (6): ComputeWindow(), TestComputeWindow_CappedAt31Days(), TestComputeWindow_ExactlyAtCap(), TestComputeWindow_FirstRun(), TestComputeWindow_ManualRunUsesLastScheduledWatermarkUnchanged(), TestComputeWindow_Watermark()

### Community 228 - "Cache"
Cohesion: 0.43
Nodes (5): Cache, New(), Cache[V], entry, V

### Community 229 - "Audit Trail"
Cohesion: 0.25
Nodes (7): Audit Trail, Endpoint, Filtering and export, Keyset (cursor) pagination, Retention, What's not recorded, What's recorded

### Community 230 - "Step by step"
Cohesion: 0.25
Nodes (8): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Step by step

### Community 231 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 232 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 233 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 234 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 235 - "RequireConnectorRole"
Cohesion: 0.26
Nodes (11): ConnectorRoleChecker, Handler, snapshotFixture(), snapshotRequest(), snapshotResponse(), TestSnapshotDiffValidationOwnershipAndAudit(), TestSnapshotOwnershipAndFullShape(), TestSnapshotsViewerAndCursor() (+3 more)

### Community 237 - ".enrollmentResult"
Cohesion: 0.15
Nodes (14): factorJSON(), Handler, GenerateRecoveryCodes(), GenerateTOTPSecret(), NormalizeRecoveryCode(), randomRecoveryChars(), TestGenerateRecoveryCodesAreUniqueAndFormatted(), TestGenerateTOTPSecretProducesScannableURL() (+6 more)

### Community 238 - "webAuthnUser"
Cohesion: 0.33
Nodes (3): webAuthnUser, github.com/go-webauthn/webauthn/webauthn.Credential, github.com/google/uuid.UUID

### Community 239 - "scanMaintenanceWindow"
Cohesion: 0.48
Nodes (3): Store, scanMaintenanceWindow(), MaintenanceWindowRecord

### Community 240 - "computeNextRun"
Cohesion: 0.43
Nodes (5): TestComputeNextRun_BackoffNeverExceedsScheduleCadence(), TestComputeNextRun_FailureUsesBackoffSchedule(), TestComputeNextRun_ManualOnlyNeverSchedules(), TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(), computeNextRun()

### Community 241 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 242 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 243 - "TestComplianceRuleValidation"
Cohesion: 0.29
Nodes (7): badRegexMessage(), complianceCondition(), TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails()

### Community 244 - "areas.sh script"
Cohesion: 0.48
Nodes (5): areas(), is_true(), areas.sh script, expect(), areas-test.sh script

### Community 245 - "Struct Scanning and NULLable Columns"
Cohesion: 0.33
Nodes (5): JSON Marshaling, NULLable Columns, Struct Scanning and NULLable Columns, Struct Scanning with pgx, Struct Scanning with sqlx

### Community 246 - "Test-Driven Debugging"
Cohesion: 0.33
Nodes (5): Debugging Flaky Tests, Expand Edge Cases with Table Tests, Reproduce the Bug in a Test, Test-Driven Debugging, Useful Test Flags

### Community 253 - "Design"
Cohesion: 0.33
Nodes (5): Context, Decisions, Design, Goals / Non-Goals, Risks / Trade-offs

### Community 254 - "Tasks"
Cohesion: 0.33
Nodes (5): 1. Classifier tweak, 2. Area helper, 3. Lefthook wiring, 4. Docs, Tasks

### Community 255 - "Design"
Cohesion: 0.33
Nodes (5): Context, Decisions, Design, Goals / Non-Goals, Risks / Trade-offs

### Community 256 - "Tasks"
Cohesion: 0.33
Nodes (5): 1. Classifier regression coverage, 2. Docker build context, 3. Scan and lint scope audit, 4. Documentation and event evidence, Tasks

### Community 257 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 258 - "Batch Processing"
Cohesion: 0.40
Nodes (5): Batch INSERT with sqlx, Batch Processing, Bulk INSERT with pgx (PostgreSQL COPY protocol), Cursor-based pagination (avoid OFFSET), Sweet spot: 100–1,000 rows per batch

### Community 259 - "Compilation Issues"
Cohesion: 0.40
Nodes (4): CGO Issues, Compilation Issues, Module Problems, Version Mismatch

### Community 260 - "Concurrency Debugging"
Cohesion: 0.40
Nodes (5): Concurrency Debugging, Deadlocks, Goroutine Leaks, Race Conditions, Table of Contents

### Community 261 - "Web Interface Guidelines"
Cohesion: 0.40
Nodes (4): Guidelines Source, How It Works, Usage, Web Interface Guidelines

### Community 263 - "golden_snapshot_test.go"
Cohesion: 0.60
Nodes (4): Store, mustCreateGoldenSnapshotConnector(), TestGetSnapshotByID(), TestPinGoldenSnapshotRoundTrip()

### Community 264 - "Authentication design"
Cohesion: 0.40
Nodes (5): Authentication design, Destructive-action pattern: confirm + blast radius (decided 2026-06-27), OIDC group→connector roles and IdP step-up (#279 part 3), OIDC provider configuration (decided 2026-06-25: file-defined, app toggles only), Permissions & step-up for mutating actions (decided 2026-06-27)

### Community 265 - "Development workflow"
Cohesion: 0.40
Nodes (5): Branching, Code quality, Commit conventions, Commit hooks (`lefthook`), Development workflow

### Community 267 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 268 - "Indexing Strategy"
Cohesion: 0.50
Nodes (4): Indexing Strategy, Use SQL MCP to check existing indexes, When to suggest adding indexes, When to suggest removing indexes

### Community 269 - "lifecycle_test.go"
Cohesion: 0.21
Nodes (12): TestExpireAlertsOnceNoExpiredAlertsIsNoop(), newTestLifecycle(), startTestLifecycle(), TestLeaderStartsSchedulerAndRunsJob(), TestLifecycleManagerOrderedShutdown(), TestLifecycleManagerShutdownCancelsWorkContext(), TestLifecycleManagerWaitsForSchedulerBeforeCancelAndDBClose(), testLogger() (+4 more)

### Community 270 - "ClassifyHealth"
Cohesion: 0.67
Nodes (3): ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold()

### Community 273 - "Saved Views"
Cohesion: 0.50
Nodes (3): Endpoints, Saved Views, Scope

### Community 274 - "Error Handling"
Cohesion: 0.67
Nodes (3): Always close rows, Common database error patterns, Error Handling

### Community 275 - "Parameterized Queries"
Cohesion: 0.67
Nodes (3): Dynamic column names, Dynamic IN clauses, Parameterized Queries

### Community 276 - "JSON Pitfalls"
Cohesion: 0.67
Nodes (3): JSON Pitfalls, Numbers into `interface{}` become `float64`, Unexported fields silently ignored

## Knowledge Gaps
- **1014 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `testHandler`, `bulkResolveRequest` (+1009 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1776 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **28 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Store` connect `Store` to `go_pkg_context`, `HashPassword`, `Errorf`, `Handler`, `time.Time`, `rowScanner`, `DecodeKey`, `createUser`, `NewEngine`, `RunMigrations`, `ExportToFile`, `NewStore`, `dispatcher_test.go`, `notifications/handlers_test.go`, `VerifyBundleFile`, `NewUser`, `response.go`, `ConnectorRecord`, `New`, `Store`, `User`, `Manager`, `log/slog.Logger`, `main`, `nilToStr`, `.call`, `Handler`, `NewEngine`, `rewritePlaceholders`, `Engine`, `.call`, `newHandler`, `ReportData`, `ErrorWithDetails`, `store/backup_test.go`, `ContextWithUser`, `Checker`, `NewRegistry`, `Deps`, `New`, `net/http.ResponseWriter`, `lifecycleManager`, `Handler`, `diagnostics/diagnostics.go`, `retention/retention_test.go`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Why does `healthFakeConnector` connect `ServiceSnapshot` to `time.Duration`, `connectors_health_test.go`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **Why does `WiseLabz — Architecture & Technical Decisions` connect `WiseLabz — Architecture & Technical Decisions` to `Authentication design`, `Development workflow`, `useRole.ts`, `ARCHITECTURE.md`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _1014 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.021608348680171884 - nodes in this community are weakly interconnected._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.020127838977288183 - nodes in this community are weakly interconnected._
- **Should `web_src_api_model_index` be split into smaller, more focused modules?**
  _Cohesion score 0.0629800307219662 - nodes in this community are weakly interconnected._