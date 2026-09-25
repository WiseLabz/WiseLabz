# Graph Report - agent-a0c2461ceb996eb06  (2026-09-25)

## Corpus Check
- 826 files · ~499,689 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 19 file(s) not represented in the graph (top: (none) 10, .toml 2, .tmpl 2)

## Summary
- 6254 nodes · 19432 edges · 231 communities (198 shown, 33 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 1711 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `190eb3ac`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- net/http.Client
- context.Context
- net/http.Request
- testApp
- SuggestRequest
- Store
- Store
- ServiceSnapshot
- Handler
- NewService
- git.go
- NewEngine
- newTestApp
- Checker
- sshStdioConn
- testing.T
- DecodeKey
- Dispatcher
- WiseLabz WebSocket Contract (`/ws`)
- SuggestWithFallback
- newDocTestStore
- CommandPalette
- initial database schema
- rewritePlaceholders
- dispatcher_test.go
- IsSecureRequest
- Compare
- Service
- SystemPage.tsx
- react
- Product
- App.tsx
- go_pkg_github_com_wiselabz_wiselabz_internal_store
- RunSync
- chat/chat.go
- Engine
- Root
- go_pkg_testing
- DashboardPage.tsx
- Bottom-dock shell
- icons.tsx
- @tanstack/react-query
- WiseLabz
- docker_test.go
- ProfilePage.tsx
- adguardhome/tables.go
- scheduler/health_test.go
- ServiceDetailPage.tsx
- export_test.go
- UsersPage.tsx
- newTestHandler
- package.json
- portainer/tables.go
- ErrorWithDetails
- rowScanner
- truenas/tables.go
- main
- time.Time
- Manager
- lefthook Commit Hooks
- templates.fixtures.ts
- fixtures.ts
- backup/backup.go
- NewMalformedResponseError
- home_assistant/tables.go
- RunMigrations
- share_links_test.go
- Store
- AppShell — Bottom Dock Shell (single variant)
- net/http.ResponseWriter
- dependencies
- NewChecker
- ThemeControls.tsx
- go_pkg_github_com_wiselabz_wiselabz_internal_connector
- traefik/tables.go
- compliance/engine.go
- Configuration & Documentation Backup (Export/Import)
- RunbooksPage.tsx
- SnapshotEntity
- DocRecord
- settings.mock.ts
- Diff viewer
- Destructive connector confirmation
- single-instance deployment model
- Graphify Knowledge Graph Rules
- Topbar Notification Center (deferred from V1)
- Database: SQLite + PostgreSQL
- React + Vite Frontend
- routerDeps
- go_pkg_strings
- response.go
- handlers.ts
- home_assistant_test.go
- NewStore
- timeline.ts
- .PostMFATOTPConfirm
- newTestHandler
- Register
- connector/connector.go
- unifi_test.go
- WiseLabz Connector Guide
- AppearancePage.tsx
- Connector
- NewEngine
- ConnectorRecord
- WiseLabz — Design Contract
- devDependencies
- Logger
- unifi/tables.go
- Runner
- portainer_test.go
- Connector
- RunDocLockSweep
- Button.tsx
- net/http/httptest.ResponseRecorder
- NewHTTPClient
- adguardhome_test.go
- templates_test.go
- Deps
- middleware.go
- go_pkg_context
- Connector
- Config
- config_test.go
- gitTarget
- Template catalog
- Change detail synthesizer
- Settings mock data
- DocTree
- pgPlaceholderDB
- safe application defaults
- Branch Naming Convention
- viper Config Loader
- chi HTTP Router
- GHCR Container Registry
- GET /api/version (undocumented ops endpoint)
- UserIDFromContext
- Handler
- truenas_test.go
- keyset_test.go
- gitFixture
- ws.ts
- compilerOptions
- diagram.go
- Connector
- traefik_test.go
- MarshalConnectorConfig
- docdiffmodel.ts
- New
- all.go
- templatefuncs.go
- httpx/retry_test.go
- ReportData
- compilerOptions
- AuthedUser
- handlers_contract_test.go
- vectorCache
- Handler
- DeliveryRecord
- NewRouter
- BackupSchedule
- dashboard/handlers_test.go
- router.go
- lifecycleDeps
- fakeRefresherConnector
- render_test.go
- NewRegistry
- .RunDigestSweep
- api/changes_test.go
- newTestHandler
- browser.ts
- createUser
- Contributor Covenant Code of Conduct
- Store
- store/backup_test.go
- Decision
- Decision
- scripts
- newTCPDockerClient
- apikey_scope.go
- Decision
- .call
- api/docs_test.go
- .call
- Changelog
- mockServiceWorker.js
- apikey_scopes_test.go
- ComplianceRuleRecord
- Hub
- openapi_contract_test.go
- release-please-config.json
- connectors_hardening_test.go
- TimeAgo
- Panel
- store package
- OpenDB
- ErrNotFound
- ErrConflict
- slog (stdlib logging)
- Zustand State Management
- Tailwind CSS
- Docker Compose Deployment
- RateLimit
- ComputeWindow
- ShareLink
- Cache
- handlers_bulk_test.go
- runbooks/handlers_test.go
- scanMaintenanceWindow
- computeNextRun
- Audit Trail
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- engine_maintenance_test.go
- Mermaid.tsx
- Security Policy
- APIKeyClaims
- Notification Channels
- compose-smoke.sh
- ClassifyHealth
- timeoutError
- RetentionSettings
- transform_test.go
- Saved Views
- WiseLabz — v2 Backlog
- fakeEmbedder
- tsconfig.json
- setup-env.sh
- CHANGE_PROVENANCE.md
- vite-env.d.ts
- github.com/WiseLabz/wiselabz

## God Nodes (most connected - your core abstractions)
1. `newTestApp()` - 219 edges
2. `Errorf()` - 173 edges
3. `Store` - 141 edges
4. `newDocTestStore()` - 136 edges
5. `UserIDFromContext()` - 77 edges
6. `SnapshotEntity` - 74 edges
7. `react` - 73 edges
8. `cn()` - 69 edges
9. `NewStore()` - 66 edges
10. `@tanstack/react-query` - 60 edges

## Surprising Connections (you probably didn't know these)
- `Panel (`Panel.tsx`)` --references--> `Panel()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `Radii — rounded but tight. Soft-dark, not pill-everything.` --references--> `Panel()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `Surfaces — depth from lightness steps + shadow, never borders alone` --references--> `Panel()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `Shared conventions for new primitives` --references--> `cn()`  [INFERRED]
  docs/DESIGN.md → web/src/lib/cn.ts
- `Client dispatch model` --references--> `WebSocketProvider()`  [INFERRED]
  docs/WS_CONTRACT.md → web/src/ws/WebSocketProvider.tsx

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Contract-First API Codegen Pipeline** — docs_architecture_orval, docs_openapi_spec_document, docs_architecture_react_query, docs_architecture_diff_contract [EXTRACTED 0.85]
- **Dual Local/OIDC Auth Mode System** — docs_architecture_auth_design, docs_architecture_oidc_provider_config, docs_openapi_oidc_provider_schema, docs_openapi_auth_config_endpoint [EXTRACTED 0.90]
- **Step-Up Confirmation Flow for Destructive Actions** — docs_architecture_permissions_stepup, docs_architecture_destructive_confirm_pattern, docs_openapi_auth_elevate_endpoint, docs_openapi_removal_impact_endpoint [EXTRACTED 0.90]

## Communities (231 total, 33 thin omitted)

### Community 0 - "net/http.Client"
Cohesion: 0.03
Nodes (29): Connector, ollamaEmbedder, NewServiceUnavailableError(), setHeaders(), TestValidateCustomURL(), tryParseEntities(), validateCustomURL(), Connector (+21 more)

### Community 1 - "context.Context"
Cohesion: 0.03
Nodes (35): fakeStatusChecker, sanitizeSessions(), Handler, Connector, Connector, existingIDs(), SnapshotRecord, Store (+27 more)

### Community 2 - "net/http.Request"
Cohesion: 0.06
Nodes (31): diffToSpec(), Handler, Handler, Handler, Handler, Handler, Handler, definition() (+23 more)

### Community 3 - "testApp"
Cohesion: 0.23
Nodes (9): testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty(), TestBackupRoutesRequireOperatorRole(), TestBackupScheduleGetDefaults(), TestBackupScheduleUpdate(), TestBackupScheduleUpdateDoesNotLeakSchedulerJobs() (+1 more)

### Community 4 - "SuggestRequest"
Cohesion: 0.10
Nodes (13): claudeProvider, openAICompatibleProvider, openAIEmbedder, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet(), TestRegistryList() (+5 more)

### Community 5 - "Store"
Cohesion: 0.06
Nodes (31): Config, Handler, Registry, NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler() (+23 more)

### Community 6 - "Store"
Cohesion: 0.06
Nodes (15): changeServiceIDs(), Store, placeholders(), changeFilterClause(), AlertRecord, ChangeRecord, Store, scanAlert() (+7 more)

### Community 7 - "ServiceSnapshot"
Cohesion: 0.04
Nodes (20): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, agentEnabled(), Connector, changePatternID(), Engine, markError() (+12 more)

### Community 8 - "Handler"
Cohesion: 0.24
Nodes (7): NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), Handler, RuleEvaluator

### Community 9 - "NewService"
Cohesion: 0.09
Nodes (36): APIKeyChecker, fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed() (+28 more)

### Community 10 - "git.go"
Cohesion: 0.07
Nodes (29): gitAuth(), Exporter, installHTTPS(), TestGitAuthHTTPSNoToken(), TestGitAuthHTTPSToken(), TestGitAuthSSH(), writeTestKey(), GitOptions (+21 more)

### Community 11 - "NewEngine"
Cohesion: 0.13
Nodes (30): NewEngine(), newEngineTestStore(), seedEngineConnector(), seedEngineTemplate(), TestGenerateFromSnapshotIncludesDependencies(), TestGenerateFromTemplateReturnsVersionPersistenceError(), TestGenerateFromTemplateStillPersists(), TestMatchingConnectorsEmptyAppliesToIsWildcard() (+22 more)

### Community 12 - "newTestApp"
Cohesion: 0.02
Nodes (176): testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow(), TestAlertsListSuccess() (+168 more)

### Community 13 - "Checker"
Cohesion: 0.20
Nodes (7): complianceRule(), Checker, RunStaleSweepOnce(), QualityFindingRecord, scanQualityFinding(), FindingNotifier, RotationConfig

### Community 14 - "sshStdioConn"
Cohesion: 0.12
Nodes (10): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), sshStdioConn, golang.org/x/crypto/ssh.Client, golang.org/x/crypto/ssh.ClientConfig, golang.org/x/crypto/ssh.Session, io.Closer (+2 more)

### Community 15 - "testing.T"
Cohesion: 0.02
Nodes (169): cursorPage, runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), newTestLifecycle() (+161 more)

### Community 16 - "DecodeKey"
Cohesion: 0.12
Nodes (19): Handler, Handler, Handler, DecodeKey(), Decrypt(), DeriveKey(), Encrypt(), TestDecodeKey() (+11 more)

### Community 17 - "Dispatcher"
Cohesion: 0.13
Nodes (15): discordPayload(), sendDiscordChannel(), sendGenericWebhookChannel(), sendSlackChannel(), slackPayload(), webhookPayload(), findChannel(), findRoute() (+7 more)

### Community 18 - "WiseLabz WebSocket Contract (`/ws`)"
Cohesion: 0.06
Nodes (33): ADR 0001 — Monorepo, ADR Index (docs/adr/), AI Doc Generation Module (opt-in, provider-agnostic), API Design — REST + WebSocket split, Dual Auth Design (Local JWT + OIDC), Changes/Diff Contract (infra vs doc format), Change-Aware Diff Engine, Monorepo with Go Workspaces (+25 more)

### Community 19 - "SuggestWithFallback"
Cohesion: 0.12
Nodes (16): Provider, StatusError, StubProvider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail() (+8 more)

### Community 20 - "newDocTestStore"
Cohesion: 0.02
Nodes (146): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+138 more)

### Community 21 - "CommandPalette"
Cohesion: 0.12
Nodes (20): Axios API client, Button and IconButton, CommandPalette, theme cycling command, ConfirmDialog, Dialog, ElevationConfirm, English translation catalog (+12 more)

### Community 22 - "initial database schema"
Cohesion: 0.16
Nodes (20): alerts, changes, connector config JSON, connectors, dashboard layouts, doc versions, docs, HashToken (+12 more)

### Community 23 - "rewritePlaceholders"
Cohesion: 0.10
Nodes (14): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), database/sql.Result, database/sql.Row, database/sql.Rows (+6 more)

### Community 24 - "dispatcher_test.go"
Cohesion: 0.18
Nodes (50): TestExpireAlertsOnceNoExpiredAlertsIsNoop(), TestExpireAlertsOnceNotifiesViaDispatcher(), testLogger(), expireAlertsOnce(), NewDispatcher(), deliveriesFor(), findDelivery(), Dispatcher (+42 more)

### Community 25 - "IsSecureRequest"
Cohesion: 0.09
Nodes (23): clearOIDCFlowCookie(), Handler, newOIDCUser(), oidcFlowCookieName(), randomOIDCToken(), readOIDCFlowCookie(), setOIDCFlowCookie(), validHostPort() (+15 more)

### Community 26 - "Compare"
Cohesion: 0.15
Nodes (18): configPushLanded(), driftDescription(), Checker, highestDriftSeverity(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare() (+10 more)

### Community 27 - "Service"
Cohesion: 0.16
Nodes (13): Claims, ElevationClaims, ElevationToken, IssuePairOptions, MFAClaims, MFATicket, Service, TokenPair (+5 more)

### Community 28 - "SystemPage.tsx"
Cohesion: 0.03
Nodes (94): react-i18next, web_src_api_generated_auth_auth_usegetauthproviders, web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey, web_src_api_generated_compliance_compliance_postcompliancerules, web_src_api_generated_compliance_compliance_postcompliancerulestest, web_src_api_generated_compliance_compliance_putcompliancerulesid (+86 more)

### Community 29 - "react"
Cohesion: 0.03
Nodes (112): clsx, @codemirror/lang-markdown, @codemirror/view, react, react-markdown, remark-gfm, tailwind-merge, @uiw/react-codemirror (+104 more)

### Community 30 - "Product"
Cohesion: 0.09
Nodes (24): Connector Guide (docs/connectors/CONNECTOR_GUIDE.md), Connector Interface (Name/Fetch/Validate), Connector Management via UI (full CRUD), Destructive-Action Pattern: Confirm + Blast Radius, Manager Actions (v1 scope), Permissions & Step-Up for Mutating Actions, Role Model — viewer/operator, ServiceSnapshot Data Structure (+16 more)

### Community 31 - "App.tsx"
Cohesion: 0.04
Nodes (65): Endpoint, Keyset (cursor) pagination, react-dom, AXIOS_INSTANCE, BodyType, ErrorType, getAccessToken(), MfaEnrollmentRequiredFn (+57 more)

### Community 32 - "go_pkg_github_com_wiselabz_wiselabz_internal_store"
Cohesion: 0.05
Nodes (46): bulkSnoozeItemResult, bulkSnoozeRequest, changePromptData(), stripPromptTags(), truncateUTF8(), versionSections(), fileName(), slugify() (+38 more)

### Community 33 - "RunSync"
Cohesion: 0.15
Nodes (14): Connector interface, connector schema registration, reverse proxy WebSocket support, OpenAPI REST contract, destructive-action step-up authentication, operational alerts, detected changes, Compare (+6 more)

### Community 34 - "chat/chat.go"
Cohesion: 0.15
Nodes (14): buildPrompt(), TestBuildPrompt(), cosineSimilarity(), Match, packVector(), SplitSections(), TestCosineSimilarityRanksClosestVectorHighest(), TestPackUnpackVectorRoundTrips() (+6 more)

### Community 35 - "Engine"
Cohesion: 0.18
Nodes (5): Engine, sync.Map, AlertNotifier, DocRegenerator, QualityChecker

### Community 36 - "Root"
Cohesion: 0.18
Nodes (12): OpenAPI client generation, Generated-code lint exclusions, Motion preference provider, Vite API and WebSocket proxy, Authentication and onboarding guards, Operator-only routes, Root(), router (+4 more)

### Community 37 - "go_pkg_testing"
Cohesion: 0.07
Nodes (18): dashboardLayout, badRegexMessage(), complianceCondition(), TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails() (+10 more)

### Community 38 - "DashboardPage.tsx"
Cohesion: 0.04
Nodes (74): Connector category icon map, Dashboard widget frame, 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress` (+66 more)

### Community 39 - "Bottom-dock shell"
Cohesion: 0.40
Nodes (5): Authenticated app frame, Bottom-dock shell, Primary navigation, Non-React navigation bridge, Floating dock navigation

### Community 40 - "icons.tsx"
Cohesion: 0.03
Nodes (94): Live dashboard state, RFC-3339, i18next, react-error-boundary, sonner, web_src_api_generated_attention_attention, web_src_api_generated_attention_attention_usegetattention, web_src_api_generated_connectors_connectors (+86 more)

### Community 41 - "@tanstack/react-query"
Cohesion: 0.03
Nodes (66): msw, react-router-dom, @tanstack/react-query, @testing-library/react, vitest, web_src_api_generated_changes_changes, web_src_api_generated_changes_changes_getgetchangesquerykey, web_src_api_generated_dashboard_dashboard (+58 more)

### Community 42 - "WiseLabz"
Cohesion: 0.20
Nodes (10): WCAG 2.2 AA accessibility, Docs-first information architecture, technical homelabbers, machine-honest interface, v1 narrow manager scope, trustworthy live documentation, WiseLabz, commit quality gates (+2 more)

### Community 43 - "docker_test.go"
Cohesion: 0.07
Nodes (38): newDockerClient(), init(), generateSelfSignedCert(), generateSSHHostKey(), serveOneHTTPExchange(), serveSSHDockerConn(), startSSHDockerServer(), TestConfigPush() (+30 more)

### Community 44 - "ProfilePage.tsx"
Cohesion: 0.05
Nodes (32): web_src_api_generated_auth_auth, web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_postauthelevate, web_src_api_generated_auth_auth_usegetauthapikeys, web_src_api_generated_auth_auth_usegetauthelevatemethods, web_src_api_generated_me_me_deletememfafactorsfactorid (+24 more)

### Community 45 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (31): statusInfo, upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo(), buildFiltering() (+23 more)

### Community 46 - "scheduler/health_test.go"
Cohesion: 0.19
Nodes (10): newFakeHealthStore(), TestJobHealthOkToFailingNotifiesOnce(), TestJobHealthPanicCountsAsFailure(), TestJobHealthPersistsAcrossRestart(), JobHealthRecord, Store, scanJobHealth(), fakeHealthStore (+2 more)

### Community 47 - "ServiceDetailPage.tsx"
Cohesion: 0.06
Nodes (47): ADR-0001, ADR-0003, 1. `service.status`, web_src_api_generated_connectors_connectors_postconnectorsconnectoridconfigpush, web_src_api_generated_connectors_connectors_postconnectorsconnectoridhealth, web_src_api_generated_connectors_connectors_postconnectorsconnectoridrestart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstop (+39 more)

### Community 48 - "export_test.go"
Cohesion: 0.20
Nodes (17): fetchAllDocs(), Exporter, IsGeneratedName(), NewExporter(), pruneStale(), RunExportOnce(), newTestStore(), readFile() (+9 more)

### Community 49 - "UsersPage.tsx"
Cohesion: 0.07
Nodes (43): axios, customInstance(), web_src_api_generated_users_users, web_src_api_generated_users_users_deleteusersuserid, web_src_api_generated_users_users_getgetusersquerykey, web_src_api_generated_users_users_postusersuseridresetmfa, web_src_api_generated_users_users_postusersuseridresetpassword, web_src_api_generated_users_users_usegetusers (+35 more)

### Community 50 - "newTestHandler"
Cohesion: 0.12
Nodes (35): doJSON(), testHandler, req(), TestChangePassword(), TestChangePasswordRevokesAPIKeys(), TestCreateUser(), TestDeleteUser(), TestLogin() (+27 more)

### Community 51 - "package.json"
Cohesion: 0.05
Nodes (43): codemirror, @codemirror/commands, @codemirror/state, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, @fontsource/ibm-plex-mono (+35 more)

### Community 52 - "portainer/tables.go"
Cohesion: 0.09
Nodes (37): WantsField(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), environmentDependencies(), putMetadata(), buildEnvironmentTable(), buildStackTable(), cell() (+29 more)

### Community 53 - "ErrorWithDetails"
Cohesion: 0.07
Nodes (26): updateUserRequest, Handler, sanitizeUser(), setRefreshCookie(), writeUserWriteError(), Handler, mustHashDummyPassword(), parseScheduleUpdates() (+18 more)

### Community 54 - "rowScanner"
Cohesion: 0.07
Nodes (24): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), NotificationRecord, Store, scanNotification() (+16 more)

