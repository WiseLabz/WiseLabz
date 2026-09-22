# Graph Report - WiseLabz  (2026-09-21)

## Corpus Check
- 708 files · ~425,898 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 17 file(s) not represented in the graph (top: (none) 10, .toml 2, .example 1)

## Summary
- 5320 nodes · 16549 edges · 200 communities (176 shown, 24 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1303 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `61cf4fc3`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestApp
- newDocTestStore
- react
- RulesPage.tsx
- @tanstack/react-query
- testing.T
- cn
- package.json
- context.Context
- App.tsx
- DashboardPage.tsx
- go_pkg_context
- go_pkg_net_http
- go_pkg_testing
- ServiceSnapshot
- diagnostics/diagnostics.go
- ServiceDetailPage.tsx
- icons.tsx
- UserIDFromContext
- connector/connector.go
- dispatcher_test.go
- router.go
- net/http.ResponseWriter
- portainer/tables.go
- net/http.Request
- backup/backup.go
- fixtures.ts
- rowScanner
- share_links_test.go
- newTestHandler
- WiseLabz — Architecture & Technical Decisions
- RunMigrations
- NewMalformedResponseError
- home_assistant/tables.go
- NewEngine
- dependencies
- response.go
- NewStore
- home_assistant_test.go
- go_pkg_github_com_wiselabz_wiselabz_internal_connector
- adguardhome/tables.go
- middleware.go
- NewEngine
- traefik/tables.go
- useRole.ts
- routerDeps
- auth_test.go
- Errorf
- compliance/engine.go
- checker_test.go
- SuggestRequest
- IsSecureRequest
- Register
- SnapshotEntity
- unifi/tables.go
- Connector
- nilToStr
- settings.mock.ts
- rewritePlaceholders
- net/http.Handler
- newTestHandler
- Sanitize
- boolToInt
- testApp
- NewAuthError
- Config
- Connector
- Dispatcher
- handlers.ts
- unifi_test.go
- AppearancePage.tsx
- timeline.ts
- Store
- New
- newTestAppWithBackupDir
- notifications/handlers_test.go
- GetTypeSchema
- WiseLabz — Design Contract
- net/http.Client
- fetch_test.go
- Checker
- devDependencies
- SystemPage.tsx
- pagination_contract_test.go
- docker_test.go
- portainer_test.go
- ConnectorRecord
- adguardhome_test.go
- Registry
- Service
- Store
- middleware_test.go
- NewRegistry
- Configuration & Documentation Backup (Export/Import)
- Handler
- Handler
- Compare
- Handler
- ws/ws_test.go
- compilerOptions
- main
- Connector
- ValidateConfig
- docdiffmodel.ts
- ws.ts
- AuthMiddleware
- config_test.go
- handlers_contract_test.go
- chat/chat.go
- diagram.go
- sshStdioConn
- compilerOptions
- Handler
- vectorCache
- all.go
- Connector
- config_cmd_test.go
- changes/handlers_test.go
- Connector
- .batchDelete
- AppShell.tsx
- templates.fixtures.ts
- newTestHandler
- templatefuncs.go
- NotificationRecord
- scheduler_test.go
- Contributing to WiseLabz
- .call
- NewService
- Store
- Decision
- scripts
- Handler
- newSSHDockerClient
- Store
- Decision
- WiseLabz Connector Guide
- Product
- .call
- handlers_bulk_test.go
- connectors_health_test.go
- connectors_maintenance_test.go
- docs_test.go
- time.Time
- transform_test.go
- Changelog
- mockServiceWorker.js
- ComplianceRuleRecord
- ratelimit.go
- openapi_contract_test.go
- newDockerClient
- Runner
- Handler
- release-please-config.json
- connectors_hardening_test.go
- dashboard/handlers_test.go
- ShareLink
- Cache
- Step by step
- WiseLabz
- newTCPDockerClient
- serveSSHDockerConn
- RunCleanupOnce
- scanMaintenanceWindow
- computeNextRun
- Contributor Covenant Code of Conduct
- Audit Trail
- Bulk Review Actions
- Handler
- PULL_REQUEST_TEMPLATE.md
- walkCursorPages
- compliance_rules_test.go
- .doRequestV5
- Store
- Security Policy
- browser.ts
- RequireConnectorRole
- auth/handlers_test.go
- Enforcement Guidelines
- compose-smoke.sh
- testHandler
- ClassifyHealth
- RetentionSettings
- timeoutError
- MISSING — deferred & future frontend features
- Saved Views
- fields_test.go
- stubEmbedder
- dockerSSHAddr
- WiseLabz — v2 Backlog
- fakeDocRegenerator
- fakeQualityChecker
- tsconfig.json
- AGENTS.md
- setup-env.sh
- CHANGE_PROVENANCE.md
- vite-env.d.ts
- github.com/WiseLabz/wiselabz

## God Nodes (most connected - your core abstractions)
1. `newTestApp()` - 207 edges
2. `Errorf()` - 151 edges
3. `newDocTestStore()` - 118 edges
4. `Store` - 115 edges
5. `react` - 72 edges
6. `cn()` - 69 edges
7. `UserIDFromContext()` - 67 edges
8. `NewStore()` - 62 edges
9. `SnapshotEntity` - 62 edges
10. `@tanstack/react-query` - 57 edges

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

## Communities (200 total, 24 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.02
Nodes (150): templateBody, testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow() (+142 more)

### Community 1 - "newDocTestStore"
Cohesion: 0.03
Nodes (135): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+127 more)

