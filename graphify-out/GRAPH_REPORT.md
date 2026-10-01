# Graph Report - fix-bugs-E  (2026-10-01)

## Corpus Check
- 964 files · ~661,726 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 24 file(s) not represented in the graph (top: (none) 13, .toml 2, .tmpl 2)

## Summary
- 7323 nodes · 21992 edges · 289 communities (263 shown, 26 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 1747 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `4995ab26`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestApp
- cn
- newDocTestStore
- testing.T
- ProfilePage.tsx
- ServicesPage.tsx
- context.Context
- go_pkg_context
- Errorf
- go_pkg_net_http
- package.json
- newTestHandler
- go_pkg_strings
- @tanstack/react-query
- App.tsx
- icons.tsx
- DashboardPage.tsx
- net/http.Client
- ErrorSection
- NewMalformedResponseError
- .runPerRevision
- UsersPage.tsx
- DecodeKey
- RunMigrations
- net/http.ResponseWriter
- Events
- ErrorWithDetails
- NewEngine
- NewStore
- ServiceDetailPage.tsx
- SuggestRequest
- fixtures.ts
- net/http.Request
- snapshotreport.go
- dispatcher_test.go
- ConnectorRecord
- SnapshotEntity
- Runner
- ThemeControls.tsx
- SystemPage.tsx
- Store
- dependencies
- NewUser
- NewEngine
- home_assistant/tables.go
- ADDED Requirements
- Error Creation
- response.go
- traefik/tables.go
- NewChecker
- ADDED Requirements
- Register
- validate.go
- Config
- portainer/tables.go
- go_pkg_testing
- NewService
- logging_test.go
- adguardhome/tables.go
- routerDeps
- connector_permission.go
- compliance/engine.go
- ws/ws_test.go
- Store
- Dispatcher
- Manager
- store/backup_test.go
- main
- home_assistant_test.go
- Connector
- Service
- unifi/tables.go
- api/docs_test.go
- Common Go Bugs
- Store
- config_test.go
- react-i18next
- .OIDCCallback
- GetTypeSchema
- rewritePlaceholders
- settings.mock.ts
- Connector
- handlers.ts
- runbooks_test.go
- api/auth/webauthn_test.go
- NotificationRecord
- NewRegistry
- unifi_test.go
- timeline.ts
- Go Code Style
- testApp
- New
- devDependencies
- Codebase Design
- middleware.go
- go_pkg_strconv
- Checker
- Hub
- WiseLabz — Design Contract
- router.go
- go_pkg_os
- Sanitize
- AppearancePage.tsx
- Compare
- portainer_test.go
- Store
- adguardhome_test.go
- HTML Report Format
- export_test.go
- backup/backup.go
- docker_test.go
- server.go
- WiseLabz — Architecture & Technical Decisions
- Configuration & Documentation Backup (Export/Import)
- ADDED Requirements
- Decisions
- Requirements
- connectors_health_test.go
- newTestHandler
- Backend test performance
- ws.ts
- dashboard_test.go
- fetch_test.go
- truenas_test.go
- Connector
- diagnostics/diagnostics.go
- gitFixture
- Decision
- compilerOptions
- compliance/handlers.go
- Handler
- ExportToFile
- SuggestWithFallback
- traefik_test.go
- time.Duration
- RunbookRecord
- sshStdioConn
- docdiffmodel.ts
- vectorCache
- notifications/handlers_test.go
- all.go
- MarshalConnectorConfig
- httpx/retry_test.go
- ADDED Requirements
- compilerOptions
- handlers_contract_test.go
- handlers_actions_test.go
- templates.fixtures.ts
- connector/connector.go
- data.go
- New
- DocRecord
- RateLimit
- changes/handlers_test.go
- runbooks/handlers_test.go
- pagination_contract_test.go
- newTestHandler
- Get
- ServiceSnapshot
- Contributing to WiseLabz
- Backup Recovery: What Comes Back, and What Doesn't
- Go Database Best Practices
- golang-troubleshooting/SKILL.md
- Delve Debugger
- General Debugging Methodology
- Production Debugging
- The Golden Rules
- Handler
- api/changes_test.go
- doJSON
- Handler
- newTestHarness
- Changelog
- WiseLabz
- ARCHITECTURE.md
- Decisions
- scripts
- .agents/skills/openspec-explore/SKILL.md
- diagram.go
- apikey_scope.go
- IsSecureRequest
- scheduler/health_test.go
- .claude/skills/openspec-explore/SKILL.md
- Contributor Covenant Code of Conduct
- Decision
- Decision
- WiseLabz Connector Guide
- main.tsx
- Engine
- Engine
- explore.md
- Decision
- done
- .call
- Cache
- .call
- Handler
- migrations.go
- transform_test.go
- Tasks
- Tasks
- Product
- changes.sh
- test-shards.sh
- mockServiceWorker.js
- pprof Reference
- Handler
- config_cmd_test.go
- TestElevateOIDC
- handlers_bulk_test.go
- openapi_contract_test.go
- connectors_maintenance_test.go
- Elector
- release-please-config.json
- Database Performance
- Transactions, Isolation Levels, and Locking
- Testing Database Code
- dashboard/handlers_test.go
- retention/retention_test.go
- BackupRun
- auth/handlers_test.go
- Handler
- Audit Trail
- Step by step
- Proposal
- Proposal
- Proposal
- Proposal
- chat/chat.go
- runbook_test.go
- WithAuth
- newHandler
- TestComplianceRuleValidation
- snapshotResponse
- scanMaintenanceWindow
- computeNextRun
- session_test.go
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- .UpdateAuthConfig
- sync.Mutex
- areas.sh script
- Struct Scanning and NULLable Columns
- Test-Driven Debugging
- walkCursorPages
- browser.ts
- withGrant
- golden_snapshot_test.go
- TestLocalLoginDisabledIsEnforced
- internal/auth/oidc.go
- versionSections
- Design
- Tasks
- Design
- Tasks
- Security Policy
- Batch Processing
- Compilation Issues
- Concurrency Debugging
- Web Interface Guidelines
- Authentication design
- Development workflow
- compose-smoke.sh
- Indexing Strategy
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

## God Nodes (most connected - your core abstractions)
1. `newTestApp()` - 239 edges
2. `Errorf()` - 188 edges
3. `newDocTestStore()` - 146 edges
4. `Store` - 143 edges
5. `UserIDFromContext()` - 88 edges
6. `SnapshotEntity` - 77 edges
7. `react` - 77 edges
8. `cn()` - 69 edges
9. `NewStore()` - 67 edges
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

## Communities (289 total, 26 thin omitted)

### Community 0 - "newTestApp"
Cohesion: 0.02
Nodes (165): templateBody, testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow() (+157 more)

### Community 1 - "cn"
Cohesion: 0.03
Nodes (97): web_src_api_generated_changes_changes_getgetchangeschangeidquerykey, web_src_api_generated_changes_changes_postchangeschangeidack, web_src_api_generated_changes_changes_postchangeschangeidaiupdate, web_src_api_generated_changes_changes_postchangeschangeiddismiss, web_src_api_generated_changes_changes_postchangeschangeidexplain, web_src_api_generated_changes_changes_usegetchangeschangeid, web_src_api_generated_chat_chat, web_src_api_generated_chat_chat_getgetchatconversationsidquerykey (+89 more)

### Community 2 - "newDocTestStore"
Cohesion: 0.02
Nodes (162): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+154 more)

### Community 3 - "testing.T"
Cohesion: 0.02
Nodes (140): TestClaudeSuggest(), TestClaudeSuggestDefaultMaxTokens(), TestClaudeSuggestErrors(), TestClaudeSuggestMultipleContentBlocks(), TestOpenAICompatibleSuggest(), TestOpenAICompatibleSuggestErrors(), TestOIDCRedirectURL(), TestFindOIDCProvider() (+132 more)

### Community 4 - "ProfilePage.tsx"
Cohesion: 0.03
Nodes (94): qrcode, react, web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_postauthelevate, web_src_api_generated_auth_auth_postauthelevatewebauthnbegin, web_src_api_generated_auth_auth_postauthloginmfawebauthnbegin (+86 more)

### Community 5 - "ServicesPage.tsx"
Cohesion: 0.05
Nodes (61): Frontend, 7. `quality.finding.created` and `quality.findings.changed`, match-sorter, motion, @radix-ui/react-popover, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve, web_src_api_generated_alerts_alerts_postalertsalertidsnooze (+53 more)

### Community 6 - "context.Context"
Cohesion: 0.02
Nodes (36): fakeStatusChecker, sanitizeSessions(), Connector, SendTest(), Store, existingIDs(), Store, scanComplianceRule() (+28 more)

### Community 7 - "go_pkg_context"
Cohesion: 0.08
Nodes (16): contains(), searchString(), go_pkg_context, go_pkg_database_sql, go_pkg_errors, go_pkg_fmt, go_pkg_github_com_google_uuid, go_pkg_github_com_jackc_pgx_v5_stdlib (+8 more)

