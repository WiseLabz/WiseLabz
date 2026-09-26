# Graph Report - agent-a27bd01e4df8201b0  (2026-09-25)

## Corpus Check
- 845 files · ~514,488 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 19 file(s) not represented in the graph (top: (none) 10, .toml 2, .tmpl 2)

## Summary
- 6269 nodes · 19828 edges · 215 communities (197 shown, 18 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1616 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ebdb3d24`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestApp
- newDocTestStore
- App.tsx
- testing.T
- context.Context
- go_pkg_net_http
- connector/connector.go
- ProfilePage.tsx
- cn
- go_pkg_context
- package.json
- react
- Errorf
- ConnectorEditPage.tsx
- vitest
- ServiceSnapshot
- go_pkg_time
- WebSocketProvider.tsx
- @tanstack/react-query
- rowScanner
- go_pkg_testing
- Manager
- NewEngine
- New
- Handler
- ErrorWithDetails
- Get
- DocRecord
- ExportToFile
- WiseLabz — Architecture & Technical Decisions
- fixtures.ts
- dispatcher_test.go
- icons.tsx
- DashboardPage.tsx
- NewService
- DecodeKey
- truenas/tables.go
- response.go
- RunMigrations
- home_assistant/tables.go
- dependencies
- NewChecker
- share_links_test.go
- Connector
- docker_test.go
- Store
- lifecycleDeps
- NewMalformedResponseError
- portainer/tables.go
- adguardhome/tables.go
- src/theme.ts
- home_assistant_test.go
- middleware.go
- NewStore
- User
- ConnectorRecord
- unifi/tables.go
- Configuration & Documentation Backup (Export/Import)
- logging_test.go
- net/http.ResponseWriter
- settings.mock.ts
- traefik/tables.go
- rewritePlaceholders
- ServiceDetailPage.tsx
- newTestHandler
- routerDeps
- main
- NewRegistry
- newTestHandler
- Register
- GetTypeSchema
- Connector
- Service
- Config
- Store
- handlers.ts
- Connector
- NewEngine
- unifi_test.go
- SuggestRequest
- net/http.Request
- createUser
- fetch_test.go
- timeline.ts
- middleware_test.go
- WiseLabz — Design Contract
- devDependencies
- AppearancePage.tsx
- AuthedUser
- go_pkg_os
- Hub
- NotificationRecord
- apikey_scope.go
- Compare
- portainer_test.go
- export_test.go
- gitTarget
- Store
- NewHTTPClient
- adguardhome_test.go
- .Fetch
- Deps
- sshStdioConn
- runRestore
- connector_permission.go
- router.go
- Handler
- chat/chat.go
- IsSecureRequest
- config_test.go
- httpx/retry_test.go
- ws.ts
- truenas_test.go
- diagnostics/diagnostics.go
- time.Duration
- Runner
- ws/ws_test.go
- compilerOptions
- Handler
- traefik_test.go
- docdiffmodel.ts
- ReportsPage.tsx
- git_internal_test.go
- all.go
- compilerOptions
- testApp
- ChangeDetailPage.tsx
- changes/handlers_test.go
- InstanceAdminFromContext
- Handler
- vectorCache
- go_pkg_encoding_base64
- templatefuncs.go
- time.Time
- Engine
- HashToken
- pagination_contract_test.go
- newTestHandler
- net/http.Client
- gitFixture
- render_test.go
- log/slog.Logger
- SnapshotEntity
- Contributing to WiseLabz
- net/http.Handler
- diagram.go
- JobHealthRecord
- Engine
- docs/handlers_test.go
- Handler
- Store
- Decision
- Connector
- scripts
- seedHealthTestConnector
- Handler
- Connector
- Decision
- WiseLabz Connector Guide
- Product
- .call
- connectors_hardening_test.go
- connectors_maintenance_test.go
- .call
- Changelog
- mockServiceWorker.js
- retention/retention_test.go
- apikey_scopes_test.go
- ComplianceRuleRecord
- RunbookRecord
- release-please-config.json
- api/auth/oidc.go
- TestComplianceRuleValidation
- Config
- ComputeWindow
- Cache
- Step by step
- WiseLabz
- RateLimit
- webAuthnUser
- pfsense.go
- scanMaintenanceWindow
- computeNextRun
- Contributor Covenant Code of Conduct
- Store
- Audit Trail
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- Mermaid.tsx
- RequireConnectorRole
- golden_snapshot_test.go
- MfaEnrollDialog
- @vitejs/plugin-react
- RetentionSettings
- engine_maintenance_test.go
- Security Policy
- main.tsx
- stubEmbedder
- fakeEmbedder
- fakeDocRegenerator
- fakeQualityChecker
- seedScopeFixture
- Enforcement Guidelines
- compose-smoke.sh
- timeoutError
- MISSING — deferred & future frontend features
- Saved Views
- WiseLabz — v2 Backlog
- tsconfig.json
- AGENTS.md
- setup-env.sh
- CHANGE_PROVENANCE.md
- vite-env.d.ts
- github.com/WiseLabz/wiselabz

## God Nodes (most connected - your core abstractions)
1. `newTestApp()` - 219 edges
2. `Errorf()` - 178 edges
3. `Store` - 141 edges
4. `newDocTestStore()` - 140 edges
5. `UserIDFromContext()` - 80 edges
6. `SnapshotEntity` - 74 edges
7. `react` - 74 edges
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
- `4. Typography` --references--> `PanelHeader()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx
- `9. Anti-slop bans` --references--> `PanelHeader()`  [INFERRED]
  docs/DESIGN.md → web/src/components/ui/Panel.tsx

## Import Cycles
- None detected.

## Communities (215 total, 18 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.02
Nodes (193): templateBody, testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow() (+185 more)

### Community 1 - "newDocTestStore"
Cohesion: 0.02
Nodes (157): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+149 more)

### Community 2 - "App.tsx"
Cohesion: 0.05
Nodes (61): Frontend shell & theme (decided 2026-06), react-error-boundary, react-i18next, sonner, web_src_api_generated_alerts_alerts, web_src_api_generated_alerts_alerts_getgetalertsquerykey, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve (+53 more)

### Community 3 - "testing.T"
Cohesion: 0.02
Nodes (154): cursorPage, TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens(), TestClaudeSuggestErrors(), TestClaudeSuggestMultipleContentBlocks(), TestOpenAICompatibleSuggest(), TestOpenAICompatibleSuggestErrors(), TestOIDCRedirectURL() (+146 more)