### Community 55 - "truenas/tables.go"
Cohesion: 0.13
Nodes (40): buildDatasets(), buildDisks(), buildInterfaces(), buildNFSShares(), buildPools(), buildReplicationTasks(), buildServices(), buildSMBShares() (+32 more)

### Community 56 - "main"
Cohesion: 0.09
Nodes (26): main(), runHealthcheck(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), Embedder (+18 more)

### Community 57 - "time.Time"
Cohesion: 0.25
Nodes (16): time.Time, ChangeEntry, ComplianceSection, ConnectorDrift, DocChangeEntry, DocsSection, DriftSection, FindingSummary (+8 more)

### Community 58 - "Manager"
Cohesion: 0.07
Nodes (17): cron.EntryID, Manager, LogPartial(), NewManager(), Store, nilToStr(), Store, ReportDefinitionRecord (+9 more)

### Community 59 - "lefthook Commit Hooks"
Cohesion: 0.50
Nodes (5): commit-msg Hook, Conventional Commits Policy, lefthook Commit Hooks, pre-commit Hook, Commit Conventions & Hook Enforcement (dev workflow)

### Community 60 - "templates.fixtures.ts"
Cohesion: 0.16
Nodes (14): web_src_api_model_index_docversion, web_src_api_model_index_template, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, renderTemplate() (+6 more)

