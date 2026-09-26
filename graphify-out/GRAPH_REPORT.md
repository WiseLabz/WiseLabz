# Graph Report - agent-a27bd01e4df8201b0  (2026-09-25)

## Corpus Check
- 845 files · ~514,579 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 19 file(s) not represented in the graph (top: (none) 10, .toml 2, .tmpl 2)

## Summary
- 6269 nodes · 19825 edges · 211 communities (189 shown, 22 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1616 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `56ea2638`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestApp
- newDocTestStore
- App.tsx
- testing.T
- context.Context
- go_pkg_github_com_wiselabz_wiselabz_internal_store
- connector/connector.go
- ProfilePage.tsx
- cn
- go_pkg_testing
- package.json
- ServiceDetailPage.tsx
- Errorf
- UsersPage.tsx
- @tanstack/react-query
- ServiceSnapshot
- go_pkg_context
- auth.ts
- icons.tsx
- rowScanner
- runbooks/handlers_test.go
- Manager
- NewEngine
- User
- Handler
- ErrorWithDetails
- Get
- DocRecord
- ExportToFile
- WiseLabz — Architecture & Technical Decisions
- fixtures.ts
- dispatcher_test.go
- HashToken
- DashboardPage.tsx
- NewService
- DecodeKey
- SnapshotEntity
- response.go
- RunMigrations
- home_assistant/tables.go
- dependencies
- NewChecker
- share_links_test.go
- Connector
- docker_test.go
- ConnectorRecord
- Compare
- lists.go
- portainer/tables.go
- adguardhome/tables.go
- MarshalConnectorConfig
- home_assistant_test.go
- middleware.go
- NewStore
- Dispatcher
- Store
- unifi/tables.go
- Configuration & Documentation Backup (Export/Import)
- net/http.Handler
- net/http.ResponseWriter
- settings.mock.ts
- traefik/tables.go
- rewritePlaceholders
- SystemPage.tsx
- newTestHandler
- routerDeps
- main
- NewRegistry
- newTestHandler
- Register
- traefik_test.go
- Connector
- Service
- Config
- Store
- handlers.ts
- Connector
- NewEngine
- unifi_test.go
- SuggestRequest
- .OIDCCallback
- createUser
- templates_test.go
- timeline.ts
- AuthMiddleware
- WiseLabz — Design Contract
- devDependencies
- system/handlers_test.go
- AuthedUser
- git.go
- Hub
- nilToStr
- apikey_scope.go
- newRouterDeps
- portainer_test.go
- export_test.go
- gitTarget
- Store
- NewHTTPClient
- adguardhome_test.go
- NewMalformedResponseError
- Deps
- time.Time
- snapshot_attributes_test.go
- connector_permission.go
- router.go
- net/http.Request
- chat/chat.go
- Handler
- config_test.go
- httpx/retry_test.go
- ws.ts
- truenas_test.go
- diagnostics/diagnostics.go
- time.Duration
- NotificationRecord
- ws/ws_test.go
- compilerOptions
- Handler
- Handler
- docdiffmodel.ts
- ReportsPage.tsx
- templates.fixtures.ts
- all.go
- compilerOptions
- testApp
- config_cmd_test.go
- changes/handlers_test.go
- connectors_maintenance_test.go
- Handler
- vectorCache
- api/auth/oidc.go
- templatefuncs.go
- data.go
- Engine
- api/mcp_test.go
- pagination_contract_test.go
- newTestHandler
- net/http.Client
- gitFixture
- render_test.go
- log/slog.Logger
- Connector
- Contributing to WiseLabz
- dashboard/handlers_test.go
- diagram.go
- Runner
- Engine
- docs/handlers_test.go
- transform_test.go
- Store
- Decision
- Connector
- scripts
- seedHealthTestConnector
- pfsense.go
- Connector
- Decision
- WiseLabz Connector Guide
- Product
- .call
- .GetConnectorUptime
- openapi_contract_test.go
- .call
- Changelog
- mockServiceWorker.js
- Store
- apikey_scopes_test.go
- ComplianceRuleRecord
- ReportData
- release-please-config.json
- APIKeyClaims
- TestComplianceRuleValidation
- internal/auth/oidc.go
- ComputeWindow
- Cache
- Step by step
- WiseLabz
- RateLimit
- webAuthnUser
- ShareLink
- scanMaintenanceWindow
- computeNextRun
- Contributor Covenant Code of Conduct
- truenas/attributes_test.go
- Audit Trail
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- dockerSSHAddr
- fakeEmbedder
- fakeDocRegenerator
- ClassifyHealth
- RetentionSettings
- engine_maintenance_test.go
- Security Policy
- browser.ts
- fields_test.go
- fakeQualityChecker
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

## Communities (211 total, 22 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.02
Nodes (187): testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow(), TestAlertsListSuccess() (+179 more)

### Community 1 - "newDocTestStore"
Cohesion: 0.02
Nodes (161): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+153 more)

### Community 2 - "App.tsx"
Cohesion: 0.05
Nodes (57): setMfaEnrollmentRequiredHandler(), web_src_api_generated_connectors_connectors_usegetconnectors, web_src_api_generated_me_me, web_src_api_generated_me_me_usegetme, AddConnectorPage, AiPage, AlertsPage, AllDocsPage (+49 more)

