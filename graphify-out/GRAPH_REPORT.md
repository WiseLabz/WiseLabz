# Graph Report - .  (2026-06-27)

## Corpus Check
- Corpus is ~37,977 words - fits in a single context window. You may not need a graph.

## Summary
- 278 nodes · 270 edges · 16 communities detected
- Extraction: 70% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 35 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Core Domain Concepts & Design|Core Domain Concepts & Design]]
- [[_COMMUNITY_Shell & Navigation Components|Shell & Navigation Components]]
- [[_COMMUNITY_App Bootstrap & Routing|App Bootstrap & Routing]]
- [[_COMMUNITY_Feature Pages & State|Feature Pages & State]]
- [[_COMMUNITY_WebSocket Mock Stack|WebSocket Mock Stack]]
- [[_COMMUNITY_Mock Data Fixtures|Mock Data Fixtures]]
- [[_COMMUNITY_Theme Engine & Palette|Theme Engine & Palette]]
- [[_COMMUNITY_Mock Service Worker Core|Mock Service Worker Core]]
- [[_COMMUNITY_Mock Enable & App Entry|Mock Enable & App Entry]]
- [[_COMMUNITY_Command Palette Logic|Command Palette Logic]]
- [[_COMMUNITY_Go Backend Store|Go Backend Store]]
- [[_COMMUNITY_Go Backend Server|Go Backend Server]]
- [[_COMMUNITY_Dashboard Widgets Module|Dashboard Widgets Module]]
- [[_COMMUNITY_Motion Provider|Motion Provider]]
- [[_COMMUNITY_Axios HTTP Layer|Axios HTTP Layer]]
- [[_COMMUNITY_Build Configuration|Build Configuration]]

## God Nodes (most connected - your core abstractions)
1. `icons.tsx - hand-built SVG icon set (24px/1.5 stroke)` - 10 edges
2. `ServiceRosterWidget - service list with status counts` - 9 edges
3. `cn - className joiner utility` - 9 edges
4. `MockWebSocket` - 7 edges
5. `Skeleton/EmptyState/ErrorState` - 7 edges
6. `handleRequest()` - 5 edges
7. `getResponse()` - 5 edges
8. `AlertSummaryWidget - pending alerts feed` - 5 edges
9. `env()` - 4 edges
10. `AppShell - auth app frame (sidebar+topbar+outlet)` - 4 edges

## Surprising Connections (you probably didn't know these)
- `WidgetFrame - dashboard grid cell` --semantically_similar_to--> `Panel/PanelHeader - framed region`  [INFERRED] [semantically similar]
  web/src/components/dashboard/WidgetFrame.tsx → web/src/components/ui/Panel.tsx
- `DocTree - hierarchical doc sidebar nav` --semantically_similar_to--> `Markdown - compact dependency-free renderer`  [INFERRED] [semantically similar]
  web/src/components/docs/DocTree.tsx → web/src/components/docs/Markdown.tsx
- `DashboardPage` --semantically_similar_to--> `ServicesPage`  [INFERRED] [semantically similar]
  web/src/features/dashboard/DashboardPage.tsx → web/src/features/services/ServicesPage.tsx
- `ServicesPage` --semantically_similar_to--> `ChangesPage`  [INFERRED] [semantically similar]
  web/src/features/services/ServicesPage.tsx → web/src/features/changes/ChangesPage.tsx
- `ChangesPage` --semantically_similar_to--> `AlertsPage`  [INFERRED] [semantically similar]
  web/src/features/changes/ChangesPage.tsx → web/src/features/alerts/AlertsPage.tsx

## Hyperedges (group relationships)
- **AppShell composes Sidebar + Topbar + CommandPalette into the app frame** —  [INFERRED 1.00]
- **Dashboard widgets share loading/empty/error state rendering pattern** —  [INFERRED 0.90]
- **UI primitives all depend on cn utility for conditional styling** —  [INFERRED 0.90]
- **Live State Consumers** — features_dashboardpage, features_servicespage, ws_provider [INFERRED 0.90]
- **Page State Pattern** — features_servicespage, features_changespage, features_changedetailpage, features_docspage, features_alertspage [INFERRED 0.85]
- **Zustand Persist Pattern** — store_dashboard, store_theme, store_settings [INFERRED 0.90]
- **app_bootstrap_sequence** — wl_app_bootstrap, wl_theme_engine, wl_mock_enable, wl_app_component [INFERRED 1.00]
- **mock_data_pipeline** — wl_fixtures_data, wl_mock_curated, wl_mock_handlers, wl_mock_browser_worker [INFERRED 1.00]
- **websocket_mock_stack** — wl_mock_enable, wl_mock_ws_install, wl_mock_ws_class, wl_mock_ws_timeline [INFERRED 1.00]

## Communities