### Community 2 - "react"
Cohesion: 0.04
Nodes (106): match-sorter, motion, @radix-ui/react-popover, react, react-i18next, web_src_api_generated_alerts_alerts, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve (+98 more)

### Community 3 - "RulesPage.tsx"
Cohesion: 0.03
Nodes (101): web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_usegetauthapikeys, web_src_api_generated_auth_auth_usegetauthproviders, web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey (+93 more)

### Community 4 - "@tanstack/react-query"
Cohesion: 0.03
Nodes (78): axios, i18next, msw, react-router-dom, @tanstack/react-query, @testing-library/react, vitest, customInstance() (+70 more)

### Community 5 - "testing.T"
Cohesion: 0.03
Nodes (109): TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens(), TestClaudeSuggestErrors(), TestClaudeSuggestMultipleContentBlocks(), TestOpenAICompatibleSuggest(), TestOpenAICompatibleSuggestErrors(), TestOIDCRedirectURL(), TestWriteConfigRejection() (+101 more)

### Community 6 - "cn"
Cohesion: 0.03
Nodes (89): clsx, tailwind-merge, web_src_api_generated_docs_docs_usegetdocstemplateschema, web_src_api_generated_notifications_notifications, web_src_api_generated_notifications_notifications_getgetnotificationsquerykey, web_src_api_generated_notifications_notifications_postnotificationsnotificationidread, web_src_api_generated_notifications_notifications_postnotificationsreadall, web_src_api_generated_notifications_notifications_usegetnotifications (+81 more)

### Community 7 - "package.json"
Cohesion: 0.03
Nodes (93): codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh (+85 more)

### Community 8 - "context.Context"
Cohesion: 0.04
Nodes (23): sanitizeSessions(), Connector, Connector, Store, existingIDs(), placeholders(), AlertRecord, ChangeRecord (+15 more)

### Community 9 - "App.tsx"
Cohesion: 0.04
Nodes (73): Sync flow, Client dispatch model, Envelope, Mock emitter (frontend-first), Naming convention, Reconnect behavior, Transport, WiseLabz WebSocket Contract (`/ws`) (+65 more)