### Community 3 - "testing.T"
Cohesion: 0.02
Nodes (145): cursorPage, TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens(), TestClaudeSuggestErrors(), TestClaudeSuggestMultipleContentBlocks(), TestOpenAICompatibleSuggest(), TestOpenAICompatibleSuggestErrors(), TestOIDCRedirectURL() (+137 more)

### Community 4 - "context.Context"
Cohesion: 0.04
Nodes (24): fakeStatusChecker, Connector, changeServiceIDs(), existingIDs(), placeholders(), AlertRecord, ChangeRecord, Store (+16 more)

### Community 5 - "go_pkg_github_com_wiselabz_wiselabz_internal_store"
Cohesion: 0.05
Nodes (43): bulkSnoozeItemResult, bulkSnoozeRequest, changePromptData(), stripPromptTags(), truncateUTF8(), versionSections(), TemplateVersionSection, bulkResolveItemResult (+35 more)

### Community 6 - "connector/connector.go"
Cohesion: 0.07
Nodes (18): TimeoutError, GuardedDialer(), IsDangerousIP(), NewAuthError(), NewTimeoutError(), TestTypedErrorsAreDistinguishableByType(), TestTypedErrorsWrapAndUnwrap(), Connector (+10 more)

### Community 7 - "ProfilePage.tsx"
Cohesion: 0.04
Nodes (50): @simplewebauthn/browser, web_src_api_generated_auth_auth, web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_postauthelevate, web_src_api_generated_auth_auth_postauthelevateoidcbegin, web_src_api_generated_auth_auth_postauthelevateoidccomplete (+42 more)

### Community 8 - "cn"
Cohesion: 0.05
Nodes (52): clsx, tailwind-merge, web_src_api_generated_docs_docs_usegetdocstemplateschema, web_src_api_generated_templates_templates_getgettemplatesquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidversionsquerykey, web_src_api_generated_templates_templates_posttemplatestemplateidpreview, web_src_api_generated_templates_templates_posttemplatestemplateidversionsrevrestore (+44 more)

### Community 9 - "go_pkg_testing"
Cohesion: 0.05
Nodes (30): dashboardLayout, TestBuildInterfaceTableAttributes(), buildEmailMessage(), sendSMTPChannel(), splitRecipients(), TestBuildEmailMessage_SanitizesSubjectNewlines(), TestSendSMTPChannel_MissingConfig(), TestSplitRecipients() (+22 more)

### Community 10 - "package.json"
Cohesion: 0.03
Nodes (89): codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh (+81 more)

### Community 11 - "ServiceDetailPage.tsx"
Cohesion: 0.03
Nodes (161): ADR-0001, ADR-0003, RFC-3339, Frontend, motion, react, react-i18next, web_src_api_generated_alerts_alerts (+153 more)

### Community 12 - "Errorf"
Cohesion: 0.07
Nodes (17): Handler, newToken(), sanitize(), Handler, diffToSpec(), Handler, Handler, Handler (+9 more)

### Community 13 - "UsersPage.tsx"
Cohesion: 0.05
Nodes (51): axios, react-markdown, remark-gfm, customInstance(), web_src_api_generated_users_users, web_src_api_generated_users_users_deleteusersuserid, web_src_api_generated_users_users_getgetusersquerykey, web_src_api_generated_users_users_postusersuseridresetmfa (+43 more)

### Community 14 - "@tanstack/react-query"
Cohesion: 0.03
Nodes (58): i18next, msw, react-router-dom, @tanstack/react-query, @testing-library/jest-dom, @testing-library/react, vitest, web_src_api_generated_connectors_connectors_deleteconnectorsconnectorid (+50 more)

### Community 15 - "ServiceSnapshot"
Cohesion: 0.03
Nodes (18): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, changePatternID(), Engine, markError(), runTransformers(), TestRunTransformersUnknownCategoryIsNoop() (+10 more)

### Community 16 - "go_pkg_context"
Cohesion: 0.07
Nodes (20): StatusError, IsTimeout(), TestIsTimeout(), contains(), searchString(), shareLinkContextKey, go_pkg_context, go_pkg_database_sql (+12 more)

### Community 17 - "auth.ts"
Cohesion: 0.04
Nodes (56): Sync flow, 3. `sync.complete`, Client dispatch model, Envelope, Mock emitter (frontend-first), Naming convention, Reconnect behavior, Transport (+48 more)

### Community 18 - "icons.tsx"
Cohesion: 0.03
Nodes (99): Frontend shell & theme (decided 2026-06), match-sorter, @radix-ui/react-popover, react-error-boundary, sonner, zustand, web_src_api_generated_attention_attention, web_src_api_generated_attention_attention_usegetattention (+91 more)

### Community 19 - "rowScanner"
Cohesion: 0.07
Nodes (25): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), Store, scanBackupRun(), changeFilterClause() (+17 more)

### Community 20 - "runbooks/handlers_test.go"
Cohesion: 0.29
Nodes (8): NewHandler(), Handler, newTestHandler(), TestCreate(), TestGetNotFound(), TestListMutuallyExclusiveFilters(), TestUpdateAndDelete(), createRequest

