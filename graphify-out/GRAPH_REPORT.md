# Graph Report - WiseLabz  (2026-09-08)

## Corpus Check
- 283 files · ~157,234 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1459 nodes · 2493 edges · 84 communities detected
- Extraction: 59% EXTRACTED · 37% INFERRED · 0% AMBIGUOUS · INFERRED: 926 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 35|Community 35]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 37|Community 37]]
- [[_COMMUNITY_Community 38|Community 38]]
- [[_COMMUNITY_Community 39|Community 39]]
- [[_COMMUNITY_Community 40|Community 40]]
- [[_COMMUNITY_Community 41|Community 41]]
- [[_COMMUNITY_Community 43|Community 43]]
- [[_COMMUNITY_Community 46|Community 46]]
- [[_COMMUNITY_Community 48|Community 48]]
- [[_COMMUNITY_Community 49|Community 49]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 52|Community 52]]
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 58|Community 58]]
- [[_COMMUNITY_Community 66|Community 66]]
- [[_COMMUNITY_Community 67|Community 67]]
- [[_COMMUNITY_Community 68|Community 68]]
- [[_COMMUNITY_Community 69|Community 69]]
- [[_COMMUNITY_Community 70|Community 70]]
- [[_COMMUNITY_Community 71|Community 71]]
- [[_COMMUNITY_Community 72|Community 72]]
- [[_COMMUNITY_Community 73|Community 73]]
- [[_COMMUNITY_Community 96|Community 96]]
- [[_COMMUNITY_Community 97|Community 97]]
- [[_COMMUNITY_Community 98|Community 98]]
- [[_COMMUNITY_Community 99|Community 99]]
- [[_COMMUNITY_Community 100|Community 100]]
- [[_COMMUNITY_Community 101|Community 101]]
- [[_COMMUNITY_Community 102|Community 102]]
- [[_COMMUNITY_Community 103|Community 103]]
- [[_COMMUNITY_Community 104|Community 104]]
- [[_COMMUNITY_Community 105|Community 105]]
- [[_COMMUNITY_Community 106|Community 106]]
- [[_COMMUNITY_Community 107|Community 107]]
- [[_COMMUNITY_Community 108|Community 108]]
- [[_COMMUNITY_Community 162|Community 162]]
- [[_COMMUNITY_Community 163|Community 163]]
- [[_COMMUNITY_Community 164|Community 164]]
- [[_COMMUNITY_Community 165|Community 165]]
- [[_COMMUNITY_Community 166|Community 166]]
- [[_COMMUNITY_Community 167|Community 167]]
- [[_COMMUNITY_Community 168|Community 168]]
- [[_COMMUNITY_Community 169|Community 169]]
- [[_COMMUNITY_Community 170|Community 170]]
- [[_COMMUNITY_Community 171|Community 171]]

## God Nodes (most connected - your core abstractions)
1. `Errorf()` - 214 edges
2. `Store` - 111 edges
3. `newTestApp()` - 98 edges
4. `Error()` - 83 edges
5. `JSON()` - 77 edges
6. `New()` - 66 edges
7. `newDocTestStore()` - 42 edges
8. `UserIDFromContext()` - 24 edges
9. `contains()` - 24 edges
10. `RunMigrations()` - 21 edges

## Surprising Connections (you probably didn't know these)
- `WiseLabz Project` --conceptually_related_to--> `Monorepo with Go Workspaces`  [INFERRED]
  README.md → docs/ARCHITECTURE.md
- `WiseLabz Project` --conceptually_related_to--> `Change-Aware Diff Engine`  [INFERRED]
  README.md → docs/ARCHITECTURE.md
- `bootstrap()` --calls--> `Import()`  [INFERRED]
  web/src/main.tsx → backend/internal/backup/backup.go
- `enableMocks()` --calls--> `Import()`  [INFERRED]
  web/src/mocks/enable.ts → backend/internal/backup/backup.go
- `TestOpenAICompatibleSuggestErrors()` --calls--> `contains()`  [INFERRED]
  backend/internal/ai/openai_test.go → backend/internal/store/user.go

