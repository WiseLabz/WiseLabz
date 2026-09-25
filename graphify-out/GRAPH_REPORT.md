# Graph Report - agent-a0c2461ceb996eb06  (2026-09-25)

## Corpus Check
- 826 files · ~499,373 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 19 file(s) not represented in the graph (top: (none) 10, .toml 2, .tmpl 2)

## Summary
- 6253 nodes · 19428 edges · 246 communities (208 shown, 38 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 1709 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `71194426`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- net/http.Client
- context.Context
- net/http.Request
- testApp
- SuggestRequest
- Store
- time.Duration
- ServiceSnapshot
- Handler
- AuthMiddleware
- diagnostics/diagnostics.go
- NewEngine
- newTestApp
- Checker
- sshStdioConn
- testing.T
- DecodeKey
- Dispatcher
- WiseLabz WebSocket Contract (`/ws`)
- Registry
- newDocTestStore
- CommandPalette
- initial database schema
- rewritePlaceholders
- dispatcher_test.go
- IsSecureRequest
- Compare
- Service
- cn
- react
- Product
- App.tsx
- go_pkg_github_com_wiselabz_wiselabz_internal_store
- RunSync
- backup/main.go
- Engine
- main.tsx
- go_pkg_testing
- DashboardPage.tsx
- Bottom-dock shell
- icons.tsx
- @tanstack/react-query
- WiseLabz
- docker_test.go
- ProfilePage.tsx
- adguardhome/tables.go
- Runner
- ServiceDetailPage.tsx
- git_test.go
- UsersPage.tsx
- newTestHandler
- package.json
- portainer/tables.go
- ErrorWithDetails
- rowScanner
- SnapshotEntity
- main
- time.Time
- Manager
- lefthook Commit Hooks
- TemplateEditorPage.tsx
- fixtures.ts
- ExportToFile
- NewMalformedResponseError
- home_assistant/tables.go
- RunMigrations
- share_links_test.go
- Store
- AppShell — Bottom Dock Shell (single variant)
- Get
- dependencies
- NewChecker
- ThemeControls.tsx
- channels.go
- traefik/tables.go
- compliance/engine.go
- Configuration & Documentation Backup (Export/Import)
- ReportsPage.tsx
- Connector
- DocRecord
- settings.mock.ts
- Diff viewer
- Destructive connector confirmation
- single-instance deployment model
- Graphify Knowledge Graph Rules
- Topbar Notification Center (deferred from V1)
- Database: SQLite + PostgreSQL
- React + Vite Frontend
- api/audit_test.go
- api/auth/oidc.go
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
- logging.go
- backup/backup.go
- Connector
- portainer_test.go
- Connector
- NotificationRecord
- useRole.ts
- NewService
- NewHTTPClient
- adguardhome_test.go
- Sanitize
- Deps
- middleware.go
- go_pkg_context
- Connector
- Config
- GetTypeSchema
- Store
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
- InstanceAdminFromContext
- Handler
- truenas_test.go
- keyset_test.go
- DiffViewer.tsx
- ws.ts
- compilerOptions
- handlers_actions_test.go
- Connector
- traefik_test.go
- MarshalConnectorConfig
- docdiffmodel.ts
- system/handlers_test.go
- all.go
- doc/engine.go
- httpx/retry_test.go
- Store
- compilerOptions
- AuthedUser
- handlers_contract_test.go
- vectorCache
- fetch_test.go
- Store
- net/http.Handler
- newHandler
- New
- pagination_contract_test.go
- Handler
- backup/backup_test.go
- render_test.go
- NewRegistry
- newTestHandler
- api/changes_test.go
- newTestHandler
- ValidateConfig
- createUser
- log/slog.Logger
- Contributor Covenant Code of Conduct
- apikey_scope.go
- Store
- store/backup_test.go
- Decision
- Decision
- scripts
- ServiceDependency
- newTCPDockerClient
- Store
- Decision
- .call
- connectors_maintenance_test.go
- NewHandler
- api/docs_test.go
- .call
- Changelog
- mockServiceWorker.js
- apikey_scopes_test.go
- ComplianceRuleRecord
- decodeBulkRequest
- openapi_contract_test.go
- net/http.Response
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
- compliance_rules_test.go
- TestBulkReauth
- runbooks/handlers_test.go
- scanMaintenanceWindow
- computeNextRun
- Audit Trail
- Bulk Review Actions
- PULL_REQUEST_TEMPLATE.md
- walkCursorPages
- .doRequestV5
- engine_maintenance_test.go
- Mermaid.tsx
- Security Policy
- APIKeyClaims
- schema.go
- seedScopeFixture
- Notification Channels
- compose-smoke.sh
- ClassifyHealth
- timeoutError
- RetentionSettings
- transform_firewall.go
- Saved Views
- internal/auth/oidc.go
- truenas/attributes_test.go
- WiseLabz — v2 Backlog
- fakeEmbedder
- fakeDocRegenerator
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
10. `Dashboard widgets` - 62 edges

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

## Hyperedges (group relationships)
- **Step-Up Confirmation Flow for Destructive Actions** — docs_architecture_permissions_stepup, docs_architecture_destructive_confirm_pattern, docs_openapi_auth_elevate_endpoint, docs_openapi_removal_impact_endpoint [EXTRACTED 0.90]
- **Contract-First API Codegen Pipeline** — docs_architecture_orval, docs_openapi_spec_document, docs_architecture_react_query, docs_architecture_diff_contract [EXTRACTED 0.85]
- **Dual Local/OIDC Auth Mode System** — docs_architecture_auth_design, docs_architecture_oidc_provider_config, docs_openapi_oidc_provider_schema, docs_openapi_auth_config_endpoint [EXTRACTED 0.90]

## Communities (246 total, 38 thin omitted)

### Community 0 - "net/http.Client"
Cohesion: 0.04
Nodes (25): Connector, ollamaEmbedder, openAIEmbedder, NewServiceUnavailableError(), setHeaders(), TestValidateCustomURL(), tryParseEntities(), validateCustomURL() (+17 more)

### Community 1 - "context.Context"
Cohesion: 0.03
Nodes (32): fakeStatusChecker, sanitizeSessions(), versionSections(), Connector, changeServiceIDs(), Store, existingIDs(), placeholders() (+24 more)

### Community 2 - "net/http.Request"
Cohesion: 0.06
Nodes (34): Handler, Handler, updateUserRequest, newToken(), sanitize(), Handler, writeUserWriteError(), Handler (+26 more)

### Community 3 - "testApp"
Cohesion: 0.23
Nodes (9): testApp, TestBackupCreateManualRun(), TestBackupCreateManualRunFailsWhenDirNotCreatable(), TestBackupListRunsEmpty(), TestBackupRoutesRequireOperatorRole(), TestBackupScheduleGetDefaults(), TestBackupScheduleUpdate(), TestBackupScheduleUpdateDoesNotLeakSchedulerJobs() (+1 more)

### Community 4 - "SuggestRequest"
Cohesion: 0.13
Nodes (11): claudeProvider, openAICompatibleProvider, testProvider, SuggestChunk, SuggestRequest, TestRegistryGet(), TestRegistryList(), TestStubProviderName() (+3 more)

### Community 5 - "Store"
Cohesion: 0.04
Nodes (62): routerDeps, Handler, NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler(), NewHandler() (+54 more)

### Community 6 - "time.Duration"
Cohesion: 0.16
Nodes (6): AuthSettings, Database, Server, time.Duration, OIDCProvider, PoolConfig

### Community 7 - "ServiceSnapshot"
Cohesion: 0.03
Nodes (21): healthFakeConnector, noopValidatedConnector, ServiceSnapshot, Connector, agentEnabled(), Connector, init(), RegisterTransformer() (+13 more)

### Community 8 - "Handler"
Cohesion: 0.24
Nodes (7): NewHandler(), response(), toRule(), validRecord(), writeRuleRejection(), Handler, RuleEvaluator

### Community 9 - "AuthMiddleware"
Cohesion: 0.13
Nodes (20): APIKeyChecker, fakeConnectorRoleChecker, testAuditCall, testAuditRecorder, AuthMiddleware(), RequireInstanceAdmin(), assertElevationAuditCalls(), boolLabel() (+12 more)

### Community 10 - "diagnostics/diagnostics.go"
Cohesion: 0.22
Nodes (17): CheckHealth(), Collect(), collectVersions(), newTestStore(), TestCheckHealthReportsDegradedOnClosedDB(), TestCollectIncludesHealthVersionsAndSchedule(), TestCollectListsRecentFailures(), TestCollectRedactsConnectorSecrets() (+9 more)

### Community 11 - "NewEngine"
Cohesion: 0.09
Nodes (43): entityNodeID(), renderLabMermaid(), renderMermaid(), shortHash(), TestRenderMermaid(), TestRenderMermaidNoLinks(), Engine, NewEngine() (+35 more)

### Community 12 - "newTestApp"
Cohesion: 0.02
Nodes (157): templateBody, TestAPIKeyCreateRejectsInvalidExpiryAndEmptyName(), TestAPIKeyRoutesEndToEnd(), TestAttentionAuthenticatedAccess(), TestAttentionDaysWindow(), TestAttentionEmptyList(), TestAttentionHidesUngrantedConnectors(), TestAttentionMergesAlertsAndFindings() (+149 more)

### Community 13 - "Checker"
Cohesion: 0.13
Nodes (8): complianceRule(), Checker, RunStaleSweepOnce(), QualityFindingRecord, Store, scanQualityFinding(), FindingNotifier, RotationConfig

### Community 14 - "sshStdioConn"
Cohesion: 0.12
Nodes (10): TestDialSSHStdioHonorsContextCancel(), closeQuietly(), dialSSHStdio(), sshStdioConn, golang.org/x/crypto/ssh.Client, golang.org/x/crypto/ssh.ClientConfig, golang.org/x/crypto/ssh.Session, io.Closer (+2 more)

### Community 15 - "testing.T"
Cohesion: 0.02
Nodes (154): runConfigCommand(), setValidEnv(), TestConfigPrintRedacted(), TestConfigSchema(), TestConfigUnknown(), TestConfigValidate(), runHealthcheck(), TestClaudeSuggest() (+146 more)

### Community 16 - "DecodeKey"
Cohesion: 0.09
Nodes (25): ProviderConfig, testHandler, Handler, Handler, Handler, primaryProviderConfig(), Handler, DecodeKey() (+17 more)

### Community 17 - "Dispatcher"
Cohesion: 0.14
Nodes (14): discordPayload(), sendDiscordChannel(), sendGenericWebhookChannel(), sendSlackChannel(), slackPayload(), webhookPayload(), findChannel(), findRoute() (+6 more)

### Community 18 - "WiseLabz WebSocket Contract (`/ws`)"
Cohesion: 0.06
Nodes (33): ADR 0001 — Monorepo, ADR Index (docs/adr/), AI Doc Generation Module (opt-in, provider-agnostic), API Design — REST + WebSocket split, Dual Auth Design (Local JWT + OIDC), Changes/Diff Contract (infra vs doc format), Change-Aware Diff Engine, Monorepo with Go Workspaces (+25 more)

### Community 19 - "Registry"
Cohesion: 0.13
Nodes (15): Provider, StatusError, StubProvider, SuggestResult, registerFailThenSucceed(), TestIsRetryable(), TestSuggestWithFallbackAdvancesOnRetryableError(), TestSuggestWithFallbackAllFail() (+7 more)

### Community 20 - "newDocTestStore"
Cohesion: 0.03
Nodes (133): TestAPIKeyLifecycle(), TestAPIKeyNotFound(), TestLookupAPIKeyReflectsLiveRole(), TestLookupAPIKeyRejectsDisabledUser(), TestRevokeAllAPIKeysForUser(), TestTouchAPIKeyLastUsed(), TestCreateAuditRecordAndListFiltering(), TestListAllAuditRecords() (+125 more)

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
Cohesion: 0.06
Nodes (85): TestExpireAlertsOnceNoExpiredAlertsIsNoop(), TestExpireAlertsOnceNotifiesViaDispatcher(), newLifecycleManager(), newTestLifecycle(), TestLifecycleManagerOrderedShutdown(), TestLifecycleManagerShutdownCancelsWorkContext(), testLogger(), expireAlertsOnce() (+77 more)

### Community 25 - "IsSecureRequest"
Cohesion: 0.09
Nodes (23): clearOIDCFlowCookie(), Handler, newOIDCUser(), oidcFlowCookieName(), randomOIDCToken(), readOIDCFlowCookie(), setOIDCFlowCookie(), validHostPort() (+15 more)

### Community 26 - "Compare"
Cohesion: 0.15
Nodes (18): configPushLanded(), driftDescription(), Checker, highestDriftSeverity(), TestCompareIgnoresEntityAttributes(), TestCompareMapKeyOrderingDoesNotAffectResult(), TestCompareStillDetectsRuleContentChanges(), Compare() (+10 more)

### Community 27 - "Service"
Cohesion: 0.16
Nodes (13): Claims, ElevationClaims, ElevationToken, IssuePairOptions, MFAClaims, MFATicket, Service, TokenPair (+5 more)

### Community 28 - "cn"
Cohesion: 0.03
Nodes (102): web_src_api_generated_auth_auth_usegetauthproviders, web_src_api_generated_compliance_compliance, web_src_api_generated_compliance_compliance_deletecompliancerulesid, web_src_api_generated_compliance_compliance_getgetcompliancerulesquerykey, web_src_api_generated_compliance_compliance_postcompliancerules, web_src_api_generated_compliance_compliance_postcompliancerulestest, web_src_api_generated_compliance_compliance_putcompliancerulesid, web_src_api_generated_compliance_compliance_usegetcompliancerules (+94 more)

### Community 29 - "react"
Cohesion: 0.05
Nodes (96): Frontend, react, react-i18next, web_src_api_generated_alerts_alerts, web_src_api_generated_alerts_alerts_postalertsalertiddismiss, web_src_api_generated_alerts_alerts_postalertsalertidresolve, web_src_api_generated_alerts_alerts_postalertsalertidsnooze, web_src_api_generated_alerts_alerts_postalertsbulksnooze (+88 more)

### Community 30 - "Product"
Cohesion: 0.09
Nodes (24): Connector Guide (docs/connectors/CONNECTOR_GUIDE.md), Connector Interface (Name/Fetch/Validate), Connector Management via UI (full CRUD), Destructive-Action Pattern: Confirm + Blast Radius, Manager Actions (v1 scope), Permissions & Step-Up for Mutating Actions, Role Model — viewer/operator, ServiceSnapshot Data Structure (+16 more)

### Community 31 - "App.tsx"
Cohesion: 0.03
Nodes (82): react-error-boundary, sonner, AXIOS_INSTANCE, BodyType, ErrorType, getAccessToken(), MfaEnrollmentRequiredFn, RefreshFn (+74 more)

### Community 32 - "go_pkg_github_com_wiselabz_wiselabz_internal_store"
Cohesion: 0.04
Nodes (59): bulkSnoozeItemResult, bulkSnoozeRequest, changePromptData(), stripPromptTags(), truncateUTF8(), bulkResolveItemResult, bulkResolveRequest, go_pkg_github_com_mark3labs_mcp_go_client (+51 more)

### Community 33 - "RunSync"
Cohesion: 0.15
Nodes (14): Connector interface, connector schema registration, reverse proxy WebSocket support, OpenAPI REST contract, destructive-action step-up authentication, operational alerts, detected changes, Compare (+6 more)

### Community 34 - "backup/main.go"
Cohesion: 0.12
Nodes (16): buildPrompt(), TestBuildPrompt(), cosineSimilarity(), Match, packVector(), SplitSections(), TestCosineSimilarityRanksClosestVectorHighest(), TestPackUnpackVectorRoundTrips() (+8 more)

### Community 35 - "Engine"
Cohesion: 0.20
Nodes (5): Engine, sync.Map, AlertNotifier, DocRegenerator, QualityChecker

### Community 36 - "main.tsx"
Cohesion: 0.14
Nodes (14): OpenAPI client generation, Generated-code lint exclusions, Motion preference provider, react-dom, Vite API and WebSocket proxy, App(), Authentication and onboarding guards, Operator-only routes (+6 more)

### Community 37 - "go_pkg_testing"
Cohesion: 0.06
Nodes (20): dashboardLayout, TestBuildHostOverrideTableAttributes(), buildHostOverrideTable(), isIPv6(), TestBuildHostOverrideTableMalformedCases(), TestBuildHostOverrideTableValidOverrides(), go_pkg_bytes, go_pkg_encoding_json (+12 more)

### Community 38 - "DashboardPage.tsx"
Cohesion: 0.03
Nodes (92): Connector category icon map, Dashboard widget frame, 10. `doc.lock.acquired`, 11. `doc.lock.released`, 12. `doc.lock.expired`, 13. `system.health`, 14. `system.notice`, 2. `sync.progress` (+84 more)

### Community 39 - "Bottom-dock shell"
Cohesion: 0.40
Nodes (5): Authenticated app frame, Bottom-dock shell, Primary navigation, Non-React navigation bridge, Floating dock navigation

### Community 40 - "icons.tsx"
Cohesion: 0.04
Nodes (79): Live dashboard state, match-sorter, motion, @radix-ui/react-popover, web_src_api_generated_connectors_connectors, web_src_api_generated_connectors_connectors_deleteconnectorsconnectorid, web_src_api_generated_connectors_connectors_deleteconnectorsconnectoridmaintenancewindow, web_src_api_generated_connectors_connectors_getgetconnectorsmaintenancewindowsquerykey (+71 more)

### Community 41 - "@tanstack/react-query"
Cohesion: 0.04
Nodes (54): i18next, msw, react-router-dom, @tanstack/react-query, @testing-library/react, vitest, web_src_api_model_index, web_src_api_model_index_attentionpage (+46 more)

### Community 42 - "WiseLabz"
Cohesion: 0.20
Nodes (10): WCAG 2.2 AA accessibility, Docs-first information architecture, technical homelabbers, machine-honest interface, v1 narrow manager scope, trustworthy live documentation, WiseLabz, commit quality gates (+2 more)

### Community 43 - "docker_test.go"
Cohesion: 0.04
Nodes (61): newDockerClient(), init(), generateSSHHostKey(), serveOneHTTPExchange(), serveSSHDockerConn(), startSSHDockerServer(), TestConfigPush(), TestDockerWritableFields() (+53 more)

### Community 44 - "ProfilePage.tsx"
Cohesion: 0.03
Nodes (55): web_src_api_generated_auth_auth, web_src_api_generated_auth_auth_deleteauthapikeysid, web_src_api_generated_auth_auth_getgetauthapikeysquerykey, web_src_api_generated_auth_auth_postauthapikeys, web_src_api_generated_auth_auth_postauthelevate, web_src_api_generated_auth_auth_usegetauthapikeys, web_src_api_generated_auth_auth_usegetauthelevatemethods, web_src_api_generated_me_me_deletememfafactorsfactorid (+47 more)

### Community 45 - "adguardhome/tables.go"
Cohesion: 0.07
Nodes (61): statusInfo, unavailable(), upstreamDependencies(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildClientTable(), buildDHCP(), buildDNSInfo() (+53 more)

### Community 46 - "Runner"
Cohesion: 0.08
Nodes (30): newFakeHealthStore(), TestJobHealthOkToFailingNotifiesOnce(), TestJobHealthPanicCountsAsFailure(), TestJobHealthPersistsAcrossRestart(), TestJobHealthWithoutStoreDoesNothing(), cron.EntryID, Runner, New() (+22 more)

### Community 47 - "ServiceDetailPage.tsx"
Cohesion: 0.04
Nodes (54): ADR-0001, ADR-0003, RFC-3339, 1. `service.status`, web_src_api_generated_changes_changes_usegetchanges, web_src_api_generated_connectors_connectors_getgetconnectorsquerykey, web_src_api_generated_connectors_connectors_postconnectors, web_src_api_generated_connectors_connectors_postconnectorsconnectoridconfigpush (+46 more)

### Community 48 - "git_test.go"
Cohesion: 0.06
Nodes (44): fetchAllDocs(), fileName(), Exporter, IsGeneratedName(), NewExporter(), pruneStale(), RunExportOnce(), slugify() (+36 more)

### Community 49 - "UsersPage.tsx"
Cohesion: 0.06
Nodes (48): axios, customInstance(), web_src_api_generated_users_users, web_src_api_generated_users_users_deleteusersuserid, web_src_api_generated_users_users_getgetusersquerykey, web_src_api_generated_users_users_postusersuseridresetmfa, web_src_api_generated_users_users_postusersuseridresetpassword, web_src_api_generated_users_users_usegetusers (+40 more)

### Community 50 - "newTestHandler"
Cohesion: 0.08
Nodes (45): doJSON(), testHandler, req(), TestChangePassword(), TestChangePasswordRevokesAPIKeys(), TestCreateUser(), TestDeleteUser(), TestLogin() (+37 more)

### Community 51 - "package.json"
Cohesion: 0.04
Nodes (48): clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view, eslint, eslint-plugin-react-hooks (+40 more)

### Community 52 - "portainer/tables.go"
Cohesion: 0.08
Nodes (40): WantsField(), TestRequestedFields(), TestWantsField(), jsonType(), TestAttributeCatalogCoversEmittedKeys(), environmentDependencies(), putMetadata(), buildEnvironmentTable() (+32 more)

### Community 53 - "ErrorWithDetails"
Cohesion: 0.07
Nodes (27): sanitizeUser(), setRefreshCookie(), Handler, mustHashDummyPassword(), instanceAdminRoleFor(), configRequestField(), parseScheduleUpdates(), validateRotationFields() (+19 more)

### Community 54 - "rowScanner"
Cohesion: 0.07
Nodes (24): actorRoleLabel(), auditFilterClause(), Store, scanAuditRecord(), scanAuditRecordRows(), Store, scanBackupRun(), changeFilterClause() (+16 more)

### Community 55 - "SnapshotEntity"
Cohesion: 0.11
Nodes (46): SnapshotEntity, TestBuildContainerTableAttributes(), buildContainerTable(), TestBuildInterfaceTableAttributes(), buildInterfaceTable(), buildDatasets(), buildDisks(), buildInterfaces() (+38 more)

### Community 56 - "main"
Cohesion: 0.09
Nodes (26): main(), splitOrigins(), RegisterClaude(), TestRegisterClaudeDefaults(), RegisterOllamaEmbedder(), RegisterOpenAIEmbedder(), Embedder, EmbedRegistry (+18 more)

### Community 57 - "time.Time"
Cohesion: 0.07
Nodes (29): digestDue(), formatDigest(), Dispatcher, TestDigestDue(), connectorFilter(), Store, time.Time, fakeRefresherConnector (+21 more)

### Community 58 - "Manager"
Cohesion: 0.08
Nodes (16): cron.EntryID, Manager, JobName(), LogPartial(), NewManager(), Store, Store, ReportDefinitionRecord (+8 more)

### Community 59 - "lefthook Commit Hooks"
Cohesion: 0.50
Nodes (5): commit-msg Hook, Conventional Commits Policy, lefthook Commit Hooks, pre-commit Hook, Commit Conventions & Hook Enforcement (dev workflow)

### Community 60 - "TemplateEditorPage.tsx"
Cohesion: 0.06
Nodes (41): web_src_api_generated_templates_templates, web_src_api_generated_templates_templates_getgettemplatesquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidquerykey, web_src_api_generated_templates_templates_getgettemplatestemplateidversionsquerykey, web_src_api_generated_templates_templates_posttemplatestemplateidpreview, web_src_api_generated_templates_templates_posttemplatestemplateidversionsrevrestore, web_src_api_generated_templates_templates_puttemplatestemplateid, web_src_api_generated_templates_templates_usegettemplatestemplateid (+33 more)

### Community 61 - "fixtures.ts"
Cohesion: 0.06
Nodes (41): web_src_api_model_index_alert, web_src_api_model_index_alertpage, web_src_api_model_index_changedetail, web_src_api_model_index_changepage, web_src_api_model_index_changesummary, web_src_api_model_index_connectortypeschema, web_src_api_model_index_dashboardoverview, web_src_api_model_index_doc (+33 more)

### Community 62 - "ExportToFile"
Cohesion: 0.09
Nodes (45): confirm(), formatCounts(), main(), runRestore(), runVerify(), newSeededStore(), TestRunRestoreImportsIntoConfiguredDatabase(), TestRunRestoreRejectsCorruptedBundle() (+37 more)

### Community 63 - "NewMalformedResponseError"
Cohesion: 0.10
Nodes (36): NewMalformedResponseError(), buildAdlistTable(), buildClientTable(), buildDomainTable(), buildGroupTable(), cell(), clientIP(), groupNames() (+28 more)

### Community 64 - "home_assistant/tables.go"
Cohesion: 0.09
Nodes (39): jsonType(), TestAttributeCatalogCoversEmittedKeys(), unavailable(), attrIP(), attrNumber(), attrString(), buildEntities(), buildIntegrations() (+31 more)

### Community 65 - "RunMigrations"
Cohesion: 0.11
Nodes (37): main(), OpenDB(), TestOpenDBEnablesSQLiteForeignKeys(), TestOpenDBSetsSQLiteDurabilityPragmas(), GetMigrationStatus(), newMigrator(), collectColumns(), postgresSchemaColumns() (+29 more)

### Community 66 - "share_links_test.go"
Cohesion: 0.17
Nodes (41): GrantConnectorRole(), instanceAdminRole(), NewUser(), TestListFiltersGrantsBeforePagination(), Handler, newTestHandler(), TestAISuggestInvalidJSON(), TestGetLockNoneHeld() (+33 more)

### Community 67 - "Store"
Cohesion: 0.24
Nodes (4): ChatConversationRecord, Store, ChatMessageRecord, DocSectionEmbeddingRecord

### Community 68 - "AppShell — Bottom Dock Shell (single variant)"
Cohesion: 0.50
Nodes (4): AppShell — Bottom Dock Shell (single variant), Theme Engine — Code Default, User-Overridable, Per-User Dashboard Layout with Admin Default (v2), DashboardLayout Schema (per-user widget layout)

### Community 69 - "Get"
Cohesion: 0.08
Nodes (18): Handler, isWritableField(), validateConfigPushRequest(), applyConnectorScalarUpdates(), Handler, validateConnectorConfig(), capitalize(), Handler (+10 more)

### Community 70 - "dependencies"
Cohesion: 0.05
Nodes (41): dependencies, axios, clsx, codemirror, @codemirror/commands, @codemirror/lang-markdown, @codemirror/state, @codemirror/view (+33 more)

### Community 71 - "NewChecker"
Cohesion: 0.17
Nodes (35): NewChecker(), createComplianceRule(), createComplianceSnapshot(), createConnector(), findings(), newTestStore(), TestCheckEmptyDetectsAndAutoResolves(), TestCheckFailingDetectsAndAutoResolves() (+27 more)

### Community 72 - "ThemeControls.tsx"
Cohesion: 0.11
Nodes (33): @fontsource/space-mono, @fontsource-variable/space-grotesk, AdvancedControls(), FONT_KEYS, OPT_KEYS, PRESET_KEYS, Segmented(), ThemeControls() (+25 more)

### Community 73 - "channels.go"
Cohesion: 0.08
Nodes (26): IsTimeout(), TestIsTimeout(), buildEmailMessage(), sendNtfyChannel(), sendSMTPChannel(), sendTelegramChannel(), splitRecipients(), TestBuildEmailMessage_SanitizesSubjectNewlines() (+18 more)

### Community 74 - "traefik/tables.go"
Cohesion: 0.14
Nodes (31): jsonType(), TestAttributeCatalogCoversEmittedKeys(), buildEntryPointTable(), buildMiddlewareTable(), buildOverview(), buildRouterTable(), buildServiceTable(), cell() (+23 more)

### Community 75 - "compliance/engine.go"
Cohesion: 0.11
Nodes (30): contains(), equal(), Evaluate(), findAttribute(), Catalog, Condition, Entity, Rule (+22 more)

### Community 76 - "Configuration & Documentation Backup (Export/Import)"
Cohesion: 0.06
Nodes (28): Bundle format, Configuration & Documentation Backup (Export/Import), Endpoints, Import behavior, Manifest, checksum, and verification, 1. Every export gets a manifest and a checksum, 2. Verifying a backup actually restores, 3. Restoring for real (+20 more)

### Community 77 - "ReportsPage.tsx"
Cohesion: 0.07
Nodes (28): web_src_api_generated_reports_reports, web_src_api_generated_reports_reports_deletereportsdefinitionsreportdefinitionid, web_src_api_generated_reports_reports_getgetreportsdefinitionsquerykey, web_src_api_generated_reports_reports_getgetreportsquerykey, web_src_api_generated_reports_reports_getreportsreportiddownload, web_src_api_generated_reports_reports_postreportsdefinitions, web_src_api_generated_reports_reports_postreportsdefinitionsreportdefinitionidrun, web_src_api_generated_reports_reports_putreportsdefinitionsreportdefinitionid (+20 more)

### Community 78 - "Connector"
Cohesion: 0.15
Nodes (11): SnapshotSection, TestBuildHostsTableV5(), buildHostsTable(), Connector, parseHosts(), TestBuildHostsTableMalformedCases(), TestBuildHostsTableValidRecords(), unavailable() (+3 more)

### Community 79 - "DocRecord"
Cohesion: 0.10
Nodes (14): seedDelivery(), nilToStr(), docSearchWhere(), escapeLike(), DocRecord, Store, scanDoc(), scanDocSummary() (+6 more)

### Community 80 - "settings.mock.ts"
Cohesion: 0.08
Nodes (27): web_src_api_model_index_aiconfig, web_src_api_model_index_aifallbackprovider, web_src_api_model_index_health, web_src_api_model_index_notificationchannel, web_src_api_model_index_notificationroute, web_src_api_model_index_profileupdate, web_src_api_model_index_role, web_src_api_model_index_session (+19 more)

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

### Community 88 - "api/audit_test.go"
Cohesion: 0.10
Nodes (28): testApp, seedAlert(), TestAlertsBulkSnoozePartialFailure(), TestAlertsBulkSnoozeRejectsTooManyIDs(), TestAlertsBulkSnoozeRoleBoundary(), TestAlertsBulkSnoozeValidation(), TestAlertsListDaysWindow(), TestAlertsListSuccess() (+20 more)

### Community 89 - "api/auth/oidc.go"
Cohesion: 0.09
Nodes (21): TestEmailDomainAllowed(), TestOIDCRoleForGroups(), emailDomainAllowed(), oidcRoleForGroups(), Config, mask(), redactDSN(), redactKVPassword() (+13 more)

### Community 90 - "response.go"
Cohesion: 0.10
Nodes (22): Handler, Cursor(), DecodeCursor(), EncodeCursor(), T, NextCursor(), TestCursorRequestModes(), TestCursorRoundTrip() (+14 more)

### Community 91 - "handlers.ts"
Cohesion: 0.07
Nodes (28): web_src_api_generated_alerts_alerts_msw, web_src_api_generated_alerts_alerts_msw_getalertsmock, web_src_api_generated_auth_auth_msw, web_src_api_generated_auth_auth_msw_getauthmock, web_src_api_generated_changes_changes_msw, web_src_api_generated_changes_changes_msw_getchangesmock, web_src_api_generated_connectors_connectors_msw, web_src_api_generated_connectors_connectors_msw_getconnectorsmock (+20 more)

### Community 92 - "home_assistant_test.go"
Cohesion: 0.14
Nodes (28): AllowLoopbackForTest(), Connector, homeAssistantAPI(), newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchAppliesMaxEntities() (+20 more)

### Community 93 - "NewStore"
Cohesion: 0.16
Nodes (26): NewHandler(), TestBulkSnooze(), TestDismissNotFound(), TestGetNotFound(), TestListEmpty(), TestResolveNotFound(), TestSnooze(), NewStore() (+18 more)

### Community 94 - "timeline.ts"
Cohesion: 0.13
Nodes (17): enableMocks(), installMockWebSocket(), Window, WsMockHandle, Listenerish, MockWebSocket, Emit, env() (+9 more)

### Community 95 - ".PostMFATOTPConfirm"
Cohesion: 0.15
Nodes (15): secondFactorInput, factorJSON(), Handler, GenerateRecoveryCodes(), GenerateTOTPSecret(), NormalizeRecoveryCode(), randomRecoveryChars(), TestGenerateRecoveryCodesAreUniqueAndFormatted() (+7 more)

### Community 96 - "newTestHandler"
Cohesion: 0.13
Nodes (25): TestDataSnapshotPaths(), TestSyncsLimitAndIsolation(), createDNSResolverConnector(), Handler, itoa(), TestConfigFieldsHandler(), TestConfigPushHandler(), TestStartStopHandler() (+17 more)

### Community 97 - "Register"
Cohesion: 0.14
Nodes (25): init(), init(), init(), init(), init(), init(), init(), init() (+17 more)

### Community 98 - "connector/connector.go"
Cohesion: 0.09
Nodes (12): TimeoutError, NewAuthError(), NewTimeoutError(), TestTypedErrorsAreDistinguishableByType(), TestTypedErrorsWrapAndUnwrap(), AuthError, CredentialRefresher, MalformedResponseError (+4 more)

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
Cohesion: 0.13
Nodes (8): init(), Connector, buildGatewayTable(), primaryGatewayName(), wanInterfaceName(), PathSegment(), ValidateRefSegment(), Connector

### Community 103 - "NewEngine"
Cohesion: 0.20
Nodes (22): RequestedFields(), TestBaseContext(), TestSyncCancellationRecordsFailureAndReleasesGuard(), TestSyncExcludesConcurrentRuns(), TestRefreshCredentialsDirect(), TestRefreshCredentialsUnsupportedConnector(), TestRunSyncFieldsPassesHintToConnector(), TestRunSyncFieldsSurvivesCredentialRefresh() (+14 more)

### Community 104 - "ConnectorRecord"
Cohesion: 0.15
Nodes (13): ConnectorRecord, Store, scanConnector(), scanConnectorRows(), nullInt64ToIntPtr(), nullStrToStr(), scanSyncRun(), connectorWithRole (+5 more)

### Community 105 - "WiseLabz — Design Contract"
Cohesion: 0.08
Nodes (23): 10. Component conventions, 1. Identity, 2. Color tokens, 3. Status grammar, 4. Typography, 5. Radii & shadows, 6. Motion, 7. Z-index scale (+15 more)

### Community 106 - "devDependencies"
Cohesion: 0.08
Nodes (24): devDependencies, eslint, eslint-plugin-react-hooks, eslint-plugin-react-refresh, @faker-js/faker, jsdom, msw, orval (+16 more)

### Community 107 - "logging.go"
Cohesion: 0.14
Nodes (18): loggablePath(), loggableQuery(), Logger(), captureLog(), TestLoggerCorrelatesErrorfWithRequestID(), TestLoggerRedactsShareToken(), TestLoggerRedactsWSTicket(), TestLoggablePathMasksShareTokenUnderV1() (+10 more)

### Community 108 - "backup/backup.go"
Cohesion: 0.21
Nodes (21): connectorIDs(), docIDs(), exportDocs(), exportTemplates(), exportWithin(), AIConfigSummary, Import(), importBundle() (+13 more)

### Community 109 - "Connector"
Cohesion: 0.10
Nodes (7): ConfigField, TestBuildPeerTableAttributes(), TestBuildPolicyTableAttributes(), buildPeerTable(), buildPolicyTable(), buildRouteTable(), Connector

### Community 110 - "portainer_test.go"
Cohesion: 0.20
Nodes (21): dockerPath(), Connector, newTestConnector(), portainerAPI(), TestAPIKeyHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchContainersStillFetchesEnvironments() (+13 more)

### Community 111 - "Connector"
Cohesion: 0.16
Nodes (8): apiMessage(), controllerName(), countByKind(), statusError(), unavailable(), Connector, sectionFetch, session

### Community 112 - "NotificationRecord"
Cohesion: 0.14
Nodes (10): Dispatcher, Dispatcher, RunDeliveryRetries(), Store, RunDocLockSweep(), runDocLockSweep(), NotificationRecord, Store (+2 more)

### Community 113 - "useRole.ts"
Cohesion: 0.15
Nodes (16): web_src_api_generated_me_me, web_src_api_generated_me_me_usegetme, RoleGate(), RoleGateProps, DocHistory(), LinkedDocPanel(), SETTINGS_SECTIONS, SettingsLayout() (+8 more)

### Community 114 - "NewService"
Cohesion: 0.17
Nodes (19): TestAuthMiddlewareAcceptsNonAdminAPIKey(), TestAuthMiddlewareAPIKeyLifecycle(), TestAuthMiddlewareRejectsExpiredAndRevokedAPIKeys(), TestAuthMiddlewareThrottlesAPIKeyLastUsed(), NewService(), TestConcurrentIssuePairUniqueTokenIDs(), TestElevationExpired(), TestElevationRequiresOwner() (+11 more)

### Community 115 - "NewHTTPClient"
Cohesion: 0.12
Nodes (16): newConnector(), Connector, newGuardedClient(), TestGuardedClientRejectsLinkLocal(), TestGuardedClientRejectsLoopback(), intConfig(), newConnector(), NewHTTPClient() (+8 more)

### Community 116 - "adguardhome_test.go"
Cohesion: 0.20
Nodes (20): adguardAPI(), Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchDegradesWhenStatusFails() (+12 more)

### Community 117 - "Sanitize"
Cohesion: 0.20
Nodes (6): Sanitize(), TestSanitize(), changePatternID(), Engine, markError(), RunResult

### Community 118 - "Deps"
Cohesion: 0.22
Nodes (19): registerListAttentionItems(), registerListChanges(), jsonResult(), registerListConnectors(), registerSearchDocs(), connectorAllowSet(), findingConnectorIDs(), registerListFindings() (+11 more)

### Community 119 - "middleware.go"
Cohesion: 0.16
Nodes (14): AuditRecorder, contextKey, elevationError, PermissionChecker, UserStatusChecker, elevationFailureReason(), extractBearerToken(), hashToken() (+6 more)

### Community 120 - "go_pkg_context"
Cohesion: 0.07
Nodes (21): contains(), searchString(), go_pkg_context, go_pkg_database_sql, go_pkg_errors, go_pkg_fmt, go_pkg_github_com_google_uuid, go_pkg_github_com_jackc_pgx_v5_stdlib (+13 more)

### Community 121 - "Connector"
Cohesion: 0.15
Nodes (7): TestBuildInterfaceTableAttributes(), buildGatewayTable(), buildInterfaceTable(), buildSystemContent(), primaryGatewayName(), wanInterfaceName(), Connector

### Community 122 - "Config"
Cohesion: 0.18
Nodes (15): newLogger(), Config, LogSettings, IsSSHRemote(), AISettings, BackupSettings, DocExportGitSettings, DocExportSettings (+7 more)

### Community 123 - "GetTypeSchema"
Cohesion: 0.14
Nodes (17): catalog(), TestRegisteredSchema(), TestAllConnectorImplementationsRegister(), TestRegisteredSchema(), TestAttributeCatalogCoversEmittedKeys(), TestAttributeCatalogCoversNewEntityKinds(), TestBuildHostsTableAttributes(), TestSchemaExposesAPIVersion() (+9 more)

### Community 136 - "InstanceAdminFromContext"
Cohesion: 0.16
Nodes (8): Handler, contextWithShareLink(), Handler, newShareToken(), shareLinkFromContext(), InstanceAdminFromContext(), shareLinkNode, shareLinkScope

### Community 137 - "Handler"
Cohesion: 0.18
Nodes (4): Handler, stripLogControlChars(), Handler, BackupSchedule

### Community 138 - "truenas_test.go"
Cohesion: 0.24
Nodes (17): Connector, newTestConnector(), TestBearerHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchIsStableAcrossCalls() (+9 more)

### Community 139 - "keyset_test.go"
Cohesion: 0.19
Nodes (16): Store, seedConnectorForChanges(), TestChangeRelatedServiceIDsAndPatternIDRoundTrip(), TestChangeRelatedServiceIDsDefaultsToEmptyArray(), TestCountRecentChangePatterns(), TestCountRecentChangesByPattern(), assertSameSet(), Store (+8 more)

### Community 140 - "DiffViewer.tsx"
Cohesion: 0.13
Nodes (11): web_src_api_model_index_diff, DiffCell(), Gutter(), LayoutToggle(), SplitCell(), UnifiedLine(), unifiedRows(), UnifiedView() (+3 more)

### Community 141 - "ws.ts"
Cohesion: 0.11
Nodes (17): AlertCreatedPayload, AlertResolvedPayload, ChangeDetectedPayload, DocAiSuggestionPayload, DocGeneratedPayload, DocLockAcquiredPayload, DocLockExpiredPayload, DocLockReleasedPayload (+9 more)

### Community 142 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, allowImportingTsExtensions, isolatedModules, jsx, lib, module, moduleDetection, moduleResolution (+9 more)

### Community 143 - "handlers_actions_test.go"
Cohesion: 0.29
Nodes (16): actionRequest(), actionResponse(), TestActionBulkGrantBoundaries(), TestActionInvalidConnectorConfig(), TestActionLifecyclePreviews(), TestActionMaintenanceLifecycle(), TestActionPermissions(), TestActionStoreFailures() (+8 more)

### Community 144 - "Connector"
Cohesion: 0.18
Nodes (6): TestAttributeCatalogCoversEmittedKeys(), TestBuildDNSRecordTableAttributes(), TestBuildTunnelTableAttributes(), buildDNSRecordTable(), buildTunnelTable(), Connector

### Community 145 - "traefik_test.go"
Cohesion: 0.25
Nodes (16): Connector, newTestConnector(), TestBasicAuthHeaderIsSent(), TestErrorMapping(), TestExpiredDeadlineMapsToTimeout(), TestFetchDegradesPerSection(), TestFetchHappyPath(), TestFetchSelectiveFields() (+8 more)

### Community 146 - "MarshalConnectorConfig"
Cohesion: 0.20
Nodes (14): IsSecretFieldType(), MarshalConnectorConfig(), SecretFieldsChanged(), init(), TestConnectorRotationFieldsRoundTrip(), TestCreateConnectorDefaultsSecretRotatedAtToCreatedAt(), TestSecretFieldsChangedFalseOnRenameOnly(), TestSecretFieldsChangedFalseOnResubmittedUnchangedSecret() (+6 more)

### Community 147 - "docdiffmodel.ts"
Cohesion: 0.21
Nodes (14): diff, buildDocDiff(), DiffRowUnit, DocDiffModel, DocRow, fold(), toUnits(), DiffLine (+6 more)

### Community 148 - "system/handlers_test.go"
Cohesion: 0.23
Nodes (15): Handler, newTestHandler(), TestDiagnostics(), TestExportAudit(), TestExportImportBackupRoundTrip(), TestGetBackupScheduleDefault(), TestGetRetentionSettingsDefault(), TestHealth() (+7 more)

### Community 149 - "all.go"
Cohesion: 0.12
Nodes (15): go_pkg_github_com_wiselabz_wiselabz_internal_connector_adguardhome, go_pkg_github_com_wiselabz_wiselabz_internal_connector_cloudflare, go_pkg_github_com_wiselabz_wiselabz_internal_connector_custom, go_pkg_github_com_wiselabz_wiselabz_internal_connector_dnsresolver, go_pkg_github_com_wiselabz_wiselabz_internal_connector_docker, go_pkg_github_com_wiselabz_wiselabz_internal_connector_home_assistant, go_pkg_github_com_wiselabz_wiselabz_internal_connector_netbird, go_pkg_github_com_wiselabz_wiselabz_internal_connector_opnsense (+7 more)

### Community 150 - "doc/engine.go"
Cohesion: 0.16
Nodes (12): dateFormat(), filterByTitle(), join(), TestDateFormat(), TestFilterByTitle(), TestJoin(), TestToJSON(), TestTruncate() (+4 more)

### Community 151 - "httpx/retry_test.go"
Cohesion: 0.34
Nodes (14): RetryTransport(), do(), fail(), status(), TestRetryTransportDisabled(), TestRetryTransportDoesNotRetry(), TestRetryTransportGivesUpAfterMaxRetries(), TestRetryTransportHonorsRetryAfterWithinCap() (+6 more)

### Community 152 - "Store"
Cohesion: 0.15
Nodes (4): SnapshotRecord, Store, Store, GoldenSnapshotRecord

### Community 153 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, allowImportingTsExtensions, isolatedModules, lib, module, moduleDetection, moduleResolution, noEmit (+7 more)

### Community 154 - "AuthedUser"
Cohesion: 0.21
Nodes (15): TestCreate(), AuthedUser(), NewHandler(), decodePaginated(), jsonHasEmptyArrayItems(), newTestStore(), TestListDeliveriesEmpty(), TestListDeliveriesNoFilter() (+7 more)

### Community 155 - "handlers_contract_test.go"
Cohesion: 0.28
Nodes (14): AssertMatchesSpec(), loadSpec(), specPath(), createForSpec(), decodeEnvelope(), fieldMsgs(), Handler, TestConnectorSuccessPayloadsMatchSpec() (+6 more)

### Community 156 - "vectorCache"
Cohesion: 0.18
Nodes (10): newVectorCache(), TestVectorCacheBoundedLRU(), TestVectorCacheConcurrent(), TestVectorCacheInvalidateDocAndStalePut(), vectorCache, vectorEntry, vectorKey, go_pkg_container_list (+2 more)

### Community 157 - "fetch_test.go"
Cohesion: 0.19
Nodes (14): entityKinds(), sectionByTitle(), TestConfigPushV5(), TestFetchAuthFailureReturnsPlaceholderSnapshot(), TestFetchBothVersions(), TestFetchDegradesPerSection(), TestRestartUnsupportedOnV5(), TestStartStopV5() (+6 more)

### Community 159 - "net/http.Handler"
Cohesion: 0.18
Nodes (11): Config, ConnectorRoleChecker, CORS(), TestCORSMatchedOrigin(), TestCORSPreflightDisallowedOriginForbidden(), TestCORSUnlistedOriginGetsNoHeaders(), TreatAsSafeMethod(), RequireConnectorRole() (+3 more)

### Community 160 - "newHandler"
Cohesion: 0.24
Nodes (14): TestEmbeddedSPAWithoutFrontendBuild(), TestList(), TestRevoke(), JWTService(), Token(), WithAuth(), Handler, newHandler() (+6 more)

### Community 161 - "New"
Cohesion: 0.15
Nodes (13): Handler, newScratchStore(), SyncDocEmbeddings(), TestSyncDocEmbeddingsKeepsOldRowsWhenEmbedFails(), TestRetrieveUsesCacheAndSyncInvalidates(), TestUpsertBackupSchedulePostgresParity(), TestUpsertQualityFindingPostgresDedup(), TestSnapshotAttributesRoundTripPostgres() (+5 more)

### Community 162 - "pagination_contract_test.go"
Cohesion: 0.21
Nodes (13): hasAllStringKeys(), httputilCalls(), receiverName(), TestBareArrayAllowlistIsCurrent(), TestListHandlersUseSharedPaginationWriter(), TestNoHandRolledPaginationEnvelopes(), writesEnvelope(), go_pkg_go_ast (+5 more)

### Community 163 - "Handler"
Cohesion: 0.24
Nodes (6): definition(), record(), reportJSON(), valid(), Handler, input

### Community 164 - "backup/backup_test.go"
Cohesion: 0.26
Nodes (13): Export(), newTestStore(), TestExportIncludesRecordsBeyondAPage(), TestExportRedactsConnectorSecrets(), TestExportToFile(), TestExportToFileCreatesDirectory(), TestExportToFileDirNotWritable(), TestExportToFilePermissions() (+5 more)

### Community 165 - "render_test.go"
Cohesion: 0.31
Nodes (13): RenderHTML(), RenderMarkdown(), sampleData(), TestRenderHTML_EscapesDocTitles(), TestRenderHTML_SectionUnavailable(), TestRenderHTML_Truncated(), TestRenderMarkdown_Golden(), TestRenderMarkdown_SectionUnavailable() (+5 more)

### Community 166 - "NewRegistry"
Cohesion: 0.45
Nodes (12): NewRegistry(), NewHandler(), TestAIConfigRoundTrip(), testConfig(), TestGetAuthConfig(), TestGetDecryptedAPIKeyNoKeyStored(), TestNotificationsConfigRoundTrip(), TestNotificationsConfigSigningSecret() (+4 more)

### Community 167 - "newTestHandler"
Cohesion: 0.22
Nodes (13): Handler, newTestHandler(), TestAcknowledgeNotFound(), TestAcknowledgeSuccess(), TestAIUpdate(), TestBulkResolve(), TestDismissNotFound(), TestDismissSuccess() (+5 more)

### Community 168 - "api/changes_test.go"
Cohesion: 0.26
Nodes (12): testApp, seedChange(), seedChangeWithSeverity(), TestChangesAcknowledgeRoleBoundary(), TestChangesAcknowledgeSuccess(), TestChangesBulkResolveEmptyIDs(), TestChangesBulkResolveInvalidStatus(), TestChangesBulkResolvePartialFailure() (+4 more)

### Community 169 - "newTestHandler"
Cohesion: 0.24
Nodes (12): templateRequest(), TestListPagination(), TestPreviewDoesNotPersist(), TestTemplateErrorPaths(), TestVersionLifecycle(), Handler, newTestHandler(), TestCreate() (+4 more)

### Community 170 - "ValidateConfig"
Cohesion: 0.26
Nodes (11): TestSchemaConfigValidation(), ValidateConfig(), TestSchemaConfigValidation(), isConfigValidationError(), testSchema(), TestTypeSchemaDegradedLatencyThreshold(), TestValidateConfigAcceptsMissingAndEmptyFields(), TestValidateConfigEnum() (+3 more)

### Community 171 - "createUser"
Cohesion: 0.28
Nodes (13): seedAlert(), TestListAttentionItems(), seedChange(), TestListChanges(), TestListConnectors(), enableFakeEmbedding(), TestSearchDocs(), seedFinding() (+5 more)

### Community 172 - "log/slog.Logger"
Cohesion: 0.35
Nodes (11): RunCleanupOnce(), newTestStore(), testLogger(), TestRunCleanupAllDBErrors(), TestRunCleanupIdempotent(), TestRunCleanupPartialFailure(), TestRunCleanupPrunesOldHealthChecks(), TestRunCleanupSkipsDisabledCategories() (+3 more)

### Community 173 - "Contributor Covenant Code of Conduct"
Cohesion: 0.15
Nodes (12): 1. Correction, 2. Warning, 3. Temporary Ban, 4. Permanent Ban, Attribution, Contributor Covenant Code of Conduct, Enforcement, Enforcement Guidelines (+4 more)

### Community 174 - "apikey_scope.go"
Cohesion: 0.24
Nodes (10): APIKeyRestriction, APIKeyRestrictionFromContext(), ClampConnectorRole(), ContextWithAPIKeyRestriction(), isSafeMethod(), TestClampConnectorRole(), treatAsSafeFromContext(), IsSafeMethod() (+2 more)

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

### Community 180 - "ServiceDependency"
Cohesion: 0.22
Nodes (5): ServiceDependency, poolDependencies(), unavailable(), networkDependencies(), Connector

### Community 181 - "newTCPDockerClient"
Cohesion: 0.18
Nodes (11): GuardedDialer(), IsDangerousIP(), buildDockerTLSConfig(), newTCPDockerClient(), TestNewTCPDockerClientNoTLSWhenNoCert(), TestNewTCPDockerClientRejectsInvalidCertPair(), Unwrap(), newWebhookClient() (+3 more)

### Community 182 - "Store"
Cohesion: 0.33
Nodes (3): Store, scanConnectorGrants(), ConnectorGrant

### Community 183 - "Decision"
Cohesion: 0.18
Nodes (10): 0003 — Config-push lab-mutating operation, Authorization / confirmation / audit, Auto-revert-then-alert on mismatch, Consequences, Context, Decision, Field-level partial update via a per-connector whitelist, Out of scope (+2 more)

### Community 184 - ".call"
Cohesion: 0.47
Nodes (7): fixture, Handler, newFixture(), TestBulkSnoozeAuthzPerItem(), TestGetAuthz(), TestListFiltersByGrantAndPaginates(), TestMutationAuthz()

### Community 185 - "connectors_maintenance_test.go"
Cohesion: 0.33
Nodes (9): testApp, seedMaintenanceConnector(), TestCloseMaintenanceWindowRoleBoundaryAndNoElevation(), TestGetMaintenanceWindowAnyAuthenticatedUser(), TestListActiveMaintenanceWindowsEndpoint(), TestOpenMaintenanceWindowConnectorNotFound(), TestOpenMaintenanceWindowInvalidDuration(), TestOpenMaintenanceWindowNoElevationRequired() (+1 more)

### Community 186 - "NewHandler"
Cohesion: 0.24
Nodes (10): NewHandler(), TestByServiceNoDocsYet(), TestGenerate(), TestGetUnknownIDFallsBackToServicePlaceholder(), TestRestore(), TestTemplateSchema(), TestTree(), TestTreeEmpty() (+2 more)

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

### Community 193 - "decodeBulkRequest"
Cohesion: 0.31
Nodes (4): decodeBulkRequest(), Handler, bulkItemResult, bulkRequest

### Community 194 - "openapi_contract_test.go"
Cohesion: 0.33
Nodes (8): normalizeParams(), routerOperations(), specOperations(), TestAPIV1AliasServesSameHandlers(), TestOpenAPIHealthProbeRoutes(), TestOpenAPIMatchesRouter(), chi.Routes, go_pkg_go_yaml_in_yaml_v3

### Community 195 - "net/http.Response"
Cohesion: 0.33
Nodes (6): retryable(), sleep(), net/http.Response, RetryPolicy, retryTransport, scripted

### Community 196 - "release-please-config.json"
Cohesion: 0.22
Nodes (8): changelog-sections, changelog-type, extra-files, include-component-in-tag, last-release-sha, packages, release-type, $schema

### Community 197 - "connectors_hardening_test.go"
Cohesion: 0.29
Nodes (7): testApp, TestConnectorsCreateAcceptsValidConfig(), TestConnectorsCreateRejectsInvalidEnum(), TestConnectorsCreateRejectsMalformedConfig(), TestConnectorsSyncAcceptsFieldsHint(), TestConnectorsUpdateRejectsMalformedConfig(), waitForSyncRuns()

### Community 208 - "RateLimit"
Cohesion: 0.29
Nodes (6): TestRateLimit(), RateLimit(), golang.org/x/time/rate.Limit, golang.org/x/time/rate.Limiter, limiterStore, visitor

### Community 209 - "ComputeWindow"
Cohesion: 0.39
Nodes (6): ComputeWindow(), TestComputeWindow_CappedAt31Days(), TestComputeWindow_ExactlyAtCap(), TestComputeWindow_FirstRun(), TestComputeWindow_ManualRunUsesLastScheduledWatermarkUnchanged(), TestComputeWindow_Watermark()

### Community 211 - "Cache"
Cohesion: 0.43
Nodes (5): Cache, New(), Cache[V], entry, V

### Community 212 - "compliance_rules_test.go"
Cohesion: 0.48
Nodes (6): badRegexMessage(), complianceCondition(), TestComplianceRulesCRUDAndAdminGate(), TestComplianceRuleValidation(), validComplianceRule(), complianceRule()

### Community 213 - "TestBulkReauth"
Cohesion: 0.48
Nodes (7): bulkReq(), createBulkFakeConnector(), Handler, registerBulkFakeConnector(), TestBulkReauth(), TestBulkRestart(), TestBulkSync()

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
Cohesion: 0.29
Nodes (6): Audit Trail, Endpoint, Keyset (cursor) pagination, Retention, What's not recorded, What's recorded

### Community 218 - "Bulk Review Actions"
Cohesion: 0.29
Nodes (6): Auditability, Bulk Review Actions, Endpoint, Frontend, Partial failure is not batch failure, What counts as low-risk

### Community 219 - "PULL_REQUEST_TEMPLATE.md"
Cohesion: 0.29
Nodes (6): Breaking changes, Checklist, Description, For connector PRs only, Screenshots or logs, Type of change

### Community 220 - "walkCursorPages"
Cohesion: 0.40
Nodes (6): cursorPage, decodeCursorPage(), testApp, TestAuditCursorPaginationTraversal(), TestChangesCursorPaginationTraversal(), walkCursorPages()

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

### Community 226 - "schema.go"
Cohesion: 0.50
Nodes (4): Schema(), schemaFor(), TestSchemaMatchesConfig(), reflect.Type

### Community 227 - "seedScopeFixture"
Cohesion: 0.60
Nodes (4): Store, seedScopeFixture(), TestListDocSectionEmbeddingsFiltersByGrant(), TestMergedAttentionItemsFiltersByGrant()

### Community 228 - "Notification Channels"
Cohesion: 0.40
Nodes (4): Adding a channel type, Channel reference, Notification Channels, Webhook signing (HMAC-SHA256)

### Community 229 - "compose-smoke.sh"
Cohesion: 0.40
Nodes (3): COMPOSE_SMOKE_ENV_FILE, COMPOSE_SMOKE_PORT, compose-smoke.sh script

### Community 231 - "ClassifyHealth"
Cohesion: 0.67
Nodes (3): ClassifyHealth(), TestClassifyHealth(), TestClassifyHealthPerTypeThreshold()

### Community 234 - "transform_firewall.go"
Cohesion: 0.67
Nodes (3): normalizeEnabledColumn(), normalizeFirewallRules(), TestNormalizeFirewallRulesRewritesEnabledColumn()

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
- **38 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `Topbar Notification Center (deferred from V1)` and `NotificationDelivery Schema (per-channel delivery/retry)`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **What is the exact relationship between `Topbar Notification Center (deferred from V1)` and `Notification / NotificationPage Schemas`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Store` connect `Store` to `net/http.Request`, `testApp`, `Handler`, `diagnostics/diagnostics.go`, `NewEngine`, `Checker`, `DecodeKey`, `Dispatcher`, `rewritePlaceholders`, `dispatcher_test.go`, `AuthedUser`, `net/http.Handler`, `newHandler`, `New`, `Engine`, `backup/backup_test.go`, `Handler`, `NewRegistry`, `createUser`, `log/slog.Logger`, `git_test.go`, `store/backup_test.go`, `rowScanner`, `.call`, `main`, `NewHandler`, `Manager`, `time.Time`, `.call`, `ExportToFile`, `RunMigrations`, `share_links_test.go`, `Get`, `NewChecker`, `DocRecord`, `NewStore`, `engine_maintenance_test.go`, `NewEngine`, `backup/backup.go`, `Sanitize`, `Deps`, `go_pkg_context`?**
  _High betweenness centrality (0.010) - this node is a cross-community bridge._
- **Why does `UserIDFromContext()` connect `net/http.Request` to `context.Context`, `decodeBulkRequest`, `Handler`, `Store`, `InstanceAdminFromContext`, `AuthMiddleware`, `ErrorWithDetails`, `Deps`, `middleware.go`, `rowScanner`, `.PostMFATOTPConfirm`, `net/http.Handler`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **Why does `vectorCache` connect `vectorCache` to `Store`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **What connects `github.com/WiseLabz/wiselabz`, `bulkSnoozeRequest`, `bulkSnoozeItemResult` to the rest of the system?**
  _567 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.Client` be split into smaller, more focused modules?**
  _Cohesion score 0.04225352112676056 - nodes in this community are weakly interconnected._