### Community 21 - "Manager"
Cohesion: 0.14
Nodes (10): NewHandler(), cron.EntryID, Manager, JobName(), LogPartial(), NewManager(), ReportDefinitionRecord, ReportRecord (+2 more)

### Community 22 - "NewEngine"
Cohesion: 0.25
Nodes (24): NewEngine(), newEngineTestStore(), seedEngineConnector(), seedEngineTemplate(), TestGenerateFromSnapshotIncludesDependencies(), TestGenerateFromTemplateReturnsVersionPersistenceError(), TestGenerateFromTemplateStillPersists(), TestMatchingConnectorsEmptyAppliesToIsWildcard() (+16 more)

### Community 23 - "User"
Cohesion: 0.06
Nodes (13): sanitizeSessions(), Store, Store, RunbookRecord, Store, scanRunbook(), boolToInt(), Session (+5 more)

### Community 24 - "Handler"
Cohesion: 0.18
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 25 - "ErrorWithDetails"
Cohesion: 0.07
Nodes (29): sanitizeUser(), setRefreshCookie(), Handler, mustHashDummyPassword(), Handler, randomOIDCToken(), instanceAdminRoleFor(), configRequestField() (+21 more)

### Community 26 - "Get"
Cohesion: 0.13
Nodes (13): Handler, isWritableField(), validateConfigPushRequest(), capitalize(), WriteElevationError(), ConfigPusher, ValidateCompositeRef(), Get() (+5 more)

### Community 27 - "DocRecord"
Cohesion: 0.20
Nodes (6): docSearchWhere(), escapeLike(), DocRecord, Store, scanDoc(), scanDocSummary()

### Community 28 - "ExportToFile"
Cohesion: 0.09
Nodes (48): runVerify(), TestRunVerifyDefaultsToLatestInDir(), TestRunVerifyPassAndFail(), TestRunVerifyRequiresABundle(), Export(), ExportToFile(), newTestStore(), TestExportIncludesRecordsBeyondAPage() (+40 more)

### Community 29 - "WiseLabz — Architecture & Technical Decisions"
Cohesion: 0.04
Nodes (45): 0001 — Lab-mutating operation boundaries, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+37 more)

### Community 30 - "fixtures.ts"
Cohesion: 0.06
Nodes (42): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+34 more)

### Community 31 - "dispatcher_test.go"
Cohesion: 0.16
Nodes (53): TestExpireAlertsOnceNoExpiredAlertsIsNoop(), TestExpireAlertsOnceNotifiesViaDispatcher(), newTestLifecycle(), TestLifecycleManagerOrderedShutdown(), TestLifecycleManagerShutdownCancelsWorkContext(), testLogger(), expireAlertsOnce(), NewDispatcher() (+45 more)

### Community 32 - "HashToken"
Cohesion: 0.14
Nodes (14): factorJSON(), Handler, GenerateRecoveryCodes(), GenerateTOTPSecret(), NormalizeRecoveryCode(), randomRecoveryChars(), TestGenerateRecoveryCodesAreUniqueAndFormatted(), TestGenerateTOTPSecretProducesScannableURL() (+6 more)

### Community 33 - "DashboardPage.tsx"
Cohesion: 0.04
Nodes (93): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 1. `service.status`, 2. `sync.progress`, 4. `change.detected` (+85 more)

### Community 34 - "NewService"
Cohesion: 0.18
Nodes (18): TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed(), NewService(), TestConcurrentIssuePairUniqueTokenIDs(), TestElevationExpired(), TestElevationRequiresOwner() (+10 more)

### Community 35 - "DecodeKey"
Cohesion: 0.11
Nodes (22): ProviderConfig, Handler, Handler, primaryProviderConfig(), Handler, DecodeKey(), Decrypt(), DeriveKey() (+14 more)

### Community 36 - "SnapshotEntity"
Cohesion: 0.12
Nodes (44): SnapshotEntity, TestBuildInterfaceTableAttributes(), buildInterfaceTable(), buildDatasets(), buildDisks(), buildInterfaces(), buildNFSShares(), buildPools() (+36 more)

### Community 37 - "response.go"
Cohesion: 0.08
Nodes (27): Handler, Handler, Cursor(), DecodeCursor(), EncodeCursor(), T, NextCursor(), TestCursorRequestModes() (+19 more)

### Community 38 - "RunMigrations"
Cohesion: 0.08
Nodes (51): confirm(), formatCounts(), main(), runRestore(), newSeededStore(), TestRunRestoreImportsIntoConfiguredDatabase(), TestRunRestoreRejectsCorruptedBundle(), TestRunRestoreRequiresFileFlag() (+43 more)

### Community 39 - "home_assistant/tables.go"
Cohesion: 0.10
Nodes (38): jsonType(), TestAttributeCatalogCoversEmittedKeys(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations(), buildOverview() (+30 more)

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
Cohesion: 0.17
Nodes (6): TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), TestBuildHostOverrideTableMalformedCases(), TestBuildHostOverrideTableValidOverrides(), Connector

### Community 44 - "docker_test.go"
Cohesion: 0.05
Nodes (48): buildDockerTLSConfig(), newDockerClient(), newTCPDockerClient(), init(), generateSelfSignedCert(), generateSSHHostKey(), serveOneHTTPExchange(), serveSSHDockerConn() (+40 more)

