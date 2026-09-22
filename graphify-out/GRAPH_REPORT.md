# Graph Report - WiseLabz  (2026-09-21)

## Corpus Check
- 713 files · ~434,958 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 17 file(s) not represented in the graph (top: (none) 10, .toml 2, .example 1)

## Summary
- 5404 nodes · 16843 edges · 198 communities (179 shown, 19 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1318 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `18c7cd62`
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
- DecodeKey
- ServiceDetailPage.tsx
- icons.tsx
- ErrorWithDetails
- connector/connector.go
- dispatcher_test.go
- router.go
- net/http.ResponseWriter
- portainer/tables.go
- net/http.Request
- backup/backup.go
- fixtures.ts
- ConnectorRecord
- share_links_test.go
- newTestHandler
- WiseLabz — Architecture & Technical Decisions
- RunMigrations
- lists.go
- home_assistant/tables.go
- NewEngine
- dependencies
- response.go
- NewStore
- home_assistant_test.go
- go_pkg_strings
- adguardhome/tables.go
- config/validate_test.go
- NewEngine
- traefik/tables.go
- UsersPage.tsx
- routerDeps
- api/audit_test.go
- Errorf
- src/theme.ts
- checker_test.go
- SuggestRequest
- IsSecureRequest
- GuardedDialer
- NewMalformedResponseError
- unifi/tables.go
- Connector
- nilToStr
- settings.mock.ts
- rewritePlaceholders
- logging.go
- ConnectorEditPage.tsx
- Hub
- RunbookRecord
- testApp
- newTestHandler
- Config
- Connector
- Dispatcher
- handlers.ts
- unifi_test.go
- AppearancePage.tsx
- timeline.ts
- Store
- snapshot_attributes_test.go
- main
- notifications/handlers_test.go
- registry.go
- WiseLabz — Design Contract
- net/http.Client
- fetch_test.go
- SnapshotEntity
- devDependencies
- SystemPage.tsx
- pagination_contract_test.go
- docker_test.go
- portainer_test.go
- middleware.go
- adguardhome_test.go
- NewRegistry
- Service
- Store
- middleware_test.go
- newHandler
- Configuration & Documentation Backup (Export/Import)
- Handler
- compliance/handlers.go
- Compare
- Handler
- ws/ws_test.go
- compilerOptions
- log/slog.Logger
- go_pkg_reflect
- GetTypeSchema
- docdiffmodel.ts
- ws.ts
- AuthMiddleware
- MarshalConnectorConfig
- handlers_contract_test.go
- chat/chat.go
- truenas/tables_test.go
- sshStdioConn
- compilerOptions
- Handler
- vectorCache
- all.go
- Connector
- Register
- changes/handlers_test.go
- Connector
- truenas_test.go
- WebSocketProvider.tsx
- templates.fixtures.ts
- newTestHandler
- templatefuncs.go
- keyset_test.go
- scheduler_test.go
- Contributing to WiseLabz
- .call
- NewService
- traefik_test.go
- Decision
- scripts
- Handler
- system/handlers_test.go
- Store
- Decision
- WiseLabz Connector Guide
- Product
- alerts/handlers_authz_test.go
- handlers_bulk_test.go
- backup/backup_test.go
- connectors_maintenance_test.go
- docs_test.go
- DocRecord
- transform.go
- Changelog
- mockServiceWorker.js
- diagnostics/diagnostics.go
- changes_test.go
- openapi_contract_test.go
- unifi/tables_test.go
- Config
- Handler
- release-please-config.json
- .applyChannelSecrets
- Store
- migrations_parity_test.go
- APIKeyClaims
- Step by step
- WiseLabz
- CORS
- engine_maintenance_test.go
- retention/retention_test.go
- scanMaintenanceWindow
- computeNextRun
- Contributor Covenant Code of Conduct
- Audit Trail
- Bulk Review Actions
- main.tsx
- PULL_REQUEST_TEMPLATE.md
- seedScopeFixture
- compliance_rules_test.go
- .Redacted
- Store
- Security Policy
- truenas/attributes_test.go
- RequireConnectorRole
- auth/handlers_test.go
- Enforcement Guidelines
- compose-smoke.sh
- testHandler
- QualityChecker
- ClassifyHealth
- timeoutError
- MISSING — deferred & future frontend features
- Saved Views
- AlertNotifier
- stubEmbedder
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
5. `SnapshotEntity` - 74 edges
6. `react` - 72 edges
7. `cn()` - 69 edges
8. `UserIDFromContext()` - 67 edges
9. `NewStore()` - 62 edges
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

## Communities (198 total, 19 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.03
Nodes (130): templateBody, TestAPIKeyCreateRejectsInvalidExpiryAndEmptyName(), TestAPIKeyRoutesEndToEnd(), testApp, loginRefreshCookie(), seedLocalUser(), TestChangePasswordWrongCurrentPassword(), TestDeleteSessionNotOwner() (+122 more)

### Community 1 - "newDocTestStore"
Cohesion: 0.03
Nodes (122): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+114 more)

### Community 2 - "react"
Cohesion: 0.04
Nodes (78): match-sorter, motion, @radix-ui/react-popover, react, web_src_api_generated_alerts_alerts, web_src_api_generated_alerts_alerts_getgetalertsquerykey, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve (+70 more)