### Community 10 - "DashboardPage.tsx"
Cohesion: 0.04
Nodes (82): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress`, 3. `sync.complete`, 4. `change.detected` (+74 more)

### Community 11 - "go_pkg_context"
Cohesion: 0.09
Nodes (15): StatusError, dashboardLayout, contains(), searchString(), go_pkg_context, go_pkg_database_sql, go_pkg_errors, go_pkg_fmt (+7 more)

### Community 12 - "go_pkg_net_http"
Cohesion: 0.08
Nodes (28): bulkSnoozeItemResult, bulkSnoozeRequest, versionSections(), TemplateVersionSection, shareLinkContextKey, go_pkg_crypto_rand, go_pkg_crypto_subtle, go_pkg_encoding_base64 (+20 more)

### Community 13 - "go_pkg_testing"
Cohesion: 0.07
Nodes (15): Handler, newTestHandler(), TestCreate(), TestGetNotFound(), TestListMutuallyExclusiveFilters(), TestUpdateAndDelete(), go_pkg_encoding_json, go_pkg_github_com_gorilla_websocket (+7 more)

### Community 14 - "ServiceSnapshot"
Cohesion: 0.03
Nodes (16): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, runTransformers(), TestRunTransformersAppliesInOrderAndStopsOnError(), TestRunTransformersUnknownCategoryIsNoop(), registryTestRefresher, actionConnector (+8 more)

### Community 15 - "diagnostics/diagnostics.go"
Cohesion: 0.06
Nodes (53): ProviderConfig, Handler, primaryProviderConfig(), Config, DecodeKey(), Decrypt(), DeriveKey(), Encrypt() (+45 more)

### Community 16 - "ServiceDetailPage.tsx"
Cohesion: 0.04
Nodes (53): ADR-0001, ADR-0003, RFC-3339, 1. `service.status`, web_src_api_generated_changes_changes_usegetchanges, web_src_api_generated_connectors_connectors, web_src_api_generated_connectors_connectors_getgetconnectorsquerykey, web_src_api_generated_connectors_connectors_postconnectors (+45 more)

### Community 17 - "icons.tsx"
Cohesion: 0.07
Nodes (50): web_src_api_generated_attention_attention, web_src_api_generated_attention_attention_usegetattention, web_src_api_generated_docs_docs, web_src_api_generated_docs_docs_getgetdocsdocidquerykey, web_src_api_generated_docs_docs_getgetdocsdocidversionsquerykey, web_src_api_generated_docs_docs_getgetdocstreequerykey, web_src_api_generated_docs_docs_postdocsdocidversionsrevrestore, web_src_api_generated_docs_docs_usegetdocsdocidversions (+42 more)

### Community 18 - "UserIDFromContext"
Cohesion: 0.07
Nodes (29): PermissionChecker, newToken(), sanitizeUser(), setRefreshCookie(), Handler, Handler, configRequestField(), parseScheduleUpdates() (+21 more)

### Community 19 - "connector/connector.go"
Cohesion: 0.05
Nodes (26): Connector, isTimeout(), NewServiceUnavailableError(), NewTimeoutError(), setHeaders(), TestValidateCustomURL(), tryParseEntities(), validateCustomURL() (+18 more)

### Community 20 - "dispatcher_test.go"
Cohesion: 0.12
Nodes (44): NewDispatcher(), deliveriesFor(), findDelivery(), newTestStore(), setChannelAndRoutingConfig(), setChannelConfig(), setWebhookConfig(), TestNotifyAlert_ConnectorCategoryAndIDFilterBothMatch() (+36 more)

### Community 21 - "router.go"
Cohesion: 0.07
Nodes (40): changePromptData(), stripPromptTags(), truncateUTF8(), bulkResolveItemResult, bulkResolveRequest, go_pkg_github_com_go_chi_chi_v5, go_pkg_github_com_wiselabz_wiselabz_internal_ai, go_pkg_github_com_wiselabz_wiselabz_internal_api (+32 more)

### Community 22 - "net/http.ResponseWriter"
Cohesion: 0.08
Nodes (25): AuditRecorder, Handler, isWritableField(), validateConfigPushRequest(), capitalize(), Handler, decodeBulkRequest(), Handler (+17 more)

### Community 23 - "portainer/tables.go"
Cohesion: 0.09
Nodes (38): WantsField(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), environmentDependencies(), isTimeout(), putMetadata(), buildEnvironmentTable(), buildStackTable() (+30 more)

### Community 24 - "net/http.Request"
Cohesion: 0.07
Nodes (17): noRedirect(), Handler, Handler, Handler, Handler, Handler, oidcProviderJSON(), boolToInt() (+9 more)

### Community 25 - "backup/backup.go"
Cohesion: 0.10
Nodes (38): connectorIDs(), docIDs(), Export(), exportDocs(), exportTemplates(), ExportToFile(), exportWithin(), AIConfigSummary (+30 more)

### Community 26 - "fixtures.ts"
Cohesion: 0.06
Nodes (41): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+33 more)

### Community 27 - "rowScanner"
Cohesion: 0.08
Nodes (23): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), changeFilterClause(), scanAlert(), scanChange() (+15 more)

### Community 28 - "share_links_test.go"
Cohesion: 0.17
Nodes (41): GrantConnectorRole(), instanceAdminRole(), NewUser(), TestListFiltersGrantsBeforePagination(), Handler, newTestHandler(), TestAISuggestInvalidJSON(), TestGetLockNoneHeld() (+33 more)

### Community 29 - "newTestHandler"
Cohesion: 0.11
Nodes (40): actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews(), TestActionMaintenanceLifecycle(), TestActionPermissions(), TestActionStoreFailures() (+32 more)

### Community 30 - "WiseLabz — Architecture & Technical Decisions"
Cohesion: 0.05
Nodes (40): 0001 — Lab-mutating operation boundaries, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+32 more)

### Community 31 - "RunMigrations"
Cohesion: 0.10
Nodes (36): main(), OpenDB(), TestOpenDBEnablesSQLiteForeignKeys(), TestOpenDBSetsSQLiteDurabilityPragmas(), newPostgresTestStore(), GetMigrationStatus(), newMigrator(), collectColumns() (+28 more)

### Community 32 - "NewMalformedResponseError"
Cohesion: 0.11
Nodes (38): NewMalformedResponseError(), buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP(), groupNames() (+30 more)

### Community 33 - "home_assistant/tables.go"
Cohesion: 0.10
Nodes (38): jsonType(), TestAttributeCatalogCoversEmittedKeys(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations(), buildOverview() (+30 more)

### Community 34 - "NewEngine"
Cohesion: 0.13
Nodes (31): Engine, NewEngine(), newEngineTestStore(), seedEngineConnector(), seedEngineTemplate(), TestGenerateFromSnapshotIncludesDependencies(), TestGenerateFromTemplateReturnsVersionPersistenceError(), TestGenerateFromTemplateStillPersists() (+23 more)

### Community 35 - "dependencies"
Cohesion: 0.05
Nodes (40): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+32 more)

### Community 36 - "response.go"
Cohesion: 0.08
Nodes (26): Handler, Cursor(), DecodeCursor(), EncodeCursor(), T, NextCursor(), TestCursorRequestModes(), TestCursorRoundTrip() (+18 more)

### Community 37 - "NewStore"
Cohesion: 0.11
Nodes (36): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+28 more)

### Community 38 - "home_assistant_test.go"
Cohesion: 0.11
Nodes (36): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+28 more)

### Community 39 - "go_pkg_github_com_wiselabz_wiselabz_internal_connector"
Cohesion: 0.13
Nodes (13): TestBuildInterfaceTableAttributes(), buildGatewayTable(), buildInterfaceTable(), buildSystemContent(), primaryGatewayName(), wanInterfaceName(), go_pkg_bytes, go_pkg_crypto_tls (+5 more)

### Community 40 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (32): statusInfo, unavailable(), upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo() (+24 more)

### Community 41 - "middleware.go"
Cohesion: 0.08
Nodes (26): contextKey, elevationError, mask(), redactDSN(), redactKVPassword(), Config, TestEveryKeyEnvOverridable(), TestRedactDSN() (+18 more)

### Community 42 - "NewEngine"
Cohesion: 0.12
Nodes (30): RequestedFields(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector(), TestRunSyncFieldsPassesHintToConnector(), TestRunSyncFieldsSurvivesCredentialRefresh() (+22 more)

### Community 43 - "traefik/tables.go"
Cohesion: 0.14
Nodes (31): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable(), cell() (+23 more)

### Community 44 - "useRole.ts"
Cohesion: 0.10
Nodes (28): Frontend, 7. `quality.finding.created` and `quality.findings.changed`, web_src_api_generated_me_me, web_src_api_generated_me_me_usegetme, SearchIcon(), Topbar(), RoleGate(), RoleGateProps (+20 more)

### Community 45 - "routerDeps"
Cohesion: 0.12
Nodes (28): routerDeps, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountConnectorRoutes(), chi.Router (+20 more)

### Community 46 - "auth_test.go"
Cohesion: 0.08
Nodes (33): testApp, loginRefreshCookie(), seedLocalUser(), TestChangePasswordWrongCurrentPassword(), TestDeleteSessionNotOwner(), TestDeleteSessionSuccess(), TestElevateSuccess(), TestElevateWrongPassword() (+25 more)

### Community 47 - "Errorf"
Cohesion: 0.12
Nodes (8): diffToSpec(), Handler, Handler, Handler, validTargetType(), Errorf(), Handler, Handler

### Community 48 - "compliance/engine.go"
Cohesion: 0.11
Nodes (30): contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity, Rule (+22 more)

### Community 49 - "checker_test.go"
Cohesion: 0.20
Nodes (30): NewChecker(), createComplianceRule(), createComplianceSnapshot(), createConnector(), findings(), newTestStore(), TestCheckEmptyDetectsAndAutoResolves(), TestCheckFailingDetectsAndAutoResolves() (+22 more)

### Community 50 - "SuggestRequest"
Cohesion: 0.09
Nodes (14): claudeProvider, ollamaEmbedder, openAICompatibleProvider, StubProvider, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet() (+6 more)

### Community 51 - "IsSecureRequest"
Cohesion: 0.12
Nodes (20): clearOIDCFlowCookie(), Handler, newOIDCUser(), oidcFlowCookieName(), randomOIDCToken(), readOIDCFlowCookie(), setOIDCFlowCookie(), validHostPort() (+12 more)

### Community 52 - "Register"
Cohesion: 0.10
Nodes (28): init(), init(), newConnector(), Connector, GuardedDialer(), init(), newGuardedClient(), TestGuardedClientRejectsLinkLocal() (+20 more)

### Community 53 - "SnapshotEntity"
Cohesion: 0.09
Nodes (20): TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), isTimeout(), SnapshotEntity, TestBuildContainerTableAttributes(), buildContainerTable() (+12 more)

### Community 54 - "unifi/tables.go"
Cohesion: 0.17
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+21 more)

### Community 55 - "Connector"
Cohesion: 0.09
Nodes (10): init(), ConfigField, Connector, buildGatewayTable(), isTimeout(), primaryGatewayName(), wanInterfaceName(), PathSegment() (+2 more)

### Community 56 - "nilToStr"
Cohesion: 0.09
Nodes (12): nilToStr(), DocVersionRecord, Store, DeliveryRecord, DeliveryStatus, Store, scanDelivery(), Store (+4 more)

### Community 57 - "settings.mock.ts"
Cohesion: 0.08
Nodes (27): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role, web_src_api_model_index_session (+19 more)

### Community 58 - "rewritePlaceholders"
Cohesion: 0.10
Nodes (14): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), database/sql.Result, database/sql.Row, database/sql.Rows (+6 more)

### Community 59 - "net/http.Handler"
Cohesion: 0.09
Nodes (25): CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders(), loggablePath(), loggableQuery(), Logger(), captureLog() (+17 more)

### Community 60 - "newTestHandler"
Cohesion: 0.15
Nodes (23): doJSON(), testHandler, req(), TestChangePassword(), TestChangePasswordRevokesAPIKeys(), TestCreateUser(), TestDeleteUser(), TestLogin() (+15 more)

### Community 61 - "Sanitize"
Cohesion: 0.13
Nodes (9): Sanitize(), TestSanitize(), Engine, changePatternID(), Engine, markError(), sync.Map, DocRegenerator (+1 more)

### Community 62 - "boolToInt"
Cohesion: 0.11
Nodes (10): Store, scanBackupRun(), Store, Store, RunbookRecord, Store, scanRunbook(), boolToInt() (+2 more)

### Community 63 - "testApp"
Cohesion: 0.11
Nodes (19): mustHashDummyPassword(), testApp, TestDashboardAdminDefaultPermissionGate(), TestDashboardResetRestoresAdminDefault(), testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty() (+11 more)

### Community 64 - "NewAuthError"
Cohesion: 0.13
Nodes (11): TestRateLimit(), NewAuthError(), TestTypedErrorsAreDistinguishableByType(), apiMessage(), controllerName(), isTimeout(), statusError(), unavailable() (+3 more)

### Community 65 - "Config"
Cohesion: 0.12
Nodes (17): Config, LogSettings, AISettings, AuthSettings, BackupSettings, Database, EncryptionSettings, QualitySettings (+9 more)

### Community 66 - "Connector"
Cohesion: 0.19
Nodes (6): SnapshotSection, Connector, isTimeout(), unavailable(), unavailable(), session

### Community 67 - "Dispatcher"
Cohesion: 0.20
Nodes (10): discordPayload(), findChannel(), findRoute(), Dispatcher, severityRank(), shouldSkipRoute(), slackPayload(), webhookPayload() (+2 more)

### Community 68 - "handlers.ts"
Cohesion: 0.07
Nodes (26): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+18 more)

### Community 69 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 70 - "AppearancePage.tsx"
Cohesion: 0.14
Nodes (22): zustand, MotionProvider(), AppearancePage(), ChoiceGroup(), ToggleRow(), AppearanceState, apply(), Contrast (+14 more)

### Community 71 - "timeline.ts"
Cohesion: 0.14
Nodes (17): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+9 more)

### Community 72 - "Store"
Cohesion: 0.14
Nodes (17): Handler, Handler, NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler() (+9 more)

### Community 73 - "New"
Cohesion: 0.12
Nodes (23): Handler, SyncDocEmbeddings(), TestSyncDocEmbeddingsKeepsOldRowsWhenEmbedFails(), TestRetrieveUsesCacheAndSyncInvalidates(), newBackupTestStore(), TestCreateBackupRun(), TestGetBackupScheduleWhenNotExists(), TestListBackupRunsPaginated() (+15 more)

### Community 74 - "newTestAppWithBackupDir"
Cohesion: 0.11
Nodes (18): Config, RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), Embedder, EmbedRegistry, NewEmbedRegistry() (+10 more)

### Community 75 - "notifications/handlers_test.go"
Cohesion: 0.17
Nodes (23): TestEmbeddedSPAWithoutFrontendBuild(), TestCreate(), TestList(), TestRevoke(), AuthedUser(), JWTService(), Token(), WithAuth() (+15 more)

### Community 76 - "GetTypeSchema"
Cohesion: 0.12
Nodes (22): catalog(), TestRegisteredSchema(), TestSchemaConfigValidation(), TestAllConnectorImplementationsRegister(), TestRegisteredSchema(), TestAPIKeyIsStoredAsPassword(), TestRegisteredSchema(), AttributeCatalog() (+14 more)

### Community 77 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 78 - "net/http.Client"
Cohesion: 0.09
Nodes (8): openAIEmbedder, Connector, isTimeout(), unavailable(), Connector, newWebhookClient(), net/http.Client, Connector

### Community 79 - "fetch_test.go"
Cohesion: 0.12
Nodes (21): newMockOIDCServer(), TestAuthURLAfterInitialization(), TestAuthURLBeforeInitialization(), TestInitializeFailure(), TestInitializeInvalidJSON(), TestInitializeSuccess(), TestIsInitializedBeforeAndAfter(), entityKinds() (+13 more)

### Community 80 - "Checker"
Cohesion: 0.21
Nodes (6): complianceRule(), Checker, QualityFindingRecord, scanQualityFinding(), FindingNotifier, RotationConfig

### Community 81 - "devDependencies"
Cohesion: 0.09
Nodes (23): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+15 more)

### Community 82 - "SystemPage.tsx"
Cohesion: 0.11
Nodes (19): web_src_api_generated_system_system, web_src_api_generated_system_system_getgetsystembackuprunsquerykey, web_src_api_generated_system_system_getgetsystembackupschedulequerykey, web_src_api_generated_system_system_getsystembackupschedule, web_src_api_generated_system_system_postsystembackuprun, web_src_api_generated_system_system_putsystembackupschedule, web_src_api_generated_system_system_usegethealth, web_src_api_generated_system_system_usegetsystembackupruns (+11 more)

### Community 83 - "pagination_contract_test.go"
Cohesion: 0.12
Nodes (19): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_github_com_getkin_kin_openapi_openapi3 (+11 more)

### Community 84 - "docker_test.go"
Cohesion: 0.10
Nodes (21): generateSelfSignedCert(), TestConfigPush(), TestDockerWritableFields(), TestDoRequestContextTimeout(), TestDoRequestErrorCases(), TestFetchBuildsSectionsFromEndpoints(), TestFetchSurfacesMalformedSystemResponse(), TestFetchToleratesEndpointFailure() (+13 more)

### Community 85 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 86 - "ConnectorRecord"
Cohesion: 0.16
Nodes (13): ConnectorRecord, Store, scanConnector(), scanConnectorRows(), nullInt64ToIntPtr(), nullStrToStr(), scanSyncRun(), connectorWithRole (+5 more)

### Community 87 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 88 - "Registry"
Cohesion: 0.16
Nodes (13): Provider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail(), TestSuggestWithFallbackFirstProviderSucceeds(), TestSuggestWithFallbackNoProviders() (+5 more)

### Community 89 - "Service"
Cohesion: 0.20
Nodes (10): Claims, ElevationClaims, ElevationToken, TokenPair, Service, hasAudience(), newTokenID(), go_pkg_github_com_golang_jwt_jwt_v5 (+2 more)

### Community 90 - "Store"
Cohesion: 0.12
Nodes (7): fakeStatusChecker, testAPIKeyChecker, sanitize(), APIKeyClaims, validAPIKey(), APIKey, Store

### Community 91 - "middleware_test.go"
Cohesion: 0.15
Nodes (16): fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, RequireInstanceAdmin(), assertElevationAuditCalls(), boolLabel(), contextWithInstanceAdmin(), requestWithUser() (+8 more)

### Community 92 - "NewRegistry"
Cohesion: 0.26
Nodes (18): NewRegistry(), Handler, newHandler(), serve(), TestConversationOwnership(), TestCreateConversationDocVisibility(), TestCreateConversationValidation(), TestPostMessageErrors() (+10 more)

### Community 93 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.11
Nodes (16): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, What's excluded, and why, What's included, Backups, PostgreSQL support (+8 more)

### Community 94 - "Handler"
Cohesion: 0.16
Nodes (8): updateUserRequest, Handler, writeUserWriteError(), extractGroups(), OIDCProvider, TestExtractGroups(), github.com/coreos/go-oidc/v3/oidc.Provider, golang.org/x/oauth2.Config

### Community 95 - "Handler"
Cohesion: 0.24
Nodes (7): NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), Handler, RuleEvaluator

### Community 96 - "Compare"
Cohesion: 0.19
Nodes (15): configPushLanded(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare(), lineCount(), relatedServiceIDs(), severityForChange() (+7 more)

### Community 97 - "Handler"
Cohesion: 0.18
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 98 - "ws/ws_test.go"
Cohesion: 0.20
Nodes (17): NewHub(), normalizeOrigin(), assertEnvelope(), setupWSConnection(), TestBroadcastFullQueueDoesNotBlock(), TestBroadcastToUserAfterUpgrade(), TestClientCloseDisconnect(), TestDocLockEventBroadcast() (+9 more)

### Community 99 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 100 - "main"
Cohesion: 0.18
Nodes (13): main(), newLogger(), runAlertExpirer(), splitOrigins(), RunDeliveryRetries(), RunStaleSweepOnce(), Store, RunDocLockSweep() (+5 more)

### Community 101 - "Connector"
Cohesion: 0.16
Nodes (7): TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), isTimeout(), TestBuildHostOverrideTableMalformedCases(), TestBuildHostOverrideTableValidOverrides(), Connector

### Community 102 - "ValidateConfig"
Cohesion: 0.18
Nodes (14): TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestBuildHostsTableAttributes(), TestSchemaExposesAPIVersion(), ValidateConfig(), TestSchemaConfigValidation(), isConfigValidationError(), testSchema() (+6 more)

### Community 103 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 104 - "ws.ts"
Cohesion: 0.12
Nodes (16): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+8 more)

### Community 105 - "AuthMiddleware"
Cohesion: 0.16
Nodes (12): APIKeyChecker, UserStatusChecker, TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed(), AuthMiddleware(), extractBearerToken() (+4 more)

### Community 106 - "config_test.go"
Cohesion: 0.17
Nodes (14): runHealthcheck(), Load(), TestAccessTokenTTLDuration(), TestLoadDefaults(), TestLoadEnvOverride(), TestLoadEnvOverrideAllFields(), TestLoadFromYAML(), TestLoadRejectsEmptyCronExpr() (+6 more)

### Community 107 - "handlers_contract_test.go"
Cohesion: 0.25
Nodes (15): AssertMatchesSpec(), loadSpec(), specPath(), createForSpec(), decodeEnvelope(), fieldMsgs(), Handler, TestConnectorSuccessPayloadsMatchSpec() (+7 more)

### Community 108 - "chat/chat.go"
Cohesion: 0.16
Nodes (14): buildPrompt(), TestBuildPrompt(), cosineSimilarity(), Match, packVector(), SplitSections(), TestCosineSimilarityRanksClosestVectorHighest(), TestPackUnpackVectorRoundTrips() (+6 more)

### Community 109 - "diagram.go"
Cohesion: 0.22
Nodes (14): ServiceDependency, networkDependencies(), entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks() (+6 more)

### Community 110 - "sshStdioConn"
Cohesion: 0.14
Nodes (10): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), sshStdioConn, golang.org/x/crypto/ssh.Client, golang.org/x/crypto/ssh.ClientConfig, golang.org/x/crypto/ssh.Session, io.Closer (+2 more)

### Community 111 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 112 - "Handler"
Cohesion: 0.20
Nodes (4): applyConnectorScalarUpdates(), Handler, validateConnectorConfig(), updateConnectorRequest

### Community 113 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 114 - "all.go"
Cohesion: 0.13
Nodes (14): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+6 more)

### Community 115 - "Connector"
Cohesion: 0.17
Nodes (7): TestBuildPeerTableAttributes(), TestBuildPolicyTableAttributes(), buildPeerTable(), buildPolicyTable(), buildRouteTable(), isTimeout(), Connector

### Community 116 - "config_cmd_test.go"
Cohesion: 0.21
Nodes (11): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), Schema(), schemaFor() (+3 more)

### Community 117 - "changes/handlers_test.go"
Cohesion: 0.32
Nodes (13): Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound(), TestDismissSuccess() (+5 more)

### Community 120 - "AppShell.tsx"
Cohesion: 0.20
Nodes (10): Frontend shell & theme (decided 2026-06), react-error-boundary, sonner, AppShell, AppShell(), NavigatorBridge(), Dock(), ShellDock() (+2 more)

### Community 121 - "templates.fixtures.ts"
Cohesion: 0.18
Nodes (11): web_src_api_model_index_docversion, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, resolveToken(), Snapshot (+3 more)

### Community 122 - "newTestHandler"
Cohesion: 0.24
Nodes (12): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), Handler, newTestHandler(), TestCreate() (+4 more)

### Community 123 - "templatefuncs.go"
Cohesion: 0.21
Nodes (11): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+3 more)

### Community 124 - "NotificationRecord"
Cohesion: 0.26
Nodes (4): Dispatcher, NotificationRecord, Store, scanNotification()

### Community 125 - "scheduler_test.go"
Cohesion: 0.40
Nodes (12): New(), TestAddJobInvalidExpression(), TestAddJobRegistersAndFires(), TestContextGivenToJobFunction(), TestInvalidJobNameHandled(), TestJobContextDerivedFromStart(), TestJobSkipsOverlappingInvocations(), testLogger() (+4 more)

### Community 126 - "Contributing to WiseLabz"
Cohesion: 0.15
Nodes (13): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+5 more)

### Community 127 - ".call"
Cohesion: 0.33
Nodes (7): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture, net/http.HandlerFunc

### Community 128 - "NewService"
Cohesion: 0.30
Nodes (11): NewService(), TestConcurrentIssuePairUniqueTokenIDs(), TestElevationExpired(), TestElevationRequiresOwner(), TestElevationWrongAction(), TestExpiredAccessToken(), TestIssueAndValidateAccess(), TestIssueAndValidateElevation() (+3 more)

### Community 129 - "Store"
Cohesion: 0.23
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 130 - "Decision"
Cohesion: 0.17
Nodes (11): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 131 - "scripts"
Cohesion: 0.17
Nodes (12): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+4 more)

### Community 133 - "newSSHDockerClient"
Cohesion: 0.25
Nodes (11): generateSSHHostKey(), startSSHDockerServer(), TestNewSSHDockerClientDialsAndExecutesDialStdio(), TestNewSSHDockerClientRejectsMissingHostKey(), TestNewSSHDockerClientRejectsWrongCredentials(), TestNewSSHDockerClientRejectsWrongHostKey(), TestNewSSHDockerClientSupportsSequentialRequests(), newSSHDockerClient() (+3 more)

### Community 134 - "Store"
Cohesion: 0.33
Nodes (3): Store, scanConnectorGrants(), ConnectorGrant

### Community 135 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 136 - "WiseLabz Connector Guide"
Cohesion: 0.18
Nodes (11): Conventions, Dependencies, Getting your connector merged, Health checks vs. sync, Keeping snapshots stable, Session-based and multi-flavour APIs, Testing without a real instance, The Connector interface (+3 more)

### Community 137 - "Product"
Cohesion: 0.18
Nodes (10): Accessibility & Inclusion, Anti-references, Brand Personality, Design Principles, Locked frontend direction (planning session, 2026-06; revised 2026-09), Product, Product decisions (pre-planning, v1), Product Purpose (+2 more)

### Community 138 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 139 - "handlers_bulk_test.go"
Cohesion: 0.47
Nodes (9): bulkReq(), bulkResults(), createBulkFakeConnector(), Handler, registerBulkFakeConnector(), TestBulkReauth(), TestBulkRestart(), TestBulkSync() (+1 more)

### Community 140 - "connectors_health_test.go"
Cohesion: 0.42
Nodes (9): testApp, registerHealthFakeType(), seedHealthTestConnector(), TestConnectorsHealthDegraded(), TestConnectorsHealthDoesNotCreateSnapshot(), TestConnectorsHealthOffline(), TestConnectorsHealthOnline(), TestConnectorsHealthRoleBoundary() (+1 more)

### Community 141 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 142 - "docs_test.go"
Cohesion: 0.36
Nodes (9): testApp, seedDoc(), TestDocLockConflict(), TestDocLockHappyPath(), TestDocLockRoleBoundary(), TestDocsListAndGetSuccess(), TestDocsSaveRoleBoundary(), TestDocsSaveSuccess() (+1 more)

### Community 143 - "time.Time"
Cohesion: 0.22
Nodes (6): digestDue(), formatDigest(), Dispatcher, TestDigestDue(), time.Time, userStatus

### Community 144 - "transform_test.go"
Cohesion: 0.24
Nodes (6): init(), normalizeEnabledColumn(), normalizeFirewallRules(), RegisterTransformer(), TestNormalizeFirewallRulesRewritesEnabledColumn(), Transformer

### Community 145 - "Changelog"
Cohesion: 0.20
Nodes (9): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features, Unreleased (+1 more)

### Community 146 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 147 - "ComplianceRuleRecord"
Cohesion: 0.36
Nodes (4): changedFields(), ComplianceRuleRecord, Store, scanComplianceRule()

### Community 148 - "ratelimit.go"
Cohesion: 0.31
Nodes (6): RateLimit(), go_pkg_golang_org_x_time_rate, golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 149 - "openapi_contract_test.go"
Cohesion: 0.33
Nodes (8): normalizeParams(), routerOperations(), specOperations(), TestAPIV1AliasServesSameHandlers(), TestOpenAPIHealthProbeRoutes(), TestOpenAPIMatchesRouter(), chi.Routes, go_pkg_go_yaml_in_yaml_v3

### Community 150 - "newDockerClient"
Cohesion: 0.22
Nodes (8): newDockerClient(), init(), TestNewDockerClientDialsUnixSocket(), TestNewDockerClientRejectsUnsupportedScheme(), TestValidateCompositeRef(), TestValidateRefSegment(), TestValidateUnixSocketPath(), ValidateUnixSocketPath()

### Community 151 - "Runner"
Cohesion: 0.31
Nodes (3): cron.EntryID, Runner, cron.Cron

### Community 153 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 154 - "connectors_hardening_test.go"
Cohesion: 0.29
Nodes (7): testApp, TestConnectorsCreateAcceptsValidConfig(), TestConnectorsCreateRejectsInvalidEnum(), TestConnectorsCreateRejectsMalformedConfig(), TestConnectorsSyncAcceptsFieldsHint(), TestConnectorsUpdateRejectsMalformedConfig(), waitForSyncRuns()

### Community 155 - "dashboard/handlers_test.go"
Cohesion: 0.43
Nodes (7): Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout()

### Community 157 - "Cache"
Cohesion: 0.43
Nodes (5): Cache, New(), Cache[V], entry, V

### Community 158 - "Step by step"
Cohesion: 0.25
Nodes (8): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Step by step

### Community 159 - "WiseLabz"
Cohesion: 0.25
Nodes (8): Code of Conduct, Configuration, Contributing, Features, License, Quick start, Supported services, WiseLabz

### Community 160 - "newTCPDockerClient"
Cohesion: 0.29
Nodes (7): IsDangerousIP(), buildDockerTLSConfig(), newTCPDockerClient(), TestNewTCPDockerClientNoTLSWhenNoCert(), TestNewTCPDockerClientRejectsInvalidCertPair(), crypto/tls.Config, net.IP

### Community 161 - "serveSSHDockerConn"
Cohesion: 0.29
Nodes (6): serveOneHTTPExchange(), serveSSHDockerConn(), bufio.ReadWriter, golang.org/x/crypto/ssh.Channel, golang.org/x/crypto/ssh.ServerConfig, net.Conn

### Community 162 - "RunCleanupOnce"
Cohesion: 0.57
Nodes (7): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupSkipsDisabledCategories()

### Community 163 - "scanMaintenanceWindow"
Cohesion: 0.48
Nodes (3): Store, scanMaintenanceWindow(), MaintenanceWindowRecord

### Community 164 - "computeNextRun"
Cohesion: 0.43
Nodes (5): TestComputeNextRun_BackoffNeverExceedsScheduleCadence(), TestComputeNextRun_FailureUsesBackoffSchedule(), TestComputeNextRun_ManualOnlyNeverSchedules(), TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(), computeNextRun()

### Community 165 - "Contributor Covenant Code of Conduct"
Cohesion: 0.29
Nodes (7): Attribution, Contributor Covenant Code of Conduct, Enforcement, Enforcement Responsibilities, Our Pledge, Our Standards, Scope

### Community 166 - "Audit Trail"
Cohesion: 0.29
Nodes (6): Audit Trail, Endpoint, Keyset (cursor) pagination, Retention, What's not recorded, What's recorded

### Community 167 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 169 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 170 - "walkCursorPages"
Cohesion: 0.40
Nodes (6): cursorPage, decodeCursorPage(), testApp, TestAuditCursorPaginationTraversal(), TestChangesCursorPaginationTraversal(), walkCursorPages()

### Community 171 - "compliance_rules_test.go"
Cohesion: 0.47
Nodes (5): TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails()

### Community 174 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 175 - "browser.ts"
Cohesion: 0.40
Nodes (4): bootstrap(), worker, enableMocks(), handlers

### Community 176 - "RequireConnectorRole"
Cohesion: 0.50
Nodes (4): ConnectorRoleChecker, RequireConnectorRole(), TestRequireConnectorRole(), TestRequireConnectorRoleCheckerError()

### Community 177 - "auth/handlers_test.go"
Cohesion: 0.40
Nodes (4): TestEmailDomainAllowed(), TestOIDCRoleForGroups(), emailDomainAllowed(), oidcRoleForGroups()

### Community 178 - "Enforcement Guidelines"
Cohesion: 0.40
Nodes (5): 1. Correction, 2. Warning, 3. Temporary Ban, 4. Permanent Ban, Enforcement Guidelines

### Community 179 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 180 - "testHandler"
Cohesion: 0.50
Nodes (3): testHandler, Handler, instanceAdminRoleFor()

### Community 182 - "ClassifyHealth"
Cohesion: 0.67
Nodes (3): ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold()

### Community 186 - "MISSING — deferred & future frontend features"
Cohesion: 0.50
Nodes (3): Deferred from V1 (decided during planning), MISSING — deferred & future frontend features, Suggested-later (raised in build, not yet planned)

### Community 187 - "Saved Views"
Cohesion: 0.50
Nodes (3): Endpoints, Saved Views, Scope

## Knowledge Gaps
- **519 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `bulkResolveRequest`, `bulkResolveItemResult` (+514 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1126 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **24 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `createTestNotification()` connect `newDocTestStore` to `context.Context`, `testing.T`?**
  _High betweenness centrality (0.022) - this node is a cross-community bridge._
- **Why does `createTestConnector()` connect `newDocTestStore` to `context.Context`, `testing.T`?**
  _High betweenness centrality (0.015) - this node is a cross-community bridge._
- **Why does `queryPlan()` connect `newDocTestStore` to `context.Context`, `testing.T`?**
  _High betweenness centrality (0.015) - this node is a cross-community bridge._
- **Are the 196 inferred relationships involving `newTestApp()` (e.g. with `TestAlertsBulkSnoozePartialFailure()` and `TestAlertsBulkSnoozeRejectsTooManyIDs()`) actually correct?**
  _`newTestApp()` has 196 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _519 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.023933954404302054 - nodes in this community are weakly interconnected._
- **Should `newDocTestStore` be split into smaller, more focused modules?**
  _Cohesion score 0.025303827317250137 - nodes in this community are weakly interconnected._