### Community 45 - "ConnectorRecord"
Cohesion: 0.15
Nodes (13): ConnectorRecord, Store, scanConnector(), scanConnectorRows(), nullInt64ToIntPtr(), nullStrToStr(), scanSyncRun(), connectorWithRole (+5 more)

### Community 46 - "Compare"
Cohesion: 0.15
Nodes (18): configPushLanded(), driftDescription(), Checker, highestDriftSeverity(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare() (+10 more)

### Community 47 - "lists.go"
Cohesion: 0.12
Nodes (34): buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP(), groupNames(), parseAdlistsV6() (+26 more)

### Community 48 - "portainer/tables.go"
Cohesion: 0.12
Nodes (33): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEnvironmentTable(), buildStackTable(), cell(), containerRows(), environmentNames(), environmentTypeName() (+25 more)

### Community 49 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (32): statusInfo, unavailable(), upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo() (+24 more)

### Community 50 - "MarshalConnectorConfig"
Cohesion: 0.18
Nodes (15): TestDiagnosticsRedactsSecrets(), IsSecretFieldType(), MarshalConnectorConfig(), SecretFieldsChanged(), init(), TestConnectorRotationFieldsRoundTrip(), TestCreateConnectorDefaultsSecretRotatedAtToCreatedAt(), TestSecretFieldsChangedFalseOnRenameOnly() (+7 more)

### Community 51 - "home_assistant_test.go"
Cohesion: 0.14
Nodes (28): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+20 more)

### Community 52 - "middleware.go"
Cohesion: 0.09
Nodes (22): AuditRecorder, ConnectorRoleChecker, contextKey, elevationError, PermissionChecker, contextWithShareLink(), Handler, newShareToken() (+14 more)

### Community 53 - "NewStore"
Cohesion: 0.15
Nodes (27): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+19 more)

### Community 54 - "Dispatcher"
Cohesion: 0.10
Nodes (22): discordPayload(), sendDiscordChannel(), sendGenericWebhookChannel(), sendNtfyChannel(), sendSlackChannel(), sendTelegramChannel(), slackPayload(), TestSendNtfyChannel_DefaultsToNtfySh() (+14 more)

### Community 55 - "Store"
Cohesion: 0.15
Nodes (4): SnapshotRecord, Store, Store, GoldenSnapshotRecord

### Community 56 - "unifi/tables.go"
Cohesion: 0.17
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+21 more)

### Community 57 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.06
Nodes (28): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, Manifest, checksum, and verification, 1. Every export gets a manifest and a checksum, 2. Verifying a backup actually restores, 3. Restoring for real (+20 more)

### Community 58 - "net/http.Handler"
Cohesion: 0.09
Nodes (25): CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders(), Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken() (+17 more)

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

### Community 63 - "SystemPage.tsx"
Cohesion: 0.03
Nodes (103): web_src_api_generated_notifications_notifications_usegetnotificationsdeliveries, web_src_api_generated_settings_settings, web_src_api_generated_settings_settings_getgetaiconfigfallbackprovidersquerykey, web_src_api_generated_settings_settings_getgetaiconfigquerykey, web_src_api_generated_settings_settings_getgetauthconfigquerykey, web_src_api_generated_settings_settings_getgetnotificationsconfigquerykey, web_src_api_generated_settings_settings_postaiconfigtest, web_src_api_generated_settings_settings_postnotificationsconfigtest (+95 more)

### Community 64 - "newTestHandler"
Cohesion: 0.05
Nodes (77): mockElevateOIDCServer, secondFactorInput, virtualAuthenticator, NewHandler(), doJSON(), testHandler, req(), TestChangePassword() (+69 more)

### Community 65 - "routerDeps"
Cohesion: 0.10
Nodes (34): routerDeps, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountConnectorRoutes(), chi.Router (+26 more)

### Community 66 - "main"
Cohesion: 0.09
Nodes (26): main(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), Embedder, EmbedRegistry (+18 more)

### Community 67 - "NewRegistry"
Cohesion: 0.13
Nodes (27): Provider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail(), TestSuggestWithFallbackFirstProviderSucceeds(), TestSuggestWithFallbackNoProviders() (+19 more)

### Community 68 - "newTestHandler"
Cohesion: 0.07
Nodes (63): AssertMatchesSpec(), loadSpec(), specPath(), actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews() (+55 more)

### Community 69 - "Register"
Cohesion: 0.13
Nodes (25): init(), init(), init(), init(), init(), init(), init(), init() (+17 more)