### Community 61 - "fixtures.ts"
Cohesion: 0.06
Nodes (41): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc, web_src_api_model_index_docversionmeta (+33 more)

### Community 62 - "backup/backup.go"
Cohesion: 0.05
Nodes (92): confirm(), formatCounts(), main(), runRestore(), runVerify(), newSeededStore(), TestRunRestoreImportsIntoConfiguredDatabase(), TestRunRestoreRejectsCorruptedBundle() (+84 more)

### Community 63 - "NewMalformedResponseError"
Cohesion: 0.10
Nodes (42): unavailable(), SnapshotSection, NewMalformedResponseError(), buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell() (+34 more)

### Community 64 - "home_assistant/tables.go"
Cohesion: 0.09
Nodes (39): jsonType(), TestAttributeCatalogCoversEmittedKeys(), unavailable(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations() (+31 more)

### Community 65 - "RunMigrations"
Cohesion: 0.06
Nodes (64): main(), SyncDocEmbeddings(), TestSyncDocEmbeddingsKeepsOldRowsWhenEmbedFails(), TestRetrieveUsesCacheAndSyncInvalidates(), RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors() (+56 more)

### Community 66 - "share_links_test.go"
Cohesion: 0.17
Nodes (42): GrantConnectorRole(), instanceAdminRole(), NewUser(), TestCreateConversationDocVisibility(), TestListFiltersGrantsBeforePagination(), Handler, newTestHandler(), TestAISuggestInvalidJSON() (+34 more)

### Community 67 - "Store"
Cohesion: 0.23
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 68 - "AppShell — Bottom Dock Shell (single variant)"
Cohesion: 0.50
Nodes (4): AppShell — Bottom Dock Shell (single variant), Theme Engine — Code Default, User-Overridable, Per-User Dashboard Layout with Admin Default (v2), DashboardLayout Schema (per-user widget layout)

### Community 69 - "net/http.ResponseWriter"
Cohesion: 0.09
Nodes (20): Handler, isWritableField(), validateConfigPushRequest(), applyConnectorScalarUpdates(), configRequestField(), Handler, validateConnectorConfig(), writeConfigRejection() (+12 more)

### Community 70 - "dependencies"
Cohesion: 0.05
Nodes (41): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+33 more)