### Community 8 - "Errorf"
Cohesion: 0.05
Nodes (29): Handler, newToken(), sanitize(), Handler, diffToSpec(), Handler, Handler, Handler (+21 more)

### Community 9 - "go_pkg_net_http"
Cohesion: 0.05
Nodes (53): bulkSnoozeItemResult, bulkSnoozeRequest, changePromptData(), stripPromptTags(), truncateUTF8(), findingConnectorIDs(), bulkResolveItemResult, bulkResolveRequest (+45 more)

### Community 10 - "package.json"
Cohesion: 0.04
Nodes (51): clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks (+43 more)

### Community 11 - "newTestHandler"
Cohesion: 0.14
Nodes (25): TestLoginMFARejectsLockedAccountEvenWithCorrectCode(), testHandler, TestConfirmingEnrollmentUpgradesSession(), TestDeleteFactorBlockedByPolicyWhenLast(), TestElevateMethodsReflectsMFA(), TestElevatePasswordRejectedWhenMFAEnabled(), TestElevateRejectsOIDCUsers(), TestElevateWithTOTPAndRecoveryCode() (+17 more)

### Community 12 - "go_pkg_strings"
Cohesion: 0.04
Nodes (47): TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), IsTimeout(), TestIsTimeout(), dateFormat(), filterByTitle() (+39 more)

### Community 13 - "@tanstack/react-query"
Cohesion: 0.03
Nodes (62): msw, @tanstack/react-query, @testing-library/react, vitest, web_src_api_generated_alerts_alerts, web_src_api_generated_alerts_alerts_getalerts, web_src_api_generated_alerts_alerts_getgetalertsquerykey, web_src_api_generated_attention_attention_getgetattentionquerykey (+54 more)

### Community 14 - "App.tsx"
Cohesion: 0.04
Nodes (69): react-router-dom, AXIOS_INSTANCE, BodyType, ErrorType, getAccessToken(), MfaEnrollmentRequiredFn, RefreshFn, setAccessToken() (+61 more)

### Community 15 - "icons.tsx"
Cohesion: 0.04
Nodes (80): i18next, web_src_api_generated_docs_docs, web_src_api_generated_docs_docs_getgetdocsdocidquerykey, web_src_api_generated_docs_docs_getgetdocsdocidversionsquerykey, web_src_api_generated_docs_docs_getgetdocstreequerykey, web_src_api_generated_docs_docs_postdocsdocidaisuggest, web_src_api_generated_docs_docs_postdocsdocidlock, web_src_api_generated_docs_docs_postdocsdocidlockrelease (+72 more)

### Community 16 - "DashboardPage.tsx"
Cohesion: 0.05
Nodes (66): 4. `change.detected`, 5. `alert.created`, 8. `doc.generated`, web_src_api_generated_dashboard_dashboard_getdashboardlayout, web_src_api_generated_dashboard_dashboard_getdashboardlayoutadmindefault, web_src_api_generated_dashboard_dashboard_getgetdashboardlayoutadmindefaultquerykey, web_src_api_generated_dashboard_dashboard_postdashboardlayoutreset, web_src_api_generated_dashboard_dashboard_putdashboardlayout (+58 more)

### Community 17 - "net/http.Client"
Cohesion: 0.03
Nodes (29): Connector, ollamaEmbedder, openAIEmbedder, setHeaders(), TestValidateCustomURL(), tryParseEntities(), validateCustomURL(), Connector (+21 more)

### Community 18 - "ErrorSection"
Cohesion: 0.06
Nodes (26): ServiceDependency, WantsField(), Connector, TestRequestedFields(), TestWantsField(), unavailable(), buildRouteTable(), buildGatewayTable() (+18 more)

### Community 19 - "NewMalformedResponseError"
Cohesion: 0.10
Nodes (41): NewMalformedResponseError(), TestBuildHostsTableAttributes(), buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP() (+33 more)

### Community 20 - ".runPerRevision"
Cohesion: 0.14
Nodes (16): fileName(), slugify(), commitMessage(), gitAuth(), Exporter, installHTTPS(), TestGitAuthHTTPSNoToken(), TestGitAuthHTTPSToken() (+8 more)

### Community 21 - "UsersPage.tsx"
Cohesion: 0.06
Nodes (48): axios, customInstance(), web_src_api_generated_users_users, web_src_api_generated_users_users_deleteusersuserid, web_src_api_generated_users_users_getgetusersquerykey, web_src_api_generated_users_users_postusersuseridresetmfa, web_src_api_generated_users_users_postusersuseridresetpassword, web_src_api_generated_users_users_usegetusers (+40 more)

### Community 22 - "DecodeKey"
Cohesion: 0.09
Nodes (25): ProviderConfig, testHandler, Handler, instanceAdminRoleFor(), Handler, Handler, primaryProviderConfig(), Handler (+17 more)

### Community 23 - "RunMigrations"
Cohesion: 0.15
Nodes (32): main(), OpenDB(), TestOpenDBEnablesSQLiteForeignKeys(), TestOpenDBSetsSQLiteDurabilityPragmas(), newPostgresTestStore(), GetMigrationStatus(), newMigrator(), collectColumns() (+24 more)

### Community 24 - "net/http.ResponseWriter"
Cohesion: 0.11
Nodes (14): validateConfigPushRequest(), applyConnectorScalarUpdates(), Handler, validateConnectorConfig(), viewOf(), capitalize(), Handler, ValidateElevationHeader() (+6 more)

### Community 25 - "Events"
Cohesion: 0.08
Nodes (32): 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 15. `system.resync`, 1. `service.status`, 2. `sync.progress` (+24 more)

### Community 26 - "ErrorWithDetails"
Cohesion: 0.08
Nodes (29): sanitizeUser(), setRefreshCookie(), errLocalLoginDisabled(), Handler, userLocked(), factorJSON(), Handler, configRequestField() (+21 more)

### Community 27 - "NewEngine"
Cohesion: 0.25
Nodes (24): NewEngine(), newEngineTestStore(), seedEngineConnector(), seedEngineTemplate(), TestGenerateFromSnapshotIncludesDependencies(), TestGenerateFromTemplateReturnsVersionPersistenceError(), TestGenerateFromTemplateStillPersists(), TestMatchingConnectorsEmptyAppliesToIsWildcard() (+16 more)

### Community 28 - "NewStore"
Cohesion: 0.15
Nodes (27): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+19 more)

### Community 29 - "ServiceDetailPage.tsx"
Cohesion: 0.04
Nodes (59): ADR-0001, ADR-0003, web_src_api_generated_connectors_connectors_postconnectorsconnectoridconfigpush, web_src_api_generated_connectors_connectors_postconnectorsconnectoridhealth, web_src_api_generated_connectors_connectors_postconnectorsconnectoridrestart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstart, web_src_api_generated_connectors_connectors_postconnectorsconnectoridstop, web_src_api_generated_connectors_connectors_usegetconnectorsconnectoridconfigfields (+51 more)

### Community 30 - "SuggestRequest"
Cohesion: 0.13
Nodes (11): claudeProvider, openAICompatibleProvider, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet(), TestRegistryList(), TestStubProviderName() (+3 more)

### Community 31 - "fixtures.ts"
Cohesion: 0.06
Nodes (43): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+35 more)

### Community 32 - "net/http.Request"
Cohesion: 0.07
Nodes (23): oidcElevateFlow, clearFlowCookie(), clearOIDCFlowCookie(), clearOIDCElevateFlowCookie(), readOIDCElevateFlowCookie(), setOIDCElevateFlowCookie(), oidcFlowCookieName(), readOIDCFlowCookie() (+15 more)

### Community 33 - "snapshotreport.go"
Cohesion: 0.14
Nodes (26): BuildSnapshotDiff(), CompareDependencies(), CompareEntities(), entityKey(), entityMap(), TestCompareDependenciesDeterministic(), TestCompareEntitiesKeepsExternalIDAndFallbackNameDistinct(), TestSnapshotDiffCSVNeutralizesFormulaCellsOnly() (+18 more)

### Community 34 - "dispatcher_test.go"
Cohesion: 0.05
Nodes (83): TestExpireAlertsOnceNoExpiredAlertsIsNoop(), TestExpireAlertsOnceNotifiesViaDispatcher(), newLifecycleManager(), newTestLifecycle(), startTestLifecycle(), TestLeaderStartsSchedulerAndRunsJob(), TestLifecycleManagerOrderedShutdown(), TestLifecycleManagerShutdownCancelsWorkContext() (+75 more)

### Community 35 - "ConnectorRecord"
Cohesion: 0.05
Nodes (36): enableFakeEmbedding(), actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), ConnectorRecord, Store (+28 more)

### Community 36 - "SnapshotEntity"
Cohesion: 0.13
Nodes (42): SnapshotEntity, buildDatasets(), buildDisks(), buildInterfaces(), buildNFSShares(), buildPools(), buildReplicationTasks(), buildServices() (+34 more)

### Community 37 - "Runner"
Cohesion: 0.16
Nodes (7): cron.EntryID, Runner, cron.Cron, HealthStore, jobEntry, JobInfo, Notifier