### Community 70 - "traefik_test.go"
Cohesion: 0.08
Nodes (42): catalog(), TestRegisteredSchema(), TestSchemaConfigValidation(), TestAllConnectorImplementationsRegister(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestSchemaExposesAPIVersion() (+34 more)

### Community 71 - "Connector"
Cohesion: 0.16
Nodes (8): apiMessage(), controllerName(), countByKind(), statusError(), unavailable(), Connector, sectionFetch, session

### Community 72 - "Service"
Cohesion: 0.16
Nodes (13): Claims, ElevationClaims, ElevationToken, IssuePairOptions, MFAClaims, MFATicket, Service, TokenPair (+5 more)

### Community 73 - "Config"
Cohesion: 0.11
Nodes (21): NewWebAuthnService(), TestWebAuthnRPConfig(), WebAuthnRPConfig(), Config, LogSettings, IsSSHRemote(), AISettings, AuthSettings (+13 more)

### Community 74 - "Store"
Cohesion: 0.22
Nodes (22): connectorIDs(), docIDs(), exportDocs(), exportTemplates(), exportWithin(), AIConfigSummary, Import(), importBundle() (+14 more)

### Community 75 - "handlers.ts"
Cohesion: 0.07
Nodes (27): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+19 more)

### Community 76 - "Connector"
Cohesion: 0.06
Nodes (16): init(), ConfigField, Connector, TestBuildPeerTableAttributes(), TestBuildPolicyTableAttributes(), buildPeerTable(), buildPolicyTable(), buildRouteTable() (+8 more)

### Community 77 - "NewEngine"
Cohesion: 0.14
Nodes (25): RequestedFields(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector(), TestRunSyncFieldsPassesHintToConnector(), TestRunSyncFieldsSurvivesCredentialRefresh() (+17 more)

### Community 78 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 79 - "SuggestRequest"
Cohesion: 0.09
Nodes (14): claudeProvider, ollamaEmbedder, openAICompatibleProvider, StubProvider, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet() (+6 more)

### Community 80 - ".OIDCCallback"
Cohesion: 0.11
Nodes (18): Handler, newOIDCUser(), validHostPort(), OIDCClaims, OIDCProvider, OIDCProvider, ClientIP(), hostOnly() (+10 more)

### Community 81 - "createUser"
Cohesion: 0.28
Nodes (13): seedAlert(), TestListAttentionItems(), seedChange(), TestListChanges(), TestListConnectors(), enableFakeEmbedding(), TestSearchDocs(), seedFinding() (+5 more)

### Community 82 - "templates_test.go"
Cohesion: 0.26
Nodes (15): templateBody, TestTemplateMutationRoleMatrix(), testApp, seedPreviewConnector(), seedTemplate(), TestTemplatesConcurrentUpdatesCreateDistinctVersions(), TestTemplatesPreviewAffectedConnectors(), TestTemplatesPreviewCapturesMissingSnapshot() (+7 more)

### Community 83 - "timeline.ts"
Cohesion: 0.14
Nodes (17): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+9 more)

### Community 84 - "AuthMiddleware"
Cohesion: 0.12
Nodes (19): APIKeyChecker, fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, UserStatusChecker, AuthMiddleware(), assertElevationAuditCalls(), boolLabel() (+11 more)

### Community 85 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 86 - "devDependencies"
Cohesion: 0.08
Nodes (24): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+16 more)

### Community 87 - "system/handlers_test.go"
Cohesion: 0.23
Nodes (15): Handler, newTestHandler(), TestDiagnostics(), TestExportAudit(), TestExportImportBackupRoundTrip(), TestGetBackupScheduleDefault(), TestGetRetentionSettingsDefault(), TestHealth() (+7 more)

### Community 88 - "AuthedUser"
Cohesion: 0.12
Nodes (30): TestEmbeddedSPAWithoutFrontendBuild(), NewHandler(), TestCreate(), TestList(), TestRevoke(), AuthedUser(), JWTService(), Token() (+22 more)

### Community 89 - "git.go"
Cohesion: 0.05
Nodes (37): fileName(), slugify(), gitAuth(), Exporter, installHTTPS(), TestGitAuthHTTPSNoToken(), TestGitAuthHTTPSToken(), TestGitAuthSSH() (+29 more)

### Community 90 - "Hub"
Cohesion: 0.14
Nodes (7): Hub, github.com/gorilla/websocket.Conn, github.com/gorilla/websocket.Upgrader, broadcastMsg, Client, Revalidator, ticket

### Community 91 - "nilToStr"
Cohesion: 0.18
Nodes (7): nilToStr(), DocVersionRecord, Store, DeliveryRecord, DeliveryStatus, Store, scanDelivery()

### Community 92 - "apikey_scope.go"
Cohesion: 0.22
Nodes (11): APIKeyRestriction, APIKeyRestrictionFromContext(), ClampConnectorRole(), ContextWithAPIKeyRestriction(), isSafeMethod(), TestClampConnectorRole(), treatAsSafeFromContext(), TreatAsSafeMethod() (+3 more)

### Community 93 - "newRouterDeps"
Cohesion: 0.16
Nodes (12): Config, chi.Router, NewRouter(), newRouterDeps(), spaHandler(), NewHandler(), TestCreateAndList(), migrationFiles() (+4 more)

### Community 94 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 95 - "export_test.go"
Cohesion: 0.20
Nodes (17): fetchAllDocs(), Exporter, IsGeneratedName(), NewExporter(), pruneStale(), RunExportOnce(), newTestStore(), readFile() (+9 more)

### Community 96 - "gitTarget"
Cohesion: 0.19
Nodes (9): commitMessage(), TestCommitMessage(), commitResult, gitTarget, git.Repository, github.com/go-git/go-git/v5/plumbing.Hash, github.com/go-git/go-git/v5/plumbing/object.Signature, github.com/go-git/go-git/v5/plumbing.ReferenceName (+1 more)