### Community 71 - "NewChecker"
Cohesion: 0.17
Nodes (35): NewChecker(), createComplianceRule(), createComplianceSnapshot(), createConnector(), findings(), newTestStore(), TestCheckEmptyDetectsAndAutoResolves(), TestCheckFailingDetectsAndAutoResolves() (+27 more)

### Community 72 - "ThemeControls.tsx"
Cohesion: 0.12
Nodes (30): @fontsource/space-mono, AdvancedControls(), FONT_KEYS, OPT_KEYS, PRESET_KEYS, Segmented(), ThemeControls(), ColorMode (+22 more)

### Community 73 - "go_pkg_github_com_wiselabz_wiselabz_internal_connector"
Cohesion: 0.06
Nodes (30): TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), TestBuildHostOverrideTableMalformedCases(), TestBuildHostOverrideTableValidOverrides(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEmailMessage() (+22 more)

### Community 74 - "traefik/tables.go"
Cohesion: 0.14
Nodes (30): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable(), cell() (+22 more)

### Community 75 - "compliance/engine.go"
Cohesion: 0.11
Nodes (31): catalog(), contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity (+23 more)

### Community 76 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.06
Nodes (28): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, Manifest, checksum, and verification, 1. Every export gets a manifest and a checksum, 2. Verifying a backup actually restores, 3. Restoring for real (+20 more)

### Community 77 - "RunbooksPage.tsx"
Cohesion: 0.05
Nodes (42): web_src_api_generated_reports_reports, web_src_api_generated_reports_reports_deletereportsdefinitionsreportdefinitionid, web_src_api_generated_reports_reports_getgetreportsdefinitionsquerykey, web_src_api_generated_reports_reports_getgetreportsquerykey, web_src_api_generated_reports_reports_postreportsdefinitions, web_src_api_generated_reports_reports_postreportsdefinitionsreportdefinitionidrun, web_src_api_generated_reports_reports_putreportsdefinitionsreportdefinitionid, web_src_api_generated_reports_reports_usegetreports (+34 more)

### Community 78 - "SnapshotEntity"
Cohesion: 0.09
Nodes (24): TestAttributeCatalogCoversEmittedKeys(), TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), SnapshotEntity, TestBuildContainerTableAttributes(), buildContainerTable() (+16 more)