## Hyperedges (group relationships)
- **Step-Up Confirmation Flow for Destructive Actions** — architecture_permissions_stepup, architecture_destructive_confirm_pattern, openapi_auth_elevate_endpoint, openapi_removal_impact_endpoint [EXTRACTED 0.90]
- **Contract-First API Codegen Pipeline** — architecture_orval, openapi_spec_document, architecture_react_query, architecture_diff_contract [EXTRACTED 0.85]
- **Dual Local/OIDC Auth Mode System** — architecture_auth_design, architecture_oidc_provider_config, openapi_oidc_provider_schema, openapi_auth_config_endpoint [EXTRACTED 0.90]

## Communities

### Community 0 - "Community 0"
Cohesion: 0.03
Nodes (40): Handler, Handler, readIP(), sanitizeSessions(), sanitizeUser(), setRefreshCookie(), signOIDCState(), verifyOIDCState() (+32 more)

### Community 1 - "Community 1"
Cohesion: 0.03
Nodes (28): Errorf(), MarshalConnectorConfig(), nilToStr(), nullInt64ToIntPtr(), nullStrToStr(), scanConnectors(), ConnectorRecord, DBTX (+20 more)

### Community 2 - "Community 2"
Cohesion: 0.03
Nodes (107): seedAlert(), TestAlertsListSuccess(), TestAlertsResolveRoleBoundary(), TestAlertsResolveSuccess(), TestAlertsSnoozeSuccess(), TestAlertsSnoozeValidation(), TestAuditListFiltersByAction(), TestAuditListRoleBoundary() (+99 more)

### Community 3 - "Community 3"
Cohesion: 0.04
Nodes (59): StubProvider, main(), TestCreateAuditRecordAndListFiltering(), TestRecordAuditFromContextMarshalsDetail(), TestConnectorOwnerRoundTrip(), TestConnectorScheduleFieldsDefaultNull(), TestConnectorScheduleFieldsRoundTrip(), TestListDueConnectors() (+51 more)

### Community 4 - "Community 4"
Cohesion: 0.06
Nodes (39): TestAllConnectorImplementationsRegister(), Factory, Get(), GetTypeSchema(), ListSchemas(), Register(), TestRegisterDefaultsToNonStub(), TestRegisterStubRoundTrips() (+31 more)

### Community 5 - "Community 5"
Cohesion: 0.04
Nodes (25): Connector, ServiceDependency, ServiceSnapshot, SnapshotSection, Connector, init(), isDangerousIP(), newGuardedClient() (+17 more)

### Community 6 - "Community 6"
Cohesion: 0.08
Nodes (37): NewHandler(), Config, NewRouter(), spaHandler(), testApp, contextKey, NewService(), TestElevationExpired() (+29 more)

### Community 7 - "Community 7"
Cohesion: 0.05
Nodes (30): RegisterOpenAICompatible(), TestOpenAICompatibleSuggestErrors(), TestRegisterOpenAICompatibleDefaults(), openAICompatibleProvider, Provider, NewRegistry(), Registry, SuggestChunk (+22 more)

### Community 8 - "Community 8"
Cohesion: 0.08
Nodes (29): AIConfigSummary, Export(), Import(), LoadAIConfigSummary(), RedactConnectorConfig(), newTestStore(), TestExportRedactsConnectorSecrets(), TestImportIsIdempotent() (+21 more)

### Community 9 - "Community 9"
Cohesion: 0.14
Nodes (15): Decrypt(), DeriveKey(), Encrypt(), TestDecryptTampered(), TestDecryptWrongKey(), TestDeriveKeyDeterministic(), TestDeriveKeyDifferent(), TestEncryptBadKeySize() (+7 more)

### Community 10 - "Community 10"
Cohesion: 0.08
Nodes (27): ADR 0001 — Monorepo, ADR Index (docs/adr/), AI Doc Generation Module (opt-in, provider-agnostic), API Design — REST + WebSocket split, Dual Auth Design (Local JWT + OIDC), Changes/Diff Contract (infra vs doc format), Change-Aware Diff Engine, Monorepo with Go Workspaces (+19 more)

### Community 11 - "Community 11"
Cohesion: 0.23
Nodes (16): Checker, NewChecker(), RunStaleSweep(), createConnector(), findings(), newTestStore(), TestCheckEmptyDetectsAndAutoResolves(), TestCheckFailingDetectsAndAutoResolves() (+8 more)