### Community 97 - "Store"
Cohesion: 0.27
Nodes (4): decodeConnectorIDs(), APIKey, Store, scanAPIKey()

### Community 98 - "NewHTTPClient"
Cohesion: 0.12
Nodes (16): newConnector(), Connector, newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), intConfig(), newConnector(), NewHTTPClient() (+8 more)

### Community 99 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 100 - "NewMalformedResponseError"
Cohesion: 0.14
Nodes (11): NewMalformedResponseError(), WantsField(), TestBuildContainerTableAttributes(), buildContainerTable(), Connector, putMetadata(), unavailable(), agentEnabled() (+3 more)

### Community 101 - "Deps"
Cohesion: 0.23
Nodes (18): registerListAttentionItems(), registerListChanges(), jsonResult(), registerListConnectors(), registerSearchDocs(), connectorAllowSet(), registerListFindings(), NewHTTPHandler() (+10 more)

### Community 102 - "time.Time"
Cohesion: 0.10
Nodes (16): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), digestDue(), formatDigest(), Dispatcher, TestDigestDue(), sshStdioConn (+8 more)

### Community 103 - "snapshot_attributes_test.go"
Cohesion: 0.29
Nodes (3): TestSnapshotAttributesRoundTripPostgres(), TestSnapshotAttributesRoundTripSQLite(), testSnapshotWithAttributes()

### Community 104 - "connector_permission.go"
Cohesion: 0.19
Nodes (9): auditConnectorGrantDiffJSON(), getConnectorGrant(), ConnectorGrantDiff, Store, highestConnectorRole(), listOIDCConnectorGrants(), scanConnectorGrants(), upsertConnectorGrant() (+1 more)

### Community 105 - "router.go"
Cohesion: 0.13
Nodes (22): wsRoleLabel(), go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts, go_pkg_github_com_wiselabz_wiselabz_internal_api_apikeys, go_pkg_github_com_wiselabz_wiselabz_internal_api_attention, go_pkg_github_com_wiselabz_wiselabz_internal_api_auth, go_pkg_github_com_wiselabz_wiselabz_internal_api_changes, go_pkg_github_com_wiselabz_wiselabz_internal_api_chat, go_pkg_github_com_wiselabz_wiselabz_internal_api_compliance (+14 more)

### Community 106 - "net/http.Request"
Cohesion: 0.06
Nodes (27): oidcElevateFlow, clearFlowCookie(), clearOIDCFlowCookie(), clearOIDCElevateFlowCookie(), readOIDCElevateFlowCookie(), setOIDCElevateFlowCookie(), oidcFlowCookieName(), readOIDCFlowCookie() (+19 more)

### Community 107 - "chat/chat.go"
Cohesion: 0.14
Nodes (18): buildPrompt(), TestBuildPrompt(), Handler, cosineSimilarity(), Match, packVector(), Retrieve(), SplitSections() (+10 more)

### Community 108 - "Handler"
Cohesion: 0.24
Nodes (6): definition(), record(), reportJSON(), valid(), Handler, input

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

### Community 115 - "NotificationRecord"
Cohesion: 0.26
Nodes (4): Dispatcher, NotificationRecord, Store, scanNotification()

### Community 116 - "ws/ws_test.go"
Cohesion: 0.20
Nodes (17): NewHub(), normalizeOrigin(), assertEnvelope(), setupWSConnection(), TestBroadcastFullQueueDoesNotBlock(), TestBroadcastToUserAfterUpgrade(), TestClientCloseDisconnect(), TestDocLockEventBroadcast() (+9 more)

### Community 117 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 118 - "Handler"
Cohesion: 0.24
Nodes (7): NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), Handler, RuleEvaluator

### Community 120 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 121 - "ReportsPage.tsx"
Cohesion: 0.11
Nodes (19): web_src_api_generated_reports_reports, web_src_api_generated_reports_reports_deletereportsdefinitionsreportdefinitionid, web_src_api_generated_reports_reports_getgetreportsdefinitionsquerykey, web_src_api_generated_reports_reports_getgetreportsquerykey, web_src_api_generated_reports_reports_postreportsdefinitions, web_src_api_generated_reports_reports_postreportsdefinitionsreportdefinitionidrun, web_src_api_generated_reports_reports_putreportsdefinitionsreportdefinitionid, web_src_api_generated_reports_reports_usegetreports (+11 more)

### Community 122 - "templates.fixtures.ts"
Cohesion: 0.18
Nodes (11): web_src_api_model_index_docversion, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, resolveToken(), Snapshot (+3 more)

### Community 123 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 124 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 125 - "testApp"
Cohesion: 0.23
Nodes (9): testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty(), TestBackupRoutesRequireOperatorRole(), TestBackupScheduleGetDefaults(), TestBackupScheduleUpdate(), TestBackupScheduleUpdateDoesNotLeakSchedulerJobs() (+1 more)

### Community 126 - "config_cmd_test.go"
Cohesion: 0.21
Nodes (11): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), Schema(), schemaFor() (+3 more)

### Community 127 - "changes/handlers_test.go"
Cohesion: 0.30
Nodes (14): NewHandler(), Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound() (+6 more)

