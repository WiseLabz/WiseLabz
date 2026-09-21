# Graph Report - issue-264-2  (2026-09-21)

## Corpus Check
- 668 files · ~373,052 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 17 file(s) not represented in the graph (top: (none) 10, .toml 2, .example 1)

## Summary
- 4779 nodes · 14683 edges · 188 communities (168 shown, 20 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1120 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `e986f853`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestApp
- testing.T
- newDocTestStore
- react
- context.Context
- @tanstack/react-query
- DashboardPage.tsx
- icons.tsx
- cn
- go_pkg_net_http
- go_pkg_context
- newTestHandler
- RulesPage.tsx
- Errorf
- ServiceSnapshot
- go_pkg_testing
- App.tsx
- ServiceDetailPage.tsx
- Store
- UsersPage.tsx
- share_links_test.go
- NewEngine
- package.json
- WebSocketProvider.tsx
- fixtures.ts
- go_pkg_github_com_wiselabz_wiselabz_internal_connector
- net/http.ResponseWriter
- docker_test.go
- WiseLabz — Architecture & Technical Decisions
- backup/backup.go
- time.Time
- dispatcher_test.go
- dependencies
- TemplateEditorPage.tsx
- router.go
- DBTX
- net/http.Request
- rowScanner
- testApp
- DecodeKey
- routerDeps
- checker_test.go
- ThemeControls.tsx
- DecodeJSON
- nilToStr
- settings.mock.ts
- OIDCProvider
- NewStore
- Register
- ConnectorRecord
- ParseConnectorConfig
- newTestHandler
- connector/connector.go
- compliance/engine.go
- Dispatcher
- src/theme.ts
- handlers.ts
- log/slog.Logger
- chat/chat.go
- timeline.ts
- RunMigrations
- registry.go
- Checker
- WiseLabz — Design Contract
- net/http.Client
- NewRegistry
- newTestHandler
- devDependencies
- MarshalConnectorConfig
- logging.go
- NewTimeoutError
- main
- middleware_test.go
- Compare
- Service
- Sanitize
- response.go
- connectors_health_test.go
- Connector
- Configuration & Documentation Backup (Export/Import)
- SuggestRequest
- Handler
- diagnostics/diagnostics.go
- compilerOptions
- Handler
- Connector
- docdiffmodel.ts
- ws.ts
- templates_test.go
- config_test.go
- notifications/handlers_test.go
- compilerOptions
- vectorCache
- .batchDelete
- time.Duration
- templates.fixtures.ts
- AuthMiddleware
- changes/handlers_test.go
- ValidateRefSegment
- Connector
- bulkFakeConnector
- AppShell.tsx
- middleware.go
- config_cmd_test.go
- changes_test.go
- New
- .Fetch
- templatefuncs.go
- scheduler_test.go
- AuditRecord
- migrations.go
- Contributing to WiseLabz
- Handler
- NewService
- Config
- config/validate_test.go
- GuardedDialer
- Connector
- store/backup_test.go
- Store
- Engine
- Decision
- main.tsx
- scripts
- Connector
- responseWriter
- RunCleanupOnce
- Store
- WiseLabz Connector Guide
- Decision
- Product
- provider_test.go
- .call
- Store
- connectors_maintenance_test.go
- docs_test.go
- all.go
- sendWebhook
- NewMalformedResponseError
- transform_test.go
- Changelog
- mockServiceWorker.js
- net/http.Handler
- WithAuth
- ComplianceRuleRecord
- .Fetch
- Contributor Covenant Code of Conduct
- release-please-config.json
- dashboard/handlers_test.go
- RateLimit
- .Fetch
- ShareLink
- Cache
- Step by step
- WiseLabz
- runbooks/handlers_test.go
- scanMaintenanceWindow
- computeNextRun
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- APIKeyClaims
- .UpdateAuthConfig
- cloudflare/attributes_test.go
- docker/snapshot.go
- Store
- engine_maintenance_test.go
- Mermaid.tsx
- Security Policy
- auth/handlers_test.go
- CORS
- seedScopeFixture
- Enforcement Guidelines
- @vitejs/plugin-react
- compose-smoke.sh
- ClassifyHealth
- timeoutError
- MISSING — deferred & future frontend features
- Saved Views
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
1. `newTestApp()` - 201 edges
2. `Errorf()` - 151 edges
3. `Store` - 115 edges
4. `newDocTestStore()` - 113 edges
5. `react` - 72 edges
6. `cn()` - 69 edges
7. `UserIDFromContext()` - 67 edges
8. `NewStore()` - 62 edges
9. `@tanstack/react-query` - 57 edges
10. `react-i18next` - 57 edges

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

## Communities (188 total, 20 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.02
Nodes (157): testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow(), TestAlertsListSuccess() (+149 more)

### Community 1 - "testing.T"
Cohesion: 0.02
Nodes (136): TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens(), TestClaudeSuggestErrors(), TestClaudeSuggestMultipleContentBlocks(), TestOpenAICompatibleSuggest(), TestOpenAICompatibleSuggestErrors(), TestOIDCRedirectURL(), TestFindOIDCProvider() (+128 more)

### Community 2 - "newDocTestStore"
Cohesion: 0.03
Nodes (124): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+116 more)

### Community 3 - "react"
Cohesion: 0.05
Nodes (94): Frontend, match-sorter, motion, @radix-ui/react-popover, react, react-i18next, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve (+86 more)