### Community 79 - "DocRecord"
Cohesion: 0.20
Nodes (6): docSearchWhere(), escapeLike(), DocRecord, Store, scanDoc(), scanDocSummary()

### Community 80 - "settings.mock.ts"
Cohesion: 0.08
Nodes (27): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationconfig, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role (+19 more)

### Community 81 - "Diff viewer"
Cohesion: 0.67
Nodes (3): Document diff model, Diff layout preference, Diff viewer

### Community 82 - "Destructive connector confirmation"
Cohesion: 0.67
Nodes (3): Destructive connector confirmation, Connector removal impact, Step-up reauthentication

### Community 83 - "single-instance deployment model"
Cohesion: 0.67
Nodes (3): PostgreSQL compose deployment, single-instance deployment model, SQLite compose deployment

### Community 84 - "Graphify Knowledge Graph Rules"
Cohesion: 0.67
Nodes (3): GRAPH_REPORT.md, Graphify Knowledge Graph Rules, Graphify Wiki Index

### Community 85 - "Topbar Notification Center (deferred from V1)"
Cohesion: 0.67
Nodes (3): Topbar Notification Center (deferred from V1), NotificationDelivery Schema (per-channel delivery/retry), Notification / NotificationPage Schemas

### Community 86 - "Database: SQLite + PostgreSQL"
Cohesion: 0.67
Nodes (3): Database: SQLite + PostgreSQL, golang-migrate, sqlc (type-safe SQL codegen)

### Community 87 - "React + Vite Frontend"
Cohesion: 0.67
Nodes (3): Frontend Testing Policy (deferred until rewrite), go:embed SPA Embedding, React + Vite Frontend

### Community 88 - "routerDeps"
Cohesion: 0.11
Nodes (30): routerDeps, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountConnectorRoutes(), chi.Router (+22 more)

### Community 89 - "go_pkg_strings"
Cohesion: 0.05
Nodes (35): TestEmailDomainAllowed(), TestOIDCRoleForGroups(), emailDomainAllowed(), oidcRoleForGroups(), Schema(), schemaFor(), TestSchemaMatchesConfig(), Config (+27 more)

### Community 90 - "response.go"
Cohesion: 0.11
Nodes (11): Error(), HandleStoreError(), JSON(), Logger(), WriteDataPaginated(), SinceFromDays(), Handler, go_pkg_github_com_wiselabz_wiselabz_internal_storeerr (+3 more)

### Community 91 - "handlers.ts"
Cohesion: 0.07
Nodes (26): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+18 more)

### Community 92 - "home_assistant_test.go"
Cohesion: 0.10
Nodes (36): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+28 more)

### Community 93 - "NewStore"
Cohesion: 0.11
Nodes (36): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+28 more)

### Community 94 - "timeline.ts"
Cohesion: 0.14
Nodes (17): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+9 more)

### Community 95 - ".PostMFATOTPConfirm"
Cohesion: 0.15
Nodes (15): secondFactorInput, factorJSON(), Handler, GenerateRecoveryCodes(), GenerateTOTPSecret(), NormalizeRecoveryCode(), randomRecoveryChars(), TestGenerateRecoveryCodesAreUniqueAndFormatted() (+7 more)

### Community 96 - "newTestHandler"
Cohesion: 0.11
Nodes (40): actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews(), TestActionMaintenanceLifecycle(), TestActionPermissions(), TestActionStoreFailures() (+32 more)

### Community 97 - "Register"
Cohesion: 0.13
Nodes (25): init(), init(), init(), init(), init(), init(), init(), init() (+17 more)

### Community 98 - "connector/connector.go"
Cohesion: 0.09
Nodes (11): ServiceDependency, TimeoutError, NewTimeoutError(), networkDependencies(), AuthError, CredentialRefresher, MalformedResponseError, Restarter (+3 more)

### Community 99 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 100 - "WiseLabz Connector Guide"
Cohesion: 0.08
Nodes (24): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Conventions (+16 more)

### Community 101 - "AppearancePage.tsx"
Cohesion: 0.14
Nodes (21): zustand, MotionProvider(), AppearancePage(), ChoiceGroup(), AppearanceState, apply(), Contrast, css() (+13 more)

### Community 102 - "Connector"
Cohesion: 0.10
Nodes (13): init(), Connector, buildGatewayTable(), primaryGatewayName(), wanInterfaceName(), PathSegment(), TestValidateCompositeRef(), TestValidateRefSegment() (+5 more)

### Community 103 - "NewEngine"
Cohesion: 0.18
Nodes (23): RequestedFields(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector(), TestRunSyncFieldsPassesHintToConnector(), TestRunSyncFieldsSurvivesCredentialRefresh() (+15 more)

### Community 104 - "ConnectorRecord"
Cohesion: 0.15
Nodes (12): ConnectorRecord, Store, scanConnector(), scanConnectorRows(), nullInt64ToIntPtr(), nullStrToStr(), connectorWithRole, database/sql.NullInt64 (+4 more)

### Community 105 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (24): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+16 more)

### Community 106 - "devDependencies"
Cohesion: 0.08
Nodes (24): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+16 more)

### Community 107 - "Logger"
Cohesion: 0.17
Nodes (16): loggablePath(), Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken(), TestLoggerRedactsWSTicket(), TestLoggablePathMasksShareTokenUnderV1(), TestGetRequestIDMissing() (+8 more)

### Community 108 - "unifi/tables.go"
Cohesion: 0.17
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+21 more)

### Community 109 - "Runner"
Cohesion: 0.16
Nodes (7): cron.EntryID, Runner, cron.Cron, HealthStore, jobEntry, JobInfo, Notifier

### Community 110 - "portainer_test.go"
Cohesion: 0.10
Nodes (35): entityKinds(), sectionByTitle(), TestConfigPushV5(), TestFetchAuthFailureReturnsPlaceholderSnapshot(), TestFetchBothVersions(), TestFetchDegradesPerSection(), TestRestartUnsupportedOnV5(), TestStartStopV5() (+27 more)