### Community 3 - "RulesPage.tsx"
Cohesion: 0.05
Nodes (39): web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_usegetauthapikeys, web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey, web_src_api_generated_compliance_compliance_postcompliancerules (+31 more)

### Community 4 - "@tanstack/react-query"
Cohesion: 0.04
Nodes (59): msw, @tanstack/react-query, @testing-library/react, vitest, web_src_api_generated_connectors_connectors, web_src_api_generated_docs_docs, web_src_api_model_index_attentionpage, web_src_api_model_index_runbookpage (+51 more)

### Community 5 - "testing.T"
Cohesion: 0.03
Nodes (120): cursorPage, runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), runHealthcheck() (+112 more)

### Community 6 - "cn"
Cohesion: 0.04
Nodes (78): web_src_api_generated_changes_changes_getgetchangeschangeidquerykey, web_src_api_generated_changes_changes_postchangeschangeidack, web_src_api_generated_changes_changes_postchangeschangeidaiupdate, web_src_api_generated_changes_changes_postchangeschangeiddismiss, web_src_api_generated_changes_changes_postchangeschangeidexplain, web_src_api_generated_changes_changes_usegetchangeschangeid, web_src_api_generated_chat_chat, web_src_api_generated_chat_chat_getgetchatconversationsidquerykey (+70 more)

### Community 7 - "package.json"
Cohesion: 0.04
Nodes (48): clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks (+40 more)

### Community 8 - "context.Context"
Cohesion: 0.03
Nodes (34): fakeStatusChecker, sanitizeSessions(), Connector, Store, existingIDs(), placeholders(), AlertRecord, ChangeRecord (+26 more)

### Community 9 - "App.tsx"
Cohesion: 0.04
Nodes (74): Frontend shell & theme (decided 2026-06), react-error-boundary, react-i18next, react-router-dom, sonner, setAccessToken(), web_src_api_generated_auth_auth, web_src_api_generated_auth_auth_postauthlogin (+66 more)