### Community 38 - "ThemeControls.tsx"
Cohesion: 0.11
Nodes (31): @fontsource/space-mono, @fontsource-variable/space-grotesk, AdvancedControls(), FONT_KEYS, OPT_KEYS, PRESET_KEYS, Segmented(), ThemeControls() (+23 more)

### Community 39 - "SystemPage.tsx"
Cohesion: 0.02
Nodes (102): react-error-boundary, sonner, web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey, web_src_api_generated_compliance_compliance_postcompliancerules, web_src_api_generated_compliance_compliance_postcompliancerulestest, web_src_api_generated_compliance_compliance_putcompliancerulesid (+94 more)

### Community 40 - "Store"
Cohesion: 0.23
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 41 - "dependencies"
Cohesion: 0.05
Nodes (42): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+34 more)

### Community 42 - "NewUser"
Cohesion: 0.17
Nodes (41): GrantConnectorRole(), instanceAdminRole(), NewUser(), TestListFiltersGrantsBeforePagination(), Handler, newTestHandler(), TestAISuggestInvalidJSON(), TestGetLockOfUnknownDoc() (+33 more)

### Community 43 - "NewEngine"
Cohesion: 0.14
Nodes (31): RequestedFields(), TestBaseContext(), TestRunDueSyncsRespectsLimits(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncDeadlinePersistsDegradedStatus(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector() (+23 more)

### Community 44 - "home_assistant/tables.go"
Cohesion: 0.10
Nodes (38): jsonType(), TestAttributeCatalogCoversEmittedKeys(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations(), buildOverview() (+30 more)

### Community 45 - "ADDED Requirements"
Cohesion: 0.05
Nodes (40): ADDED Requirements, Purpose, Requirement: Classification is explained and self-tested, Requirement: Code paths map to their CI areas, Requirement: CodeQL scans only on relevant changes, Requirement: Draft pull requests defer heavy jobs, Requirement: Non-code changes skip all heavy CI jobs, Requirement: Postgres tests run only when their dependencies change (+32 more)

### Community 46 - "Error Creation"
Cohesion: 0.05
Nodes (36): Creating Errors, Custom Error Types, Custom types that wrap other errors, Decision table: which error strategy to use, Error Creation, Error String Conventions, Errors as Values, `errors.New` — static error messages (+28 more)

### Community 47 - "response.go"
Cohesion: 0.07
Nodes (28): decodeStoredSnapshot(), Handler, snapshotStoreError(), Handler, Cursor(), DecodeCursor(), EncodeCursor(), T (+20 more)

### Community 48 - "traefik/tables.go"
Cohesion: 0.14
Nodes (31): secondFactorInput, jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable() (+23 more)

### Community 49 - "NewChecker"
Cohesion: 0.17
Nodes (35): NewChecker(), createComplianceRule(), createComplianceSnapshot(), createConnector(), findings(), newTestStore(), TestCheckEmptyDetectsAndAutoResolves(), TestCheckFailingDetectsAndAutoResolves() (+27 more)

### Community 50 - "ADDED Requirements"
Cohesion: 0.05
Nodes (39): ADDED Requirements, Purpose, Requirement: Container images are pinned and updated, Requirement: Every action reference is pinned to a commit SHA, Requirement: GitHub Actions are updated monthly in one grouped PR, Requirement: Go modules and web packages are updated monthly, Requirement: Merging several PRs does not re-run CI on every open PR, Requirement: Minor and patch Dependabot PRs merge without further action (+31 more)

### Community 51 - "Register"
Cohesion: 0.14
Nodes (25): init(), init(), init(), newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), init(), init() (+17 more)

### Community 52 - "validate.go"
Cohesion: 0.32
Nodes (5): Config, mask(), redactDSN(), redactKVPassword(), TestRedactDSN()

### Community 53 - "Config"
Cohesion: 0.11
Nodes (22): NewWebAuthnService(), TestWebAuthnRPConfig(), WebAuthnRPConfig(), Config, LogSettings, IsSSHRemote(), AISettings, AuthSettings (+14 more)

### Community 54 - "portainer/tables.go"
Cohesion: 0.12
Nodes (33): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEnvironmentTable(), buildStackTable(), cell(), containerRows(), environmentNames(), environmentTypeName() (+25 more)

### Community 55 - "go_pkg_testing"
Cohesion: 0.03
Nodes (44): testApp, mintAPIKey(), TestMCPConnectorRestrictedKey(), TestMCPEndToEnd(), CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders() (+36 more)

### Community 56 - "NewService"
Cohesion: 0.09
Nodes (37): APIKeyChecker, fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed() (+29 more)

### Community 57 - "logging_test.go"
Cohesion: 0.21
Nodes (14): Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken(), TestLoggerRedactsWSTicket(), TestGetRequestIDMissing(), TestRecovererPassThrough(), TestRecovererReturns500OnPanic() (+6 more)

### Community 58 - "adguardhome/tables.go"
Cohesion: 0.14
Nodes (31): statusInfo, upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo(), buildFiltering() (+23 more)

### Community 59 - "routerDeps"
Cohesion: 0.11
Nodes (30): routerDeps, chi.Router, mountAuthRoutes(), mountMeRoutes(), mountUserRoutes(), chi.Router, mountConnectorRoutes(), chi.Router (+22 more)

### Community 60 - "connector_permission.go"
Cohesion: 0.18
Nodes (9): auditConnectorGrantDiffJSON(), getConnectorGrant(), ConnectorGrantDiff, Store, highestConnectorRole(), listOIDCConnectorGrants(), scanConnectorGrants(), upsertConnectorGrant() (+1 more)

### Community 61 - "compliance/engine.go"
Cohesion: 0.12
Nodes (29): contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity, Rule (+21 more)

### Community 62 - "ws/ws_test.go"
Cohesion: 0.12
Nodes (33): TestBroadcastDocEventScoping(), Envelope, NewHub(), normalizeOrigin(), assertEnvelope(), assertNoFrame(), connectorHub(), decodeEnvelope() (+25 more)

### Community 63 - "Store"
Cohesion: 0.06
Nodes (18): seedAlert(), changeServiceIDs(), seedChange(), Store, placeholders(), changeFilterClause(), AlertRecord, ChangeRecord (+10 more)

### Community 64 - "Dispatcher"
Cohesion: 0.13
Nodes (14): discordPayload(), sendDiscordChannel(), sendGenericWebhookChannel(), sendSlackChannel(), slackPayload(), webhookPayload(), findChannel(), findRoute() (+6 more)

### Community 65 - "Manager"
Cohesion: 0.07
Nodes (18): cron.EntryID, Manager, JobName(), LogPartial(), NewManager(), Store, nilToStr(), DeliveryRecord (+10 more)

### Community 66 - "store/backup_test.go"
Cohesion: 0.32
Nodes (11): newBackupTestStore(), TestCreateBackupRun(), TestGetBackupScheduleWhenNotExists(), TestListBackupRunsPaginated(), TestPruneBackupRunsByAge(), TestPruneBackupRunsByCount(), TestPruneBackupRunsCombinedLimits(), TestPruneBackupRunsNegativeMaxBackups() (+3 more)

### Community 67 - "main"
Cohesion: 0.13
Nodes (23): main(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), NewEmbedRegistry(), RegisterOpenAICompatible() (+15 more)

### Community 68 - "home_assistant_test.go"
Cohesion: 0.11
Nodes (33): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+25 more)

### Community 69 - "Connector"
Cohesion: 0.08
Nodes (10): init(), ConfigField, Connector, buildGatewayTable(), primaryGatewayName(), wanInterfaceName(), Connector, PathSegment() (+2 more)

### Community 70 - "Service"
Cohesion: 0.12
Nodes (15): Claims, ElevationClaims, ElevationToken, IssuePairOptions, MFAClaims, MFATicket, authSettingsSource(), RuntimeSettings (+7 more)

### Community 71 - "unifi/tables.go"
Cohesion: 0.17
Nodes (29): jsonType(), TestAttributeCatalogCoversEmittedKeys(), boolOr(), buildClientSummary(), buildDeviceTable(), buildFirewallTable(), buildNetworkTable(), buildSiteTable() (+21 more)

### Community 72 - "api/docs_test.go"
Cohesion: 0.11
Nodes (29): createKey(), testApp, newConnector(), TestAPIKeyCreateValidation(), TestAPIKeyDefaultsToFullScope(), TestConnectorRestrictedAPIKey(), TestReadOnlyAPIKey(), TestReadOnlyAPIKeyCapsConnectorRoleAtViewer() (+21 more)

### Community 73 - "Common Go Bugs"
Cohesion: 0.06
Nodes (31): `break` in `select`/`switch` Inside `for` Loop, Closed Channel in `select` Causes Busy Loop, Common Go Bugs, Concurrent Map Read/Write (Fatal), Context Misuse, Copying sync Types, Defer Gotchas, Enum Zero Value with `iota` (+23 more)

### Community 74 - "Store"
Cohesion: 0.08
Nodes (32): Provider, Config, Handler, Embedder, EmbedRegistry, Registry, NewHandler(), NewHandler() (+24 more)

### Community 75 - "config_test.go"
Cohesion: 0.10
Nodes (28): runHealthcheck(), Load(), TestAccessTokenTTLDuration(), TestDocExportGitCommitModeValidation(), TestDocExportGitValidate(), TestLoadDefaults(), TestLoadEnvOverride(), TestLoadEnvOverrideAllFields() (+20 more)

### Community 76 - "react-i18next"
Cohesion: 0.05
Nodes (46): RFC-3339, Frontend shell & theme (decided 2026-06), react-i18next, zustand, web_src_api_generated_attention_attention, web_src_api_generated_attention_attention_usegetattention, web_src_api_generated_connectors_connectors_postconnectors, web_src_api_generated_connectors_connectors_postconnectorsconnectoridsync (+38 more)

### Community 77 - ".OIDCCallback"
Cohesion: 0.14
Nodes (10): Handler, Handler, newOIDCUser(), randomOIDCToken(), validHostPort(), OIDCClaims, OIDCProvider, OIDCProvider (+2 more)

### Community 78 - "GetTypeSchema"
Cohesion: 0.10
Nodes (29): TestRegisteredSchema(), TestSchemaConfigValidation(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestSchemaExposesAPIVersion(), TestAPIKeyIsStoredAsPassword(), TestRegisteredSchema() (+21 more)

### Community 79 - "rewritePlaceholders"
Cohesion: 0.10
Nodes (14): TestAPIKeyLastUsedThrottle(), doRewritePlaceholders(), rewritePlaceholders(), TestRewritePlaceholders(), TestRewritePlaceholdersCached(), database/sql.Result, database/sql.Row, database/sql.Rows (+6 more)

### Community 80 - "settings.mock.ts"
Cohesion: 0.08
Nodes (27): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role, web_src_api_model_index_session (+19 more)

### Community 81 - "Connector"
Cohesion: 0.18
Nodes (7): unavailable(), SnapshotSection, Connector, parseHosts(), unavailable(), unavailable(), session

### Community 82 - "handlers.ts"
Cohesion: 0.07
Nodes (26): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+18 more)

### Community 83 - "runbooks_test.go"
Cohesion: 0.14
Nodes (26): runbookResp, runbookStepResp, TestAttentionRunbookLinkForAlert(), TestAttentionRunbookLinkForFinding(), createRunbookWithStep(), testApp, seedProxmoxConnector(), seedRunbook() (+18 more)

### Community 84 - "api/auth/webauthn_test.go"
Cohesion: 0.18
Nodes (24): virtualAuthenticator, flowCookieFrom(), beginWebAuthnLogin(), credentialResponse(), finishWebAuthnLogin(), testHandler, newVirtualAuthenticator(), registerVirtualAuthenticator() (+16 more)

### Community 85 - "NotificationRecord"
Cohesion: 0.26
Nodes (4): Dispatcher, NotificationRecord, Store, scanNotification()

### Community 86 - "NewRegistry"
Cohesion: 0.50
Nodes (11): NewRegistry(), NewHandler(), TestAIConfigRoundTrip(), testConfig(), TestGetAuthConfig(), TestGetDecryptedAPIKeyNoKeyStored(), TestNotificationsConfigRoundTrip(), TestNotificationsConfigSigningSecret() (+3 more)

### Community 87 - "unifi_test.go"
Cohesion: 0.19
Nodes (25): authorized(), decodeJSONBody(), Connector, newTestConnector(), passwordConfig(), TestAPIKeyIsNotSentInPasswordMode(), TestAutoDetectReportsUniFiOSError(), TestControllerErrorMessageIsSurfaced() (+17 more)

### Community 88 - "timeline.ts"
Cohesion: 0.15
Nodes (16): installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env(), heartbeat() (+8 more)

### Community 89 - "Go Code Style"
Cohesion: 0.08
Nodes (23): Code Style Details, Extract Complex Conditions, Value vs Pointer Arguments, Code Organization Within Files, Complex Conditions & Init Scope, Composite Literals, Control Flow, Cross-References (+15 more)

### Community 90 - "testApp"
Cohesion: 0.23
Nodes (9): testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty(), TestBackupRoutesRequireOperatorRole(), TestBackupScheduleGetDefaults(), TestBackupScheduleUpdate(), TestBackupScheduleUpdateDoesNotLeakSchedulerJobs() (+1 more)

### Community 91 - "New"
Cohesion: 0.08
Nodes (31): confirm(), formatCounts(), main(), runRestore(), runVerify(), newSeededStore(), TestRunRestoreImportsIntoConfiguredDatabase(), TestRunRestoreRejectsCorruptedBundle() (+23 more)

### Community 92 - "devDependencies"
Cohesion: 0.08
Nodes (25): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+17 more)