### Community 4 - "context.Context"
Cohesion: 0.03
Nodes (24): fakeStatusChecker, sanitizeSessions(), Connector, Connector, SnapshotRecord, Store, Store, MFAFactor (+16 more)

### Community 5 - "go_pkg_net_http"
Cohesion: 0.07
Nodes (35): bulkSnoozeItemResult, bulkSnoozeRequest, dashboardLayout, changePromptData(), stripPromptTags(), truncateUTF8(), bulkResolveItemResult, bulkResolveRequest (+27 more)

### Community 6 - "connector/connector.go"
Cohesion: 0.05
Nodes (25): TimeoutError, GuardedDialer(), IsDangerousIP(), NewServiceUnavailableError(), NewTimeoutError(), setHeaders(), TestValidateCustomURL(), tryParseEntities() (+17 more)

### Community 7 - "ProfilePage.tsx"
Cohesion: 0.03
Nodes (73): setAccessToken(), web_src_api_generated_auth_auth, web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_postauthelevate, web_src_api_generated_auth_auth_postauthelevateoidcbegin, web_src_api_generated_auth_auth_postauthelevateoidccomplete (+65 more)

### Community 8 - "cn"
Cohesion: 0.03
Nodes (92): web_src_api_generated_docs_docs_getgetdocsdocidquerykey, web_src_api_generated_docs_docs_getgetdocsdocidversionsquerykey, web_src_api_generated_docs_docs_getgetdocstreequerykey, web_src_api_generated_docs_docs_postdocsdocidaisuggest, web_src_api_generated_docs_docs_postdocsdocidlock, web_src_api_generated_docs_docs_postdocsdocidlockrelease, web_src_api_generated_docs_docs_postdocsdocidversionsrevrestore, web_src_api_generated_docs_docs_putdocsdocid (+84 more)

### Community 9 - "go_pkg_context"
Cohesion: 0.06
Nodes (32): buildEmailMessage(), sendSMTPChannel(), splitRecipients(), TestBuildEmailMessage_SanitizesSubjectNewlines(), TestSendSMTPChannel_MissingConfig(), TestSplitRecipients(), redactURLError(), normalizeEnabledColumn() (+24 more)

### Community 10 - "package.json"
Cohesion: 0.04
Nodes (46): clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks (+38 more)

### Community 11 - "react"
Cohesion: 0.02
Nodes (134): motion, react, web_src_api_generated_changes_changes_postchangesbulkresolve, web_src_api_generated_changes_changes_usegetchanges, web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey, web_src_api_generated_compliance_compliance_postcompliancerules (+126 more)

### Community 12 - "Errorf"
Cohesion: 0.07
Nodes (17): Handler, newToken(), sanitize(), Handler, diffToSpec(), Handler, Handler, Handler (+9 more)

### Community 13 - "ConnectorEditPage.tsx"
Cohesion: 0.07
Nodes (27): RFC-3339, web_src_api_generated_connectors_connectors_getgetconnectorsquerykey, web_src_api_generated_connectors_connectors_postconnectors, web_src_api_generated_connectors_connectors_postconnectorsconnectoridsync, web_src_api_generated_connectors_connectors_postconnectorsconnectoridtest, web_src_api_generated_connectors_connectors_putconnectorsconnectorid, web_src_api_generated_connectors_connectors_usegetconnectorsconnectorid, web_src_api_generated_connectors_connectors_usegetconnectorsschema (+19 more)

### Community 14 - "vitest"
Cohesion: 0.03
Nodes (62): axios, i18next, msw, @testing-library/react, vitest, web_src_api_model_index_attentionpage, web_src_api_model_index_runbookpage, TODO: fold into docs/openapi.yaml and regenerate via `npm run gen:api` (+54 more)

### Community 15 - "ServiceSnapshot"
Cohesion: 0.04
Nodes (14): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, Connector, runTransformers(), TestRunTransformersUnknownCategoryIsNoop(), registryTestRefresher, actionConnector (+6 more)

### Community 16 - "go_pkg_time"
Cohesion: 0.08
Nodes (19): versionSections(), ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold(), TemplateVersionSection, contains(), searchString(), go_pkg_database_sql (+11 more)

### Community 17 - "WebSocketProvider.tsx"
Cohesion: 0.05
Nodes (51): Client dispatch model, Envelope, Mock emitter (frontend-first), Naming convention, Reconnect behavior, Transport, WiseLabz WebSocket Contract (`/ws`), AXIOS_INSTANCE (+43 more)

### Community 18 - "@tanstack/react-query"
Cohesion: 0.05
Nodes (58): Frontend, 7. `quality.finding.created` and `quality.findings.changed`, match-sorter, react-router-dom, @tanstack/react-query, zustand, setMfaEnrollmentRequiredHandler(), web_src_api_generated_connectors_connectors (+50 more)

### Community 19 - "rowScanner"
Cohesion: 0.07
Nodes (25): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), Store, scanBackupRun(), scanDoc() (+17 more)

### Community 20 - "go_pkg_testing"
Cohesion: 0.04
Nodes (25): NewHandler(), Handler, newTestHandler(), TestCreate(), TestGetNotFound(), TestListMutuallyExclusiveFilters(), TestUpdateAndDelete(), TestSchemaMatchesConfig() (+17 more)

### Community 21 - "Manager"
Cohesion: 0.09
Nodes (14): NewHandler(), cron.EntryID, Manager, JobName(), LogPartial(), NewManager(), Store, Store (+6 more)

### Community 22 - "NewEngine"
Cohesion: 0.25
Nodes (24): NewEngine(), newEngineTestStore(), seedEngineConnector(), seedEngineTemplate(), TestGenerateFromSnapshotIncludesDependencies(), TestGenerateFromTemplateReturnsVersionPersistenceError(), TestGenerateFromTemplateStillPersists(), TestMatchingConnectorsEmptyAppliesToIsWildcard() (+16 more)

### Community 23 - "New"
Cohesion: 0.22
Nodes (19): newFakeHealthStore(), TestJobHealthOkToFailingNotifiesOnce(), TestJobHealthPanicCountsAsFailure(), TestJobHealthPersistsAcrossRestart(), TestJobHealthWithoutStoreDoesNothing(), New(), TestAddJobInvalidExpression(), TestAddJobRegistersAndFires() (+11 more)