### Community 128 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 130 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 131 - "api/auth/oidc.go"
Cohesion: 0.06
Nodes (29): TestEmailDomainAllowed(), TestOIDCConnectorRolesForGroups(), TestOIDCRoleForGroups(), connectorRoleLess(), emailDomainAllowed(), oidcConnectorRolesForGroups(), oidcRoleForGroups(), Config (+21 more)

### Community 132 - "templatefuncs.go"
Cohesion: 0.17
Nodes (12): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+4 more)

### Community 133 - "data.go"
Cohesion: 0.24
Nodes (15): ChangeEntry, ComplianceSection, ConnectorDrift, DefinitionSummary, DocChangeEntry, DocsSection, DriftSection, FindingSummary (+7 more)

### Community 134 - "Engine"
Cohesion: 0.21
Nodes (7): Engine, dedupKey(), matchReason(), TemplateFuncs(), GenerateResult, renderResult, text/template.FuncMap

### Community 135 - "api/mcp_test.go"
Cohesion: 0.17
Nodes (8): go_pkg_github_com_mark3labs_mcp_go_client, go_pkg_github_com_mark3labs_mcp_go_client_transport, go_pkg_github_com_mark3labs_mcp_go_mcp, go_pkg_github_com_mark3labs_mcp_go_server, go_pkg_github_com_wiselabz_wiselabz_internal_chat, changeSummary, connectorSummary, findingSummary

### Community 136 - "pagination_contract_test.go"
Cohesion: 0.21
Nodes (13): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_go_ast (+5 more)

### Community 137 - "newTestHandler"
Cohesion: 0.22
Nodes (13): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), NewHandler(), Handler, newTestHandler() (+5 more)

### Community 138 - "net/http.Client"
Cohesion: 0.05
Nodes (22): Connector, openAIEmbedder, NewServiceUnavailableError(), setHeaders(), TestValidateCustomURL(), tryParseEntities(), validateCustomURL(), Connector (+14 more)

### Community 139 - "gitFixture"
Cohesion: 0.32
Nodes (8): SetBeforePushForTest(), newGitFixture(), TestGitExportLifecycle(), TestGitExportPushRejectionReturnsError(), TestGitExportRefusesForeignDirectory(), gitFixture, Exporter, github.com/go-git/go-git/v5/plumbing/object.Commit

### Community 140 - "render_test.go"
Cohesion: 0.31
Nodes (13): RenderHTML(), RenderMarkdown(), sampleData(), TestRenderHTML_EscapesDocTitles(), TestRenderHTML_SectionUnavailable(), TestRenderHTML_Truncated(), TestRenderMarkdown_Golden(), TestRenderMarkdown_SectionUnavailable() (+5 more)

### Community 141 - "log/slog.Logger"
Cohesion: 0.12
Nodes (23): newLifecycleManager(), newLogger(), Dispatcher, RunDeliveryRetries(), RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors() (+15 more)

### Community 142 - "Connector"
Cohesion: 0.14
Nodes (13): SnapshotSection, MapTransportError(), TestMapTransportError(), TestBuildHostsTableAttributes(), TestBuildHostsTableV5(), buildHostsTable(), Connector, parseHosts() (+5 more)

### Community 143 - "Contributing to WiseLabz"
Cohesion: 0.15
Nodes (13): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+5 more)

### Community 144 - "dashboard/handlers_test.go"
Cohesion: 0.36
Nodes (8): NewHandler(), Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout()

### Community 145 - "diagram.go"
Cohesion: 0.18
Nodes (16): ServiceDependency, environmentDependencies(), poolDependencies(), networkDependencies(), entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash() (+8 more)

### Community 146 - "Runner"
Cohesion: 0.08
Nodes (30): newFakeHealthStore(), TestJobHealthOkToFailingNotifiesOnce(), TestJobHealthPanicCountsAsFailure(), TestJobHealthPersistsAcrossRestart(), TestJobHealthWithoutStoreDoesNothing(), cron.EntryID, Runner, New() (+22 more)

### Community 147 - "Engine"
Cohesion: 0.25
Nodes (4): NewHandler(), Engine, sync.Map, DocRegenerator

### Community 148 - "docs/handlers_test.go"
Cohesion: 0.19
Nodes (16): NewHandler(), TestAISuggestInvalidJSON(), TestByServiceNoDocsYet(), TestGenerate(), TestGetLockNoneHeld(), TestGetRootIsSynthetic(), TestGetUnknownIDFallsBackToServicePlaceholder(), TestListEmpty() (+8 more)

### Community 149 - "transform_test.go"
Cohesion: 0.24
Nodes (8): init(), normalizeEnabledColumn(), normalizeFirewallRules(), RegisterTransformer(), TestNormalizeFirewallRulesRewritesEnabledColumn(), TestRunTransformersAppliesInOrderAndStopsOnError(), Transformer, TransformerFunc

### Community 150 - "Store"
Cohesion: 0.16
Nodes (7): Store, ChatConversationRecord, Store, apiKeyConnectorFilter(), AttentionItem, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 151 - "Decision"
Cohesion: 0.17
Nodes (11): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+3 more)

### Community 153 - "scripts"
Cohesion: 0.17
Nodes (12): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+4 more)