### Community 12 - "Community 12"
Cohesion: 0.17
Nodes (13): channelCfg, Dispatcher, NewDispatcher(), sendWebhook(), deliveriesFor(), findDelivery(), newTestStore(), setWebhookConfig() (+5 more)

### Community 13 - "Community 13"
Cohesion: 0.12
Nodes (20): Axios API client, Button and IconButton, CommandPalette, theme cycling command, ConfirmDialog, Dialog, ElevationConfirm, English translation catalog (+12 more)

### Community 14 - "Community 14"
Cohesion: 0.16
Nodes (20): alerts, changes, connector config JSON, connectors, dashboard layouts, doc versions, docs, HashToken (+12 more)

### Community 15 - "Community 15"
Cohesion: 0.12
Nodes (8): renderAuthGuard(), renderRoleGuard(), Engine, GenerateResult, renderResult, templateData, renderDialog(), renderPanel()

### Community 16 - "Community 16"
Cohesion: 0.14
Nodes (5): pgPlaceholderDB, pgTransactionDB, rewritePlaceholders(), TestRewritePlaceholders(), transactionDB

### Community 17 - "Community 17"
Cohesion: 0.16
Nodes (4): dashboardOverview(), minsAgo(), serviceSnapshot(), syncRunsFor()

### Community 18 - "Community 18"
Cohesion: 0.2
Nodes (7): Claims, ElevationClaims, ElevationToken, hasAudience(), newTokenID(), Service, TokenPair

### Community 19 - "Community 19"
Cohesion: 0.17
Nodes (8): broadcastMsg, Client, Envelope, Hub, NewHub(), assertEnvelope(), TestHubBroadcastRouting(), TestHubConfiguredOrigin()

### Community 20 - "Community 20"
Cohesion: 0.16
Nodes (15): Connector Interface (Name/Fetch/Validate), Connector Management via UI (full CRUD), Destructive-Action Pattern: Confirm + Blast Radius, Manager Actions (v1 scope), Permissions & Step-Up for Mutating Actions, PRODUCT.md, Role Model — viewer/operator, ServiceSnapshot Data Structure (+7 more)

### Community 21 - "Community 21"
Cohesion: 0.19
Nodes (6): MockWebSocket, env(), heartbeat(), newId(), startTimeline(), syncProgress()

### Community 22 - "Community 22"
Cohesion: 0.16
Nodes (7): getAccessToken(), setAccessToken(), apply(), clear(), handle(), jump(), wsUrl()

### Community 23 - "Community 23"
Cohesion: 0.15
Nodes (14): Connector interface, connector schema registration, reverse proxy WebSocket support, OpenAPI REST contract, destructive-action step-up authentication, operational alerts, detected changes, Compare (+6 more)

### Community 24 - "Community 24"
Cohesion: 0.17
Nodes (13): OpenAPI client generation, Authentication and onboarding guards, Operator-only routes, Application root, Application router, Generated-code lint exclusions, Client bootstrap, Conditional mock bootstrap (+5 more)

### Community 25 - "Community 25"
Cohesion: 0.17
Nodes (6): Logger(), Recoverer(), GetRequestID(), RequestID(), requestIDKey, responseWriter

### Community 26 - "Community 26"
Cohesion: 0.2
Nodes (4): durationLabel(), relativeTime(), relativeTime(), cn()

### Community 27 - "Community 27"
Cohesion: 0.24
Nodes (9): AlertNotifier, TestComputeNextRun_BackoffNeverExceedsScheduleCadence(), TestComputeNextRun_FailureUsesBackoffSchedule(), TestComputeNextRun_ManualOnlyNeverSchedules(), TestComputeNextRun_SuccessSchedulesAtCadenceAndResetsRetries(), computeNextRun(), markError(), QualityChecker (+1 more)

### Community 28 - "Community 28"
Cohesion: 0.18
Nodes (11): Connector category icon map, Live dashboard state, Dashboard widget frame, Dashboard widgets, Shared SVG icon family, Authenticated app frame, Bottom-dock shell, Floating dock navigation (+3 more)