### Community 93 - "Codebase Design"
Cohesion: 0.09
Nodes (21): 1. In-process, 2. Local-substitutable, 3. Remote but owned (Ports & Adapters), 4. True external (Mock), Deepening, Dependency categories, Seam discipline, Testing strategy: replace, don't layer (+13 more)

### Community 94 - "middleware.go"
Cohesion: 0.08
Nodes (27): AuditRecorder, ConnectorRoleChecker, contextKey, elevationError, MFAChecker, PermissionChecker, UserStatusChecker, SecurityHeaders() (+19 more)

### Community 95 - "go_pkg_strconv"
Cohesion: 0.22
Nodes (5): IPKey(), TestIPKey(), go_pkg_golang_org_x_time_rate, go_pkg_net_netip, go_pkg_strconv

### Community 96 - "Checker"
Cohesion: 0.20
Nodes (7): complianceRule(), Checker, RunStaleSweepOnce(), QualityFindingRecord, scanQualityFinding(), FindingNotifier, RotationConfig

### Community 97 - "Hub"
Cohesion: 0.15
Nodes (10): Hub, newEnvelope(), newHeartbeat(), github.com/gorilla/websocket.Upgrader, Audience, broadcastMsg, ConnectorAudience, Identity (+2 more)

### Community 98 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 99 - "router.go"
Cohesion: 0.15
Nodes (21): wsRoleLabel(), go_pkg_github_com_wiselabz_wiselabz_internal_api_alerts, go_pkg_github_com_wiselabz_wiselabz_internal_api_apikeys, go_pkg_github_com_wiselabz_wiselabz_internal_api_attention, go_pkg_github_com_wiselabz_wiselabz_internal_api_auth, go_pkg_github_com_wiselabz_wiselabz_internal_api_changes, go_pkg_github_com_wiselabz_wiselabz_internal_api_chat, go_pkg_github_com_wiselabz_wiselabz_internal_api_compliance (+13 more)

### Community 100 - "go_pkg_os"
Cohesion: 0.06
Nodes (34): TestCommitMessage(), keys(), writeExportState(), TestSnapshotAttributesRoundTripPostgres(), TestSnapshotAttributesRoundTripSQLite(), testSnapshotWithAttributes(), exportCursor, exportState (+26 more)

### Community 101 - "Sanitize"
Cohesion: 0.12
Nodes (12): Handler, isWritableField(), decodeBulkRequest(), Handler, loggablePath(), loggableQuery(), ConfigPusher, Err() (+4 more)

### Community 102 - "AppearancePage.tsx"
Cohesion: 0.14
Nodes (21): MotionProvider(), AppearancePage(), ChoiceGroup(), ToggleRow(), AppearanceState, apply(), Contrast, css() (+13 more)

