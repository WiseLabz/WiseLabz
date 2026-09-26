# Graph Report - feat-snapshot-browser-and-time-travel-diff  (2026-09-26)

## Corpus Check
- 857 files · ~522,427 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 21 file(s) not represented in the graph (top: (none) 10, .toml 2, .tmpl 2)

## Summary
- 6340 nodes · 20091 edges · 208 communities (190 shown, 18 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1634 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `deb9c831`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestApp
- ServiceDetailPage.tsx
- newDocTestStore
- testing.T
- context.Context
- DashboardPage.tsx
- AlertsPage.tsx
- NewChecker
- cn
- go_pkg_net_http
- go_pkg_context
- go_pkg_time
- @tanstack/react-query
- UserIDFromContext
- net/http.Client
- go_pkg_testing
- Handler
- App.tsx
- UsersPage.tsx
- ServiceSnapshot
- User
- package.json
- router.go
- Get
- rowScanner
- Store
- fixtures.ts
- DecodeKey
- WiseLabz — Architecture & Technical Decisions
- WebSocketProvider.tsx
- RulesPage.tsx
- snapshotdiff.go
- dispatcher_test.go
- net/http.ResponseWriter
- docker_test.go
- mountAPIRoutes
- response.go
- SnapshotEntity
- home_assistant/tables.go
- react
- RunMigrations
- NewEngine
- dependencies
- Connector
- portainer/tables.go
- Dispatcher
- net/http.Request
- adguardhome/tables.go
- log/slog.Logger
- Compare
- ThemeControls.tsx
- New
- traefik/tables.go
- Hub
- ExportToFile
- unifi/tables.go
- Configuration & Documentation Backup (Export/Import)
- share_links_test.go
- go_pkg_os
- Connector
- traefik_test.go
- Connector
- settings.mock.ts
- GetTypeSchema
- NewMalformedResponseError
- rewritePlaceholders
- nilToStr
- NewStore
- home_assistant_test.go
- SuggestRequest
- NewService
- newTestHandler
- config_test.go
- Service
- Store
- handlers.ts
- ws/ws_test.go
- logging_test.go
- NewEngine
- unifi_test.go
- timeline.ts
- AuthedUser
- Connector
- AppearancePage.tsx
- NewRegistry
- newTestHandler
- WiseLabz — Design Contract
- devDependencies
- Manager
- New
- Config
- portainer_test.go
- ConnectorRecord
- Handler
- NewHTTPClient
- adguardhome_test.go
- DecodeJSON
- docs/handlers_test.go
- templates.fixtures.ts
- time.Time
- Runner
- main
- connector_permission.go
- .batchDelete
- Connector
- export_test.go
- Deps
- Handler
- Handler
- fetch_test.go
- truenas_test.go
- diagnostics/diagnostics.go
- keyset_test.go
- ws.ts
- compilerOptions
- connectors_health_test.go
- Register
- gitTarget
- MarshalConnectorConfig
- time.Duration
- docdiffmodel.ts
- quality_test.go
- templates_test.go
- HashToken
- JobHealthRecord
- chat/chat.go
- all.go
- httpx/retry_test.go
- compilerOptions
- testApp
- Retrieve
- Handler
- vectorCache
- config/validate_test.go
- decodePaginated
- snapshotreport.go
- gitFixture
- DocRecord
- apikey_scope.go
- RequirePermission
- config_cmd_test.go
- changes/handlers_test.go
- registryTestRefresher
- newTestHandler
- fakeRefresherConnector
- .GetConnectorUptime
- truenas/attributes_test.go
- api/changes_test.go
- dockerSSHAddr
- fakeEmbedder
- createUser
- Contributing to WiseLabz
- scripts
- Engine
- fakeQualityChecker
- NewClient
- Store
- Store
- Decision
- main.tsx
- dialSSHStdio
- Decision
- WiseLabz Connector Guide
- Product
- .call
- api/auth/oidc.go
- .call
- connector/validate_test.go
- Changelog
- mockServiceWorker.js
- apikey_scopes_test.go
- ComplianceRuleRecord
- retention/retention_test.go
- release-please-config.json
- connectors_hardening_test.go
- dashboard/handlers_test.go
- RateLimit
- Step by step
- WiseLabz
- TestComplianceRuleValidation
- snapshotResponse
- scanMaintenanceWindow
- Contributor Covenant Code of Conduct
- Audit Trail
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- walkCursorPages
- Store
- engine_maintenance_test.go
- Mermaid.tsx
- Security Policy
- net/http.Handler
- routerOperations
- seedScopeFixture
- Enforcement Guidelines
- MfaEnrollDialog
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
2. `Errorf()` - 182 edges
3. `newDocTestStore()` - 142 edges
4. `Store` - 141 edges
5. `UserIDFromContext()` - 81 edges
6. `SnapshotEntity` - 77 edges
7. `react` - 75 edges
8. `cn()` - 69 edges
9. `NewStore()` - 66 edges
10. `@tanstack/react-query` - 62 edges

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

## Communities (208 total, 18 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.02
Nodes (176): testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow(), TestAlertsListSuccess() (+168 more)

### Community 1 - "ServiceDetailPage.tsx"
Cohesion: 0.02
Nodes (132): ADR-0001, ADR-0003, RFC-3339, 1. `service.status`, match-sorter, motion, @radix-ui/react-popover, web_src_api_generated_connectors_connectors (+124 more)

### Community 2 - "newDocTestStore"
Cohesion: 0.03
Nodes (140): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+132 more)

### Community 3 - "testing.T"
Cohesion: 0.02
Nodes (147): TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens(), TestClaudeSuggestErrors(), TestClaudeSuggestMultipleContentBlocks(), TestOpenAICompatibleSuggest(), TestOpenAICompatibleSuggestErrors(), TestOIDCRedirectURL(), TestFindOIDCProvider() (+139 more)

### Community 4 - "context.Context"
Cohesion: 0.03
Nodes (31): fakeStatusChecker, sanitizeSessions(), MFAEnrollOnlyFromContext(), Connector, Connector, existingIDs(), SnapshotRecord, Store (+23 more)