### Community 111 - "Connector"
Cohesion: 0.11
Nodes (12): NewAuthError(), TestTypedErrorsAreDistinguishableByType(), TestTypedErrorsWrapAndUnwrap(), Connector, apiMessage(), controllerName(), countByKind(), statusError() (+4 more)

### Community 112 - "RunDocLockSweep"
Cohesion: 0.21
Nodes (7): Dispatcher, Dispatcher, RunDeliveryRetries(), Store, RunDocLockSweep(), runDocLockSweep(), DocLockRecord

### Community 113 - "Button.tsx"
Cohesion: 0.04
Nodes (83): Frontend, 7. `quality.finding.created` and `quality.findings.changed`, motion, @radix-ui/react-popover, web_src_api_generated_alerts_alerts, web_src_api_generated_alerts_alerts_getgetalertsquerykey, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve (+75 more)

### Community 114 - "net/http/httptest.ResponseRecorder"
Cohesion: 0.14
Nodes (12): refreshCookie(), TestDeleteSession(), TestLogout(), TestRefresh(), testHandler, Handler, instanceAdminRoleFor(), spaHandler() (+4 more)

### Community 115 - "NewHTTPClient"
Cohesion: 0.13
Nodes (15): newConnector(), Connector, newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), intConfig(), newConnector(), NewHTTPClient() (+7 more)

### Community 116 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 117 - "templates_test.go"
Cohesion: 0.26
Nodes (15): templateBody, TestTemplateMutationRoleMatrix(), testApp, seedPreviewConnector(), seedTemplate(), TestTemplatesConcurrentUpdatesCreateDistinctVersions(), TestTemplatesPreviewAffectedConnectors(), TestTemplatesPreviewCapturesMissingSnapshot() (+7 more)

### Community 118 - "Deps"
Cohesion: 0.22
Nodes (19): registerListAttentionItems(), registerListChanges(), jsonResult(), registerListConnectors(), registerSearchDocs(), connectorAllowSet(), findingConnectorIDs(), registerListFindings() (+11 more)

### Community 119 - "middleware.go"
Cohesion: 0.10
Nodes (22): AuditRecorder, ConnectorRoleChecker, contextKey, elevationError, PermissionChecker, UserStatusChecker, elevationFailureReason(), extractBearerToken() (+14 more)

### Community 120 - "go_pkg_context"
Cohesion: 0.07
Nodes (23): contains(), searchString(), shareLinkContextKey, go_pkg_context, go_pkg_database_sql, go_pkg_errors, go_pkg_fmt, go_pkg_github_com_google_uuid (+15 more)

### Community 121 - "Connector"
Cohesion: 0.18
Nodes (5): buildGatewayTable(), buildSystemContent(), primaryGatewayName(), wanInterfaceName(), Connector

### Community 122 - "Config"
Cohesion: 0.10
Nodes (20): newLogger(), Config, LogSettings, IsSSHRemote(), AISettings, AuthSettings, BackupSettings, Database (+12 more)

### Community 123 - "config_test.go"
Cohesion: 0.17
Nodes (15): Load(), TestAccessTokenTTLDuration(), TestDocExportGitValidate(), TestLoadDefaults(), TestLoadEnvOverride(), TestLoadEnvOverrideAllFields(), TestLoadEnvOverrideDocExportGitSSH(), TestLoadFromYAML() (+7 more)

### Community 124 - "gitTarget"
Cohesion: 0.19
Nodes (9): commitMessage(), TestCommitMessage(), commitResult, gitTarget, git.Repository, github.com/go-git/go-git/v5/plumbing.Hash, github.com/go-git/go-git/v5/plumbing/object.Signature, github.com/go-git/go-git/v5/plumbing.ReferenceName (+1 more)

### Community 136 - "UserIDFromContext"
Cohesion: 0.08
Nodes (20): Handler, newToken(), sanitize(), Handler, Handler, contextWithShareLink(), Handler, newShareToken() (+12 more)

### Community 137 - "Handler"
Cohesion: 0.19
Nodes (3): Handler, stripLogControlChars(), Handler

### Community 138 - "truenas_test.go"
Cohesion: 0.24
Nodes (17): Connector, newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchIsStableAcrossCalls() (+9 more)

### Community 139 - "keyset_test.go"
Cohesion: 0.33
Nodes (10): assertSameSet(), Store, T, queryPlan(), TestKeysetQueriesUseCoveringIndexes(), TestListAuditRecordsKeysetHonoursFilters(), TestListAuditRecordsKeysetTraversal(), TestListChangesKeysetTraversal() (+2 more)

### Community 140 - "gitFixture"
Cohesion: 0.29
Nodes (9): SetBeforePushForTest(), keys(), newGitFixture(), TestGitExportLifecycle(), TestGitExportPushRejectionReturnsError(), TestGitExportRefusesForeignDirectory(), gitFixture, Exporter (+1 more)

### Community 141 - "ws.ts"
Cohesion: 0.11
Nodes (17): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+9 more)

### Community 142 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 143 - "diagram.go"
Cohesion: 0.26
Nodes (12): entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks(), relatedEntities(), EntityLink (+4 more)

### Community 144 - "Connector"
Cohesion: 0.09
Nodes (4): ConfigField, Connector, Connector, failingPushConnector

### Community 145 - "traefik_test.go"
Cohesion: 0.08
Nodes (41): TestRegisteredSchema(), TestSchemaConfigValidation(), TestAllConnectorImplementationsRegister(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestSchemaExposesAPIVersion(), TestAPIKeyIsStoredAsPassword() (+33 more)

### Community 146 - "MarshalConnectorConfig"
Cohesion: 0.18
Nodes (15): TestDiagnosticsRedactsSecrets(), IsSecretFieldType(), MarshalConnectorConfig(), SecretFieldsChanged(), init(), TestConnectorRotationFieldsRoundTrip(), TestCreateConnectorDefaultsSecretRotatedAtToCreatedAt(), TestSecretFieldsChangedFalseOnRenameOnly() (+7 more)

### Community 147 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 148 - "New"
Cohesion: 0.36
Nodes (13): TestJobHealthWithoutStoreDoesNothing(), New(), TestAddJobInvalidExpression(), TestAddJobRegistersAndFires(), TestContextGivenToJobFunction(), TestInvalidJobNameHandled(), TestJobContextDerivedFromStart(), TestJobSkipsOverlappingInvocations() (+5 more)

### Community 149 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 150 - "templatefuncs.go"
Cohesion: 0.23
Nodes (10): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+2 more)