### Community 29 - "Community 29"
Cohesion: 0.2
Nodes (10): WCAG 2.2 AA accessibility, Docs-first information architecture, technical homelabbers, machine-honest interface, v1 narrow manager scope, trustworthy live documentation, WiseLabz, commit quality gates (+2 more)

### Community 30 - "Community 30"
Cohesion: 0.25
Nodes (4): buildCommands(), onKeyDown(), run(), registeredCommands()

### Community 31 - "Community 31"
Cohesion: 0.31
Nodes (6): buildDocDiff(), fold(), toUnits(), diffStats(), lineDiff(), wordDiff()

### Community 32 - "Community 32"
Cohesion: 0.39
Nodes (5): addSection(), moveSection(), removeSection(), update(), updateSection()

### Community 33 - "Community 33"
Cohesion: 0.36
Nodes (6): applyTokens(), makePalette(), presetOpts(), commit(), load(), tokensFor()

### Community 34 - "Community 34"
Cohesion: 0.46
Nodes (6): runCleanup(), RunScheduler(), newTestStore(), testLogger(), TestRunCleanupIdempotent(), TestRunCleanupSkipsDisabledCategories()

### Community 35 - "Community 35"
Cohesion: 0.38
Nodes (4): navigateTo(), invalidate(), markAllRead(), markRead()

### Community 36 - "Community 36"
Cohesion: 0.62
Nodes (6): getResponse(), handleRequest(), resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 37 - "Community 37"
Cohesion: 0.29
Nodes (2): OIDCClaims, OIDCProvider

### Community 38 - "Community 38"
Cohesion: 0.29
Nodes (6): DocRecord, DocVersionRecord, TemplateRecord, TemplateSectionRecord, TemplateVersionRecord, TemplateVersionSection

### Community 39 - "Community 39"
Cohesion: 0.4
Nodes (2): findRoute(), rowSeverity()

### Community 40 - "Community 40"
Cohesion: 0.53
Nodes (5): Compare(), lineCount(), severityForChange(), DiffPatch, DiffResult

### Community 41 - "Community 41"
Cohesion: 0.6
Nodes (3): fillBody(), generatePreview(), renderTemplate()

### Community 43 - "Community 43"
Cohesion: 0.5
Nodes (3): useCanMutate(), useRole(), RoleGate()

### Community 46 - "Community 46"
Cohesion: 0.5
Nodes (5): Commit Conventions & Hook Enforcement (dev workflow), commit-msg Hook, Conventional Commits Policy, lefthook Commit Hooks, pre-commit Hook

### Community 48 - "Community 48"
Cohesion: 0.83
Nodes (3): close(), onKey(), reset()

### Community 49 - "Community 49"
Cohesion: 0.83
Nodes (3): close(), onKey(), reset()

### Community 51 - "Community 51"
Cohesion: 0.5
Nodes (1): TestWebSocket

### Community 52 - "Community 52"
Cohesion: 0.5
Nodes (2): runSync(), triggerMockSync()

### Community 53 - "Community 53"
Cohesion: 0.67
Nodes (2): commit(), move()

### Community 55 - "Community 55"
Cohesion: 0.67
Nodes (2): apply(), css()

### Community 56 - "Community 56"
Cohesion: 0.5
Nodes (3): FailedSyncRun, SyncRunRecord, SyncRunStatus

### Community 57 - "Community 57"
Cohesion: 0.5
Nodes (2): ClassifyHealth(), TestClassifyHealth()

### Community 58 - "Community 58"
Cohesion: 0.5
Nodes (4): AppShell — Bottom Dock Shell (single variant), Theme Engine — Code Default, User-Overridable, Per-User Dashboard Layout with Admin Default (v2), DashboardLayout Schema (per-user widget layout)

### Community 66 - "Community 66"
Cohesion: 0.67
Nodes (2): AlertRecord, ChangeRecord

### Community 67 - "Community 67"
Cohesion: 0.67
Nodes (3): Document diff model, Diff layout preference, Diff viewer

### Community 68 - "Community 68"
Cohesion: 0.67
Nodes (3): Destructive connector confirmation, Connector removal impact, Step-up reauthentication