### Community 5 - "DashboardPage.tsx"
Cohesion: 0.04
Nodes (78): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress`, 3. `sync.complete`, 4. `change.detected` (+70 more)

### Community 6 - "AlertsPage.tsx"
Cohesion: 0.05
Nodes (52): Frontend, 7. `quality.finding.created` and `quality.findings.changed`, web_src_api_generated_alerts_alerts, web_src_api_generated_alerts_alerts_getgetalertsquerykey, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve, web_src_api_generated_alerts_alerts_postalertsalertidsnooze, web_src_api_generated_alerts_alerts_postalertsbulksnooze (+44 more)

### Community 7 - "NewChecker"
Cohesion: 0.06
Nodes (72): contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity, Rule (+64 more)

### Community 8 - "cn"
Cohesion: 0.03
Nodes (128): react-i18next, sonner, web_src_api_generated_changes_changes_getgetchangeschangeidquerykey, web_src_api_generated_changes_changes_postchangeschangeidack, web_src_api_generated_changes_changes_postchangeschangeidaiupdate, web_src_api_generated_changes_changes_postchangeschangeiddismiss, web_src_api_generated_changes_changes_postchangeschangeidexplain, web_src_api_generated_changes_changes_usegetchangeschangeid (+120 more)

### Community 9 - "go_pkg_net_http"
Cohesion: 0.05
Nodes (49): bulkSnoozeItemResult, bulkSnoozeRequest, dashboardLayout, changePromptData(), stripPromptTags(), truncateUTF8(), bulkResolveItemResult, bulkResolveRequest (+41 more)

### Community 10 - "go_pkg_context"
Cohesion: 0.06
Nodes (29): StatusError, buildEmailMessage(), sendSMTPChannel(), splitRecipients(), TestBuildEmailMessage_SanitizesSubjectNewlines(), TestSendSMTPChannel_MissingConfig(), TestSplitRecipients(), redactURLError() (+21 more)

### Community 11 - "go_pkg_time"
Cohesion: 0.07
Nodes (19): contextKey, elevationError, versionSections(), ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold(), TemplateVersionSection, contains() (+11 more)

### Community 12 - "@tanstack/react-query"
Cohesion: 0.03
Nodes (72): Frontend shell & theme (decided 2026-06), i18next, msw, react-router-dom, @tanstack/react-query, @testing-library/react, vitest, web_src_api_model_index_attentionpage (+64 more)

### Community 13 - "UserIDFromContext"
Cohesion: 0.06
Nodes (27): routerDeps, Handler, newToken(), sanitize(), Handler, configRequestField(), parseScheduleUpdates(), validateRotationFields() (+19 more)

### Community 14 - "net/http.Client"
Cohesion: 0.04
Nodes (30): Connector, ollamaEmbedder, openAIEmbedder, NewServiceUnavailableError(), setHeaders(), TestValidateCustomURL(), tryParseEntities(), validateCustomURL() (+22 more)

### Community 15 - "go_pkg_testing"
Cohesion: 0.04
Nodes (32): NewHandler(), Handler, newTestHandler(), TestCreate(), TestGetNotFound(), TestListMutuallyExclusiveFilters(), TestUpdateAndDelete(), TestSchemaMatchesConfig() (+24 more)

### Community 16 - "Handler"
Cohesion: 0.08
Nodes (24): updateUserRequest, Handler, sanitizeUser(), setRefreshCookie(), writeUserWriteError(), Handler, mustHashDummyPassword(), HashPassword() (+16 more)

### Community 17 - "App.tsx"
Cohesion: 0.02
Nodes (108): setAccessToken(), setMfaEnrollmentRequiredHandler(), web_src_api_generated_auth_auth, web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_postauthelevate, web_src_api_generated_auth_auth_postauthelevateoidcbegin (+100 more)

### Community 18 - "UsersPage.tsx"
Cohesion: 0.05
Nodes (52): axios, AXIOS_INSTANCE, BodyType, customInstance(), ErrorType, getAccessToken(), MfaEnrollmentRequiredFn, RefreshFn (+44 more)

### Community 19 - "ServiceSnapshot"
Cohesion: 0.03
Nodes (25): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, Connector, agentEnabled(), Connector, changePatternID(), Engine (+17 more)

### Community 20 - "User"
Cohesion: 0.09
Nodes (21): oidcElevateFlow, clearFlowCookie(), clearOIDCFlowCookie(), clearOIDCElevateFlowCookie(), Handler, readOIDCElevateFlowCookie(), setOIDCElevateFlowCookie(), Handler (+13 more)

### Community 21 - "package.json"
Cohesion: 0.04
Nodes (49): clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks (+41 more)

### Community 22 - "router.go"
Cohesion: 0.17
Nodes (19): go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts, go_pkg_github_com_wiselabz_wiselabz_internal_api_apikeys, go_pkg_github_com_wiselabz_wiselabz_internal_api_attention, go_pkg_github_com_wiselabz_wiselabz_internal_api_auth, go_pkg_github_com_wiselabz_wiselabz_internal_api_changes, go_pkg_github_com_wiselabz_wiselabz_internal_api_chat, go_pkg_github_com_wiselabz_wiselabz_internal_api_compliance, go_pkg_github_com_wiselabz_wiselabz_internal_api_connectors (+11 more)

### Community 23 - "Get"
Cohesion: 0.11
Nodes (14): Handler, isWritableField(), validateConfigPushRequest(), capitalize(), Handler, WriteElevationError(), ConfigPusher, ValidateCompositeRef() (+6 more)

### Community 24 - "rowScanner"
Cohesion: 0.05
Nodes (30): Dispatcher, actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), Store, scanBackupRun() (+22 more)

### Community 25 - "Store"
Cohesion: 0.09
Nodes (8): changeServiceIDs(), placeholders(), changeFilterClause(), AlertRecord, ChangeRecord, Store, ChangeSummary, fakeNotifier

### Community 26 - "fixtures.ts"
Cohesion: 0.06
Nodes (43): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+35 more)

### Community 27 - "DecodeKey"
Cohesion: 0.09
Nodes (25): ProviderConfig, testHandler, Handler, instanceAdminRoleFor(), Handler, Handler, primaryProviderConfig(), Handler (+17 more)

### Community 28 - "WiseLabz — Architecture & Technical Decisions"
Cohesion: 0.04
Nodes (45): 0001 — Lab-mutating operation boundaries, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+37 more)

### Community 29 - "WebSocketProvider.tsx"
Cohesion: 0.09
Nodes (26): Client dispatch model, Envelope, Mock emitter (frontend-first), Naming convention, Reconnect behavior, Transport, WiseLabz WebSocket Contract (`/ws`), web_src_api_generated_changes_changes (+18 more)

### Community 30 - "RulesPage.tsx"
Cohesion: 0.05
Nodes (43): web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey, web_src_api_generated_compliance_compliance_postcompliancerules, web_src_api_generated_compliance_compliance_postcompliancerulestest, web_src_api_generated_compliance_compliance_putcompliancerulesid, web_src_api_generated_compliance_compliance_usegetcompliancerules, web_src_api_generated_compliance_compliance_usegetcomplianceschema (+35 more)

### Community 31 - "snapshotdiff.go"
Cohesion: 0.14
Nodes (24): BuildSnapshotDiff(), CompareDependencies(), CompareEntities(), entityKey(), entityMap(), TestCompareDependenciesDeterministic(), TestCompareEntitiesKeepsExternalIDAndFallbackNameDistinct(), TestSnapshotDiffCSVNeutralizesFormulaCellsOnly() (+16 more)

### Community 32 - "dispatcher_test.go"
Cohesion: 0.21
Nodes (45): NewDispatcher(), deliveriesFor(), findDelivery(), Dispatcher, newTestStore(), setChannelAndRoutingConfig(), setChannelConfig(), setChannelConfigJSON() (+37 more)

### Community 33 - "net/http.ResponseWriter"
Cohesion: 0.07
Nodes (21): Handler, diffToSpec(), Handler, Handler, decodeStoredSnapshot(), Handler, snapshotStoreError(), Handler (+13 more)

### Community 34 - "docker_test.go"
Cohesion: 0.06
Nodes (44): buildDockerTLSConfig(), newDockerClient(), newTCPDockerClient(), init(), generateSelfSignedCert(), generateSSHHostKey(), serveOneHTTPExchange(), serveSSHDockerConn() (+36 more)

### Community 35 - "mountAPIRoutes"
Cohesion: 0.09
Nodes (30): AuditRecorder, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountConnectorRoutes(), chi.Router (+22 more)

### Community 36 - "response.go"
Cohesion: 0.08
Nodes (27): Handler, Handler, Cursor(), DecodeCursor(), EncodeCursor(), T, NextCursor(), TestCursorRequestModes() (+19 more)

### Community 37 - "SnapshotEntity"
Cohesion: 0.13
Nodes (42): SnapshotEntity, buildDatasets(), buildDisks(), buildInterfaces(), buildNFSShares(), buildPools(), buildReplicationTasks(), buildServices() (+34 more)

### Community 38 - "home_assistant/tables.go"
Cohesion: 0.09
Nodes (39): jsonType(), TestAttributeCatalogCoversEmittedKeys(), unavailable(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations() (+31 more)

### Community 39 - "react"
Cohesion: 0.03
Nodes (96): react, web_src_api_generated_connectors_connectors_deleteconnectorsconnectoridgoldensnapshot, web_src_api_generated_connectors_connectors_getgetconnectorsconnectoridgoldensnapshotquerykey, web_src_api_generated_connectors_connectors_postconnectorsconnectoridgoldensnapshot, web_src_api_generated_connectors_connectors_usegetconnectorsconnectoridgoldensnapshot, web_src_api_generated_connectors_connectors_usegetconnectorsconnectoridsnapshotsdiff, web_src_api_generated_connectors_connectors_usegetconnectorsconnectoridsnapshotssnapshotid, web_src_api_generated_notifications_notifications_usegetnotificationsdeliveries (+88 more)

### Community 40 - "RunMigrations"
Cohesion: 0.11
Nodes (37): main(), OpenDB(), TestOpenDBEnablesSQLiteForeignKeys(), TestOpenDBSetsSQLiteDurabilityPragmas(), GetMigrationStatus(), newMigrator(), collectColumns(), postgresSchemaColumns() (+29 more)

### Community 41 - "NewEngine"
Cohesion: 0.09
Nodes (44): NewHandler(), entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks(), Engine (+36 more)

### Community 42 - "dependencies"
Cohesion: 0.05
Nodes (42): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+34 more)

### Community 43 - "Connector"
Cohesion: 0.08
Nodes (42): unavailable(), SnapshotSection, buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP() (+34 more)

### Community 44 - "portainer/tables.go"
Cohesion: 0.12
Nodes (33): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEnvironmentTable(), buildStackTable(), cell(), containerRows(), environmentNames(), environmentTypeName() (+25 more)

### Community 45 - "Dispatcher"
Cohesion: 0.12
Nodes (15): discordPayload(), sendDiscordChannel(), sendGenericWebhookChannel(), sendSlackChannel(), slackPayload(), webhookPayload(), findChannel(), findRoute() (+7 more)

### Community 46 - "net/http.Request"
Cohesion: 0.09
Nodes (12): applyConnectorScalarUpdates(), Handler, validateConnectorConfig(), Handler, Handler, Handler, Handler, cron.EntryID (+4 more)

### Community 47 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (31): statusInfo, upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo(), buildFiltering() (+23 more)

### Community 48 - "log/slog.Logger"
Cohesion: 0.09
Nodes (24): TestExpireAlertsOnceNoExpiredAlertsIsNoop(), TestExpireAlertsOnceNotifiesViaDispatcher(), newLifecycleManager(), newTestLifecycle(), TestLifecycleManagerOrderedShutdown(), TestLifecycleManagerShutdownCancelsWorkContext(), testLogger(), expireAlertsOnce() (+16 more)

### Community 49 - "Compare"
Cohesion: 0.15
Nodes (18): configPushLanded(), driftDescription(), Checker, highestDriftSeverity(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare() (+10 more)

### Community 50 - "ThemeControls.tsx"
Cohesion: 0.11
Nodes (31): @fontsource/space-mono, @fontsource-variable/space-grotesk, AdvancedControls(), FONT_KEYS, OPT_KEYS, PRESET_KEYS, Segmented(), ThemeControls() (+23 more)

### Community 51 - "New"
Cohesion: 0.22
Nodes (19): newFakeHealthStore(), TestJobHealthOkToFailingNotifiesOnce(), TestJobHealthPanicCountsAsFailure(), TestJobHealthPersistsAcrossRestart(), TestJobHealthWithoutStoreDoesNothing(), New(), TestAddJobInvalidExpression(), TestAddJobRegistersAndFires() (+11 more)

### Community 52 - "traefik/tables.go"
Cohesion: 0.15
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable(), cell() (+21 more)

### Community 53 - "Hub"
Cohesion: 0.08
Nodes (15): decodeBulkRequest(), Handler, loggablePath(), loggableQuery(), Sanitize(), TestSanitize(), Hub, bulkRequest (+7 more)

### Community 54 - "ExportToFile"
Cohesion: 0.10
Nodes (44): Export(), ExportToFile(), newTestStore(), TestExportIncludesRecordsBeyondAPage(), TestExportRedactsConnectorSecrets(), TestExportToFile(), TestExportToFileCreatesDirectory(), TestExportToFileDirNotWritable() (+36 more)

### Community 55 - "unifi/tables.go"
Cohesion: 0.17
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+21 more)

### Community 56 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.06
Nodes (28): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, Manifest, checksum, and verification, 1. Every export gets a manifest and a checksum, 2. Verifying a backup actually restores, 3. Restoring for real (+20 more)

### Community 57 - "share_links_test.go"
Cohesion: 0.23
Nodes (35): GrantConnectorRole(), instanceAdminRole(), NewUser(), TestListFiltersGrantsBeforePagination(), Handler, newTestHandler(), asUser(), createTestShareLink() (+27 more)

### Community 58 - "go_pkg_os"
Cohesion: 0.04
Nodes (53): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), fileName() (+45 more)

### Community 59 - "Connector"
Cohesion: 0.11
Nodes (12): NewAuthError(), TestTypedErrorsAreDistinguishableByType(), TestTypedErrorsWrapAndUnwrap(), Connector, apiMessage(), controllerName(), countByKind(), statusError() (+4 more)

### Community 60 - "traefik_test.go"
Cohesion: 0.25
Nodes (16): Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchSelectiveFields() (+8 more)

### Community 61 - "Connector"
Cohesion: 0.21
Nodes (5): TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), Connector

### Community 62 - "settings.mock.ts"
Cohesion: 0.08
Nodes (27): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role, web_src_api_model_index_session (+19 more)

### Community 63 - "GetTypeSchema"
Cohesion: 0.11
Nodes (28): catalog(), TestRegisteredSchema(), TestSchemaConfigValidation(), TestAllConnectorImplementationsRegister(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestBuildHostsTableAttributes() (+20 more)

### Community 64 - "NewMalformedResponseError"
Cohesion: 0.06
Nodes (24): ServiceDependency, TimeoutError, NewMalformedResponseError(), NewTimeoutError(), WantsField(), TestRequestedFields(), TestWantsField(), TestBuildHostsTableV5() (+16 more)

### Community 65 - "rewritePlaceholders"
Cohesion: 0.10
Nodes (14): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), database/sql.Result, database/sql.Row, database/sql.Rows (+6 more)

### Community 66 - "nilToStr"
Cohesion: 0.10
Nodes (10): nilToStr(), DocVersionRecord, Store, DeliveryRecord, DeliveryStatus, Store, scanDelivery(), Store (+2 more)

### Community 67 - "NewStore"
Cohesion: 0.15
Nodes (27): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+19 more)

### Community 68 - "home_assistant_test.go"
Cohesion: 0.10
Nodes (35): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+27 more)

### Community 69 - "SuggestRequest"
Cohesion: 0.11
Nodes (12): claudeProvider, openAICompatibleProvider, StubProvider, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet(), TestRegistryList() (+4 more)

### Community 70 - "NewService"
Cohesion: 0.07
Nodes (42): APIKeyChecker, fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, UserStatusChecker, TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys() (+34 more)

### Community 71 - "newTestHandler"
Cohesion: 0.06
Nodes (72): mockElevateOIDCServer, secondFactorInput, virtualAuthenticator, doJSON(), testHandler, req(), TestChangePassword(), TestChangePasswordRevokesAPIKeys() (+64 more)

### Community 72 - "config_test.go"
Cohesion: 0.15
Nodes (18): Load(), TestAccessTokenTTLDuration(), TestDocExportGitValidate(), TestLoadDefaults(), TestLoadEnvOverride(), TestLoadEnvOverrideAllFields(), TestLoadEnvOverrideDocExportGitSSH(), TestLoadFromYAML() (+10 more)

### Community 73 - "Service"
Cohesion: 0.16
Nodes (13): Claims, ElevationClaims, ElevationToken, IssuePairOptions, MFAClaims, MFATicket, Service, TokenPair (+5 more)

### Community 74 - "Store"
Cohesion: 0.22
Nodes (22): connectorIDs(), docIDs(), exportDocs(), exportTemplates(), exportWithin(), AIConfigSummary, Import(), importBundle() (+14 more)

### Community 75 - "handlers.ts"
Cohesion: 0.07
Nodes (26): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+18 more)

### Community 76 - "ws/ws_test.go"
Cohesion: 0.20
Nodes (17): NewHub(), normalizeOrigin(), assertEnvelope(), setupWSConnection(), TestBroadcastFullQueueDoesNotBlock(), TestBroadcastToUserAfterUpgrade(), TestClientCloseDisconnect(), TestDocLockEventBroadcast() (+9 more)

### Community 77 - "logging_test.go"
Cohesion: 0.18
Nodes (15): Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken(), TestLoggerRedactsWSTicket(), TestGetRequestIDMissing(), TestRecovererPassThrough(), TestRecovererReturns500OnPanic() (+7 more)

### Community 78 - "NewEngine"
Cohesion: 0.18
Nodes (23): RequestedFields(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector(), TestRunSyncFieldsPassesHintToConnector(), TestRunSyncFieldsSurvivesCredentialRefresh() (+15 more)

### Community 79 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 80 - "timeline.ts"
Cohesion: 0.14
Nodes (17): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+9 more)

### Community 81 - "AuthedUser"
Cohesion: 0.14
Nodes (24): TestEmbeddedSPAWithoutFrontendBuild(), NewHandler(), TestCreate(), TestList(), TestRevoke(), AuthedUser(), JWTService(), Token() (+16 more)

### Community 82 - "Connector"
Cohesion: 0.07
Nodes (11): init(), ConfigField, Connector, buildRouteTable(), buildGatewayTable(), primaryGatewayName(), wanInterfaceName(), PathSegment() (+3 more)

### Community 83 - "AppearancePage.tsx"
Cohesion: 0.14
Nodes (21): zustand, MotionProvider(), AppearancePage(), ChoiceGroup(), AppearanceState, apply(), Contrast, css() (+13 more)

### Community 84 - "NewRegistry"
Cohesion: 0.21
Nodes (22): SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail(), TestSuggestWithFallbackFirstProviderSucceeds(), TestSuggestWithFallbackNoProviders(), TestSuggestWithFallbackStopsOnNonRetryableError() (+14 more)

### Community 85 - "newTestHandler"
Cohesion: 0.07
Nodes (63): AssertMatchesSpec(), loadSpec(), specPath(), actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews() (+55 more)

### Community 86 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 87 - "devDependencies"
Cohesion: 0.08
Nodes (24): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+16 more)

### Community 88 - "Manager"
Cohesion: 0.16
Nodes (9): NewHandler(), cron.EntryID, Manager, LogPartial(), NewManager(), ReportDefinitionRecord, ReportRecord, Store (+1 more)

### Community 89 - "New"
Cohesion: 0.12
Nodes (22): confirm(), formatCounts(), main(), runRestore(), runVerify(), newSeededStore(), TestRunRestoreImportsIntoConfiguredDatabase(), TestRunRestoreRejectsCorruptedBundle() (+14 more)

### Community 90 - "Config"
Cohesion: 0.11
Nodes (22): NewHandler(), NewWebAuthnService(), TestWebAuthnRPConfig(), WebAuthnRPConfig(), Config, LogSettings, IsSSHRemote(), AISettings (+14 more)

### Community 91 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 92 - "ConnectorRecord"
Cohesion: 0.12
Nodes (17): Store, ConnectorRecord, Store, Store, scanConnector(), scanConnectorRows(), nullInt64ToIntPtr(), nullStrToStr() (+9 more)

### Community 94 - "NewHTTPClient"
Cohesion: 0.12
Nodes (16): newConnector(), Connector, newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), intConfig(), newConnector(), NewHTTPClient() (+8 more)

### Community 95 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 96 - "DecodeJSON"
Cohesion: 0.12
Nodes (11): webAuthnFlow, webAuthnUser, Handler, Handler, oidcProviderJSON(), boolToInt(), DecodeJSON(), T (+3 more)

### Community 97 - "docs/handlers_test.go"
Cohesion: 0.19
Nodes (16): NewHandler(), TestAISuggestInvalidJSON(), TestByServiceNoDocsYet(), TestGenerate(), TestGetLockNoneHeld(), TestGetRootIsSynthetic(), TestGetUnknownIDFallsBackToServicePlaceholder(), TestListEmpty() (+8 more)

### Community 98 - "templates.fixtures.ts"
Cohesion: 0.16
Nodes (14): web_src_api_model_index_docversion, web_src_api_model_index_template, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, renderTemplate() (+6 more)

### Community 99 - "time.Time"
Cohesion: 0.05
Nodes (53): digestDue(), formatDigest(), Dispatcher, TestDigestDue(), connectorFilter(), RenderHTML(), RenderMarkdown(), sampleData() (+45 more)

### Community 100 - "Runner"
Cohesion: 0.16
Nodes (7): cron.EntryID, Runner, cron.Cron, HealthStore, jobEntry, JobInfo, Notifier

### Community 101 - "main"
Cohesion: 0.08
Nodes (29): Provider, Config, main(), newLogger(), runHealthcheck(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults() (+21 more)

### Community 102 - "connector_permission.go"
Cohesion: 0.20
Nodes (9): auditConnectorGrantDiffJSON(), getConnectorGrant(), ConnectorGrantDiff, Store, highestConnectorRole(), listOIDCConnectorGrants(), scanConnectorGrants(), upsertConnectorGrant() (+1 more)

### Community 104 - "Connector"
Cohesion: 0.15
Nodes (7): TestBuildInterfaceTableAttributes(), buildGatewayTable(), buildInterfaceTable(), buildSystemContent(), primaryGatewayName(), wanInterfaceName(), Connector

### Community 105 - "export_test.go"
Cohesion: 0.20
Nodes (17): fetchAllDocs(), Exporter, IsGeneratedName(), NewExporter(), pruneStale(), RunExportOnce(), newTestStore(), readFile() (+9 more)

### Community 106 - "Deps"
Cohesion: 0.23
Nodes (18): registerListAttentionItems(), registerListChanges(), jsonResult(), registerListConnectors(), registerSearchDocs(), connectorAllowSet(), registerListFindings(), NewHTTPHandler() (+10 more)

### Community 107 - "Handler"
Cohesion: 0.24
Nodes (7): NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), Handler, RuleEvaluator

### Community 108 - "Handler"
Cohesion: 0.18
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 109 - "fetch_test.go"
Cohesion: 0.19
Nodes (14): entityKinds(), sectionByTitle(), TestConfigPushV5(), TestFetchAuthFailureReturnsPlaceholderSnapshot(), TestFetchBothVersions(), TestFetchDegradesPerSection(), TestRestartUnsupportedOnV5(), TestStartStopV5() (+6 more)

### Community 110 - "truenas_test.go"
Cohesion: 0.24
Nodes (17): Connector, newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchIsStableAcrossCalls() (+9 more)

### Community 111 - "diagnostics/diagnostics.go"
Cohesion: 0.22
Nodes (17): CheckHealth(), Collect(), collectVersions(), newTestStore(), TestCheckHealthReportsDegradedOnClosedDB(), TestCollectIncludesHealthVersionsAndSchedule(), TestCollectListsRecentFailures(), TestCollectRedactsConnectorSecrets() (+9 more)

### Community 112 - "keyset_test.go"
Cohesion: 0.19
Nodes (16): Store, seedConnectorForChanges(), TestChangeRelatedServiceIDsAndPatternIDRoundTrip(), TestChangeRelatedServiceIDsDefaultsToEmptyArray(), TestCountRecentChangePatterns(), TestCountRecentChangesByPattern(), assertSameSet(), Store (+8 more)

### Community 113 - "ws.ts"
Cohesion: 0.11
Nodes (17): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+9 more)

### Community 114 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 115 - "connectors_health_test.go"
Cohesion: 0.32
Nodes (12): testApp, registerHealthFakeType(), seedHealthTestConnector(), TestConnectorsHealthDegraded(), TestConnectorsHealthDoesNotCreateSnapshot(), TestConnectorsHealthOffline(), TestConnectorsHealthOnline(), TestConnectorsHealthRecordsTimeSeriesRow() (+4 more)

### Community 116 - "Register"
Cohesion: 0.21
Nodes (17): init(), init(), init(), init(), init(), init(), init(), init() (+9 more)

### Community 117 - "gitTarget"
Cohesion: 0.18
Nodes (10): commitMessage(), Exporter, TestCommitMessage(), commitResult, gitTarget, git.Repository, github.com/go-git/go-git/v5/plumbing.Hash, github.com/go-git/go-git/v5/plumbing/object.Signature (+2 more)

### Community 118 - "MarshalConnectorConfig"
Cohesion: 0.20
Nodes (14): IsSecretFieldType(), MarshalConnectorConfig(), SecretFieldsChanged(), init(), TestConnectorRotationFieldsRoundTrip(), TestCreateConnectorDefaultsSecretRotatedAtToCreatedAt(), TestSecretFieldsChangedFalseOnRenameOnly(), TestSecretFieldsChangedFalseOnResubmittedUnchangedSecret() (+6 more)

### Community 119 - "time.Duration"
Cohesion: 0.15
Nodes (9): Cache, New(), Database, Server, time.Duration, PoolConfig, Cache[V], entry (+1 more)

### Community 120 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 121 - "quality_test.go"
Cohesion: 0.26
Nodes (10): TestComplianceFindingRuleDedupAndResolve(), TestComplianceRuleCRUD(), Store, newConcurrentQualityTestStore(), seedQualityConnector(), TestListQualityFindingsFilters(), TestResolveThenReopenCreatesFreshRow(), TestUpsertQualityFindingConcurrentDedup() (+2 more)

### Community 122 - "templates_test.go"
Cohesion: 0.26
Nodes (15): templateBody, TestTemplateMutationRoleMatrix(), testApp, seedPreviewConnector(), seedTemplate(), TestTemplatesConcurrentUpdatesCreateDistinctVersions(), TestTemplatesPreviewAffectedConnectors(), TestTemplatesPreviewCapturesMissingSnapshot() (+7 more)

### Community 123 - "HashToken"
Cohesion: 0.15
Nodes (15): factorJSON(), Handler, GenerateRecoveryCodes(), GenerateTOTPSecret(), NormalizeRecoveryCode(), randomRecoveryChars(), TestGenerateRecoveryCodesAreUniqueAndFormatted(), TestGenerateTOTPSecretProducesScannableURL() (+7 more)

### Community 124 - "JobHealthRecord"
Cohesion: 0.29
Nodes (4): JobHealthRecord, Store, scanJobHealth(), fakeHealthStore

### Community 125 - "chat/chat.go"
Cohesion: 0.22
Nodes (9): buildPrompt(), TestBuildPrompt(), cosineSimilarity(), Match, SplitSections(), TestCosineSimilarityRanksClosestVectorHighest(), TestSplitSections(), Section (+1 more)

### Community 126 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 127 - "httpx/retry_test.go"
Cohesion: 0.20
Nodes (20): retryable(), RetryTransport(), sleep(), do(), fail(), status(), TestRetryTransportDisabled(), TestRetryTransportDoesNotRetry() (+12 more)

### Community 128 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 129 - "testApp"
Cohesion: 0.23
Nodes (9): testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty(), TestBackupRoutesRequireOperatorRole(), TestBackupScheduleGetDefaults(), TestBackupScheduleUpdate(), TestBackupScheduleUpdateDoesNotLeakSchedulerJobs() (+1 more)

### Community 130 - "Retrieve"
Cohesion: 0.25
Nodes (8): Handler, packVector(), Retrieve(), SyncDocEmbeddings(), TestPackUnpackVectorRoundTrips(), TestSyncDocEmbeddingsKeepsOldRowsWhenEmbedFails(), unpackVector(), TestRetrieveUsesCacheAndSyncInvalidates()

### Community 131 - "Handler"
Cohesion: 0.22
Nodes (7): definition(), record(), reportJSON(), valid(), JobName(), Handler, input

### Community 132 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 133 - "config/validate_test.go"
Cohesion: 0.18
Nodes (11): Config, mask(), redactDSN(), redactKVPassword(), Config, TestEveryKeyEnvOverridable(), TestRedactDSN(), TestRedacted() (+3 more)

### Community 134 - "decodePaginated"
Cohesion: 0.36
Nodes (8): decodePaginated(), jsonHasEmptyArrayItems(), newTestStore(), seedDelivery(), TestListDeliveriesEmpty(), TestListDeliveriesNoFilter(), TestListDeliveriesStatusFilter(), PaginatedResponse

### Community 135 - "snapshotreport.go"
Cohesion: 0.14
Nodes (14): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+6 more)

### Community 136 - "gitFixture"
Cohesion: 0.29
Nodes (9): SetBeforePushForTest(), keys(), newGitFixture(), TestGitExportLifecycle(), TestGitExportPushRejectionReturnsError(), TestGitExportRefusesForeignDirectory(), gitFixture, Exporter (+1 more)

### Community 137 - "DocRecord"
Cohesion: 0.20
Nodes (6): docSearchWhere(), escapeLike(), DocRecord, Store, scanDoc(), scanDocSummary()

### Community 138 - "apikey_scope.go"
Cohesion: 0.14
Nodes (14): APIKeyRestriction, testAPIKeyChecker, APIKeyRestrictionFromContext(), ClampConnectorRole(), ContextWithAPIKeyRestriction(), isSafeMethod(), TestClampConnectorRole(), treatAsSafeFromContext() (+6 more)

### Community 139 - "RequirePermission"
Cohesion: 0.38
Nodes (5): PermissionChecker, chi.Router, mountDashboardRoutes(), mountWorkflowRoutes(), RequirePermission()

### Community 140 - "config_cmd_test.go"
Cohesion: 0.23
Nodes (10): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), Schema(), schemaFor() (+2 more)

### Community 141 - "changes/handlers_test.go"
Cohesion: 0.32
Nodes (13): Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound(), TestDismissSuccess() (+5 more)

### Community 143 - "newTestHandler"
Cohesion: 0.24
Nodes (12): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), Handler, newTestHandler(), TestCreate() (+4 more)

### Community 145 - ".GetConnectorUptime"
Cohesion: 0.33
Nodes (3): Store, HealthCheckRecord, UptimeStats

### Community 147 - "api/changes_test.go"
Cohesion: 0.26
Nodes (12): testApp, seedChange(), seedChangeWithSeverity(), TestChangesAcknowledgeRoleBoundary(), TestChangesAcknowledgeSuccess(), TestChangesBulkResolveEmptyIDs(), TestChangesBulkResolveInvalidStatus(), TestChangesBulkResolvePartialFailure() (+4 more)

### Community 150 - "createUser"
Cohesion: 0.28
Nodes (13): seedAlert(), TestListAttentionItems(), seedChange(), TestListChanges(), TestListConnectors(), enableFakeEmbedding(), TestSearchDocs(), seedFinding() (+5 more)

### Community 151 - "Contributing to WiseLabz"
Cohesion: 0.15
Nodes (13): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+5 more)

### Community 152 - "scripts"
Cohesion: 0.15
Nodes (13): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+5 more)

### Community 153 - "Engine"
Cohesion: 0.18
Nodes (6): NewHandler(), Engine, sync.Map, AlertNotifier, DocRegenerator, QualityChecker

### Community 155 - "NewClient"
Cohesion: 0.15
Nodes (16): GuardedDialer(), IsDangerousIP(), clientTimeout(), NewClient(), NewTransport(), NoRedirect(), TestNewClientDoesNotFollowRedirects(), TestNewClientInsecureSkipVerifyConnects() (+8 more)

### Community 157 - "Store"
Cohesion: 0.27
Nodes (4): decodeConnectorIDs(), APIKey, Store, scanAPIKey()

### Community 159 - "Store"
Cohesion: 0.23
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 161 - "Decision"
Cohesion: 0.17
Nodes (11): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 162 - "main.tsx"
Cohesion: 0.21
Nodes (8): react-dom, App(), USE_MOCKS, web_src_index, bootstrap(), worker, enableMocks(), handlers

### Community 164 - "dialSSHStdio"
Cohesion: 0.15
Nodes (8): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), bufio.ReadWriter, golang.org/x/crypto/ssh.ClientConfig, io.Closer, net.Conn, responseWriter

### Community 165 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 166 - "WiseLabz Connector Guide"
Cohesion: 0.18
Nodes (11): Conventions, Dependencies, Getting your connector merged, Health checks vs. sync, Keeping snapshots stable, Session-based and multi-flavour APIs, Testing without a real instance, The Connector interface (+3 more)

### Community 167 - "Product"
Cohesion: 0.18
Nodes (10): Accessibility & Inclusion, Anti-references, Brand Personality, Design Principles, Locked frontend direction (planning session, 2026-06; revised 2026-09), Product, Product decisions (pre-planning, v1), Product Purpose (+2 more)

### Community 168 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 170 - "api/auth/oidc.go"
Cohesion: 0.19
Nodes (10): TestEmailDomainAllowed(), TestOIDCConnectorRolesForGroups(), TestOIDCRoleForGroups(), connectorRoleLess(), emailDomainAllowed(), oidcConnectorRolesForGroups(), oidcRoleForGroups(), go_pkg_crypto_rand (+2 more)

### Community 173 - ".call"
Cohesion: 0.44
Nodes (6): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture

### Community 174 - "connector/validate_test.go"
Cohesion: 0.31
Nodes (8): isConfigValidationError(), testSchema(), TestTypeSchemaDegradedLatencyThreshold(), TestValidateConfigAcceptsMissingAndEmptyFields(), TestValidateConfigEnum(), TestValidateConfigLength(), TestValidateConfigPattern(), ConfigValidationError

### Community 175 - "Changelog"
Cohesion: 0.20
Nodes (9): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features, Unreleased (+1 more)

### Community 176 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 177 - "apikey_scopes_test.go"
Cohesion: 0.47
Nodes (8): createKey(), testApp, newConnector(), TestAPIKeyCreateValidation(), TestAPIKeyDefaultsToFullScope(), TestConnectorRestrictedAPIKey(), TestReadOnlyAPIKey(), TestReadOnlyAPIKeyCapsConnectorRoleAtViewer()

### Community 178 - "ComplianceRuleRecord"
Cohesion: 0.36
Nodes (4): changedFields(), ComplianceRuleRecord, Store, scanComplianceRule()

### Community 180 - "retention/retention_test.go"
Cohesion: 0.35
Nodes (10): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupPrunesOldHealthChecks(), TestRunCleanupSkipsDisabledCategories() (+2 more)

### Community 181 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 182 - "connectors_hardening_test.go"
Cohesion: 0.29
Nodes (7): testApp, TestConnectorsCreateAcceptsValidConfig(), TestConnectorsCreateRejectsInvalidEnum(), TestConnectorsCreateRejectsMalformedConfig(), TestConnectorsSyncAcceptsFieldsHint(), TestConnectorsUpdateRejectsMalformedConfig(), waitForSyncRuns()

### Community 183 - "dashboard/handlers_test.go"
Cohesion: 0.36
Nodes (8): NewHandler(), Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout()

### Community 184 - "RateLimit"
Cohesion: 0.29
Nodes (6): TestRateLimit(), RateLimit(), golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 189 - "Step by step"
Cohesion: 0.25
Nodes (8): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Step by step

### Community 190 - "WiseLabz"
Cohesion: 0.25
Nodes (8): Code of Conduct, Configuration, Contributing, Features, License, Quick start, Supported services, WiseLabz

### Community 193 - "TestComplianceRuleValidation"
Cohesion: 0.29
Nodes (7): badRegexMessage(), complianceCondition(), TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails()

### Community 194 - "snapshotResponse"
Cohesion: 0.48
Nodes (7): Handler, snapshotFixture(), snapshotRequest(), snapshotResponse(), TestSnapshotDiffValidationOwnershipAndAudit(), TestSnapshotOwnershipAndFullShape(), TestSnapshotsViewerAndCursor()

### Community 196 - "scanMaintenanceWindow"
Cohesion: 0.48
Nodes (3): Store, scanMaintenanceWindow(), MaintenanceWindowRecord

### Community 198 - "Contributor Covenant Code of Conduct"
Cohesion: 0.29
Nodes (7): Attribution, Contributor Covenant Code of Conduct, Enforcement, Enforcement Responsibilities, Our Pledge, Our Standards, Scope

### Community 199 - "Audit Trail"
Cohesion: 0.29
Nodes (6): Audit Trail, Endpoint, Keyset (cursor) pagination, Retention, What's not recorded, What's recorded

### Community 200 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 201 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 202 - "walkCursorPages"
Cohesion: 0.40
Nodes (6): cursorPage, decodeCursorPage(), testApp, TestAuditCursorPaginationTraversal(), TestChangesCursorPaginationTraversal(), walkCursorPages()

### Community 204 - "engine_maintenance_test.go"
Cohesion: 0.60
Nodes (5): driftingSnapshot(), setupMaintenanceTestConnector(), TestRunSyncExpiredMaintenanceWindowBehavesNormally(), TestRunSyncNoMaintenanceWindowBehavesNormally(), TestRunSyncSuppressesChangesDuringMaintenanceWindow()

### Community 205 - "Mermaid.tsx"
Cohesion: 0.47
Nodes (4): mermaid, cssVar(), Mermaid(), resolveColor()

### Community 206 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 207 - "net/http.Handler"
Cohesion: 0.16
Nodes (12): ConnectorRoleChecker, CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders(), SecurityHeaders(), TestSecurityHeaders(), RequireConnectorRole() (+4 more)

### Community 209 - "routerOperations"
Cohesion: 0.50
Nodes (5): normalizeParams(), routerOperations(), specOperations(), TestOpenAPIMatchesRouter(), chi.Routes

### Community 210 - "seedScopeFixture"
Cohesion: 0.60
Nodes (4): Store, seedScopeFixture(), TestListDocSectionEmbeddingsFiltersByGrant(), TestMergedAttentionItemsFiltersByGrant()

### Community 211 - "Enforcement Guidelines"
Cohesion: 0.40
Nodes (5): 1. Correction, 2. Warning, 3. Temporary Ban, 4. Permanent Ban, Enforcement Guidelines

### Community 212 - "MfaEnrollDialog"
Cohesion: 0.50
Nodes (5): Sync flow, qrcode, MfaEnrollDialog(), close(), done()

### Community 213 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 217 - "MISSING — deferred & future frontend features"
Cohesion: 0.50
Nodes (3): Deferred from V1 (decided during planning), MISSING — deferred & future frontend features, Suggested-later (raised in build, not yet planned)

### Community 218 - "Saved Views"
Cohesion: 0.50
Nodes (3): Endpoints, Saved Views, Scope

## Knowledge Gaps
- **565 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `bulkResolveRequest`, `bulkResolveItemResult` (+560 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1281 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **18 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Store` connect `Store` to `testApp`, `Retrieve`, `testing.T`, `Handler`, `decodePaginated`, `NewChecker`, `gitFixture`, `go_pkg_time`, `UserIDFromContext`, `go_pkg_testing`, `Handler`, `ServiceSnapshot`, `createUser`, `rowScanner`, `Engine`, `DecodeKey`, `dispatcher_test.go`, `net/http.ResponseWriter`, `response.go`, `.call`, `NewEngine`, `RunMigrations`, `Dispatcher`, `net/http.Request`, `.call`, `log/slog.Logger`, `retention/retention_test.go`, `Hub`, `ExportToFile`, `dashboard/handlers_test.go`, `share_links_test.go`, `rewritePlaceholders`, `NewStore`, `engine_maintenance_test.go`, `NewEngine`, `AuthedUser`, `NewRegistry`, `Manager`, `New`, `Config`, `Handler`, `docs/handlers_test.go`, `time.Time`, `main`, `export_test.go`, `Deps`, `Handler`, `diagnostics/diagnostics.go`?**
  _High betweenness centrality (0.010) - this node is a cross-community bridge._
- **Why does `gitFixture` connect `gitFixture` to `testing.T`, `context.Context`, `export_test.go`, `Store`, `log/slog.Logger`, `go_pkg_os`?**
  _High betweenness centrality (0.009) - this node is a cross-community bridge._
- **Why does `AuthError` connect `NewMalformedResponseError` to `Connector`?**
  _High betweenness centrality (0.007) - this node is a cross-community bridge._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _565 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.020439988861041494 - nodes in this community are weakly interconnected._
- **Should `ServiceDetailPage.tsx` be split into smaller, more focused modules?**
  _Cohesion score 0.02409097710609008 - nodes in this community are weakly interconnected._
- **Should `newDocTestStore` be split into smaller, more focused modules?**
  _Cohesion score 0.026385442514474774 - nodes in this community are weakly interconnected._