### Community 151 - "httpx/retry_test.go"
Cohesion: 0.17
Nodes (23): isSafeMethod(), IsSafeMethod(), idempotent(), retryable(), RetryTransport(), sleep(), do(), fail() (+15 more)

### Community 152 - "ReportData"
Cohesion: 0.35
Nodes (6): connectorFilter(), NewGenerator(), TestGeneratorPersistsPartialReportWhenASectionQueryFails(), DefinitionSummary, Generator, ReportData

### Community 153 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 154 - "AuthedUser"
Cohesion: 0.12
Nodes (28): TestEmbeddedSPAWithoutFrontendBuild(), TestCreate(), TestList(), TestRevoke(), AuthedUser(), JWTService(), Token(), WithAuth() (+20 more)

### Community 155 - "handlers_contract_test.go"
Cohesion: 0.28
Nodes (14): AssertMatchesSpec(), loadSpec(), specPath(), createForSpec(), decodeEnvelope(), fieldMsgs(), Handler, TestConnectorSuccessPayloadsMatchSpec() (+6 more)

### Community 156 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 158 - "DeliveryRecord"
Cohesion: 0.36
Nodes (5): seedDelivery(), DeliveryRecord, DeliveryStatus, Store, scanDelivery()

### Community 159 - "NewRouter"
Cohesion: 0.24
Nodes (9): CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders(), SecurityHeaders(), TestSecurityHeaders(), chi.Router, NewRouter() (+1 more)

### Community 160 - "BackupSchedule"
Cohesion: 0.31
Nodes (4): BackupSchedule, Store, scanBackupRun(), BackupRun

### Community 161 - "dashboard/handlers_test.go"
Cohesion: 0.43
Nodes (7): Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout()

### Community 162 - "router.go"
Cohesion: 0.08
Nodes (33): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts (+25 more)

### Community 163 - "lifecycleDeps"
Cohesion: 0.33
Nodes (6): newLifecycleManager(), context.CancelFunc, golang.org/x/sync/errgroup.Group, net/http.Server, lifecycleDeps, lifecycleManager

### Community 165 - "render_test.go"
Cohesion: 0.31
Nodes (13): RenderHTML(), RenderMarkdown(), sampleData(), TestRenderHTML_EscapesDocTitles(), TestRenderHTML_SectionUnavailable(), TestRenderHTML_Truncated(), TestRenderMarkdown_Golden(), TestRenderMarkdown_SectionUnavailable() (+5 more)

### Community 166 - "NewRegistry"
Cohesion: 0.17
Nodes (25): NewRegistry(), Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound() (+17 more)

### Community 167 - ".RunDigestSweep"
Cohesion: 0.40
Nodes (4): digestDue(), formatDigest(), Dispatcher, TestDigestDue()

### Community 168 - "api/changes_test.go"
Cohesion: 0.26
Nodes (12): testApp, seedChange(), seedChangeWithSeverity(), TestChangesAcknowledgeRoleBoundary(), TestChangesAcknowledgeSuccess(), TestChangesBulkResolveEmptyIDs(), TestChangesBulkResolveInvalidStatus(), TestChangesBulkResolvePartialFailure() (+4 more)

### Community 169 - "newTestHandler"
Cohesion: 0.26
Nodes (12): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), Handler, newTestHandler(), TestCreate() (+4 more)

### Community 170 - "browser.ts"
Cohesion: 0.50
Nodes (3): worker, enableMocks(), handlers

### Community 171 - "createUser"
Cohesion: 0.27
Nodes (12): seedAlert(), TestListAttentionItems(), seedChange(), TestListChanges(), TestListConnectors(), enableFakeEmbedding(), TestSearchDocs(), seedFinding() (+4 more)

### Community 173 - "Contributor Covenant Code of Conduct"
Cohesion: 0.15
Nodes (12): 1. Correction, 2. Warning, 3. Temporary Ban, 4. Permanent Ban, Attribution, Contributor Covenant Code of Conduct, Enforcement, Enforcement Guidelines (+4 more)

### Community 175 - "Store"
Cohesion: 0.27
Nodes (4): decodeConnectorIDs(), APIKey, Store, scanAPIKey()

### Community 176 - "store/backup_test.go"
Cohesion: 0.32
Nodes (11): newBackupTestStore(), TestCreateBackupRun(), TestGetBackupScheduleWhenNotExists(), TestListBackupRunsPaginated(), TestPruneBackupRunsByAge(), TestPruneBackupRunsByCount(), TestPruneBackupRunsCombinedLimits(), TestPruneBackupRunsNegativeMaxBackups() (+3 more)

### Community 177 - "Decision"
Cohesion: 0.17
Nodes (11): 0001 — Lab-mutating operation boundaries, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 178 - "Decision"
Cohesion: 0.17
Nodes (11): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 179 - "scripts"
Cohesion: 0.17
Nodes (12): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+4 more)

### Community 181 - "newTCPDockerClient"
Cohesion: 0.18
Nodes (11): GuardedDialer(), IsDangerousIP(), buildDockerTLSConfig(), newTCPDockerClient(), TestNewTCPDockerClientNoTLSWhenNoCert(), TestNewTCPDockerClientRejectsInvalidCertPair(), Unwrap(), newWebhookClient() (+3 more)

### Community 182 - "apikey_scope.go"
Cohesion: 0.15
Nodes (12): APIKeyRestriction, APIKeyRestrictionFromContext(), ClampConnectorRole(), ContextWithAPIKeyRestriction(), TestClampConnectorRole(), treatAsSafeFromContext(), TreatAsSafeMethod(), restrictedCtx() (+4 more)

### Community 183 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 184 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 187 - "api/docs_test.go"
Cohesion: 0.36
Nodes (9): testApp, seedDoc(), TestDocLockConflict(), TestDocLockHappyPath(), TestDocLockRoleBoundary(), TestDocsListAndGetSuccess(), TestDocsSaveRoleBoundary(), TestDocsSaveSuccess() (+1 more)

### Community 188 - ".call"
Cohesion: 0.44
Nodes (6): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture

### Community 189 - "Changelog"
Cohesion: 0.20
Nodes (9): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features, Unreleased (+1 more)

### Community 190 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 191 - "apikey_scopes_test.go"
Cohesion: 0.47
Nodes (8): createKey(), testApp, newConnector(), TestAPIKeyCreateValidation(), TestAPIKeyDefaultsToFullScope(), TestConnectorRestrictedAPIKey(), TestReadOnlyAPIKey(), TestReadOnlyAPIKeyCapsConnectorRoleAtViewer()