### Community 103 - "Compare"
Cohesion: 0.15
Nodes (18): configPushLanded(), driftDescription(), Checker, highestDriftSeverity(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare() (+10 more)

### Community 104 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 105 - "Store"
Cohesion: 0.27
Nodes (4): decodeConnectorIDs(), APIKey, Store, scanAPIKey()

### Community 106 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 107 - "HTML Report Format"
Cohesion: 0.10
Nodes (18): Call-graph collapse, Candidate card, Cross-section (good for layered shallowness), Diagram patterns, Hand-built boxes-and-arrows (when Mermaid's layout fights you), Header, HTML Report Format, Mass diagram (good for "interface as wide as implementation") (+10 more)

### Community 108 - "export_test.go"
Cohesion: 0.19
Nodes (18): fetchAllDocs(), Exporter, IsGeneratedName(), NewExporter(), pruneStale(), RunExportOnce(), newTestStore(), readFile() (+10 more)

### Community 109 - "backup/backup.go"
Cohesion: 0.19
Nodes (24): connectorIDs(), docIDs(), exportDocs(), exportTemplates(), exportWithin(), AIConfigSummary, Import(), importBundle() (+16 more)

### Community 110 - "docker_test.go"
Cohesion: 0.06
Nodes (46): buildDockerTLSConfig(), newDockerClient(), newTCPDockerClient(), init(), generateSelfSignedCert(), generateSSHHostKey(), serveOneHTTPExchange(), serveSSHDockerConn() (+38 more)

### Community 111 - "server.go"
Cohesion: 0.34
Nodes (15): registerListAttentionItems(), registerListChanges(), jsonResult(), registerListConnectors(), registerSearchDocs(), connectorAllowSet(), registerListFindings(), NewHTTPHandler() (+7 more)

### Community 112 - "WiseLabz — Architecture & Technical Decisions"
Cohesion: 0.10
Nodes (20): ADR index, AI module, API design, Backend, Build pipeline, Changes / diff contract (decided 2026-06), Connector interface, Connector management via UI (decided 2026-06-27) (+12 more)

### Community 113 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.10
Nodes (17): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, Manifest, checksum, and verification, What's excluded, and why, What's included, Bundle format (+9 more)

### Community 114 - "ADDED Requirements"
Cohesion: 0.10
Nodes (19): ADDED Requirements, Purpose, Requirement: Checks follow CI change classification, Requirement: Fail-safe to full checks, Requirement: Full-run escape hatch, Requirement: Narrowed scope within an area, Requirement: Workflow linting, Scenario: Backend-only commit (+11 more)

### Community 115 - "Decisions"
Cohesion: 0.10
Nodes (19): Context, D10. Postgres job gated on the Go dependency closure, D11. `govulncheck` gating and nightly workflow, D12. "CodeQL – Code Quality" (GitHub's built-in Code Quality scan): check before acting, D1. Keep `dorny/paths-filter` to list files; classify in `scripts/ci/changes.sh`, D2. Rules are an ordered Bash `case` table; first match wins, D3. `web/package.json` version-only rule, pull requests only, D4. Fixture table and `check` mode (+11 more)

### Community 116 - "Requirements"
Cohesion: 0.10
Nodes (19): pre-commit-hooks Specification, Purpose, Requirement: Checks follow CI change classification, Requirement: Fail-safe to full checks, Requirement: Full-run escape hatch, Requirement: Narrowed scope within an area, Requirement: Workflow linting, Requirements (+11 more)

### Community 117 - "connectors_health_test.go"
Cohesion: 0.32
Nodes (12): testApp, registerHealthFakeType(), seedHealthTestConnector(), TestConnectorsHealthDegraded(), TestConnectorsHealthDoesNotCreateSnapshot(), TestConnectorsHealthOffline(), TestConnectorsHealthOnline(), TestConnectorsHealthRecordsTimeSeriesRow() (+4 more)

### Community 118 - "newTestHandler"
Cohesion: 0.14
Nodes (24): createDNSResolverConnector(), Handler, itoa(), TestConfigFieldsHandler(), TestConfigPushHandler(), TestStartStopHandler(), Handler, newTestHandler() (+16 more)

### Community 119 - "Backend test performance"
Cohesion: 0.11
Nodes (19): Backend test performance, CI job times, CI measurements, Coverage strategy, Dependency updates and action pinning, Deterministic scheduled exports (#408), Fixture reuse and lifecycle tests (#405–#407), Follow-ups (+11 more)

### Community 120 - "ws.ts"
Cohesion: 0.10
Nodes (19): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+11 more)

### Community 121 - "dashboard_test.go"
Cohesion: 0.15
Nodes (19): dashboardLayout, scopedOverview, mustHashDummyPassword(), assertOnlyConnector(), dashboardConnector(), getOverview(), testApp, TestDashboardAdminDefaultPermissionGate() (+11 more)

### Community 122 - "fetch_test.go"
Cohesion: 0.15
Nodes (17): testApp, wsDial(), entityKinds(), sectionByTitle(), TestConfigPushV5(), TestFetchAuthFailureReturnsError(), TestFetchBothVersions(), TestFetchDegradesPerSection() (+9 more)

### Community 123 - "truenas_test.go"
Cohesion: 0.24
Nodes (17): Connector, newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchIsStableAcrossCalls() (+9 more)

### Community 124 - "Connector"
Cohesion: 0.10
Nodes (13): NewAuthError(), NewServiceUnavailableError(), TestTypedErrorsAreDistinguishableByType(), TestTypedErrorsWrapAndUnwrap(), Connector, apiMessage(), controllerName(), countByKind() (+5 more)

### Community 125 - "diagnostics/diagnostics.go"
Cohesion: 0.22
Nodes (17): CheckHealth(), Collect(), collectVersions(), newTestStore(), TestCheckHealthReportsDegradedOnClosedDB(), TestCollectIncludesHealthVersionsAndSchedule(), TestCollectListsRecentFailures(), TestCollectRedactsConnectorSecrets() (+9 more)

### Community 126 - "gitFixture"
Cohesion: 0.31
Nodes (11): SetBeforePushForTest(), newGitFixture(), TestGitExportLifecycle(), TestGitExportPushRejectionReturnsError(), TestGitExportRefusesForeignDirectory(), TestGitPerRevisionBootstrapReplayAndCap(), TestGitPerRevisionBotCatchUpAndRejectedPush(), TestGitPerRevisionMissingIntermediateAndEngineAuthor() (+3 more)

### Community 127 - "Decision"
Cohesion: 0.12
Nodes (14): 0002 — Start/stop lab-mutating operations, Audit, Authorization, Confirmation / step-up, Consequences, Context, Decision, Dry-run (+6 more)

### Community 128 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 129 - "compliance/handlers.go"
Cohesion: 0.20
Nodes (12): catalog(), changedFields(), NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), ComplianceRuleRecord (+4 more)

### Community 130 - "Handler"
Cohesion: 0.19
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 131 - "ExportToFile"
Cohesion: 0.09
Nodes (46): exists(), Handler, seedBackupRuns(), TestPruneBackupsAgeLimit(), Export(), ExportToFile(), newTestStore(), TestExportIncludesRecordsBeyondAPage() (+38 more)

### Community 132 - "SuggestWithFallback"
Cohesion: 0.20
Nodes (12): StatusError, StubProvider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail(), TestSuggestWithFallbackFirstProviderSucceeds() (+4 more)

### Community 133 - "traefik_test.go"
Cohesion: 0.25
Nodes (16): Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchSelectiveFields() (+8 more)

### Community 134 - "time.Duration"
Cohesion: 0.14
Nodes (5): healthFakeConnector, Database, Server, time.Duration, PoolConfig

### Community 135 - "RunbookRecord"
Cohesion: 0.29
Nodes (5): RunbookRecord, RunbookStepRecord, Store, scanRunbook(), scanRunbookStep()

### Community 136 - "sshStdioConn"
Cohesion: 0.10
Nodes (12): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), sshStdioConn, bufio.ReadWriter, golang.org/x/crypto/ssh.Client, golang.org/x/crypto/ssh.ClientConfig, golang.org/x/crypto/ssh.Session (+4 more)

### Community 137 - "docdiffmodel.ts"
Cohesion: 0.23
Nodes (13): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, fold(), toUnits(), DiffLine, DiffLineType (+5 more)

### Community 138 - "vectorCache"
Cohesion: 0.17
Nodes (11): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), TestVectorCachePutExistingKeyUpdatesInPlace(), vectorCache, vectorEntry, vectorKey (+3 more)

### Community 139 - "notifications/handlers_test.go"
Cohesion: 0.26
Nodes (16): TestCreate(), AuthedUser(), NewHandler(), decodePaginated(), jsonHasEmptyArrayItems(), newTestStore(), seedDelivery(), TestListDeliveriesEmpty() (+8 more)

### Community 140 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 141 - "MarshalConnectorConfig"
Cohesion: 0.20
Nodes (14): IsSecretFieldType(), MarshalConnectorConfig(), SecretFieldsChanged(), init(), TestConnectorRotationFieldsRoundTrip(), TestCreateConnectorDefaultsSecretRotatedAtToCreatedAt(), TestSecretFieldsChangedFalseOnRenameOnly(), TestSecretFieldsChangedFalseOnResubmittedUnchangedSecret() (+6 more)

### Community 142 - "httpx/retry_test.go"
Cohesion: 0.20
Nodes (20): retryable(), RetryTransport(), sleep(), do(), fail(), status(), TestRetryTransportDisabled(), TestRetryTransportDoesNotRetry() (+12 more)

### Community 143 - "ADDED Requirements"
Cohesion: 0.12
Nodes (15): ADDED Requirements, Purpose, Requirement: Agent-only changes select no application jobs, Requirement: Agent tooling is not in the Docker build context, Requirement: Application checks stay scoped to application sources, Requirement: Mixed and unknown changes keep application checks, Scenario: Agent files plus application code, Scenario: Context contents (+7 more)

### Community 144 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 145 - "handlers_contract_test.go"
Cohesion: 0.28
Nodes (14): AssertMatchesSpec(), loadSpec(), specPath(), createForSpec(), decodeEnvelope(), fieldMsgs(), Handler, TestConnectorSuccessPayloadsMatchSpec() (+6 more)

### Community 146 - "handlers_actions_test.go"
Cohesion: 0.26
Nodes (17): actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews(), TestActionMaintenanceLifecycle(), TestActionPermissions(), TestActionStoreFailures() (+9 more)

### Community 147 - "templates.fixtures.ts"
Cohesion: 0.16
Nodes (14): web_src_api_model_index_docversion, web_src_api_model_index_template, web_src_api_model_index_templateinput, fillBody(), generatePreview(), PreviewConnector, previewConnectors, renderTemplate() (+6 more)