### Community 69 - "Community 69"
Cohesion: 0.67
Nodes (3): PostgreSQL compose deployment, single-instance deployment model, SQLite compose deployment

### Community 70 - "Community 70"
Cohesion: 0.67
Nodes (3): GRAPH_REPORT.md, Graphify Knowledge Graph Rules, Graphify Wiki Index

### Community 71 - "Community 71"
Cohesion: 0.67
Nodes (3): Database: SQLite + PostgreSQL, golang-migrate, sqlc (type-safe SQL codegen)

### Community 72 - "Community 72"
Cohesion: 0.67
Nodes (3): go:embed SPA Embedding, React + Vite Frontend, Frontend Testing Policy (deferred until rewrite)

### Community 73 - "Community 73"
Cohesion: 0.67
Nodes (3): Topbar Notification Center (deferred from V1), NotificationDelivery Schema (per-channel delivery/retry), Notification / NotificationPage Schemas

### Community 96 - "Community 96"
Cohesion: 1.0
Nodes (1): AuditRecord

### Community 97 - "Community 97"
Cohesion: 1.0
Nodes (1): SavedView

### Community 98 - "Community 98"
Cohesion: 1.0
Nodes (2): Template catalog, Template preview generator

### Community 99 - "Community 99"
Cohesion: 1.0
Nodes (2): Change detail synthesizer, Homelab mock data

### Community 100 - "Community 100"
Cohesion: 1.0
Nodes (2): Settings mock data, Notification routing matrix

### Community 101 - "Community 101"
Cohesion: 1.0
Nodes (2): DocTree, Markdown

### Community 102 - "Community 102"
Cohesion: 1.0
Nodes (2): pgPlaceholderDB, rewritePlaceholders

### Community 103 - "Community 103"
Cohesion: 1.0
Nodes (2): safe application defaults, WISELABZ environment overrides

### Community 104 - "Community 104"
Cohesion: 1.0
Nodes (2): Branch Naming Convention, Pull Request Process

### Community 105 - "Community 105"
Cohesion: 1.0
Nodes (2): chi HTTP Router, gorilla/websocket

### Community 106 - "Community 106"
Cohesion: 1.0
Nodes (2): viper Config Loader, WISELABZ_ Env Var Config Override

### Community 107 - "Community 107"
Cohesion: 1.0
Nodes (2): GHCR Container Registry, GitHub Actions CI

### Community 108 - "Community 108"
Cohesion: 1.0
Nodes (2): GET /api/version (undocumented ops endpoint), GET /system/info

### Community 162 - "Community 162"
Cohesion: 1.0
Nodes (1): TimeAgo

### Community 163 - "Community 163"
Cohesion: 1.0
Nodes (1): Panel

### Community 164 - "Community 164"
Cohesion: 1.0
Nodes (1): store package

### Community 165 - "Community 165"
Cohesion: 1.0
Nodes (1): OpenDB

### Community 166 - "Community 166"
Cohesion: 1.0
Nodes (1): ErrNotFound

### Community 167 - "Community 167"
Cohesion: 1.0
Nodes (1): ErrConflict

### Community 168 - "Community 168"
Cohesion: 1.0
Nodes (1): slog (stdlib logging)

### Community 169 - "Community 169"
Cohesion: 1.0
Nodes (1): Zustand State Management

### Community 170 - "Community 170"
Cohesion: 1.0
Nodes (1): Tailwind CSS

### Community 171 - "Community 171"
Cohesion: 1.0
Nodes (1): Docker Compose Deployment

## Ambiguous Edges - Review These
- `Topbar Notification Center (deferred from V1)` → `Notification / NotificationPage Schemas`  [AMBIGUOUS]
  docs/MISSING.md · relation: conceptually_related_to
- `Topbar Notification Center (deferred from V1)` → `NotificationDelivery Schema (per-channel delivery/retry)`  [AMBIGUOUS]
  docs/MISSING.md · relation: conceptually_related_to