### Community 24 - "Handler"
Cohesion: 0.18
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 25 - "ErrorWithDetails"
Cohesion: 0.08
Nodes (22): sanitizeUser(), Handler, mustHashDummyPassword(), Handler, randomOIDCToken(), instanceAdminRoleFor(), validTargetType(), Handler (+14 more)

### Community 26 - "Get"
Cohesion: 0.11
Nodes (14): Handler, isWritableField(), validateConfigPushRequest(), capitalize(), Handler, WriteElevationError(), ConfigPusher, ValidateCompositeRef() (+6 more)

### Community 27 - "DocRecord"
Cohesion: 0.06
Nodes (20): seedDelivery(), fetchAllDocs(), fileName(), slugify(), existingIDs(), nilToStr(), docSearchWhere(), escapeLike() (+12 more)

### Community 28 - "ExportToFile"
Cohesion: 0.11
Nodes (41): Export(), ExportToFile(), newTestStore(), TestExportIncludesRecordsBeyondAPage(), TestExportRedactsConnectorSecrets(), TestExportToFile(), TestExportToFileCreatesDirectory(), TestExportToFileDirNotWritable() (+33 more)

### Community 29 - "WiseLabz — Architecture & Technical Decisions"
Cohesion: 0.04
Nodes (45): 0001 — Lab-mutating operation boundaries, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+37 more)

### Community 30 - "fixtures.ts"
Cohesion: 0.06
Nodes (42): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+34 more)

### Community 31 - "dispatcher_test.go"
Cohesion: 0.16
Nodes (53): TestExpireAlertsOnceNoExpiredAlertsIsNoop(), TestExpireAlertsOnceNotifiesViaDispatcher(), newTestLifecycle(), TestLifecycleManagerOrderedShutdown(), TestLifecycleManagerShutdownCancelsWorkContext(), testLogger(), expireAlertsOnce(), NewDispatcher() (+45 more)

### Community 32 - "icons.tsx"
Cohesion: 0.05
Nodes (60): @radix-ui/react-popover, web_src_api_generated_chat_chat, web_src_api_generated_chat_chat_getgetchatconversationsidquerykey, web_src_api_generated_chat_chat_getgetchatconversationsquerykey, web_src_api_generated_chat_chat_postchatconversations, web_src_api_generated_chat_chat_postchatconversationsidmessages, web_src_api_generated_chat_chat_usegetchatconversations, web_src_api_generated_chat_chat_usegetchatconversationsid (+52 more)