### Community 192 - "ComplianceRuleRecord"
Cohesion: 0.36
Nodes (4): changedFields(), ComplianceRuleRecord, Store, scanComplianceRule()

### Community 193 - "Hub"
Cohesion: 0.09
Nodes (13): decodeBulkRequest(), Handler, loggableQuery(), Sanitize(), TestSanitize(), Hub, bulkRequest, github.com/gorilla/websocket.Conn (+5 more)

### Community 194 - "openapi_contract_test.go"
Cohesion: 0.48
Nodes (6): normalizeParams(), routerOperations(), specOperations(), TestOpenAPIMatchesRouter(), chi.Routes, go_pkg_go_yaml_in_yaml_v3

### Community 196 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 197 - "connectors_hardening_test.go"
Cohesion: 0.25
Nodes (8): testApp, init(), TestConnectorsCreateAcceptsValidConfig(), TestConnectorsCreateRejectsInvalidEnum(), TestConnectorsCreateRejectsMalformedConfig(), TestConnectorsSyncAcceptsFieldsHint(), TestConnectorsUpdateRejectsMalformedConfig(), waitForSyncRuns()

### Community 208 - "RateLimit"
Cohesion: 0.29
Nodes (6): TestRateLimit(), RateLimit(), golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 209 - "ComputeWindow"
Cohesion: 0.39
Nodes (6): ComputeWindow(), TestComputeWindow_CappedAt31Days(), TestComputeWindow_ExactlyAtCap(), TestComputeWindow_FirstRun(), TestComputeWindow_ManualRunUsesLastScheduledWatermarkUnchanged(), TestComputeWindow_Watermark()

### Community 211 - "Cache"
Cohesion: 0.43
Nodes (5): Cache, New(), Cache[V], entry, V

### Community 213 - "handlers_bulk_test.go"
Cohesion: 0.47
Nodes (9): bulkReq(), bulkResults(), createBulkFakeConnector(), Handler, registerBulkFakeConnector(), TestBulkReauth(), TestBulkRestart(), TestBulkSync() (+1 more)

### Community 214 - "runbooks/handlers_test.go"
Cohesion: 0.48
Nodes (6): Handler, newTestHandler(), TestCreate(), TestGetNotFound(), TestListMutuallyExclusiveFilters(), TestUpdateAndDelete()

### Community 215 - "scanMaintenanceWindow"
Cohesion: 0.48
Nodes (3): Store, scanMaintenanceWindow(), MaintenanceWindowRecord

### Community 216 - "computeNextRun"
Cohesion: 0.43
Nodes (5): TestComputeNextRun_BackoffNeverExceedsScheduleCadence(), TestComputeNextRun_FailureUsesBackoffSchedule(), TestComputeNextRun_ManualOnlyNeverSchedules(), TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(), computeNextRun()

### Community 217 - "Audit Trail"
Cohesion: 0.40
Nodes (4): Audit Trail, Retention, What's not recorded, What's recorded

### Community 218 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 219 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 222 - "engine_maintenance_test.go"
Cohesion: 0.60
Nodes (5): driftingSnapshot(), setupMaintenanceTestConnector(), TestRunSyncExpiredMaintenanceWindowBehavesNormally(), TestRunSyncNoMaintenanceWindowBehavesNormally(), TestRunSyncSuppressesChangesDuringMaintenanceWindow()

### Community 223 - "Mermaid.tsx"
Cohesion: 0.47
Nodes (4): mermaid, cssVar(), Mermaid(), resolveColor()

### Community 224 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 225 - "APIKeyClaims"
Cohesion: 0.50
Nodes (3): testAPIKeyChecker, APIKeyClaims, validAPIKey()

### Community 228 - "Notification Channels"
Cohesion: 0.40
Nodes (4): Adding a channel type, Channel reference, Notification Channels, Webhook signing (HMAC-SHA256)

### Community 229 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 231 - "ClassifyHealth"
Cohesion: 0.67
Nodes (3): ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold()

### Community 234 - "transform_test.go"
Cohesion: 0.24
Nodes (6): init(), normalizeEnabledColumn(), normalizeFirewallRules(), RegisterTransformer(), TestNormalizeFirewallRulesRewritesEnabledColumn(), Transformer

### Community 235 - "Saved Views"
Cohesion: 0.50
Nodes (3): Endpoints, Saved Views, Scope

## Ambiguous Edges - Review These
- `Topbar Notification Center (deferred from V1)` → `NotificationDelivery Schema (per-channel delivery/retry)`  [AMBIGUOUS]
  docs/MISSING.md · relation: conceptually_related_to
- `Topbar Notification Center (deferred from V1)` → `Notification / NotificationPage Schemas`  [AMBIGUOUS]
  docs/MISSING.md · relation: conceptually_related_to

## Knowledge Gaps
- **567 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `bulkResolveRequest`, `bulkResolveItemResult` (+562 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1297 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **33 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `Topbar Notification Center (deferred from V1)` and `NotificationDelivery Schema (per-channel delivery/retry)`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **What is the exact relationship between `Topbar Notification Center (deferred from V1)` and `Notification / NotificationPage Schemas`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Store` connect `Store` to `net/http.Request`, `testApp`, `ServiceSnapshot`, `UserIDFromContext`, `Handler`, `NewEngine`, `gitFixture`, `Checker`, `testing.T`, `Dispatcher`, `rewritePlaceholders`, `dispatcher_test.go`, `ReportData`, `AuthedUser`, `Handler`, `DeliveryRecord`, `Engine`, `lifecycleDeps`, `NewRegistry`, `createUser`, `export_test.go`, `store/backup_test.go`, `ErrorWithDetails`, `rowScanner`, `.call`, `time.Time`, `Manager`, `.call`, `backup/backup.go`, `RunMigrations`, `share_links_test.go`, `net/http.ResponseWriter`, `NewChecker`, `response.go`, `NewStore`, `engine_maintenance_test.go`, `NewEngine`, `net/http/httptest.ResponseRecorder`, `Deps`, `go_pkg_context`?**
  _High betweenness centrality (0.019) - this node is a cross-community bridge._
- **Why does `testHandler` connect `net/http/httptest.ResponseRecorder` to `go_pkg_github_com_wiselabz_wiselabz_internal_store`, `newTestHandler`, `Service`, `Store`?**
  _High betweenness centrality (0.007) - this node is a cross-community bridge._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _567 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.Client` be split into smaller, more focused modules?**
  _Cohesion score 0.03227722772277228 - nodes in this community are weakly interconnected._
- **Should `context.Context` be split into smaller, more focused modules?**
  _Cohesion score 0.025903673790997735 - nodes in this community are weakly interconnected._