### Community 4 - "context.Context"
Cohesion: 0.04
Nodes (28): fakeStatusChecker, sanitizeSessions(), Connector, Store, existingIDs(), placeholders(), AlertRecord, ChangeRecord (+20 more)

### Community 5 - "@tanstack/react-query"
Cohesion: 0.03
Nodes (67): i18next, msw, react-router-dom, @tanstack/react-query, @testing-library/react, vitest, web_src_api_generated_chat_chat, web_src_api_generated_chat_chat_getgetchatconversationsidquerykey (+59 more)

### Community 6 - "DashboardPage.tsx"
Cohesion: 0.03
Nodes (86): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress`, 3. `sync.complete`, 4. `change.detected` (+78 more)

### Community 7 - "icons.tsx"
Cohesion: 0.04
Nodes (80): web_src_api_generated_attention_attention, web_src_api_generated_attention_attention_usegetattention, web_src_api_generated_changes_changes_getgetchangeschangeidquerykey, web_src_api_generated_changes_changes_postchangeschangeidack, web_src_api_generated_changes_changes_postchangeschangeidaiupdate, web_src_api_generated_changes_changes_postchangeschangeiddismiss, web_src_api_generated_changes_changes_postchangeschangeidexplain, web_src_api_generated_changes_changes_usegetchangeschangeid (+72 more)

### Community 8 - "cn"
Cohesion: 0.04
Nodes (82): web_src_api_generated_notifications_notifications, web_src_api_generated_notifications_notifications_getgetnotificationsquerykey, web_src_api_generated_notifications_notifications_postnotificationsnotificationidread, web_src_api_generated_notifications_notifications_postnotificationsreadall, web_src_api_generated_notifications_notifications_usegetnotifications, web_src_api_generated_notifications_notifications_usegetnotificationsdeliveries, web_src_api_generated_settings_settings_getgetaiconfigfallbackprovidersquerykey, web_src_api_generated_settings_settings_getgetaiconfigquerykey (+74 more)

### Community 9 - "go_pkg_net_http"
Cohesion: 0.08
Nodes (29): StatusError, bulkSnoozeItemResult, bulkSnoozeRequest, shareLinkContextKey, go_pkg_crypto_rand, go_pkg_crypto_subtle, go_pkg_encoding_base64, go_pkg_errors (+21 more)

### Community 10 - "go_pkg_context"
Cohesion: 0.09
Nodes (17): dashboardLayout, versionSections(), TemplateVersionSection, contains(), searchString(), go_pkg_context, go_pkg_database_sql, go_pkg_fmt (+9 more)

### Community 11 - "newTestHandler"
Cohesion: 0.06
Nodes (67): AssertMatchesSpec(), loadSpec(), specPath(), actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews() (+59 more)

### Community 12 - "RulesPage.tsx"
Cohesion: 0.04
Nodes (60): web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_usegetauthapikeys, web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey, web_src_api_generated_compliance_compliance_postcompliancerules (+52 more)

### Community 13 - "Errorf"
Cohesion: 0.07
Nodes (21): Handler, newToken(), Handler, Handler, Handler, Handler, contextWithShareLink(), Handler (+13 more)

### Community 14 - "ServiceSnapshot"
Cohesion: 0.05
Nodes (17): healthFakeConnector, noopValidatedConnector, Connector, ServiceSnapshot, changePatternID(), Engine, markError(), runTransformers() (+9 more)

### Community 15 - "go_pkg_testing"
Cohesion: 0.08
Nodes (10): go_pkg_encoding_json, go_pkg_github_com_wiselabz_wiselabz_internal_api_apitest, go_pkg_github_com_wiselabz_wiselabz_internal_config, go_pkg_io_fs, go_pkg_net_http_httptest, go_pkg_reflect, go_pkg_strconv, go_pkg_strings (+2 more)

### Community 16 - "App.tsx"
Cohesion: 0.05
Nodes (47): Audit Trail, Endpoint, Retention, What's not recorded, What's recorded, web_src_api_generated_connectors_connectors, web_src_api_generated_connectors_connectors_usegetconnectors, AiPage (+39 more)

### Community 17 - "ServiceDetailPage.tsx"
Cohesion: 0.04
Nodes (54): ADR-0001, ADR-0003, RFC-3339, 1. `service.status`, web_src_api_generated_changes_changes_usegetchanges, web_src_api_generated_connectors_connectors_getgetconnectorsquerykey, web_src_api_generated_connectors_connectors_postconnectors, web_src_api_generated_connectors_connectors_postconnectorsconnectoridconfigpush (+46 more)

### Community 18 - "Store"
Cohesion: 0.06
Nodes (29): Provider, Config, Handler, Registry, NewHandler(), NewHandler(), NewHandler(), NewHandler() (+21 more)

### Community 19 - "UsersPage.tsx"
Cohesion: 0.06
Nodes (46): axios, customInstance(), web_src_api_generated_me_me, web_src_api_generated_me_me_usegetme, web_src_api_generated_users_users, web_src_api_generated_users_users_deleteusersuserid, web_src_api_generated_users_users_getgetusersquerykey, web_src_api_generated_users_users_postusersuseridresetpassword (+38 more)

### Community 20 - "share_links_test.go"
Cohesion: 0.12
Nodes (54): GrantConnectorRole(), instanceAdminRole(), NewUser(), Handler, newHandler(), serve(), TestConversationOwnership(), TestCreateConversationDocVisibility() (+46 more)

### Community 21 - "NewEngine"
Cohesion: 0.09
Nodes (43): SnapshotEntity, tryParseEntities(), entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks() (+35 more)

### Community 22 - "package.json"
Cohesion: 0.04
Nodes (44): clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks (+36 more)

### Community 23 - "WebSocketProvider.tsx"
Cohesion: 0.06
Nodes (40): Client dispatch model, Envelope, Mock emitter (frontend-first), Naming convention, Reconnect behavior, Transport, WiseLabz WebSocket Contract (`/ws`), AXIOS_INSTANCE (+32 more)

### Community 24 - "fixtures.ts"
Cohesion: 0.06
Nodes (41): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+33 more)

### Community 25 - "go_pkg_github_com_wiselabz_wiselabz_internal_connector"
Cohesion: 0.09
Nodes (22): TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), TestBuildHostOverrideTableMalformedCases(), TestBuildHostOverrideTableValidOverrides(), TestBuildPeerTableAttributes(), TestBuildPolicyTableAttributes(), buildPeerTable() (+14 more)

### Community 26 - "net/http.ResponseWriter"
Cohesion: 0.08
Nodes (11): Handler, Handler, Handler, Handler, Handler, Handler, cron.EntryID, Handler (+3 more)

### Community 27 - "docker_test.go"
Cohesion: 0.07
Nodes (41): buildDockerTLSConfig(), newDockerClient(), newTCPDockerClient(), init(), generateSelfSignedCert(), generateSSHHostKey(), startSSHDockerServer(), TestConfigPush() (+33 more)

### Community 28 - "WiseLabz — Architecture & Technical Decisions"
Cohesion: 0.05
Nodes (40): 0001 — Lab-mutating operation boundaries, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+32 more)

### Community 29 - "backup/backup.go"
Cohesion: 0.12
Nodes (36): connectorIDs(), docIDs(), Export(), exportDocs(), exportTemplates(), ExportToFile(), exportWithin(), AIConfigSummary (+28 more)

### Community 30 - "time.Time"
Cohesion: 0.05
Nodes (19): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), digestDue(), formatDigest(), Dispatcher, TestDigestDue(), registryTestRefresher (+11 more)

### Community 31 - "dispatcher_test.go"
Cohesion: 0.21
Nodes (39): NewDispatcher(), deliveriesFor(), findDelivery(), newTestStore(), setChannelAndRoutingConfig(), setChannelConfig(), setWebhookConfig(), TestNotifyAlert_ConnectorCategoryAndIDFilterBothMatch() (+31 more)

### Community 32 - "dependencies"
Cohesion: 0.05
Nodes (40): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+32 more)

### Community 33 - "TemplateEditorPage.tsx"
Cohesion: 0.07
Nodes (34): web_src_api_generated_docs_docs_usegetdocstemplateschema, web_src_api_generated_templates_templates, web_src_api_generated_templates_templates_getgettemplatesquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidversionsquerykey, web_src_api_generated_templates_templates_posttemplatestemplateidpreview, web_src_api_generated_templates_templates_posttemplatestemplateidversionsrevrestore, web_src_api_generated_templates_templates_puttemplatestemplateid (+26 more)

### Community 34 - "router.go"
Cohesion: 0.09
Nodes (33): changePromptData(), stripPromptTags(), truncateUTF8(), bulkResolveItemResult, bulkResolveRequest, go_pkg_github_com_wiselabz_wiselabz_internal_api, go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts, go_pkg_github_com_wiselabz_wiselabz_internal_api_apikeys (+25 more)

### Community 35 - "DBTX"
Cohesion: 0.07
Nodes (15): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), DBTX, database/sql.Result, database/sql.Row (+7 more)

### Community 36 - "net/http.Request"
Cohesion: 0.11
Nodes (12): noRedirect(), applyConnectorScalarUpdates(), Handler, NewHandler(), validTargetType(), Handler, intQuery(), Paginate() (+4 more)

### Community 37 - "rowScanner"
Cohesion: 0.09
Nodes (17): Store, scanBackupRun(), docSearchWhere(), escapeLike(), DocRecord, Store, scanDoc(), scanDocSummary() (+9 more)

### Community 38 - "testApp"
Cohesion: 0.08
Nodes (24): mustHashDummyPassword(), testHandler, Handler, instanceAdminRoleFor(), testApp, TestDashboardAdminDefaultPermissionGate(), TestDashboardResetRestoresAdminDefault(), testApp (+16 more)

### Community 39 - "DecodeKey"
Cohesion: 0.12
Nodes (22): ProviderConfig, Handler, primaryProviderConfig(), boolToInt(), Config, DecodeKey(), Decrypt(), DeriveKey() (+14 more)

### Community 40 - "routerDeps"
Cohesion: 0.13
Nodes (28): routerDeps, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountConnectorRoutes(), chi.Router (+20 more)

### Community 41 - "checker_test.go"
Cohesion: 0.20
Nodes (30): NewChecker(), createComplianceRule(), createComplianceSnapshot(), createConnector(), findings(), newTestStore(), TestCheckEmptyDetectsAndAutoResolves(), TestCheckFailingDetectsAndAutoResolves() (+22 more)

### Community 42 - "ThemeControls.tsx"
Cohesion: 0.09
Nodes (27): zustand, MotionProvider(), AppearancePage(), ChoiceGroup(), FONT_KEYS, OPT_KEYS, PRESET_KEYS, Segmented() (+19 more)

### Community 43 - "DecodeJSON"
Cohesion: 0.12
Nodes (19): updateUserRequest, Handler, sanitizeUser(), setRefreshCookie(), writeUserWriteError(), Handler, ClientIP(), hostOnly() (+11 more)

### Community 44 - "nilToStr"
Cohesion: 0.09
Nodes (9): nilToStr(), DeliveryStatus, Store, NotificationRecord, Store, scanNotification(), Store, TemplateSectionRecord (+1 more)

### Community 45 - "settings.mock.ts"
Cohesion: 0.08
Nodes (27): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role, web_src_api_model_index_session (+19 more)

### Community 46 - "OIDCProvider"
Cohesion: 0.12
Nodes (15): clearOIDCFlowCookie(), Handler, newOIDCUser(), oidcFlowCookieName(), randomOIDCToken(), readOIDCFlowCookie(), setOIDCFlowCookie(), validHostPort() (+7 more)

### Community 47 - "NewStore"
Cohesion: 0.14
Nodes (28): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+20 more)

### Community 48 - "Register"
Cohesion: 0.18
Nodes (26): RequestedFields(), TestRequestedFields(), Register(), init(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect() (+18 more)

### Community 49 - "ConnectorRecord"
Cohesion: 0.11
Nodes (17): ConnectorRecord, Store, scanConnector(), scanConnectorRows(), nullInt64ToIntPtr(), nullStrToStr(), FailedSyncRun, Store (+9 more)

### Community 50 - "ParseConnectorConfig"
Cohesion: 0.12
Nodes (12): Handler, isWritableField(), validateConfigPushRequest(), capitalize(), Handler, WriteElevationError(), ConfigPusher, Get() (+4 more)

### Community 51 - "newTestHandler"
Cohesion: 0.12
Nodes (23): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), spaHandler(), templateRequest(), TestListPagination() (+15 more)

### Community 52 - "connector/connector.go"
Cohesion: 0.09
Nodes (13): NewAuthError(), NewServiceUnavailableError(), Connector, isTimeout(), TestTypedErrorsAreDistinguishableByType(), TestTypedErrorsWrapAndUnwrap(), AuthError, CredentialRefresher (+5 more)

### Community 53 - "compliance/engine.go"
Cohesion: 0.14
Nodes (25): contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity, Rule (+17 more)

### Community 54 - "Dispatcher"
Cohesion: 0.20
Nodes (10): discordPayload(), findChannel(), findRoute(), Dispatcher, severityRank(), shouldSkipRoute(), slackPayload(), webhookPayload() (+2 more)

### Community 55 - "src/theme.ts"
Cohesion: 0.14
Nodes (25): @fontsource/space-mono, @fontsource-variable/space-grotesk, AdvancedControls(), ColorMode, commit(), load(), Persisted, PRESETS_FONTS (+17 more)

### Community 56 - "handlers.ts"
Cohesion: 0.07
Nodes (26): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+18 more)

### Community 57 - "log/slog.Logger"
Cohesion: 0.10
Nodes (14): newLogger(), runAlertExpirer(), Dispatcher, RunDeliveryRetries(), cron.EntryID, Runner, Store, RunDocLockSweep() (+6 more)

### Community 58 - "chat/chat.go"
Cohesion: 0.10
Nodes (22): buildPrompt(), TestBuildPrompt(), normalizeParams(), routerOperations(), specOperations(), TestOpenAPIMatchesRouter(), cosineSimilarity(), Match (+14 more)

### Community 59 - "timeline.ts"
Cohesion: 0.14
Nodes (17): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+9 more)

### Community 60 - "RunMigrations"
Cohesion: 0.19
Nodes (24): main(), OpenDB(), newPostgresTestStore(), newMigrator(), collectColumns(), postgresSchemaColumns(), sqliteSchemaColumns(), TestMigrationSchemaParity() (+16 more)

### Community 61 - "registry.go"
Cohesion: 0.14
Nodes (20): catalog(), AttributeCatalog(), IsCredentialRefresherType(), ListSchemas(), TestIsCredentialRefresherType(), TestRegisterStubRoundTrips(), ValidateConfig(), isConfigValidationError() (+12 more)

### Community 62 - "Checker"
Cohesion: 0.20
Nodes (7): complianceRule(), Checker, RunStaleSweepOnce(), QualityFindingRecord, scanQualityFinding(), FindingNotifier, RotationConfig

### Community 63 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 64 - "net/http.Client"
Cohesion: 0.09
Nodes (10): ollamaEmbedder, openAIEmbedder, Connector, LimitedBody(), Connector, isTimeout(), Connector, newWebhookClient() (+2 more)

### Community 65 - "NewRegistry"
Cohesion: 0.22
Nodes (21): SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail(), TestSuggestWithFallbackFirstProviderSucceeds(), TestSuggestWithFallbackNoProviders(), TestSuggestWithFallbackStopsOnNonRetryableError() (+13 more)

### Community 66 - "newTestHandler"
Cohesion: 0.16
Nodes (20): doJSON(), testHandler, req(), TestChangePassword(), TestChangePasswordRevokesAPIKeys(), TestCreateUser(), TestDeleteUser(), TestLogin() (+12 more)

### Community 67 - "devDependencies"
Cohesion: 0.09
Nodes (23): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+15 more)

### Community 68 - "MarshalConnectorConfig"
Cohesion: 0.15
Nodes (19): TestDiagnosticsRedactsSecrets(), RedactConnectorConfig(), TestAllConnectorImplementationsRegister(), GetTypeSchema(), TestRegisterDefaultsToNonStub(), IsSecretFieldType(), MarshalConnectorConfig(), SecretFieldsChanged() (+11 more)

### Community 69 - "logging.go"
Cohesion: 0.15
Nodes (18): loggablePath(), loggableQuery(), Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken(), TestLoggerRedactsWSTicket(), TestLoggablePathMasksShareTokenUnderV1() (+10 more)

### Community 70 - "NewTimeoutError"
Cohesion: 0.19
Nodes (9): NewTimeoutError(), ReadBody(), TestReadBodyLimit(), TestBuildHostsTableAttributes(), buildHostsTable(), isTimeout(), TestBuildHostsTableMalformedCases(), TestBuildHostsTableValidRecords() (+1 more)

### Community 71 - "main"
Cohesion: 0.15
Nodes (16): main(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), Embedder, EmbedRegistry (+8 more)

### Community 72 - "middleware_test.go"
Cohesion: 0.13
Nodes (18): fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, RequireInstanceAdmin(), assertElevationAuditCalls(), boolLabel(), contextWithInstanceAdmin(), requestWithUser() (+10 more)

### Community 73 - "Compare"
Cohesion: 0.15
Nodes (18): configPushLanded(), ServiceDependency, SnapshotSection, TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare(), lineCount() (+10 more)

### Community 74 - "Service"
Cohesion: 0.20
Nodes (10): Claims, ElevationClaims, ElevationToken, TokenPair, Service, hasAudience(), newTokenID(), go_pkg_github_com_golang_jwt_jwt_v5 (+2 more)

### Community 75 - "Sanitize"
Cohesion: 0.15
Nodes (7): decodeBulkRequest(), Handler, Handler, Sanitize(), TestSanitize(), bulkRequest, notificationConfigDoc

### Community 76 - "response.go"
Cohesion: 0.12
Nodes (15): TestParseScheduleUpdates(), parseScheduleUpdates(), validateConnectorConfig(), validateRotationFields(), Error(), ErrorWithDetails(), FieldError, HandleStoreError() (+7 more)

### Community 77 - "connectors_health_test.go"
Cohesion: 0.16
Nodes (17): testApp, init(), TestConnectorsCreateAcceptsValidConfig(), TestConnectorsCreateRejectsInvalidEnum(), TestConnectorsCreateRejectsMalformedConfig(), TestConnectorsSyncAcceptsFieldsHint(), TestConnectorsUpdateRejectsMalformedConfig(), waitForSyncRuns() (+9 more)

### Community 78 - "Connector"
Cohesion: 0.13
Nodes (4): ConfigField, buildRouteTable(), isTimeout(), Connector

### Community 79 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.11
Nodes (16): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, What's excluded, and why, What's included, Backups, PostgreSQL support (+8 more)

### Community 80 - "SuggestRequest"
Cohesion: 0.17
Nodes (6): claudeProvider, openAICompatibleProvider, StubProvider, SuggestChunk, SuggestRequest, countingProvider

### Community 81 - "Handler"
Cohesion: 0.18
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 82 - "diagnostics/diagnostics.go"
Cohesion: 0.22
Nodes (17): CheckHealth(), Collect(), collectVersions(), newTestStore(), TestCheckHealthReportsDegradedOnClosedDB(), TestCollectIncludesHealthVersionsAndSchedule(), TestCollectListsRecentFailures(), TestCollectRedactsConnectorSecrets() (+9 more)

### Community 83 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 84 - "Handler"
Cohesion: 0.24
Nodes (6): NewHandler(), response(), toRule(), validRecord(), Handler, RuleEvaluator

### Community 85 - "Connector"
Cohesion: 0.18
Nodes (5): buildGatewayTable(), isTimeout(), primaryGatewayName(), wanInterfaceName(), Connector

### Community 86 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 87 - "ws.ts"
Cohesion: 0.12
Nodes (16): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+8 more)

### Community 88 - "templates_test.go"
Cohesion: 0.26
Nodes (15): templateBody, TestTemplateMutationRoleMatrix(), testApp, seedPreviewConnector(), seedTemplate(), TestTemplatesConcurrentUpdatesCreateDistinctVersions(), TestTemplatesPreviewAffectedConnectors(), TestTemplatesPreviewCapturesMissingSnapshot() (+7 more)

### Community 89 - "config_test.go"
Cohesion: 0.17
Nodes (14): runHealthcheck(), Load(), TestAccessTokenTTLDuration(), TestLoadDefaults(), TestLoadEnvOverride(), TestLoadEnvOverrideAllFields(), TestLoadFromYAML(), TestLoadRejectsEmptyCronExpr() (+6 more)

### Community 90 - "notifications/handlers_test.go"
Cohesion: 0.28
Nodes (15): AuthedUser(), NewHandler(), decodePaginated(), jsonHasEmptyArrayItems(), newTestStore(), seedDelivery(), TestListDeliveriesEmpty(), TestListDeliveriesNoFilter() (+7 more)

### Community 91 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 92 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 94 - "time.Duration"
Cohesion: 0.17
Nodes (6): AuthSettings, Database, Server, time.Duration, OIDCProvider, PoolConfig

### Community 95 - "templates.fixtures.ts"
Cohesion: 0.18
Nodes (12): web_src_api_model_index_docversion, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, renderTemplate(), resolveToken() (+4 more)

### Community 96 - "AuthMiddleware"
Cohesion: 0.19
Nodes (10): APIKeyChecker, UserStatusChecker, TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed(), AuthMiddleware(), extractBearerToken() (+2 more)

### Community 97 - "changes/handlers_test.go"
Cohesion: 0.32
Nodes (13): Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound(), TestDismissSuccess() (+5 more)

### Community 98 - "ValidateRefSegment"
Cohesion: 0.22
Nodes (8): Connector, PathSegment(), TestValidateCompositeRef(), TestValidateRefSegment(), TestValidateUnixSocketPath(), ValidateCompositeRef(), ValidateRefSegment(), ValidateUnixSocketPath()

### Community 100 - "bulkFakeConnector"
Cohesion: 0.14
Nodes (3): actionConnector, bulkFakeConnector, failingPushConnector

### Community 101 - "AppShell.tsx"
Cohesion: 0.20
Nodes (10): Frontend shell & theme (decided 2026-06), react-error-boundary, sonner, AppShell, AppShell(), NavigatorBridge(), Dock(), ShellDock() (+2 more)

### Community 102 - "middleware.go"
Cohesion: 0.27
Nodes (9): AuditRecorder, contextKey, elevationError, PermissionChecker, elevationFailureReason(), recordElevationAudit(), RequireElevation(), RequirePermission() (+1 more)

### Community 103 - "config_cmd_test.go"
Cohesion: 0.23
Nodes (11): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), Schema(), schemaFor() (+3 more)

### Community 104 - "changes_test.go"
Cohesion: 0.26
Nodes (12): testApp, seedChange(), seedChangeWithSeverity(), TestChangesAcknowledgeRoleBoundary(), TestChangesAcknowledgeSuccess(), TestChangesBulkResolveEmptyIDs(), TestChangesBulkResolveInvalidStatus(), TestChangesBulkResolvePartialFailure() (+4 more)

### Community 105 - "New"
Cohesion: 0.17
Nodes (12): Handler, SyncDocEmbeddings(), TestSyncDocEmbeddingsKeepsOldRowsWhenEmbedFails(), TestRetrieveUsesCacheAndSyncInvalidates(), TestUpsertBackupSchedulePostgresParity(), TestUpsertQualityFindingPostgresDedup(), TestSnapshotAttributesRoundTripPostgres(), TestSnapshotAttributesRoundTripSQLite() (+4 more)

### Community 106 - ".Fetch"
Cohesion: 0.17
Nodes (11): TestAttributeCatalogCoversEmittedKeys(), TestBuildInterfaceTableAttributes(), TestBuildRuleTableAttributes(), buildGatewayTable(), buildInterfaceTable(), buildRuleTable(), buildSystemContent(), primaryGatewayName() (+3 more)

### Community 107 - "templatefuncs.go"
Cohesion: 0.21
Nodes (11): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+3 more)

### Community 108 - "scheduler_test.go"
Cohesion: 0.40
Nodes (12): New(), TestAddJobInvalidExpression(), TestAddJobRegistersAndFires(), TestContextGivenToJobFunction(), TestInvalidJobNameHandled(), TestJobContextDerivedFromStart(), TestJobSkipsOverlappingInvocations(), testLogger() (+4 more)

### Community 109 - "AuditRecord"
Cohesion: 0.32
Nodes (6): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), AuditRecord

### Community 110 - "migrations.go"
Cohesion: 0.18
Nodes (9): GetMigrationStatus(), TestGetMigrationStatus(), go_pkg_embed, go_pkg_github_com_golang_migrate_migrate_v4, go_pkg_github_com_golang_migrate_migrate_v4_database_postgres, go_pkg_github_com_golang_migrate_migrate_v4_database_sqlite3, go_pkg_github_com_golang_migrate_migrate_v4_source_file, go_pkg_github_com_golang_migrate_migrate_v4_source_iofs (+1 more)

### Community 111 - "Contributing to WiseLabz"
Cohesion: 0.15
Nodes (13): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+5 more)

### Community 113 - "NewService"
Cohesion: 0.30
Nodes (11): NewService(), TestConcurrentIssuePairUniqueTokenIDs(), TestElevationExpired(), TestElevationRequiresOwner(), TestElevationWrongAction(), TestExpiredAccessToken(), TestIssueAndValidateAccess(), TestIssueAndValidateElevation() (+3 more)

### Community 114 - "Config"
Cohesion: 0.27
Nodes (11): Config, LogSettings, AISettings, BackupSettings, EncryptionSettings, QualitySettings, RetentionSettings, RotationSettings (+3 more)

### Community 115 - "config/validate_test.go"
Cohesion: 0.23
Nodes (9): mask(), redactDSN(), redactKVPassword(), Config, TestEveryKeyEnvOverridable(), TestRedactDSN(), TestRedacted(), TestValidate() (+1 more)

### Community 116 - "GuardedDialer"
Cohesion: 0.26
Nodes (12): init(), GuardedDialer(), IsDangerousIP(), init(), init(), init(), init(), init() (+4 more)

### Community 118 - "store/backup_test.go"
Cohesion: 0.32
Nodes (11): newBackupTestStore(), TestCreateBackupRun(), TestGetBackupScheduleWhenNotExists(), TestListBackupRunsPaginated(), TestPruneBackupRunsByAge(), TestPruneBackupRunsByCount(), TestPruneBackupRunsCombinedLimits(), TestPruneBackupRunsNegativeMaxBackups() (+3 more)

### Community 119 - "Store"
Cohesion: 0.23
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 120 - "Engine"
Cohesion: 0.18
Nodes (5): Engine, sync.Map, AlertNotifier, DocRegenerator, QualityChecker

### Community 121 - "Decision"
Cohesion: 0.17
Nodes (11): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 122 - "main.tsx"
Cohesion: 0.21
Nodes (8): react-dom, App(), USE_MOCKS, web_src_index, bootstrap(), worker, enableMocks(), handlers

### Community 123 - "scripts"
Cohesion: 0.17
Nodes (12): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+4 more)

### Community 125 - "responseWriter"
Cohesion: 0.18
Nodes (7): serveOneHTTPExchange(), serveSSHDockerConn(), bufio.ReadWriter, golang.org/x/crypto/ssh.Channel, golang.org/x/crypto/ssh.ServerConfig, net.Conn, responseWriter

### Community 126 - "RunCleanupOnce"
Cohesion: 0.31
Nodes (9): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupSkipsDisabledCategories(), RetentionSettings (+1 more)

### Community 127 - "Store"
Cohesion: 0.33
Nodes (3): Store, scanConnectorGrants(), ConnectorGrant

### Community 128 - "WiseLabz Connector Guide"
Cohesion: 0.18
Nodes (9): Conventions, Dependencies, Getting your connector merged, Health checks vs. sync, Sync flow, Testing without a real instance, The Connector interface, The ServiceSnapshot (+1 more)

### Community 129 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 130 - "Product"
Cohesion: 0.18
Nodes (10): Accessibility & Inclusion, Anti-references, Brand Personality, Design Principles, Locked frontend direction (planning session, 2026-06; revised 2026-09), Product, Product decisions (pre-planning, v1), Product Purpose (+2 more)

### Community 131 - "provider_test.go"
Cohesion: 0.29
Nodes (6): testProvider, TestRegistryGet(), TestRegistryList(), TestStubProviderName(), TestStubProviderSuggest(), TestStubProviderSuggestStream()

### Community 132 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 133 - "Store"
Cohesion: 0.27
Nodes (3): sanitize(), APIKey, Store

### Community 134 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 135 - "docs_test.go"
Cohesion: 0.36
Nodes (9): testApp, seedDoc(), TestDocLockConflict(), TestDocLockHappyPath(), TestDocLockRoleBoundary(), TestDocsListAndGetSuccess(), TestDocsSaveRoleBoundary(), TestDocsSaveSuccess() (+1 more)

### Community 136 - "all.go"
Cohesion: 0.20
Nodes (9): go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense, go_pkg_github_com_wiselabz_wiselabz_internal_connector_pfsense, go_pkg_github_com_wiselabz_wiselabz_internal_connector_pihole (+1 more)

### Community 137 - "sendWebhook"
Cohesion: 0.27
Nodes (10): AllowLoopbackForTest(), redactURLError(), sendWebhook(), signWebhook(), TestSendWebhook_BlocksLoopbackAndLinkLocal(), TestSendWebhook_BoundsResponseRead(), TestSendWebhook_DoesNotFollowRedirects(), TestSendWebhook_ErrorOmitsURL() (+2 more)

### Community 138 - "NewMalformedResponseError"
Cohesion: 0.29
Nodes (3): NewMalformedResponseError(), Connector, MalformedResponseError

### Community 139 - "transform_test.go"
Cohesion: 0.24
Nodes (6): init(), normalizeEnabledColumn(), normalizeFirewallRules(), RegisterTransformer(), TestNormalizeFirewallRulesRewritesEnabledColumn(), Transformer

### Community 140 - "Changelog"
Cohesion: 0.20
Nodes (9): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features, Unreleased (+1 more)

### Community 141 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 142 - "net/http.Handler"
Cohesion: 0.25
Nodes (7): ConnectorRoleChecker, SecurityHeaders(), TestSecurityHeaders(), RequireConnectorRole(), TestRequireConnectorRole(), TestRequireConnectorRoleCheckerError(), net/http.Handler

### Community 143 - "WithAuth"
Cohesion: 0.36
Nodes (9): TestEmbeddedSPAWithoutFrontendBuild(), NewHandler(), TestCreate(), TestList(), TestRevoke(), JWTService(), Token(), WithAuth() (+1 more)

### Community 144 - "ComplianceRuleRecord"
Cohesion: 0.36
Nodes (4): changedFields(), ComplianceRuleRecord, Store, scanComplianceRule()

### Community 145 - ".Fetch"
Cohesion: 0.28
Nodes (5): WantsField(), Connector, TestWantsField(), agentEnabled(), Connector

### Community 146 - "Contributor Covenant Code of Conduct"
Cohesion: 0.22
Nodes (7): Attribution, Contributor Covenant Code of Conduct, Enforcement, Enforcement Responsibilities, Our Pledge, Our Standards, Scope

### Community 147 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 148 - "dashboard/handlers_test.go"
Cohesion: 0.43
Nodes (7): Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout()

### Community 149 - "RateLimit"
Cohesion: 0.29
Nodes (6): TestRateLimit(), RateLimit(), golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 150 - ".Fetch"
Cohesion: 0.32
Nodes (3): setHeaders(), validateCustomURL(), Connector

### Community 152 - "Cache"
Cohesion: 0.43
Nodes (5): Cache, New(), Cache[V], entry, V

### Community 153 - "Step by step"
Cohesion: 0.25
Nodes (8): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Step by step

### Community 154 - "WiseLabz"
Cohesion: 0.25
Nodes (8): Code of Conduct, Configuration, Contributing, Features, License, Quick start, Supported services, WiseLabz

### Community 155 - "runbooks/handlers_test.go"
Cohesion: 0.48
Nodes (6): Handler, newTestHandler(), TestCreate(), TestGetNotFound(), TestListMutuallyExclusiveFilters(), TestUpdateAndDelete()

### Community 156 - "scanMaintenanceWindow"
Cohesion: 0.48
Nodes (3): Store, scanMaintenanceWindow(), MaintenanceWindowRecord

### Community 157 - "computeNextRun"
Cohesion: 0.43
Nodes (5): TestComputeNextRun_BackoffNeverExceedsScheduleCadence(), TestComputeNextRun_FailureUsesBackoffSchedule(), TestComputeNextRun_ManualOnlyNeverSchedules(), TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(), computeNextRun()

### Community 158 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 159 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 160 - "APIKeyClaims"
Cohesion: 0.40
Nodes (3): testAPIKeyChecker, APIKeyClaims, validAPIKey()

### Community 162 - "cloudflare/attributes_test.go"
Cohesion: 0.33
Nodes (5): TestAttributeCatalogCoversEmittedKeys(), TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable()

### Community 165 - "engine_maintenance_test.go"
Cohesion: 0.60
Nodes (5): driftingSnapshot(), setupMaintenanceTestConnector(), TestRunSyncExpiredMaintenanceWindowBehavesNormally(), TestRunSyncNoMaintenanceWindowBehavesNormally(), TestRunSyncSuppressesChangesDuringMaintenanceWindow()

### Community 166 - "Mermaid.tsx"
Cohesion: 0.47
Nodes (4): mermaid, cssVar(), Mermaid(), resolveColor()

### Community 167 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 168 - "auth/handlers_test.go"
Cohesion: 0.40
Nodes (4): TestEmailDomainAllowed(), TestOIDCRoleForGroups(), emailDomainAllowed(), oidcRoleForGroups()

### Community 169 - "CORS"
Cohesion: 0.60
Nodes (4): CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders()

### Community 170 - "seedScopeFixture"
Cohesion: 0.60
Nodes (4): Store, seedScopeFixture(), TestListDocSectionEmbeddingsFiltersByGrant(), TestMergedAttentionItemsFiltersByGrant()

### Community 171 - "Enforcement Guidelines"
Cohesion: 0.40
Nodes (5): 1. Correction, 2. Warning, 3. Temporary Ban, 4. Permanent Ban, Enforcement Guidelines

### Community 172 - "@vitejs/plugin-react"
Cohesion: 0.40
Nodes (3): @tailwindcss/vite, vite, @vitejs/plugin-react

### Community 173 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 174 - "ClassifyHealth"
Cohesion: 0.67
Nodes (3): ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold()

### Community 176 - "MISSING — deferred & future frontend features"
Cohesion: 0.50
Nodes (3): Deferred from V1 (decided during planning), MISSING — deferred & future frontend features, Suggested-later (raised in build, not yet planned)

### Community 177 - "Saved Views"
Cohesion: 0.50
Nodes (3): Endpoints, Saved Views, Scope

## Knowledge Gaps
- **513 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `bulkResolveRequest`, `bulkResolveItemResult` (+508 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1079 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **20 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `createTestConnector()` connect `newDocTestStore` to `testing.T`, `context.Context`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **Why does `createTestNotification()` connect `newDocTestStore` to `testing.T`, `context.Context`?**
  _High betweenness centrality (0.020) - this node is a cross-community bridge._
- **Why does `Store` connect `Store` to `.call`, `go_pkg_context`, `Errorf`, `ServiceSnapshot`, `WithAuth`, `share_links_test.go`, `NewEngine`, `net/http.ResponseWriter`, `backup/backup.go`, `time.Time`, `dispatcher_test.go`, `DBTX`, `net/http.Request`, `engine_maintenance_test.go`, `testApp`, `checker_test.go`, `DecodeJSON`, `NewStore`, `Register`, `newTestHandler`, `Dispatcher`, `log/slog.Logger`, `RunMigrations`, `Checker`, `NewRegistry`, `diagnostics/diagnostics.go`, `Handler`, `notifications/handlers_test.go`, `New`, `migrations.go`, `Handler`, `store/backup_test.go`, `Engine`, `RunCleanupOnce`?**
  _High betweenness centrality (0.017) - this node is a cross-community bridge._
- **Are the 190 inferred relationships involving `newTestApp()` (e.g. with `TestAlertsBulkSnoozePartialFailure()` and `TestAlertsBulkSnoozeRejectsTooManyIDs()`) actually correct?**
  _`newTestApp()` has 190 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _513 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.022703818369453045 - nodes in this community are weakly interconnected._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.024669774669774668 - nodes in this community are weakly interconnected._