### Community 33 - "DashboardPage.tsx"
Cohesion: 0.04
Nodes (85): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress`, 3. `sync.complete`, 4. `change.detected` (+77 more)

### Community 34 - "NewService"
Cohesion: 0.18
Nodes (18): TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed(), NewService(), TestConcurrentIssuePairUniqueTokenIDs(), TestElevationExpired(), TestElevationRequiresOwner() (+10 more)

### Community 35 - "DecodeKey"
Cohesion: 0.10
Nodes (22): ProviderConfig, Handler, Handler, primaryProviderConfig(), Handler, DecodeKey(), Decrypt(), DeriveKey() (+14 more)

### Community 36 - "truenas/tables.go"
Cohesion: 0.13
Nodes (40): buildDatasets(), buildDisks(), buildInterfaces(), buildNFSShares(), buildPools(), buildReplicationTasks(), buildServices(), buildSMBShares() (+32 more)

### Community 37 - "response.go"
Cohesion: 0.08
Nodes (26): Handler, Handler, Cursor(), DecodeCursor(), EncodeCursor(), T, NextCursor(), TestCursorRequestModes() (+18 more)

### Community 38 - "RunMigrations"
Cohesion: 0.08
Nodes (46): main(), Handler, newScratchStore(), SyncDocEmbeddings(), TestSyncDocEmbeddingsKeepsOldRowsWhenEmbedFails(), TestRetrieveUsesCacheAndSyncInvalidates(), TestUpsertBackupSchedulePostgresParity(), OpenDB() (+38 more)

### Community 39 - "home_assistant/tables.go"
Cohesion: 0.09
Nodes (39): jsonType(), TestAttributeCatalogCoversEmittedKeys(), unavailable(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations() (+31 more)

### Community 40 - "dependencies"
Cohesion: 0.05
Nodes (42): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+34 more)

### Community 41 - "NewChecker"
Cohesion: 0.06
Nodes (72): contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity, Rule (+64 more)

### Community 42 - "share_links_test.go"
Cohesion: 0.23
Nodes (35): GrantConnectorRole(), instanceAdminRole(), NewUser(), TestListFiltersGrantsBeforePagination(), Handler, newTestHandler(), asUser(), createTestShareLink() (+27 more)

### Community 43 - "Connector"
Cohesion: 0.19
Nodes (6): TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), TestBuildHostOverrideTableMalformedCases(), TestBuildHostOverrideTableValidOverrides(), Connector

### Community 44 - "docker_test.go"
Cohesion: 0.06
Nodes (44): buildDockerTLSConfig(), newDockerClient(), newTCPDockerClient(), init(), generateSelfSignedCert(), generateSSHHostKey(), serveOneHTTPExchange(), serveSSHDockerConn() (+36 more)

### Community 45 - "Store"
Cohesion: 0.06
Nodes (17): changeServiceIDs(), Store, placeholders(), changeFilterClause(), AlertRecord, ChangeRecord, Store, scanAlert() (+9 more)

### Community 46 - "lifecycleDeps"
Cohesion: 0.20
Nodes (8): newLifecycleManager(), context.CancelFunc, golang.org/x/sync/errgroup.Group, net/http.Server, sync/atomic.Bool, lifecycleDeps, lifecycleManager, ReadyState

### Community 47 - "NewMalformedResponseError"
Cohesion: 0.12
Nodes (36): NewMalformedResponseError(), buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP(), groupNames() (+28 more)

### Community 48 - "portainer/tables.go"
Cohesion: 0.12
Nodes (33): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEnvironmentTable(), buildStackTable(), cell(), containerRows(), environmentNames(), environmentTypeName() (+25 more)

### Community 49 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (32): statusInfo, unavailable(), upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo() (+24 more)

### Community 50 - "src/theme.ts"
Cohesion: 0.15
Nodes (23): @fontsource/space-mono, @fontsource-variable/space-grotesk, ColorMode, commit(), load(), Persisted, PRESETS_FONTS, ThemeState (+15 more)

### Community 51 - "home_assistant_test.go"
Cohesion: 0.22
Nodes (19): Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities(), TestFetchConfigIsRequestedOnce() (+11 more)

### Community 52 - "middleware.go"
Cohesion: 0.15
Nodes (15): APIKeyChecker, AuditRecorder, contextKey, elevationError, UserStatusChecker, AuthMiddleware(), elevationFailureReason(), extractBearerToken() (+7 more)

### Community 53 - "NewStore"
Cohesion: 0.13
Nodes (31): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+23 more)

### Community 54 - "User"
Cohesion: 0.12
Nodes (16): discordPayload(), sendDiscordChannel(), sendGenericWebhookChannel(), sendSlackChannel(), slackPayload(), webhookPayload(), findChannel(), findRoute() (+8 more)

### Community 55 - "ConnectorRecord"
Cohesion: 0.15
Nodes (13): ConnectorRecord, Store, scanConnector(), scanConnectorRows(), nullInt64ToIntPtr(), nullStrToStr(), scanSyncRun(), connectorWithRole (+5 more)

### Community 56 - "unifi/tables.go"
Cohesion: 0.17
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+21 more)

### Community 57 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.06
Nodes (28): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, Manifest, checksum, and verification, 1. Every export gets a manifest and a checksum, 2. Verifying a backup actually restores, 3. Restoring for real (+20 more)

### Community 58 - "logging_test.go"
Cohesion: 0.18
Nodes (15): Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken(), TestLoggerRedactsWSTicket(), TestGetRequestIDMissing(), TestRecovererPassThrough(), TestRecovererReturns500OnPanic() (+7 more)

### Community 59 - "net/http.ResponseWriter"
Cohesion: 0.09
Nodes (16): updateUserRequest, webAuthnFlow, Handler, writeUserWriteError(), Handler, Handler, cron.EntryID, Handler (+8 more)

### Community 60 - "settings.mock.ts"
Cohesion: 0.09
Nodes (25): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role, web_src_api_model_index_session (+17 more)

### Community 61 - "traefik/tables.go"
Cohesion: 0.11
Nodes (31): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable(), cell() (+23 more)

### Community 62 - "rewritePlaceholders"
Cohesion: 0.10
Nodes (14): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), database/sql.Result, database/sql.Row, database/sql.Rows (+6 more)

### Community 63 - "ServiceDetailPage.tsx"
Cohesion: 0.04
Nodes (57): ADR-0001, ADR-0003, 1. `service.status`, web_src_api_generated_connectors_connectors_postconnectorsconnectoridconfigpush, web_src_api_generated_connectors_connectors_postconnectorsconnectoridhealth, web_src_api_generated_connectors_connectors_postconnectorsconnectoridrestart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstop (+49 more)

### Community 64 - "newTestHandler"
Cohesion: 0.05
Nodes (77): mockElevateOIDCServer, secondFactorInput, virtualAuthenticator, NewHandler(), doJSON(), testHandler, req(), TestChangePassword() (+69 more)

### Community 65 - "routerDeps"
Cohesion: 0.10
Nodes (33): routerDeps, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountConnectorRoutes(), chi.Router (+25 more)

### Community 66 - "main"
Cohesion: 0.11
Nodes (23): main(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), Embedder, EmbedRegistry (+15 more)

### Community 67 - "NewRegistry"
Cohesion: 0.13
Nodes (27): Provider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail(), TestSuggestWithFallbackFirstProviderSucceeds(), TestSuggestWithFallbackNoProviders() (+19 more)

### Community 68 - "newTestHandler"
Cohesion: 0.07
Nodes (63): AssertMatchesSpec(), loadSpec(), specPath(), actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews() (+55 more)

### Community 69 - "Register"
Cohesion: 0.13
Nodes (25): init(), init(), init(), init(), init(), init(), init(), init() (+17 more)

### Community 70 - "GetTypeSchema"
Cohesion: 0.07
Nodes (42): TestBackupExportRedactsSecrets(), catalog(), TestDiagnosticsRedactsSecrets(), TestRegisteredSchema(), TestSchemaConfigValidation(), TestAllConnectorImplementationsRegister(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys() (+34 more)

### Community 71 - "Connector"
Cohesion: 0.12
Nodes (10): NewAuthError(), Connector, apiMessage(), controllerName(), countByKind(), statusError(), unavailable(), Connector (+2 more)

### Community 72 - "Service"
Cohesion: 0.16
Nodes (13): Claims, ElevationClaims, ElevationToken, IssuePairOptions, MFAClaims, MFATicket, Service, TokenPair (+5 more)

### Community 73 - "Config"
Cohesion: 0.11
Nodes (21): NewWebAuthnService(), TestWebAuthnRPConfig(), WebAuthnRPConfig(), Config, LogSettings, IsSSHRemote(), AISettings, AuthSettings (+13 more)

### Community 74 - "Store"
Cohesion: 0.18
Nodes (25): connectorIDs(), docIDs(), exportDocs(), exportTemplates(), exportWithin(), AIConfigSummary, Import(), importBundle() (+17 more)

### Community 75 - "handlers.ts"
Cohesion: 0.07
Nodes (27): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+19 more)

### Community 76 - "Connector"
Cohesion: 0.09
Nodes (11): init(), ConfigField, Connector, TestBuildInterfaceTableAttributes(), buildGatewayTable(), buildInterfaceTable(), primaryGatewayName(), wanInterfaceName() (+3 more)

### Community 77 - "NewEngine"
Cohesion: 0.15
Nodes (25): RequestedFields(), TestRequestedFields(), TestWantsField(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector() (+17 more)

### Community 78 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 79 - "SuggestRequest"
Cohesion: 0.09
Nodes (14): claudeProvider, ollamaEmbedder, openAICompatibleProvider, StubProvider, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet() (+6 more)

### Community 80 - "net/http.Request"
Cohesion: 0.08
Nodes (25): oidcElevateFlow, clearFlowCookie(), clearOIDCFlowCookie(), clearOIDCElevateFlowCookie(), readOIDCElevateFlowCookie(), setOIDCElevateFlowCookie(), Handler, newOIDCUser() (+17 more)

### Community 81 - "createUser"
Cohesion: 0.25
Nodes (14): ContextWithAPIKeyRestriction(), seedAlert(), TestListAttentionItems(), seedChange(), TestListChanges(), TestListConnectors(), enableFakeEmbedding(), TestSearchDocs() (+6 more)

### Community 82 - "fetch_test.go"
Cohesion: 0.19
Nodes (14): entityKinds(), sectionByTitle(), TestConfigPushV5(), TestFetchAuthFailureReturnsPlaceholderSnapshot(), TestFetchBothVersions(), TestFetchDegradesPerSection(), TestRestartUnsupportedOnV5(), TestStartStopV5() (+6 more)

### Community 83 - "timeline.ts"
Cohesion: 0.14
Nodes (17): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+9 more)

### Community 84 - "middleware_test.go"
Cohesion: 0.13
Nodes (19): fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, RequireElevation(), RequireInstanceAdmin(), assertElevationAuditCalls(), boolLabel(), contextWithInstanceAdmin() (+11 more)

### Community 85 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 86 - "devDependencies"
Cohesion: 0.08
Nodes (24): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+16 more)

### Community 87 - "AppearancePage.tsx"
Cohesion: 0.14
Nodes (21): MotionProvider(), AppearancePage(), ChoiceGroup(), ToggleRow(), AppearanceState, apply(), Contrast, css() (+13 more)

### Community 88 - "AuthedUser"
Cohesion: 0.12
Nodes (29): TestEmbeddedSPAWithoutFrontendBuild(), NewHandler(), TestCreate(), TestList(), TestRevoke(), AuthedUser(), JWTService(), Token() (+21 more)

### Community 89 - "go_pkg_os"
Cohesion: 0.07
Nodes (26): keys(), TestOpenDBEnablesSQLiteForeignKeys(), TestOpenDBSetsSQLiteDurabilityPragmas(), TestPoolConfigWithDefaults(), TestWithinTransactionRollsBack(), TestSnapshotAttributesRoundTripPostgres(), TestSnapshotAttributesRoundTripSQLite(), testSnapshotWithAttributes() (+18 more)

### Community 90 - "Hub"
Cohesion: 0.09
Nodes (14): decodeBulkRequest(), Handler, loggablePath(), loggableQuery(), Sanitize(), TestSanitize(), Hub, bulkRequest (+6 more)

### Community 91 - "NotificationRecord"
Cohesion: 0.26
Nodes (4): Dispatcher, NotificationRecord, Store, scanNotification()

### Community 92 - "apikey_scope.go"
Cohesion: 0.16
Nodes (12): APIKeyRestriction, testAPIKeyChecker, APIKeyRestrictionFromContext(), ClampConnectorRole(), isSafeMethod(), TestClampConnectorRole(), treatAsSafeFromContext(), TreatAsSafeMethod() (+4 more)

### Community 93 - "Compare"
Cohesion: 0.15
Nodes (18): configPushLanded(), driftDescription(), Checker, highestDriftSeverity(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare() (+10 more)

### Community 94 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 95 - "export_test.go"
Cohesion: 0.24
Nodes (15): Exporter, IsGeneratedName(), NewExporter(), pruneStale(), RunExportOnce(), newTestStore(), readFile(), TestDocExportDefaultCronExprIsValid() (+7 more)

### Community 96 - "gitTarget"
Cohesion: 0.23
Nodes (7): commitMessage(), commitResult, gitTarget, git.Repository, github.com/go-git/go-git/v5/plumbing.Hash, github.com/go-git/go-git/v5/plumbing/object.Signature, github.com/go-git/go-git/v5/plumbing.ReferenceName

### Community 97 - "Store"
Cohesion: 0.27
Nodes (4): decodeConnectorIDs(), APIKey, Store, scanAPIKey()

### Community 98 - "NewHTTPClient"
Cohesion: 0.12
Nodes (16): newConnector(), Connector, newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), intConfig(), newConnector(), NewHTTPClient() (+8 more)

### Community 99 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 100 - ".Fetch"
Cohesion: 0.10
Nodes (13): ServiceDependency, WantsField(), environmentDependencies(), putMetadata(), unavailable(), agentEnabled(), Connector, poolDependencies() (+5 more)

### Community 101 - "Deps"
Cohesion: 0.23
Nodes (18): registerListAttentionItems(), registerListChanges(), jsonResult(), registerListConnectors(), registerSearchDocs(), connectorAllowSet(), registerListFindings(), NewHTTPHandler() (+10 more)

### Community 102 - "sshStdioConn"
Cohesion: 0.10
Nodes (13): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), sshStdioConn, bufio.ReadWriter, golang.org/x/crypto/ssh.Client, golang.org/x/crypto/ssh.ClientConfig, golang.org/x/crypto/ssh.Session (+5 more)

### Community 103 - "runRestore"
Cohesion: 0.21
Nodes (13): confirm(), formatCounts(), main(), runRestore(), runVerify(), newSeededStore(), TestRunRestoreImportsIntoConfiguredDatabase(), TestRunRestoreRejectsCorruptedBundle() (+5 more)

### Community 104 - "connector_permission.go"
Cohesion: 0.19
Nodes (9): auditConnectorGrantDiffJSON(), getConnectorGrant(), ConnectorGrantDiff, Store, highestConnectorRole(), listOIDCConnectorGrants(), scanConnectorGrants(), upsertConnectorGrant() (+1 more)

### Community 105 - "router.go"
Cohesion: 0.08
Nodes (30): wsRoleLabel(), go_pkg_github_com_wiselabz_wiselabz_internal_api, go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts, go_pkg_github_com_wiselabz_wiselabz_internal_api_apikeys, go_pkg_github_com_wiselabz_wiselabz_internal_api_attention, go_pkg_github_com_wiselabz_wiselabz_internal_api_auth, go_pkg_github_com_wiselabz_wiselabz_internal_api_changes, go_pkg_github_com_wiselabz_wiselabz_internal_api_chat (+22 more)

### Community 106 - "Handler"
Cohesion: 0.15
Nodes (9): applyConnectorScalarUpdates(), configRequestField(), Handler, parseScheduleUpdates(), validateConnectorConfig(), validateRotationFields(), writeConfigRejection(), FieldError (+1 more)

### Community 107 - "chat/chat.go"
Cohesion: 0.18
Nodes (14): buildPrompt(), TestBuildPrompt(), cosineSimilarity(), Match, packVector(), Retrieve(), SplitSections(), TestCosineSimilarityRanksClosestVectorHighest() (+6 more)

### Community 108 - "IsSecureRequest"
Cohesion: 0.30
Nodes (10): ClientIP(), hostOnly(), IsSecureRequest(), isTrustedProxy(), TestClientIPRejectsNonIPForwardedFor(), TestClientIPTrustedPeerUsesForwardedFor(), TestClientIPUntrustedPeerIgnoresHeaders(), TestIsSecureRequestTLS() (+2 more)

### Community 109 - "config_test.go"
Cohesion: 0.15
Nodes (19): runHealthcheck(), Load(), TestAccessTokenTTLDuration(), TestDocExportGitValidate(), TestLoadDefaults(), TestLoadEnvOverride(), TestLoadEnvOverrideAllFields(), TestLoadEnvOverrideDocExportGitSSH() (+11 more)

### Community 110 - "httpx/retry_test.go"
Cohesion: 0.26
Nodes (17): retryable(), RetryTransport(), do(), fail(), status(), TestRetryTransportDisabled(), TestRetryTransportDoesNotRetry(), TestRetryTransportGivesUpAfterMaxRetries() (+9 more)

### Community 111 - "ws.ts"
Cohesion: 0.11
Nodes (17): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+9 more)

### Community 112 - "truenas_test.go"
Cohesion: 0.24
Nodes (17): Connector, newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchIsStableAcrossCalls() (+9 more)

### Community 113 - "diagnostics/diagnostics.go"
Cohesion: 0.22
Nodes (17): CheckHealth(), Collect(), collectVersions(), newTestStore(), TestCheckHealthReportsDegradedOnClosedDB(), TestCollectIncludesHealthVersionsAndSchedule(), TestCollectListsRecentFailures(), TestCollectRedactsConnectorSecrets() (+9 more)

### Community 114 - "time.Duration"
Cohesion: 0.16
Nodes (7): sleep(), Database, Server, time.Duration, RetryPolicy, retryTransport, PoolConfig

### Community 115 - "Runner"
Cohesion: 0.16
Nodes (7): cron.EntryID, Runner, cron.Cron, HealthStore, jobEntry, JobInfo, Notifier

### Community 116 - "ws/ws_test.go"
Cohesion: 0.20
Nodes (17): NewHub(), normalizeOrigin(), assertEnvelope(), setupWSConnection(), TestBroadcastFullQueueDoesNotBlock(), TestBroadcastToUserAfterUpgrade(), TestClientCloseDisconnect(), TestDocLockEventBroadcast() (+9 more)

### Community 117 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 118 - "Handler"
Cohesion: 0.24
Nodes (7): NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), Handler, RuleEvaluator

### Community 119 - "traefik_test.go"
Cohesion: 0.11
Nodes (32): AllowLoopbackForTest(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath() (+24 more)

### Community 120 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 121 - "ReportsPage.tsx"
Cohesion: 0.11
Nodes (19): web_src_api_generated_reports_reports, web_src_api_generated_reports_reports_deletereportsdefinitionsreportdefinitionid, web_src_api_generated_reports_reports_getgetreportsdefinitionsquerykey, web_src_api_generated_reports_reports_getgetreportsquerykey, web_src_api_generated_reports_reports_postreportsdefinitions, web_src_api_generated_reports_reports_postreportsdefinitionsreportdefinitionidrun, web_src_api_generated_reports_reports_putreportsdefinitionsreportdefinitionid, web_src_api_generated_reports_reports_usegetreports (+11 more)

### Community 122 - "git_internal_test.go"
Cohesion: 0.14
Nodes (16): gitAuth(), Exporter, installHTTPS(), SetBeforePushForTest(), TestCommitMessage(), TestGitAuthHTTPSNoToken(), TestGitAuthHTTPSToken(), TestGitAuthSSH() (+8 more)

### Community 123 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 124 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 125 - "testApp"
Cohesion: 0.23
Nodes (9): testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty(), TestBackupRoutesRequireOperatorRole(), TestBackupScheduleGetDefaults(), TestBackupScheduleUpdate(), TestBackupScheduleUpdateDoesNotLeakSchedulerJobs() (+1 more)

### Community 126 - "ChangeDetailPage.tsx"
Cohesion: 0.14
Nodes (14): web_src_api_generated_changes_changes_getgetchangeschangeidquerykey, web_src_api_generated_changes_changes_postchangeschangeidack, web_src_api_generated_changes_changes_postchangeschangeidaiupdate, web_src_api_generated_changes_changes_postchangeschangeiddismiss, web_src_api_generated_changes_changes_postchangeschangeidexplain, web_src_api_generated_changes_changes_usegetchangeschangeid, ChangeDetailPage, DiffViewer() (+6 more)

### Community 127 - "changes/handlers_test.go"
Cohesion: 0.30
Nodes (14): NewHandler(), Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound() (+6 more)

### Community 128 - "InstanceAdminFromContext"
Cohesion: 0.20
Nodes (7): contextWithShareLink(), Handler, newShareToken(), shareLinkFromContext(), InstanceAdminFromContext(), shareLinkNode, shareLinkScope

### Community 129 - "Handler"
Cohesion: 0.24
Nodes (6): definition(), record(), reportJSON(), valid(), Handler, input

### Community 130 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 131 - "go_pkg_encoding_base64"
Cohesion: 0.09
Nodes (22): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), Schema(), schemaFor() (+14 more)

### Community 132 - "templatefuncs.go"
Cohesion: 0.17
Nodes (12): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+4 more)

### Community 133 - "time.Time"
Cohesion: 0.08
Nodes (28): digestDue(), formatDigest(), Dispatcher, TestDigestDue(), connectorFilter(), Store, time.Time, fakeRefresherConnector (+20 more)

### Community 134 - "Engine"
Cohesion: 0.21
Nodes (7): Engine, dedupKey(), matchReason(), TemplateFuncs(), GenerateResult, renderResult, text/template.FuncMap

### Community 135 - "HashToken"
Cohesion: 0.14
Nodes (14): setRefreshCookie(), factorJSON(), Handler, GenerateRecoveryCodes(), GenerateTOTPSecret(), NormalizeRecoveryCode(), randomRecoveryChars(), TestGenerateRecoveryCodesAreUniqueAndFormatted() (+6 more)

### Community 136 - "pagination_contract_test.go"
Cohesion: 0.21
Nodes (13): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_go_ast (+5 more)

### Community 137 - "newTestHandler"
Cohesion: 0.22
Nodes (13): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), NewHandler(), Handler, newTestHandler() (+5 more)

### Community 138 - "net/http.Client"
Cohesion: 0.06
Nodes (14): Connector, openAIEmbedder, Connector, CheckStatus(), TestCheckStatus(), TestBuildPeerTableAttributes(), TestBuildPolicyTableAttributes(), buildPeerTable() (+6 more)

### Community 139 - "gitFixture"
Cohesion: 0.41
Nodes (6): newGitFixture(), TestGitExportLifecycle(), TestGitExportPushRejectionReturnsError(), TestGitExportRefusesForeignDirectory(), gitFixture, github.com/go-git/go-git/v5/plumbing/object.Commit

### Community 140 - "render_test.go"
Cohesion: 0.31
Nodes (13): RenderHTML(), RenderMarkdown(), sampleData(), TestRenderHTML_EscapesDocTitles(), TestRenderHTML_SectionUnavailable(), TestRenderHTML_Truncated(), TestRenderMarkdown_Golden(), TestRenderMarkdown_SectionUnavailable() (+5 more)

### Community 141 - "log/slog.Logger"
Cohesion: 0.20
Nodes (9): newLogger(), Dispatcher, RunDeliveryRetries(), Store, RunDocLockSweep(), runDocLockSweep(), Engine, log/slog.Logger (+1 more)

### Community 142 - "SnapshotEntity"
Cohesion: 0.12
Nodes (18): SnapshotEntity, SnapshotSection, TestBuildContainerTableAttributes(), buildContainerTable(), ReadBody(), TestReadBodyLimit(), TestBuildHostsTableAttributes(), buildHostsTable() (+10 more)

### Community 143 - "Contributing to WiseLabz"
Cohesion: 0.15
Nodes (13): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+5 more)

### Community 144 - "net/http.Handler"
Cohesion: 0.21
Nodes (9): PermissionChecker, CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders(), SecurityHeaders(), TestSecurityHeaders(), RequirePermission() (+1 more)

### Community 145 - "diagram.go"
Cohesion: 0.26
Nodes (12): entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks(), relatedEntities(), EntityLink (+4 more)

### Community 146 - "JobHealthRecord"
Cohesion: 0.29
Nodes (4): JobHealthRecord, Store, scanJobHealth(), fakeHealthStore

### Community 147 - "Engine"
Cohesion: 0.17
Nodes (6): NewHandler(), Engine, sync.Map, AlertNotifier, DocRegenerator, QualityChecker

### Community 148 - "docs/handlers_test.go"
Cohesion: 0.19
Nodes (16): NewHandler(), TestAISuggestInvalidJSON(), TestByServiceNoDocsYet(), TestGenerate(), TestGetLockNoneHeld(), TestGetRootIsSynthetic(), TestGetUnknownIDFallsBackToServicePlaceholder(), TestListEmpty() (+8 more)

### Community 150 - "Store"
Cohesion: 0.23
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 151 - "Decision"
Cohesion: 0.17
Nodes (11): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 153 - "scripts"
Cohesion: 0.17
Nodes (12): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+4 more)

### Community 154 - "seedHealthTestConnector"
Cohesion: 0.31
Nodes (10): testApp, registerHealthFakeType(), seedHealthTestConnector(), TestConnectorsHealthDegraded(), TestConnectorsHealthDoesNotCreateSnapshot(), TestConnectorsHealthOffline(), TestConnectorsHealthOnline(), TestConnectorsHealthRecordsTimeSeriesRow() (+2 more)

### Community 156 - "Connector"
Cohesion: 0.21
Nodes (5): TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), Connector

### Community 157 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 158 - "WiseLabz Connector Guide"
Cohesion: 0.18
Nodes (11): Conventions, Dependencies, Getting your connector merged, Health checks vs. sync, Keeping snapshots stable, Session-based and multi-flavour APIs, Testing without a real instance, The Connector interface (+3 more)

### Community 159 - "Product"
Cohesion: 0.18
Nodes (10): Accessibility & Inclusion, Anti-references, Brand Personality, Design Principles, Locked frontend direction (planning session, 2026-06; revised 2026-09), Product, Product decisions (pre-planning, v1), Product Purpose (+2 more)

### Community 160 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 161 - "connectors_hardening_test.go"
Cohesion: 0.25
Nodes (8): testApp, init(), TestConnectorsCreateAcceptsValidConfig(), TestConnectorsCreateRejectsInvalidEnum(), TestConnectorsCreateRejectsMalformedConfig(), TestConnectorsSyncAcceptsFieldsHint(), TestConnectorsUpdateRejectsMalformedConfig(), waitForSyncRuns()

### Community 162 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 163 - ".call"
Cohesion: 0.44
Nodes (6): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture

### Community 164 - "Changelog"
Cohesion: 0.20
Nodes (9): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features, Unreleased (+1 more)

### Community 165 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 166 - "retention/retention_test.go"
Cohesion: 0.61
Nodes (8): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupPrunesOldHealthChecks(), TestRunCleanupSkipsDisabledCategories()

### Community 167 - "apikey_scopes_test.go"
Cohesion: 0.47
Nodes (8): createKey(), testApp, newConnector(), TestAPIKeyCreateValidation(), TestAPIKeyDefaultsToFullScope(), TestConnectorRestrictedAPIKey(), TestReadOnlyAPIKey(), TestReadOnlyAPIKeyCapsConnectorRoleAtViewer()

### Community 168 - "ComplianceRuleRecord"
Cohesion: 0.36
Nodes (4): changedFields(), ComplianceRuleRecord, Store, scanComplianceRule()

### Community 169 - "RunbookRecord"
Cohesion: 0.44
Nodes (3): RunbookRecord, Store, scanRunbook()

### Community 170 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 171 - "api/auth/oidc.go"
Cohesion: 0.07
Nodes (24): StatusError, TestEmailDomainAllowed(), TestOIDCConnectorRolesForGroups(), TestOIDCRoleForGroups(), connectorRoleLess(), emailDomainAllowed(), oidcConnectorRolesForGroups(), oidcRoleForGroups() (+16 more)

### Community 172 - "TestComplianceRuleValidation"
Cohesion: 0.29
Nodes (7): badRegexMessage(), complianceCondition(), TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails()

### Community 173 - "Config"
Cohesion: 0.18
Nodes (14): Config, Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout() (+6 more)

### Community 174 - "ComputeWindow"
Cohesion: 0.39
Nodes (6): ComputeWindow(), TestComputeWindow_CappedAt31Days(), TestComputeWindow_ExactlyAtCap(), TestComputeWindow_FirstRun(), TestComputeWindow_ManualRunUsesLastScheduledWatermarkUnchanged(), TestComputeWindow_Watermark()

### Community 175 - "Cache"
Cohesion: 0.43
Nodes (5): Cache, New(), Cache[V], entry, V

### Community 176 - "Step by step"
Cohesion: 0.25
Nodes (8): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Step by step

### Community 177 - "WiseLabz"
Cohesion: 0.25
Nodes (8): Code of Conduct, Configuration, Contributing, Features, License, Quick start, Supported services, WiseLabz

### Community 178 - "RateLimit"
Cohesion: 0.29
Nodes (6): TestRateLimit(), RateLimit(), golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 179 - "webAuthnUser"
Cohesion: 0.33
Nodes (3): webAuthnUser, github.com/go-webauthn/webauthn/webauthn.Credential, github.com/google/uuid.UUID

### Community 180 - "pfsense.go"
Cohesion: 0.48
Nodes (5): buildGatewayTable(), buildInterfaceTable(), buildSystemContent(), primaryGatewayName(), wanInterfaceName()

### Community 181 - "scanMaintenanceWindow"
Cohesion: 0.48
Nodes (3): Store, scanMaintenanceWindow(), MaintenanceWindowRecord

### Community 182 - "computeNextRun"
Cohesion: 0.43
Nodes (5): TestComputeNextRun_BackoffNeverExceedsScheduleCadence(), TestComputeNextRun_FailureUsesBackoffSchedule(), TestComputeNextRun_ManualOnlyNeverSchedules(), TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(), computeNextRun()

### Community 183 - "Contributor Covenant Code of Conduct"
Cohesion: 0.29
Nodes (7): Attribution, Contributor Covenant Code of Conduct, Enforcement, Enforcement Responsibilities, Our Pledge, Our Standards, Scope

### Community 185 - "Audit Trail"
Cohesion: 0.29
Nodes (6): Audit Trail, Endpoint, Keyset (cursor) pagination, Retention, What's not recorded, What's recorded

### Community 186 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 187 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 188 - "Mermaid.tsx"
Cohesion: 0.47
Nodes (4): mermaid, cssVar(), Mermaid(), resolveColor()

### Community 189 - "RequireConnectorRole"
Cohesion: 0.50
Nodes (4): ConnectorRoleChecker, RequireConnectorRole(), TestRequireConnectorRole(), TestRequireConnectorRoleCheckerError()

### Community 190 - "golden_snapshot_test.go"
Cohesion: 0.60
Nodes (4): Store, mustCreateGoldenSnapshotConnector(), TestGetSnapshotByID(), TestPinGoldenSnapshotRoundTrip()

### Community 191 - "MfaEnrollDialog"
Cohesion: 0.50
Nodes (5): Sync flow, qrcode, MfaEnrollDialog(), close(), done()

### Community 192 - "@vitejs/plugin-react"
Cohesion: 0.40
Nodes (3): @tailwindcss/vite, vite, @vitejs/plugin-react

### Community 194 - "engine_maintenance_test.go"
Cohesion: 0.60
Nodes (5): driftingSnapshot(), setupMaintenanceTestConnector(), TestRunSyncExpiredMaintenanceWindowBehavesNormally(), TestRunSyncNoMaintenanceWindowBehavesNormally(), TestRunSyncSuppressesChangesDuringMaintenanceWindow()

### Community 195 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 196 - "main.tsx"
Cohesion: 0.21
Nodes (8): react-dom, App(), USE_MOCKS, web_src_index, bootstrap(), worker, enableMocks(), handlers

### Community 201 - "seedScopeFixture"
Cohesion: 0.60
Nodes (4): Store, seedScopeFixture(), TestListDocSectionEmbeddingsFiltersByGrant(), TestMergedAttentionItemsFiltersByGrant()

### Community 202 - "Enforcement Guidelines"
Cohesion: 0.40
Nodes (5): 1. Correction, 2. Warning, 3. Temporary Ban, 4. Permanent Ban, Enforcement Guidelines

### Community 203 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 207 - "MISSING — deferred & future frontend features"
Cohesion: 0.50
Nodes (3): Deferred from V1 (decided during planning), MISSING — deferred & future frontend features, Suggested-later (raised in build, not yet planned)

### Community 208 - "Saved Views"
Cohesion: 0.50
Nodes (3): Endpoints, Saved Views, Scope

## Knowledge Gaps
- **562 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `bulkResolveRequest`, `bulkResolveItemResult` (+557 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1266 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **18 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Store` connect `Store` to `Handler`, `testing.T`, `time.Time`, `Engine`, `newTestHandler`, `gitFixture`, `Errorf`, `go_pkg_time`, `Engine`, `docs/handlers_test.go`, `Handler`, `Manager`, `go_pkg_testing`, `NewEngine`, `rowScanner`, `ErrorWithDetails`, `DocRecord`, `ExportToFile`, `Handler`, `dispatcher_test.go`, `.call`, `.call`, `response.go`, `RunMigrations`, `retention/retention_test.go`, `NewChecker`, `share_links_test.go`, `Config`, `lifecycleDeps`, `Store`, `NewStore`, `User`, `net/http.ResponseWriter`, `rewritePlaceholders`, `newTestHandler`, `main`, `NewRegistry`, `engine_maintenance_test.go`, `NewEngine`, `createUser`, `AuthedUser`, `export_test.go`, `Deps`, `runRestore`, `Handler`, `chat/chat.go`, `diagnostics/diagnostics.go`, `Handler`, `testApp`, `changes/handlers_test.go`?**
  _High betweenness centrality (0.016) - this node is a cross-community bridge._
- **Why does `Hub` connect `Hub` to `NewRegistry`, `NewChecker`, `Handler`, `api/auth/oidc.go`, `Errorf`, `Config`, `log/slog.Logger`, `NewEngine`, `lifecycleDeps`, `time.Duration`, `Engine`, `docs/handlers_test.go`, `ws/ws_test.go`, `User`, `net/http.ResponseWriter`, `dispatcher_test.go`, `testApp`, `changes/handlers_test.go`?**
  _High betweenness centrality (0.009) - this node is a cross-community bridge._
- **Why does `gitTarget` connect `gitTarget` to `go_pkg_os`, `git_internal_test.go`, `export_test.go`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _562 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.019276775855723224 - nodes in this community are weakly interconnected._
- **Should `newDocTestStore` be split into smaller, more focused modules?**
  _Cohesion score 0.022589861138794763 - nodes in this community are weakly interconnected._
- **Should `App.tsx` be split into smaller, more focused modules?**
  _Cohesion score 0.04736842105263158 - nodes in this community are weakly interconnected._