### Community 154 - "seedHealthTestConnector"
Cohesion: 0.31
Nodes (10): testApp, registerHealthFakeType(), seedHealthTestConnector(), TestConnectorsHealthDegraded(), TestConnectorsHealthDoesNotCreateSnapshot(), TestConnectorsHealthOffline(), TestConnectorsHealthOnline(), TestConnectorsHealthRecordsTimeSeriesRow() (+2 more)

### Community 155 - "pfsense.go"
Cohesion: 0.48
Nodes (5): buildGatewayTable(), buildInterfaceTable(), buildSystemContent(), primaryGatewayName(), wanInterfaceName()

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

### Community 161 - ".GetConnectorUptime"
Cohesion: 0.33
Nodes (3): Store, HealthCheckRecord, UptimeStats

### Community 162 - "openapi_contract_test.go"
Cohesion: 0.33
Nodes (8): normalizeParams(), routerOperations(), specOperations(), TestAPIV1AliasServesSameHandlers(), TestOpenAPIHealthProbeRoutes(), TestOpenAPIMatchesRouter(), chi.Routes, go_pkg_go_yaml_in_yaml_v3

### Community 163 - ".call"
Cohesion: 0.44
Nodes (6): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture

### Community 164 - "Changelog"
Cohesion: 0.20
Nodes (9): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features, Unreleased (+1 more)

### Community 165 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 167 - "apikey_scopes_test.go"
Cohesion: 0.47
Nodes (8): createKey(), testApp, newConnector(), TestAPIKeyCreateValidation(), TestAPIKeyDefaultsToFullScope(), TestConnectorRestrictedAPIKey(), TestReadOnlyAPIKey(), TestReadOnlyAPIKeyCapsConnectorRoleAtViewer()

### Community 168 - "ComplianceRuleRecord"
Cohesion: 0.36
Nodes (4): changedFields(), ComplianceRuleRecord, Store, scanComplianceRule()

### Community 169 - "ReportData"
Cohesion: 0.56
Nodes (3): connectorFilter(), Generator, ReportData

### Community 170 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 171 - "APIKeyClaims"
Cohesion: 0.50
Nodes (3): testAPIKeyChecker, APIKeyClaims, validAPIKey()

### Community 172 - "TestComplianceRuleValidation"
Cohesion: 0.29
Nodes (7): badRegexMessage(), complianceCondition(), TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails()

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

### Community 192 - "ClassifyHealth"
Cohesion: 0.67
Nodes (3): ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold()

### Community 194 - "engine_maintenance_test.go"
Cohesion: 0.60
Nodes (5): driftingSnapshot(), setupMaintenanceTestConnector(), TestRunSyncExpiredMaintenanceWindowBehavesNormally(), TestRunSyncNoMaintenanceWindowBehavesNormally(), TestRunSyncSuppressesChangesDuringMaintenanceWindow()

### Community 195 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 196 - "browser.ts"
Cohesion: 0.40
Nodes (4): bootstrap(), worker, enableMocks(), handlers

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
- **22 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Store` connect `Store` to `Handler`, `testing.T`, `Engine`, `newTestHandler`, `gitFixture`, `Errorf`, `log/slog.Logger`, `ServiceSnapshot`, `dashboard/handlers_test.go`, `go_pkg_context`, `Engine`, `docs/handlers_test.go`, `Manager`, `runbooks/handlers_test.go`, `NewEngine`, `rowScanner`, `ExportToFile`, `dispatcher_test.go`, `.call`, `.call`, `response.go`, `RunMigrations`, `NewChecker`, `share_links_test.go`, `ReportData`, `NewStore`, `Dispatcher`, `net/http.ResponseWriter`, `rewritePlaceholders`, `newTestHandler`, `main`, `NewRegistry`, `engine_maintenance_test.go`, `NewEngine`, `createUser`, `AuthedUser`, `newRouterDeps`, `export_test.go`, `Deps`, `time.Time`, `net/http.Request`, `chat/chat.go`, `Handler`, `diagnostics/diagnostics.go`, `Handler`, `Handler`, `testApp`, `changes/handlers_test.go`?**
  _High betweenness centrality (0.016) - this node is a cross-community bridge._
- **Why does `Hub` connect `Hub` to `api/auth/oidc.go`, `NewRegistry`, `NewChecker`, `net/http.Request`, `Errorf`, `log/slog.Logger`, `NewEngine`, `time.Duration`, `Engine`, `docs/handlers_test.go`, `testApp`, `Dispatcher`, `ws/ws_test.go`, `net/http.ResponseWriter`, `dispatcher_test.go`, `newRouterDeps`, `changes/handlers_test.go`?**
  _High betweenness centrality (0.009) - this node is a cross-community bridge._
- **Why does `gitTarget` connect `gitTarget` to `git.go`, `export_test.go`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _562 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.019167926644881237 - nodes in this community are weakly interconnected._
- **Should `newDocTestStore` be split into smaller, more focused modules?**
  _Cohesion score 0.02184420312598079 - nodes in this community are weakly interconnected._
- **Should `App.tsx` be split into smaller, more focused modules?**
  _Cohesion score 0.0456140350877193 - nodes in this community are weakly interconnected._