### Community 148 - "connector/connector.go"
Cohesion: 0.06
Nodes (24): newConnector(), Connector, TimeoutError, GuardedDialer(), IsDangerousIP(), LifecycleOp(), NewTimeoutError(), supportedLifecycleVerbs() (+16 more)

### Community 149 - "data.go"
Cohesion: 0.09
Nodes (37): connectorFilter(), RenderHTML(), RenderMarkdown(), sampleData(), TestRenderHTML_EscapesDocTitles(), TestRenderHTML_SectionUnavailable(), TestRenderHTML_Truncated(), TestRenderMarkdown_Golden() (+29 more)

### Community 150 - "New"
Cohesion: 0.34
Nodes (14): TestJobHealthWithoutStoreDoesNothing(), New(), TestAddJobInvalidExpression(), TestAddJobRegistersAndFires(), TestContextGivenToJobFunction(), TestInvalidJobNameHandled(), TestJobContextDerivedFromStart(), TestJobSkipsOverlappingInvocations() (+6 more)

### Community 151 - "DocRecord"
Cohesion: 0.18
Nodes (6): docSearchWhere(), escapeLike(), DocRecord, Store, scanDoc(), scanDocSummary()

### Community 152 - "RateLimit"
Cohesion: 0.31
Nodes (6): TestRateLimit(), RateLimit(), golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 153 - "changes/handlers_test.go"
Cohesion: 0.32
Nodes (13): Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound(), TestDismissSuccess() (+5 more)

### Community 154 - "runbooks/handlers_test.go"
Cohesion: 0.32
Nodes (11): Handler, newTestHandler(), seedProxmoxConnector(), TestCreate(), TestCreateStepsValidation(), TestExecuteStepForbiddenWithoutOperatorGrant(), TestExecuteStepNotFound(), TestGetNotFound() (+3 more)

### Community 155 - "pagination_contract_test.go"
Cohesion: 0.21
Nodes (13): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_go_ast (+5 more)

### Community 156 - "newTestHandler"
Cohesion: 0.26
Nodes (12): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), Handler, newTestHandler(), TestCreate() (+4 more)

### Community 157 - "Get"
Cohesion: 0.17
Nodes (14): countLifecycle(), TestAllConnectorImplementationsRegister(), TestConnectorCapabilitiesMatchOptionalInterfaces(), TestConnectorFailureContract(), Capabilities(), CapabilityDescriptor, Get(), Connector (+6 more)

### Community 158 - "ServiceSnapshot"
Cohesion: 0.03
Nodes (31): noopValidatedConnector, ServiceSnapshot, SnapshotError(), calendarDaysBetween(), digestDue(), formatDigest(), Dispatcher, TestDigestDue() (+23 more)

### Community 159 - "Contributing to WiseLabz"
Cohesion: 0.14
Nodes (14): Branch naming, Commit hooks, Commit messages, Contributing to WiseLabz, Getting help, Prerequisites, Pull request process, Releasing (+6 more)

### Community 160 - "Backup Recovery: What Comes Back, and What Doesn't"
Cohesion: 0.14
Nodes (12): 1. Every export gets a manifest and a checksum, 2. Verifying a backup actually restores, 3. Restoring for real, 4. What a restore does *not* bring back, Backup Recovery: What Comes Back, and What Doesn't, Recovery runbook (suggested order), Backups, PostgreSQL support (+4 more)

### Community 161 - "Go Database Best Practices"
Cohesion: 0.15
Nodes (13): Avoid Hidden SQL Features, Best Practices Summary, Connection Pool, Context Propagation, Cross-References, Deep Dives, Go Database Best Practices, Library Choice (+5 more)

### Community 162 - "golang-troubleshooting/SKILL.md"
Cohesion: 0.19
Nodes (5): Code Review Red Flags, CPU Profiling, Lock Contention, Memory Profiling, Performance Troubleshooting

### Community 163 - "Delve Debugger"
Cohesion: 0.15
Nodes (13): Advanced Analysis, Basic Usage, Common Commands, Delve Debugger, Diagnostic Tools, GC Tracing, Go documentation command, GOTRACEBACK (+5 more)

### Community 164 - "General Debugging Methodology"
Cohesion: 0.15
Nodes (13): General Debugging Methodology, Step 10: Defense-in-Depth, Step 1: Understand Expected vs Actual, Step 2: Get the Full Error, Step 3: Isolate the Problem, Step 4: Check External Dependencies, Step 5: Check Observability Tools, Step 6: Compare with Working Code (+5 more)