### Community 0 - "Core Domain Concepts & Design"
Cohesion: 0.07
Nodes (37): AI Module, Alert System, Authentication System, Calm Precise Brand, Change Detection, Connector Interface, Connector System, Conventional Commits (+29 more)

### Community 2 - "Shell & Navigation Components"
Cohesion: 0.15
Nodes (26): AppShell - auth app frame (sidebar+topbar+outlet), Button/IconButton - UI button vocabulary, categoryIcon.ts - connector category to icon map, cn - className joiner utility, CommandPalette - global keyboard command palette, DiffViewer - unified infra/doc diff viewer, DocDiff - line-level document diff, InfraDiff - structured key-path diff rows (+18 more)

### Community 3 - "App Bootstrap & Routing"
Cohesion: 0.09
Nodes (24): AlertsPage, main.tsx - App Bootstrap, App.tsx - Root Component, AppShell, ChangeDetailPage, ChangesPage, DashboardPage, DocsPage (+16 more)

### Community 4 - "Feature Pages & State"
Cohesion: 0.12
Nodes (18): QueryClient, AlertsPage, ChangeDetailPage, ChangesPage, DashboardPage, DocHistory, DocsPage, ServicesPage (+10 more)

### Community 5 - "WebSocket Mock Stack"
Cohesion: 0.19
Nodes (6): MockWebSocket, env(), heartbeat(), newId(), startTimeline(), syncProgress()

### Community 6 - "Mock Data Fixtures"
Cohesion: 0.25
Nodes (2): dashboardOverview(), minsAgo()

### Community 7 - "Theme Engine & Palette"
Cohesion: 0.36
Nodes (6): applyTokens(), makePalette(), presetOpts(), commit(), load(), tokensFor()

### Community 8 - "Mock Service Worker Core"
Cohesion: 0.62
Nodes (6): getResponse(), handleRequest(), resolveMainClient(), respondWithMock(), sendToClient(), serializeRequest()

### Community 10 - "Mock Enable & App Entry"
Cohesion: 0.33
Nodes (3): enableMocks(), bootstrap(), installMockWebSocket()

### Community 11 - "Command Palette Logic"
Cohesion: 0.5
Nodes (2): onKeyDown(), run()

### Community 12 - "Go Backend Store"
Cohesion: 0.4
Nodes (3): Store, New(), TestNew()

### Community 21 - "Go Backend Server"
Cohesion: 0.67
Nodes (3): cmd/server/main.go - Go Entry, internal/store/store.go - Central Store, internal/store/store_test.go - Store Tests

### Community 59 - "Dashboard Widgets Module"
Cohesion: 1.0
Nodes (1): Dashboard Widgets

### Community 60 - "Motion Provider"
Cohesion: 1.0
Nodes (1): MotionProvider - framer-motion config bridge

### Community 61 - "Axios HTTP Layer"
Cohesion: 1.0
Nodes (1): Axios Instance

### Community 62 - "Build Configuration"
Cohesion: 1.0
Nodes (1): Environment Config

## Knowledge Gaps
- **12 isolated node(s):** `Store`, `MotionProvider - framer-motion config bridge`, `DocDiff - line-level document diff`, `SyncActivityWidget - live sync timeline`, `Markdown - compact dependency-free renderer` (+7 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Mock Data Fixtures`** (9 nodes): `alertPage()`, `changeDetail()`, `changePage()`, `dashboardOverview()`, `daysAgo()`, `hrsAgo()`, `minsAgo()`, `svcDoc()`, `fixtures.ts`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Command Palette Logic`** (5 nodes): `buildCommands()`, `CommandPalette()`, `onKeyDown()`, `run()`, `CommandPalette.tsx`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Dashboard Widgets Module`** (1 nodes): `Dashboard Widgets`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Motion Provider`** (1 nodes): `MotionProvider - framer-motion config bridge`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Axios HTTP Layer`** (1 nodes): `Axios Instance`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Build Configuration`** (1 nodes): `Environment Config`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Are the 9 inferred relationships involving `icons.tsx - hand-built SVG icon set (24px/1.5 stroke)` (e.g. with `Topbar - global app top bar` and `Sidebar - flat workspace navigation`) actually correct?**
  _`icons.tsx - hand-built SVG icon set (24px/1.5 stroke)` has 9 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Store`, `MotionProvider - framer-motion config bridge`, `DocDiff - line-level document diff` to the rest of the system?**
  _12 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Core Domain Concepts & Design` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._
- **Should `Icon System (24px SVG Set)` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._
- **Should `App Bootstrap & Routing` be split into smaller, more focused modules?**
  _Cohesion score 0.09 - nodes in this community are weakly interconnected._
- **Should `Feature Pages & State` be split into smaller, more focused modules?**
  _Cohesion score 0.12 - nodes in this community are weakly interconnected._