## Knowledge Gaps
- **136 isolated node(s):** `contextKey`, `Claims`, `ElevationClaims`, `TokenPair`, `ElevationToken` (+131 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 37`** (7 nodes): `OIDCClaims`, `OIDCProvider`, `.AuthURL()`, `.Exchange()`, `.Initialize()`, `.IsInitialized()`, `oidc.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 39`** (6 nodes): `eventLabel()`, `findRoute()`, `rowSeverity()`, `setCell()`, `setRowSeverity()`, `EventRoutingTable.tsx`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 51`** (4 nodes): `WebSocketProvider.test.tsx`, `TestWebSocket`, `.close()`, `.constructor()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 52`** (4 nodes): `runSync()`, `runSync.ts`, `triggerSync.ts`, `triggerMockSync()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 53`** (4 nodes): `commit()`, `move()`, `widgetTitle()`, `DashboardPage.tsx`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 55`** (4 nodes): `apply()`, `css()`, `seed()`, `appearance.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 57`** (4 nodes): `health.go`, `health_test.go`, `ClassifyHealth()`, `TestClassifyHealth()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 66`** (3 nodes): `change.go`, `AlertRecord`, `ChangeRecord`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 96`** (2 nodes): `audit.go`, `AuditRecord`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 97`** (2 nodes): `saved_view.go`, `SavedView`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 98`** (2 nodes): `Template catalog`, `Template preview generator`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 99`** (2 nodes): `Change detail synthesizer`, `Homelab mock data`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 100`** (2 nodes): `Settings mock data`, `Notification routing matrix`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 101`** (2 nodes): `DocTree`, `Markdown`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 102`** (2 nodes): `pgPlaceholderDB`, `rewritePlaceholders`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 103`** (2 nodes): `safe application defaults`, `WISELABZ environment overrides`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 104`** (2 nodes): `Branch Naming Convention`, `Pull Request Process`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 105`** (2 nodes): `chi HTTP Router`, `gorilla/websocket`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 106`** (2 nodes): `viper Config Loader`, `WISELABZ_ Env Var Config Override`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 107`** (2 nodes): `GHCR Container Registry`, `GitHub Actions CI`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 108`** (2 nodes): `GET /api/version (undocumented ops endpoint)`, `GET /system/info`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 162`** (1 nodes): `TimeAgo`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 163`** (1 nodes): `Panel`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 164`** (1 nodes): `store package`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 165`** (1 nodes): `OpenDB`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 166`** (1 nodes): `ErrNotFound`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 167`** (1 nodes): `ErrConflict`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 168`** (1 nodes): `slog (stdlib logging)`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 169`** (1 nodes): `Zustand State Management`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 170`** (1 nodes): `Tailwind CSS`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 171`** (1 nodes): `Docker Compose Deployment`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `Topbar Notification Center (deferred from V1)` and `Notification / NotificationPage Schemas`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **What is the exact relationship between `Topbar Notification Center (deferred from V1)` and `NotificationDelivery Schema (per-channel delivery/retry)`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Errorf()` connect `Community 1` to `Community 0`, `Community 3`, `Community 4`, `Community 37`, `Community 5`, `Community 7`, `Community 8`, `Community 9`, `Community 11`, `Community 12`, `Community 15`, `Community 18`?**
  _High betweenness centrality (0.223) - this node is a cross-community bridge._
- **Why does `newTestApp()` connect `Community 2` to `Community 1`, `Community 3`, `Community 4`, `Community 6`?**
  _High betweenness centrality (0.103) - this node is a cross-community bridge._
- **Why does `New()` connect `Community 1` to `Community 0`, `Community 2`, `Community 3`, `Community 4`, `Community 34`, `Community 6`, `Community 7`, `Community 8`, `Community 11`, `Community 12`, `Community 15`, `Community 25`, `Community 57`?**
  _High betweenness centrality (0.093) - this node is a cross-community bridge._
- **Are the 212 inferred relationships involving `Errorf()` (e.g. with `.IssuePair()` and `.IssueElevation()`) actually correct?**
  _`Errorf()` has 212 INFERRED edges - model-reasoned connections that need verification._
- **Are the 97 inferred relationships involving `newTestApp()` (e.g. with `TestAlertsListSuccess()` and `TestAlertsResolveRoleBoundary()`) actually correct?**
  _`newTestApp()` has 97 INFERRED edges - model-reasoned connections that need verification._