### Community 10 - "DashboardPage.tsx"
Cohesion: 0.04
Nodes (85): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress`, 3. `sync.complete`, 4. `change.detected` (+77 more)

### Community 11 - "go_pkg_context"
Cohesion: 0.09
Nodes (18): StatusError, noRedirect(), contains(), searchString(), bulkRequest, go_pkg_context, go_pkg_database_sql, go_pkg_encoding_hex (+10 more)

### Community 12 - "go_pkg_net_http"
Cohesion: 0.06
Nodes (44): bulkSnoozeItemResult, bulkSnoozeRequest, updateUserRequest, newToken(), changePromptData(), diffToSpec(), stripPromptTags(), truncateUTF8() (+36 more)

### Community 13 - "go_pkg_testing"
Cohesion: 0.05
Nodes (34): dashboardLayout, TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens(), TestClaudeSuggestErrors(), TestClaudeSuggestMultipleContentBlocks(), TestRegisterClaudeDefaults(), TestOIDCRedirectURL(), Handler (+26 more)

### Community 14 - "ServiceSnapshot"
Cohesion: 0.03
Nodes (17): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, runTransformers(), TestRunTransformersUnknownCategoryIsNoop(), registryTestRefresher, actionConnector, bulkFakeConnector (+9 more)

### Community 15 - "DecodeKey"
Cohesion: 0.15
Nodes (19): ProviderConfig, primaryProviderConfig(), DecodeKey(), Decrypt(), DeriveKey(), Encrypt(), TestDecodeKey(), TestDecodeKeyUsableForEncryptDecrypt() (+11 more)

### Community 16 - "ServiceDetailPage.tsx"
Cohesion: 0.04
Nodes (69): ADR-0001, ADR-0003, Frontend, 1. `service.status`, 7. `quality.finding.created` and `quality.findings.changed`, web_src_api_generated_connectors_connectors_postconnectorsconnectoridconfigpush, web_src_api_generated_connectors_connectors_postconnectorsconnectoridhealth, web_src_api_generated_connectors_connectors_postconnectorsconnectoridrestart (+61 more)

### Community 17 - "icons.tsx"
Cohesion: 0.05
Nodes (58): web_src_api_generated_docs_docs_getgetdocsdocidquerykey, web_src_api_generated_docs_docs_getgetdocsdocidversionsquerykey, web_src_api_generated_docs_docs_getgetdocstreequerykey, web_src_api_generated_docs_docs_postdocsdocidaisuggest, web_src_api_generated_docs_docs_postdocsdocidlock, web_src_api_generated_docs_docs_postdocsdocidlockrelease, web_src_api_generated_docs_docs_postdocsdocidversionsrevrestore, web_src_api_generated_docs_docs_putdocsdocid (+50 more)

### Community 18 - "ErrorWithDetails"
Cohesion: 0.10
Nodes (17): sanitizeUser(), setRefreshCookie(), Handler, contextWithShareLink(), Handler, shareLinkFromContext(), validTargetType(), Handler (+9 more)

### Community 19 - "connector/connector.go"
Cohesion: 0.04
Nodes (44): isTimeout(), isTimeout(), ServiceDependency, NewAuthError(), NewServiceUnavailableError(), NewTimeoutError(), setHeaders(), TestValidateCustomURL() (+36 more)

### Community 20 - "dispatcher_test.go"
Cohesion: 0.24
Nodes (38): NewDispatcher(), deliveriesFor(), findDelivery(), Dispatcher, newTestStore(), setChannelAndRoutingConfig(), setChannelConfig(), setWebhookConfig() (+30 more)

### Community 21 - "router.go"
Cohesion: 0.13
Nodes (25): newTestStore(), TestCheckHealthReportsDegradedOnClosedDB(), TestCollectIncludesHealthVersionsAndSchedule(), TestCollectListsRecentFailures(), TestCollectRedactsConnectorSecrets(), go_pkg_github_com_go_chi_chi_v5, go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts, go_pkg_github_com_wiselabz_wiselabz_internal_api_apikeys (+17 more)

### Community 22 - "net/http.ResponseWriter"
Cohesion: 0.09
Nodes (16): Handler, isWritableField(), Handler, decodeBulkRequest(), Handler, Handler, Handler, Handler (+8 more)

### Community 23 - "portainer/tables.go"
Cohesion: 0.12
Nodes (33): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEnvironmentTable(), buildStackTable(), cell(), containerRows(), environmentNames(), environmentTypeName() (+25 more)

### Community 24 - "net/http.Request"
Cohesion: 0.09
Nodes (13): Handler, cron.EntryID, Handler, createVersion(), templateResponse(), templateVersionResponse(), intQuery(), Paginate() (+5 more)

### Community 25 - "backup/backup.go"
Cohesion: 0.25
Nodes (19): connectorIDs(), docIDs(), Export(), exportDocs(), exportTemplates(), exportWithin(), AIConfigSummary, importBundle() (+11 more)

### Community 26 - "fixtures.ts"
Cohesion: 0.06
Nodes (42): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+34 more)

### Community 27 - "ConnectorRecord"
Cohesion: 0.06
Nodes (35): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), changeFilterClause(), scanAlert(), ConnectorRecord (+27 more)

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
Cohesion: 0.12
Nodes (31): main(), OpenDB(), TestOpenDBEnablesSQLiteForeignKeys(), TestOpenDBSetsSQLiteDurabilityPragmas(), GetMigrationStatus(), newMigrator(), RunMigrations(), RunMigrationsDown() (+23 more)

### Community 32 - "lists.go"
Cohesion: 0.13
Nodes (33): buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP(), groupNames(), parseAdlistsV6() (+25 more)

### Community 33 - "home_assistant/tables.go"
Cohesion: 0.10
Nodes (38): jsonType(), TestAttributeCatalogCoversEmittedKeys(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations(), buildOverview() (+30 more)

### Community 34 - "NewEngine"
Cohesion: 0.09
Nodes (43): entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks(), Engine, NewEngine() (+35 more)

### Community 35 - "dependencies"
Cohesion: 0.05
Nodes (40): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+32 more)

### Community 36 - "response.go"
Cohesion: 0.09
Nodes (23): Handler, Cursor(), DecodeCursor(), EncodeCursor(), T, NextCursor(), TestCursorRequestModes(), TestCursorRoundTrip() (+15 more)

### Community 37 - "NewStore"
Cohesion: 0.11
Nodes (36): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+28 more)

### Community 38 - "home_assistant_test.go"
Cohesion: 0.12
Nodes (31): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+23 more)

### Community 39 - "go_pkg_strings"
Cohesion: 0.10
Nodes (20): TestAttributeCatalogCoversEmittedKeys(), TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), normalizeEnabledColumn(), normalizeFirewallRules(), TestNormalizeFirewallRulesRewritesEnabledColumn() (+12 more)

### Community 40 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (32): statusInfo, unavailable(), upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo() (+24 more)

### Community 41 - "config/validate_test.go"
Cohesion: 0.27
Nodes (8): redactDSN(), redactKVPassword(), Config, TestEveryKeyEnvOverridable(), TestRedactDSN(), TestRedacted(), TestValidate(), validConfig()

### Community 42 - "NewEngine"
Cohesion: 0.18
Nodes (24): RequestedFields(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector(), TestRunSyncFieldsPassesHintToConnector(), TestRunSyncFieldsSurvivesCredentialRefresh() (+16 more)

### Community 43 - "traefik/tables.go"
Cohesion: 0.14
Nodes (31): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable(), cell() (+23 more)

### Community 44 - "UsersPage.tsx"
Cohesion: 0.05
Nodes (54): axios, AXIOS_INSTANCE, BodyType, customInstance(), ErrorType, getAccessToken(), RefreshFn, setRefreshHandler() (+46 more)

### Community 45 - "routerDeps"
Cohesion: 0.14
Nodes (25): routerDeps, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountChatRoutes(), mountDocRoutes() (+17 more)

### Community 46 - "api/audit_test.go"
Cohesion: 0.05
Nodes (52): testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow(), TestAlertsListSuccess() (+44 more)

### Community 47 - "Errorf"
Cohesion: 0.08
Nodes (14): Handler, Handler, Handler, Handler, Handler, Handler, Handler, Handler (+6 more)

### Community 48 - "src/theme.ts"
Cohesion: 0.11
Nodes (29): @fontsource/space-mono, @fontsource-variable/space-grotesk, mermaid, cssVar(), Mermaid(), resolveColor(), AdvancedControls(), ColorMode (+21 more)

### Community 49 - "checker_test.go"
Cohesion: 0.06
Nodes (65): contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity, Rule (+57 more)

### Community 50 - "SuggestRequest"
Cohesion: 0.11
Nodes (12): claudeProvider, openAICompatibleProvider, StubProvider, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet(), TestRegistryList() (+4 more)

### Community 51 - "IsSecureRequest"
Cohesion: 0.20
Nodes (14): clearOIDCFlowCookie(), oidcFlowCookieName(), readOIDCFlowCookie(), setOIDCFlowCookie(), ClientIP(), hostOnly(), IsSecureRequest(), isTrustedProxy() (+6 more)

### Community 52 - "GuardedDialer"
Cohesion: 0.13
Nodes (15): newConnector(), Connector, GuardedDialer(), IsDangerousIP(), newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), intConfig() (+7 more)

### Community 53 - "NewMalformedResponseError"
Cohesion: 0.06
Nodes (18): ConfigField, NewMalformedResponseError(), WantsField(), Connector, TestBuildHostsTableV5(), buildHostsTableV5(), Connector, parseGroupsV5() (+10 more)

### Community 54 - "unifi/tables.go"
Cohesion: 0.33
Nodes (17): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+9 more)

### Community 55 - "Connector"
Cohesion: 0.09
Nodes (13): validateConfigPushRequest(), init(), Connector, buildGatewayTable(), primaryGatewayName(), wanInterfaceName(), PathSegment(), TestValidateCompositeRef() (+5 more)

### Community 56 - "nilToStr"
Cohesion: 0.10
Nodes (11): ChatConversationRecord, Store, nilToStr(), DocVersionRecord, Store, DeliveryRecord, DeliveryStatus, Store (+3 more)

### Community 57 - "settings.mock.ts"
Cohesion: 0.08
Nodes (29): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationconfig, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role (+21 more)

### Community 58 - "rewritePlaceholders"
Cohesion: 0.10
Nodes (14): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), database/sql.Result, database/sql.Row, database/sql.Rows (+6 more)

### Community 59 - "logging.go"
Cohesion: 0.16
Nodes (17): loggablePath(), loggableQuery(), Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken(), TestLoggerRedactsWSTicket(), TestLoggablePathMasksShareTokenUnderV1() (+9 more)

### Community 60 - "ConnectorEditPage.tsx"
Cohesion: 0.08
Nodes (22): RFC-3339, web_src_api_generated_connectors_connectors_getgetconnectorsquerykey, web_src_api_generated_connectors_connectors_postconnectors, web_src_api_generated_connectors_connectors_postconnectorsconnectoridtest, web_src_api_generated_connectors_connectors_putconnectorsconnectorid, web_src_api_generated_connectors_connectors_usegetconnectorsconnectorid, web_src_api_generated_connectors_connectors_usegetconnectorsschema, web_src_api_model_index_connector (+14 more)

### Community 61 - "Hub"
Cohesion: 0.07
Nodes (15): Sanitize(), TestSanitize(), Engine, changePatternID(), Engine, markError(), Hub, github.com/gorilla/websocket.Conn (+7 more)

### Community 62 - "RunbookRecord"
Cohesion: 0.50
Nodes (3): RunbookRecord, Store, scanRunbook()

### Community 63 - "testApp"
Cohesion: 0.12
Nodes (18): mustHashDummyPassword(), testApp, TestDashboardAdminDefaultPermissionGate(), TestDashboardResetRestoresAdminDefault(), testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty() (+10 more)

### Community 64 - "newTestHandler"
Cohesion: 0.07
Nodes (34): doJSON(), testHandler, req(), TestChangePassword(), TestChangePasswordRevokesAPIKeys(), TestCreateUser(), TestDeleteUser(), TestLogin() (+26 more)

### Community 65 - "Config"
Cohesion: 0.09
Nodes (22): Config, LogSettings, Cache, New(), AISettings, AuthSettings, BackupSettings, Database (+14 more)

### Community 66 - "Connector"
Cohesion: 0.21
Nodes (5): SnapshotSection, Connector, parseHosts(), unavailable(), session

### Community 67 - "Dispatcher"
Cohesion: 0.19
Nodes (11): discordPayload(), findChannel(), findRoute(), Dispatcher, severityRank(), shouldSkipRoute(), slackPayload(), webhookPayload() (+3 more)

### Community 68 - "handlers.ts"
Cohesion: 0.07
Nodes (28): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+20 more)

### Community 69 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 70 - "AppearancePage.tsx"
Cohesion: 0.13
Nodes (23): zustand, MotionProvider(), AppearancePage(), ChoiceGroup(), ToggleRow(), ThemeControls(), AppearanceState, apply() (+15 more)

### Community 71 - "timeline.ts"
Cohesion: 0.12
Nodes (19): bootstrap(), enableMocks(), installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit (+11 more)

### Community 72 - "Store"
Cohesion: 0.16
Nodes (15): Handler, NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler() (+7 more)

### Community 73 - "snapshot_attributes_test.go"
Cohesion: 0.83
Nodes (3): TestSnapshotAttributesRoundTripPostgres(), TestSnapshotAttributesRoundTripSQLite(), testSnapshotWithAttributes()

### Community 74 - "main"
Cohesion: 0.15
Nodes (16): main(), splitOrigins(), RegisterClaude(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), Embedder, EmbedRegistry, NewEmbedRegistry() (+8 more)

### Community 75 - "notifications/handlers_test.go"
Cohesion: 0.26
Nodes (16): TestCreate(), AuthedUser(), NewHandler(), decodePaginated(), jsonHasEmptyArrayItems(), newTestStore(), seedDelivery(), TestListDeliveriesEmpty() (+8 more)

### Community 76 - "registry.go"
Cohesion: 0.19
Nodes (10): IsCredentialRefresherType(), ListSchemas(), TestIsCredentialRefresherType(), TestRegisterStubRoundTrips(), AttributeSpec, ConfigValidationError, Factory, SchemaField (+2 more)

### Community 77 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 78 - "net/http.Client"
Cohesion: 0.07
Nodes (9): Connector, ollamaEmbedder, openAIEmbedder, Connector, LimitedBody(), Connector, newWebhookClient(), Connector (+1 more)

### Community 79 - "fetch_test.go"
Cohesion: 0.12
Nodes (21): newMockOIDCServer(), TestAuthURLAfterInitialization(), TestAuthURLBeforeInitialization(), TestInitializeFailure(), TestInitializeInvalidJSON(), TestInitializeSuccess(), TestIsInitializedBeforeAndAfter(), entityKinds() (+13 more)

### Community 80 - "SnapshotEntity"
Cohesion: 0.26
Nodes (24): SnapshotEntity, buildDatasets(), buildDisks(), buildInterfaces(), buildNFSShares(), buildPools(), buildReplicationTasks(), buildServices() (+16 more)

### Community 81 - "devDependencies"
Cohesion: 0.09
Nodes (23): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+15 more)

### Community 82 - "SystemPage.tsx"
Cohesion: 0.05
Nodes (54): web_src_api_generated_settings_settings_getgetaiconfigfallbackprovidersquerykey, web_src_api_generated_settings_settings_getgetaiconfigquerykey, web_src_api_generated_settings_settings_getgetauthconfigquerykey, web_src_api_generated_settings_settings_postaiconfigtest, web_src_api_generated_settings_settings_putaiconfig, web_src_api_generated_settings_settings_putaiconfigfallbackproviders, web_src_api_generated_settings_settings_putauthconfig, web_src_api_generated_settings_settings_putauthprovidersprovideridenabled (+46 more)

### Community 83 - "pagination_contract_test.go"
Cohesion: 0.21
Nodes (13): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_go_ast (+5 more)

### Community 84 - "docker_test.go"
Cohesion: 0.05
Nodes (49): buildDockerTLSConfig(), newDockerClient(), newTCPDockerClient(), init(), generateSelfSignedCert(), generateSSHHostKey(), startSSHDockerServer(), TestConfigPush() (+41 more)

### Community 85 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 86 - "middleware.go"
Cohesion: 0.15
Nodes (17): AuditRecorder, contextKey, elevationError, PermissionChecker, SecurityHeaders(), TestSecurityHeaders(), elevationFailureReason(), extractBearerToken() (+9 more)

### Community 87 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 88 - "NewRegistry"
Cohesion: 0.14
Nodes (24): Provider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail(), TestSuggestWithFallbackFirstProviderSucceeds(), TestSuggestWithFallbackNoProviders() (+16 more)

### Community 89 - "Service"
Cohesion: 0.20
Nodes (10): Claims, ElevationClaims, ElevationToken, TokenPair, Service, hasAudience(), newTokenID(), go_pkg_github_com_golang_jwt_jwt_v5 (+2 more)

### Community 90 - "Store"
Cohesion: 0.27
Nodes (3): sanitize(), APIKey, Store

### Community 91 - "middleware_test.go"
Cohesion: 0.16
Nodes (14): fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, assertElevationAuditCalls(), boolLabel(), requestWithUser(), TestAuthMiddlewareAllowsCurrentRoleClaim(), TestAuthMiddlewareMissingHeader() (+6 more)

### Community 92 - "newHandler"
Cohesion: 0.24
Nodes (14): TestEmbeddedSPAWithoutFrontendBuild(), TestList(), TestRevoke(), JWTService(), Token(), WithAuth(), Handler, newHandler() (+6 more)

### Community 93 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.11
Nodes (16): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, What's excluded, and why, What's included, Backups, PostgreSQL support (+8 more)

### Community 94 - "Handler"
Cohesion: 0.11
Nodes (13): Handler, writeUserWriteError(), Handler, newOIDCUser(), randomOIDCToken(), validHostPort(), extractGroups(), OIDCClaims (+5 more)

### Community 95 - "compliance/handlers.go"
Cohesion: 0.14
Nodes (14): catalog(), changedFields(), NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), ComplianceRuleRecord (+6 more)

### Community 96 - "Compare"
Cohesion: 0.19
Nodes (15): configPushLanded(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare(), lineCount(), relatedServiceIDs(), severityForChange() (+7 more)

### Community 97 - "Handler"
Cohesion: 0.18
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 98 - "ws/ws_test.go"
Cohesion: 0.19
Nodes (18): NewHub(), normalizeOrigin(), assertEnvelope(), setupWSConnection(), TestBroadcastFullQueueDoesNotBlock(), TestBroadcastToUserAfterUpgrade(), TestClientCloseDisconnect(), TestDocLockEventBroadcast() (+10 more)

### Community 99 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 100 - "log/slog.Logger"
Cohesion: 0.07
Nodes (22): newLogger(), runAlertExpirer(), WithLogger(), digestDue(), formatDigest(), Dispatcher, TestDigestDue(), Dispatcher (+14 more)

### Community 101 - "go_pkg_reflect"
Cohesion: 0.06
Nodes (27): Schema(), schemaFor(), TestSchemaMatchesConfig(), TestAttributeCatalogCoversEmittedKeys(), TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), TestBuildHostOverrideTableMalformedCases() (+19 more)

### Community 102 - "GetTypeSchema"
Cohesion: 0.10
Nodes (29): TestRegisteredSchema(), TestSchemaConfigValidation(), TestAllConnectorImplementationsRegister(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestBuildHostsTableAttributes(), TestSchemaExposesAPIVersion() (+21 more)

### Community 103 - "docdiffmodel.ts"
Cohesion: 0.23
Nodes (13): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, fold(), toUnits(), DiffLine, DiffLineType (+5 more)

### Community 104 - "ws.ts"
Cohesion: 0.12
Nodes (16): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+8 more)

### Community 105 - "AuthMiddleware"
Cohesion: 0.21
Nodes (9): APIKeyChecker, UserStatusChecker, TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed(), AuthMiddleware(), TestAuthMiddlewareInvalidToken() (+1 more)

### Community 106 - "MarshalConnectorConfig"
Cohesion: 0.20
Nodes (16): TestExportRedactsConnectorSecrets(), IsSecretFieldType(), MarshalConnectorConfig(), ParseConnectorConfig(), SecretFieldsChanged(), init(), TestConnectorRotationFieldsRoundTrip(), TestCreateConnectorDefaultsSecretRotatedAtToCreatedAt() (+8 more)

### Community 107 - "handlers_contract_test.go"
Cohesion: 0.25
Nodes (15): AssertMatchesSpec(), loadSpec(), specPath(), createForSpec(), decodeEnvelope(), fieldMsgs(), Handler, TestConnectorSuccessPayloadsMatchSpec() (+7 more)

### Community 108 - "chat/chat.go"
Cohesion: 0.14
Nodes (18): buildPrompt(), TestBuildPrompt(), Handler, cosineSimilarity(), Match, packVector(), Retrieve(), SplitSections() (+10 more)

### Community 109 - "truenas/tables_test.go"
Cohesion: 0.12
Nodes (18): buildSystem(), coreSummary(), flavor(), humanBytes(), TestBuildDatasetsFlattensTree(), TestBuildDisks(), TestBuildersOnEmptyAndMalformedInput(), TestBuildInterfaces() (+10 more)

### Community 110 - "sshStdioConn"
Cohesion: 0.07
Nodes (17): serveOneHTTPExchange(), serveSSHDockerConn(), TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), sshStdioConn, bufio.ReadWriter, golang.org/x/crypto/ssh.Channel (+9 more)

### Community 111 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 112 - "Handler"
Cohesion: 0.16
Nodes (8): applyConnectorScalarUpdates(), Handler, parseScheduleUpdates(), validateConnectorConfig(), validateRotationFields(), writeConfigRejection(), FieldError, updateConnectorRequest

### Community 113 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 114 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 115 - "Connector"
Cohesion: 0.16
Nodes (6): TestBuildPeerTableAttributes(), TestBuildPolicyTableAttributes(), buildPeerTable(), buildPolicyTable(), buildRouteTable(), Connector

### Community 116 - "Register"
Cohesion: 0.19
Nodes (18): init(), init(), init(), init(), init(), init(), init(), init() (+10 more)

### Community 117 - "changes/handlers_test.go"
Cohesion: 0.32
Nodes (13): Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound(), TestDismissSuccess() (+5 more)

### Community 118 - "Connector"
Cohesion: 0.16
Nodes (5): buildGatewayTable(), buildSystemContent(), primaryGatewayName(), wanInterfaceName(), Connector

### Community 119 - "truenas_test.go"
Cohesion: 0.24
Nodes (17): Connector, newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchIsStableAcrossCalls() (+9 more)

### Community 120 - "WebSocketProvider.tsx"
Cohesion: 0.09
Nodes (27): Client dispatch model, Envelope, Mock emitter (frontend-first), Naming convention, Reconnect behavior, Transport, WiseLabz WebSocket Contract (`/ws`), web_src_api_generated_changes_changes (+19 more)

### Community 121 - "templates.fixtures.ts"
Cohesion: 0.18
Nodes (11): web_src_api_model_index_docversion, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, resolveToken(), Snapshot (+3 more)

### Community 122 - "newTestHandler"
Cohesion: 0.26
Nodes (12): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), Handler, newTestHandler(), TestCreate() (+4 more)

### Community 123 - "templatefuncs.go"
Cohesion: 0.21
Nodes (11): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+3 more)

### Community 124 - "keyset_test.go"
Cohesion: 0.19
Nodes (16): Store, seedConnectorForChanges(), TestChangeRelatedServiceIDsAndPatternIDRoundTrip(), TestChangeRelatedServiceIDsDefaultsToEmptyArray(), TestCountRecentChangePatterns(), TestCountRecentChangesByPattern(), assertSameSet(), Store (+8 more)

### Community 125 - "scheduler_test.go"
Cohesion: 0.40
Nodes (12): New(), TestAddJobInvalidExpression(), TestAddJobRegistersAndFires(), TestContextGivenToJobFunction(), TestInvalidJobNameHandled(), TestJobContextDerivedFromStart(), TestJobSkipsOverlappingInvocations(), testLogger() (+4 more)

### Community 126 - "Contributing to WiseLabz"
Cohesion: 0.15
Nodes (13): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+5 more)

### Community 127 - ".call"
Cohesion: 0.44
Nodes (6): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture

### Community 128 - "NewService"
Cohesion: 0.30
Nodes (11): NewService(), TestConcurrentIssuePairUniqueTokenIDs(), TestElevationExpired(), TestElevationRequiresOwner(), TestElevationWrongAction(), TestExpiredAccessToken(), TestIssueAndValidateAccess(), TestIssueAndValidateElevation() (+3 more)

### Community 129 - "traefik_test.go"
Cohesion: 0.25
Nodes (16): Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchSelectiveFields() (+8 more)

### Community 130 - "Decision"
Cohesion: 0.17
Nodes (11): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 131 - "scripts"
Cohesion: 0.17
Nodes (12): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+4 more)

### Community 133 - "system/handlers_test.go"
Cohesion: 0.23
Nodes (15): Handler, newTestHandler(), TestDiagnostics(), TestExportAudit(), TestExportImportBackupRoundTrip(), TestGetBackupScheduleDefault(), TestGetRetentionSettingsDefault(), TestHealth() (+7 more)

### Community 134 - "Store"
Cohesion: 0.33
Nodes (3): Store, scanConnectorGrants(), ConnectorGrant

### Community 135 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 136 - "WiseLabz Connector Guide"
Cohesion: 0.17
Nodes (12): Conventions, Dependencies, Getting your connector merged, Health checks vs. sync, Keeping snapshots stable, Session-based and multi-flavour APIs, Sync flow, Testing without a real instance (+4 more)

### Community 137 - "Product"
Cohesion: 0.18
Nodes (10): Accessibility & Inclusion, Anti-references, Brand Personality, Design Principles, Locked frontend direction (planning session, 2026-06; revised 2026-09), Product, Product decisions (pre-planning, v1), Product Purpose (+2 more)

### Community 138 - "alerts/handlers_authz_test.go"
Cohesion: 0.30
Nodes (9): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz(), net/http.HandlerFunc (+1 more)

### Community 139 - "handlers_bulk_test.go"
Cohesion: 0.47
Nodes (9): bulkReq(), bulkResults(), createBulkFakeConnector(), Handler, registerBulkFakeConnector(), TestBulkReauth(), TestBulkRestart(), TestBulkSync() (+1 more)

### Community 140 - "backup/backup_test.go"
Cohesion: 0.25
Nodes (15): ExportToFile(), Import(), newTestStore(), TestExportIncludesRecordsBeyondAPage(), TestExportToFile(), TestExportToFileCreatesDirectory(), TestExportToFileDirNotWritable(), TestExportToFilePermissions() (+7 more)

### Community 141 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 142 - "docs_test.go"
Cohesion: 0.36
Nodes (9): testApp, seedDoc(), TestDocLockConflict(), TestDocLockHappyPath(), TestDocLockRoleBoundary(), TestDocsListAndGetSuccess(), TestDocsSaveRoleBoundary(), TestDocsSaveSuccess() (+1 more)

### Community 143 - "DocRecord"
Cohesion: 0.20
Nodes (6): docSearchWhere(), escapeLike(), DocRecord, Store, scanDoc(), scanDocSummary()

### Community 144 - "transform.go"
Cohesion: 0.43
Nodes (5): init(), RegisterTransformer(), TestRunTransformersAppliesInOrderAndStopsOnError(), Transformer, TransformerFunc

### Community 145 - "Changelog"
Cohesion: 0.20
Nodes (9): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features, Unreleased (+1 more)

### Community 146 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 147 - "diagnostics/diagnostics.go"
Cohesion: 0.29
Nodes (13): CheckHealth(), Collect(), collectVersions(), AuthProviders, Bundle, Component, Health, OIDCProviderSummary (+5 more)

### Community 148 - "changes_test.go"
Cohesion: 0.26
Nodes (12): testApp, seedChange(), seedChangeWithSeverity(), TestChangesAcknowledgeRoleBoundary(), TestChangesAcknowledgeSuccess(), TestChangesBulkResolveEmptyIDs(), TestChangesBulkResolveInvalidStatus(), TestChangesBulkResolvePartialFailure() (+4 more)

### Community 149 - "openapi_contract_test.go"
Cohesion: 0.33
Nodes (8): normalizeParams(), routerOperations(), specOperations(), TestAPIV1AliasServesSameHandlers(), TestOpenAPIHealthProbeRoutes(), TestOpenAPIMatchesRouter(), chi.Routes, go_pkg_go_yaml_in_yaml_v3

### Community 150 - "unifi/tables_test.go"
Cohesion: 0.22
Nodes (12): byExternalID(), TestBuildClientSummary(), TestBuildClientSummaryGroupsUnknown(), TestBuildDeviceTable(), TestBuildersHandleEmptyAndMalformedPayloads(), TestBuildFirewallTable(), TestBuildNetworkTable(), TestBuildNetworkTableDefaultsEnabled() (+4 more)

### Community 151 - "Config"
Cohesion: 0.33
Nodes (6): Config, chi.Router, NewRouter(), spaHandler(), wsRoleLabel(), io/fs.FS

### Community 153 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 155 - "Store"
Cohesion: 0.38
Nodes (3): Store, scanBackupRun(), BackupRun

### Community 156 - "migrations_parity_test.go"
Cohesion: 0.52
Nodes (6): collectColumns(), migrationFiles(), postgresSchemaColumns(), sqliteSchemaColumns(), TestMigrationSchemaParity(), TestMigrationVersionParity()

### Community 157 - "APIKeyClaims"
Cohesion: 0.40
Nodes (3): testAPIKeyChecker, APIKeyClaims, validAPIKey()

### Community 158 - "Step by step"
Cohesion: 0.25
Nodes (8): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Step by step

### Community 159 - "WiseLabz"
Cohesion: 0.25
Nodes (8): Code of Conduct, Configuration, Contributing, Features, License, Quick start, Supported services, WiseLabz

### Community 160 - "CORS"
Cohesion: 0.47
Nodes (4): CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders()

### Community 161 - "engine_maintenance_test.go"
Cohesion: 0.60
Nodes (5): driftingSnapshot(), setupMaintenanceTestConnector(), TestRunSyncExpiredMaintenanceWindowBehavesNormally(), TestRunSyncNoMaintenanceWindowBehavesNormally(), TestRunSyncSuppressesChangesDuringMaintenanceWindow()

### Community 162 - "retention/retention_test.go"
Cohesion: 0.35
Nodes (9): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupSkipsDisabledCategories(), RetentionSettings (+1 more)

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

### Community 168 - "main.tsx"
Cohesion: 0.40
Nodes (4): react-dom, App(), USE_MOCKS, web_src_index

### Community 169 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 170 - "seedScopeFixture"
Cohesion: 0.60
Nodes (4): Store, seedScopeFixture(), TestListDocSectionEmbeddingsFiltersByGrant(), TestMergedAttentionItemsFiltersByGrant()

### Community 171 - "compliance_rules_test.go"
Cohesion: 0.47
Nodes (5): TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails()

### Community 174 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 176 - "RequireConnectorRole"
Cohesion: 0.29
Nodes (6): ConnectorRoleChecker, chi.Router, mountConnectorRoutes(), RequireConnectorRole(), TestRequireConnectorRole(), TestRequireConnectorRoleCheckerError()

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

### Community 188 - "AlertNotifier"
Cohesion: 0.30
Nodes (3): TestRequestedFields(), TestWantsField(), AlertNotifier

## Knowledge Gaps
- **519 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `bulkResolveRequest`, `bulkResolveItemResult` (+514 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1135 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **19 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Store` connect `Store` to `Handler`, `testing.T`, `alerts/handlers_authz_test.go`, `go_pkg_context`, `backup/backup_test.go`, `ServiceSnapshot`, `diagnostics/diagnostics.go`, `dispatcher_test.go`, `router.go`, `Config`, `net/http.Request`, `backup/backup.go`, `Handler`, `ConnectorRecord`, `share_links_test.go`, `RunMigrations`, `engine_maintenance_test.go`, `NewEngine`, `retention/retention_test.go`, `NewStore`, `NewEngine`, `Errorf`, `checker_test.go`, `testHandler`, `rewritePlaceholders`, `Hub`, `testApp`, `Dispatcher`, `main`, `notifications/handlers_test.go`, `NewRegistry`, `newHandler`, `Handler`, `compliance/handlers.go`, `log/slog.Logger`, `chat/chat.go`, `Handler`, `.call`?**
  _High betweenness centrality (0.017) - this node is a cross-community bridge._
- **Why does `queryPlan()` connect `keyset_test.go` to `context.Context`, `testing.T`?**
  _High betweenness centrality (0.013) - this node is a cross-community bridge._
- **Why does `createTestNotification()` connect `newDocTestStore` to `context.Context`, `testing.T`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Are the 196 inferred relationships involving `newTestApp()` (e.g. with `TestAlertsBulkSnoozePartialFailure()` and `TestAlertsBulkSnoozeRejectsTooManyIDs()`) actually correct?**
  _`newTestApp()` has 196 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _519 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.02757805574706983 - nodes in this community are weakly interconnected._
- **Should `newDocTestStore` be split into smaller, more focused modules?**
  _Cohesion score 0.029302832244008713 - nodes in this community are weakly interconnected._