### Community 165 - "Production Debugging"
Cohesion: 0.15
Nodes (12): HTTP Client Issues, Logging & Observability, Network & HTTP Debugging, Production Debugging, Production Debugging Checklist, Request ID Tracing, Step 1: Capture Immediately (don't restart!), Step 2: System Metrics (+4 more)

### Community 166 - "The Golden Rules"
Cohesion: 0.15
Nodes (13): 1. Read the Error Message First, 2. Reproduce Before You Fix, 3. If You Don't Measure It, You're Guessing, 4. One Hypothesis at a Time, 5. Find the Root Cause — No Workarounds, 6. Research the Codebase, Not Just the Diff, 7. Start Simple, Cross-References (+5 more)

### Community 167 - "Handler"
Cohesion: 0.18
Nodes (6): webAuthnFlow, webAuthnUser, Handler, github.com/go-webauthn/webauthn/webauthn.Credential, github.com/go-webauthn/webauthn/webauthn.SessionData, github.com/google/uuid.UUID

### Community 168 - "api/changes_test.go"
Cohesion: 0.26
Nodes (12): testApp, seedChange(), seedChangeWithSeverity(), TestChangesAcknowledgeRoleBoundary(), TestChangesAcknowledgeSuccess(), TestChangesBulkResolveEmptyIDs(), TestChangesBulkResolveInvalidStatus(), TestChangesBulkResolvePartialFailure() (+4 more)

### Community 169 - "doJSON"
Cohesion: 0.25
Nodes (12): TestElevateFailuresCountTowardLockout(), doJSON(), testHandler, req(), TestChangePassword(), TestChangePasswordRevokesAPIKeys(), TestCreateUser(), TestDeleteUser() (+4 more)

### Community 170 - "Handler"
Cohesion: 0.26
Nodes (6): stepAuditDetail(), validTargetType(), validVerb(), Handler, runbookResponse, stepResponse

### Community 171 - "newTestHarness"
Cohesion: 0.27
Nodes (14): TestListAttentionItems(), TestListChanges(), TestListConnectors(), TestSearchDocs(), seedFinding(), TestListFindings(), createConnector(), createUser() (+6 more)

### Community 172 - "Changelog"
Cohesion: 0.15
Nodes (12): [0.2.0](https://github.com/WiseLabz/WiseLabz/compare/v0.1.0...v0.2.0) (2026-09-12), 0.3.0 (2026-09-14), 1.0.0 (2026-10-01), ⚠ BREAKING CHANGES, Bug Fixes, Changelog, Changelog, Features (+4 more)

### Community 173 - "WiseLabz"
Cohesion: 0.15
Nodes (8): Code of Conduct, Configuration, Contributing, Features, License, Quick start, Supported services, WiseLabz

### Community 174 - "ARCHITECTURE.md"
Cohesion: 0.17
Nodes (8): 0004 — PostgreSQL leader election for background workers, Consequences, Context, Decision, Adding a channel type, Channel reference, Notification Channels, Webhook signing (HMAC-SHA256)

### Community 175 - "Decisions"
Cohesion: 0.15
Nodes (12): Checkout at v7.0.1 rather than v6.1.0, Context, Decisions, Design, `directories` with a glob for github-actions, not `directory: "/"`, `.github/dependabot.yml` is ignored (`-`), not `w`, Goals / Non-Goals, Grouping and commit conventions (+4 more)

### Community 176 - "scripts"
Cohesion: 0.15
Nodes (13): scripts, build, dev, format, gen:api, gen:api:watch, lint, prebuild (+5 more)

### Community 177 - ".agents/skills/openspec-explore/SKILL.md"
Cohesion: 0.17
Nodes (11): Check for context, Ending Discovery, Guardrails, Handling Different Entry Points, OpenSpec Awareness, Planning a Change, The Stance, What You Don't Have To Do (+3 more)

### Community 178 - "diagram.go"
Cohesion: 0.26
Nodes (12): entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks(), relatedEntities(), EntityLink (+4 more)

### Community 179 - "apikey_scope.go"
Cohesion: 0.18
Nodes (13): APIKeyRestriction, testAPIKeyChecker, APIKeyRestrictionFromContext(), ClampConnectorRole(), ContextWithAPIKeyRestriction(), isSafeMethod(), RejectRestrictedAPIKey(), TestClampConnectorRole() (+5 more)

### Community 180 - "IsSecureRequest"
Cohesion: 0.30
Nodes (10): ClientIP(), hostOnly(), IsSecureRequest(), isTrustedProxy(), TestClientIPRejectsNonIPForwardedFor(), TestClientIPTrustedPeerUsesForwardedFor(), TestClientIPUntrustedPeerIgnoresHeaders(), TestIsSecureRequestTLS() (+2 more)

### Community 181 - "scheduler/health_test.go"
Cohesion: 0.19
Nodes (10): newFakeHealthStore(), TestJobHealthOkToFailingNotifiesOnce(), TestJobHealthPanicCountsAsFailure(), TestJobHealthPersistsAcrossRestart(), JobHealthRecord, Store, scanJobHealth(), fakeHealthStore (+2 more)

### Community 182 - ".claude/skills/openspec-explore/SKILL.md"
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

### Community 187 - "main.tsx"
Cohesion: 0.20
Nodes (8): mermaid, react-dom, App(), cssVar(), Mermaid(), resolveColor(), USE_MOCKS, web_src_index

### Community 188 - "Engine"
Cohesion: 0.23
Nodes (7): Engine, dedupKey(), matchReason(), TemplateFuncs(), GenerateResult, renderResult, text/template.FuncMap

### Community 189 - "Engine"
Cohesion: 0.14
Nodes (6): Engine, sync.Map, sync.WaitGroup, AlertNotifier, DocRegenerator, QualityChecker

### Community 190 - "explore.md"
Cohesion: 0.18
Nodes (10): Check for context, Ending Discovery, Guardrails, OpenSpec Awareness, Planning a Change, The Stance, What You Don't Have To Do, What You Might Do (+2 more)

### Community 191 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 193 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 194 - "Cache"
Cohesion: 0.43
Nodes (5): Cache, New(), Cache[V], entry, V

### Community 195 - ".call"
Cohesion: 0.44
Nodes (6): Handler, newFixture(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestResolveAuthz(), fixture

### Community 197 - "migrations.go"
Cohesion: 0.18
Nodes (7): go_pkg_embed, go_pkg_github_com_golang_migrate_migrate_v4, go_pkg_github_com_golang_migrate_migrate_v4_database_postgres, go_pkg_github_com_golang_migrate_migrate_v4_database_sqlite3, go_pkg_github_com_golang_migrate_migrate_v4_source_file, go_pkg_github_com_golang_migrate_migrate_v4_source_iofs, MigrationStatus

### Community 198 - "transform_test.go"
Cohesion: 0.21
Nodes (10): init(), normalizeEnabledColumn(), normalizeFirewallRules(), RegisterTransformer(), runTransformers(), TestNormalizeFirewallRulesRewritesEnabledColumn(), TestRunTransformersAppliesInOrderAndStopsOnError(), TestRunTransformersUnknownCategoryIsNoop() (+2 more)

### Community 199 - "Tasks"
Cohesion: 0.20
Nodes (9): 1. Pin actions/checkout, 2. Upgrade Node 20 actions to Node 24, 3. Dependabot configuration, 4. Change classification, 5. Documentation, 6. Reduce Dependabot CI load, 7. Release job and Dependabot automation, 8. Integration checks (+1 more)

### Community 200 - "Tasks"
Cohesion: 0.20
Nodes (9): 1. Classifier script and fixtures, 2. Wire the classifier into `ci.yml`, 3. Composite Bun action, 4. CodeQL workflow, 5. Nightly vulnerability scan, 6. "CodeQL – Code Quality" check, 7. Documentation, 8. End-to-end verification (+1 more)

### Community 201 - "Product"
Cohesion: 0.20
Nodes (10): Accessibility & Inclusion, Anti-references, Brand Personality, Design Principles, Locked frontend direction (planning session, 2026-06; revised 2026-09), Product, Product decisions (pre-planning, v1), Product Purpose (+2 more)

### Community 202 - "changes.sh"
Cohesion: 0.40
Nodes (9): classify_path(), cmd_check(), cmd_classify(), cmd_go_closure(), describe_flags(), die(), go_package_dirs(), package_version_only() (+1 more)

### Community 203 - "test-shards.sh"
Cohesion: 0.40
Nodes (8): cmd_check(), cmd_matrix(), cmd_profile(), cmd_run(), cmd_timings(), die(), test-shards.sh script, shard_json()

### Community 204 - "mockServiceWorker.js"
Cohesion: 0.36
Nodes (8): activeClientIds, getResponse(), handleRequest(), IS_MOCKED_RESPONSE, resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 205 - "pprof Reference"
Cohesion: 0.22
Nodes (9): Analyzing and Interpreting Profiles, Capturing Profiles, Enable pprof HTTP Server, pprof Reference, Profile Types, Quick Setup (Development), Remote Profiling (Production), Secure Setup (Production) (+1 more)

### Community 206 - "Handler"
Cohesion: 0.27
Nodes (4): updateUserRequest, Handler, writeUserWriteError(), NoContent()

### Community 207 - "config_cmd_test.go"
Cohesion: 0.43
Nodes (7): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), io.Writer

### Community 208 - "TestElevateOIDC"
Cohesion: 0.27
Nodes (9): mockElevateOIDCServer, beginElevate(), containsCode(), defaultClaims(), elevateOIDCTestSetup(), testHandler, newMockElevateOIDCServer(), TestElevateOIDC() (+1 more)

### Community 209 - "handlers_bulk_test.go"
Cohesion: 0.47
Nodes (9): bulkReq(), bulkResults(), createBulkFakeConnector(), Handler, registerBulkFakeConnector(), TestBulkReauth(), TestBulkRestart(), TestBulkSync() (+1 more)

### Community 210 - "openapi_contract_test.go"
Cohesion: 0.33
Nodes (8): normalizeParams(), routerOperations(), specOperations(), TestAPIV1AliasServesSameHandlers(), TestOpenAPIHealthProbeRoutes(), TestOpenAPIMatchesRouter(), chi.Routes, go_pkg_go_yaml_in_yaml_v3

### Community 211 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 212 - "Elector"
Cohesion: 0.24
Nodes (6): New(), postgresDB(), TestSecondElectorWaitsThenTakesOver(), TestWatchReportsTerminatedSession(), database/sql.Conn, Elector

### Community 213 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 214 - "Database Performance"
Cohesion: 0.25
Nodes (7): Configuration, Connection Pool Sizing, Database Performance, Monitoring, Prometheus Metrics, Query Performance Tips, Table of Contents

### Community 215 - "Transactions, Isolation Levels, and Locking"
Cohesion: 0.25
Nodes (5): Basic transaction pattern, Custom isolation level, Locking variants, SELECT FOR UPDATE — prevent race conditions, Transactions, Isolation Levels, and Locking

### Community 216 - "Testing Database Code"
Cohesion: 0.25
Nodes (8): Integration Tests, Mock for service-layer tests, sqlmock for Query-Level Testing, Table of Contents, Test database with testcontainers-go, Testing Database Code, Unit Tests with Mocks, What to Test

### Community 217 - "dashboard/handlers_test.go"
Cohesion: 0.43
Nodes (7): Handler, newTestHandler(), TestGetAdminDefault(), TestGetLayoutFallsBackToAdminDefault(), TestOverview(), TestPutAdminDefault(), TestSaveAndResetLayout()

### Community 218 - "retention/retention_test.go"
Cohesion: 0.61
Nodes (8): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupPrunesOldHealthChecks(), TestRunCleanupSkipsDisabledCategories()

### Community 219 - "BackupRun"
Cohesion: 0.32
Nodes (3): BackupRun, Store, scanBackupRun()

### Community 220 - "auth/handlers_test.go"
Cohesion: 0.25
Nodes (7): TestEmailDomainAllowed(), TestOIDCConnectorRolesForGroups(), TestOIDCRoleForGroups(), connectorRoleLess(), emailDomainAllowed(), oidcConnectorRolesForGroups(), oidcRoleForGroups()

### Community 222 - "Audit Trail"
Cohesion: 0.25
Nodes (7): Audit Trail, Endpoint, Filtering and export, Keyset (cursor) pagination, Retention, What's not recorded, What's recorded

### Community 223 - "Step by step"
Cohesion: 0.25
Nodes (8): 1. Create the package, 2. Define your config schema, 3. Implement the interface, 4. Register the connector, 5. Add the barrel import, 6. Write tests, 7. Document config fields, Step by step

### Community 224 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 225 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 226 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 227 - "Proposal"
Cohesion: 0.25
Nodes (7): Capabilities, Impact, Modified Capabilities, New Capabilities, Proposal, What Changes, Why

### Community 228 - "chat/chat.go"
Cohesion: 0.29
Nodes (7): buildPrompt(), TestBuildPrompt(), Match, SplitSections(), TestSplitSections(), Section, go_pkg_math

### Community 229 - "runbook_test.go"
Cohesion: 0.32
Nodes (7): Store, newCascadeTestStore(), TestGetRunbookByTarget(), TestRunbookRoundTrip(), TestRunbookStepsCascadeOnConnectorDelete(), TestRunbookStepsCascadeOnRunbookDelete(), TestRunbookStepsRoundTrip()

### Community 230 - "WithAuth"
Cohesion: 0.48
Nodes (7): TestEmbeddedSPAWithoutFrontendBuild(), TestList(), TestRevoke(), JWTService(), Token(), WithAuth(), TestDelete()

### Community 231 - "newHandler"
Cohesion: 0.31
Nodes (8): Handler, newHandler(), serve(), TestConversationOwnership(), TestCreateConversationDocVisibility(), TestCreateConversationValidation(), TestPostMessageErrors(), net/http.HandlerFunc

### Community 232 - "TestComplianceRuleValidation"
Cohesion: 0.29
Nodes (7): badRegexMessage(), complianceCondition(), TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule(), TestValidationErrorDetails()

### Community 233 - "snapshotResponse"
Cohesion: 0.48
Nodes (7): Handler, snapshotFixture(), snapshotRequest(), snapshotResponse(), TestSnapshotDiffValidationOwnershipAndAudit(), TestSnapshotOwnershipAndFullShape(), TestSnapshotsViewerAndCursor()

### Community 234 - "scanMaintenanceWindow"
Cohesion: 0.48
Nodes (3): Store, scanMaintenanceWindow(), MaintenanceWindowRecord

### Community 235 - "computeNextRun"
Cohesion: 0.43
Nodes (5): TestComputeNextRun_BackoffNeverExceedsScheduleCadence(), TestComputeNextRun_FailureUsesBackoffSchedule(), TestComputeNextRun_ManualOnlyNeverSchedules(), TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(), computeNextRun()

### Community 236 - "session_test.go"
Cohesion: 0.38
Nodes (6): refreshCookie(), TestDeleteSession(), TestElevate(), TestElevateMethods(), TestLogout(), TestRefresh()

### Community 237 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 238 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 239 - ".UpdateAuthConfig"
Cohesion: 0.43
Nodes (3): Handler, oidcProviderJSON(), boolToInt()

### Community 240 - "sync.Mutex"
Cohesion: 0.29
Nodes (3): sync.Mutex, fakeDocRegenerator, fakeQualityChecker

### Community 241 - "areas.sh script"
Cohesion: 0.48
Nodes (5): areas(), is_true(), areas.sh script, expect(), areas-test.sh script

### Community 242 - "Struct Scanning and NULLable Columns"
Cohesion: 0.33
Nodes (5): JSON Marshaling, NULLable Columns, Struct Scanning and NULLable Columns, Struct Scanning with pgx, Struct Scanning with sqlx

### Community 243 - "Test-Driven Debugging"
Cohesion: 0.33
Nodes (5): Debugging Flaky Tests, Expand Edge Cases with Table Tests, Reproduce the Bug in a Test, Test-Driven Debugging, Useful Test Flags

### Community 244 - "walkCursorPages"
Cohesion: 0.40
Nodes (6): cursorPage, decodeCursorPage(), testApp, TestAuditCursorPaginationTraversal(), TestChangesCursorPaginationTraversal(), walkCursorPages()

### Community 245 - "browser.ts"
Cohesion: 0.40
Nodes (4): bootstrap(), worker, enableMocks(), handlers

### Community 246 - "withGrant"
Cohesion: 0.33
Nodes (6): TestByServiceNoDocsYet(), TestGenerate(), TestGetUnknownIDFallsBackToServicePlaceholder(), TestRestore(), TestTree(), withGrant()

### Community 247 - "golden_snapshot_test.go"
Cohesion: 0.60
Nodes (4): Store, mustCreateGoldenSnapshotConnector(), TestGetSnapshotByID(), TestPinGoldenSnapshotRoundTrip()

### Community 248 - "TestLocalLoginDisabledIsEnforced"
Cohesion: 0.50
Nodes (3): testHandler, TestLocalLoginDisabledIsEnforced(), TestTokenTTLsAndStepUpComeFromSettings()

### Community 252 - "Design"
Cohesion: 0.33
Nodes (5): Context, Decisions, Design, Goals / Non-Goals, Risks / Trade-offs

### Community 253 - "Tasks"
Cohesion: 0.33
Nodes (5): 1. Classifier tweak, 2. Area helper, 3. Lefthook wiring, 4. Docs, Tasks

### Community 254 - "Design"
Cohesion: 0.33
Nodes (5): Context, Decisions, Design, Goals / Non-Goals, Risks / Trade-offs

### Community 255 - "Tasks"
Cohesion: 0.33
Nodes (5): 1. Classifier regression coverage, 2. Docker build context, 3. Scan and lint scope audit, 4. Documentation and event evidence, Tasks

### Community 256 - "Security Policy"
Cohesion: 0.33
Nodes (5): Reporting a vulnerability, Security Policy, Supported versions, What counts as a security vulnerability, What we commit to

### Community 257 - "Batch Processing"
Cohesion: 0.40
Nodes (5): Batch INSERT with sqlx, Batch Processing, Bulk INSERT with pgx (PostgreSQL COPY protocol), Cursor-based pagination (avoid OFFSET), Sweet spot: 100–1,000 rows per batch

### Community 258 - "Compilation Issues"
Cohesion: 0.40
Nodes (4): CGO Issues, Compilation Issues, Module Problems, Version Mismatch

### Community 259 - "Concurrency Debugging"
Cohesion: 0.40
Nodes (5): Concurrency Debugging, Deadlocks, Goroutine Leaks, Race Conditions, Table of Contents

### Community 260 - "Web Interface Guidelines"
Cohesion: 0.40
Nodes (4): Guidelines Source, How It Works, Usage, Web Interface Guidelines

### Community 264 - "Authentication design"
Cohesion: 0.40
Nodes (5): Authentication design, Destructive-action pattern: confirm + blast radius (decided 2026-06-27), OIDC group→connector roles and IdP step-up (#279 part 3), OIDC provider configuration (decided 2026-06-25: file-defined, app toggles only), Permissions & step-up for mutating actions (decided 2026-06-27)

### Community 265 - "Development workflow"
Cohesion: 0.40
Nodes (5): Branching, Code quality, Commit conventions, Commit hooks (`lefthook`), Development workflow

### Community 266 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 267 - "Indexing Strategy"
Cohesion: 0.50
Nodes (4): Indexing Strategy, Use SQL MCP to check existing indexes, When to suggest adding indexes, When to suggest removing indexes

### Community 271 - "Saved Views"
Cohesion: 0.50
Nodes (3): Endpoints, Saved Views, Scope

### Community 272 - "Error Handling"
Cohesion: 0.67
Nodes (3): Always close rows, Common database error patterns, Error Handling

### Community 273 - "Parameterized Queries"
Cohesion: 0.67
Nodes (3): Dynamic column names, Dynamic IN clauses, Parameterized Queries

### Community 274 - "JSON Pitfalls"
Cohesion: 0.67
Nodes (3): JSON Pitfalls, Numbers into `interface{}` become `float64`, Unexported fields silently ignored

## Knowledge Gaps
- **1014 isolated node(s):** `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult`, `testHandler`, `bulkResolveRequest` (+1009 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1774 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **26 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Store` connect `Store` to `compliance/handlers.go`, `ExportToFile`, `go_pkg_context`, `Errorf`, `notifications/handlers_test.go`, `data.go`, `DecodeKey`, `RunMigrations`, `net/http.ResponseWriter`, `NewEngine`, `NewStore`, `ServiceSnapshot`, `net/http.Request`, `dispatcher_test.go`, `ConnectorRecord`, `NewUser`, `newTestHarness`, `NewEngine`, `Handler`, `response.go`, `NewChecker`, `Engine`, `Engine`, `Store`, `Dispatcher`, `.call`, `Manager`, `main`, `Handler`, `store/backup_test.go`, `Service`, `migrations.go`, `.call`, `Handler`, `rewritePlaceholders`, `NewRegistry`, `testApp`, `New`, `retention/retention_test.go`, `Handler`, `Checker`, `WithAuth`, `newHandler`, `export_test.go`, `backup/backup.go`, `server.go`, `sync.Mutex`, `withGrant`, `diagnostics/diagnostics.go`, `gitFixture`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Why does `UserIDFromContext()` connect `Errorf` to `net/http.Request`, `ConnectorRecord`, `Handler`, `Sanitize`, `context.Context`, `Handler`, `Handler`, `.OIDCCallback`, `Handler`, `response.go`, `server.go`, `withGrant`, `NewService`, `net/http.ResponseWriter`, `ErrorWithDetails`, `routerDeps`, `Handler`, `middleware.go`?**
  _High betweenness centrality (0.009) - this node is a cross-community bridge._
- **Why does `Engine` connect `Engine` to `Hub`, `context.Context`, `go_pkg_context`, `time.Duration`, `Store`, `NewEngine`, `rewritePlaceholders`, `sync.Mutex`, `net/http.ResponseWriter`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _1014 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestApp` be split into smaller, more focused modules?**
  _Cohesion score 0.022158056619170172 - nodes in this community are weakly interconnected._
- **Should `cn` be split into smaller, more focused modules?**
  _Cohesion score 0.028129032258064516 - nodes in this community are weakly interconnected._
- **Should `newDocTestStore` be split into smaller, more focused modules?**
  _Cohesion score 0.022027550307877865 - nodes in this community are weakly interconnected._