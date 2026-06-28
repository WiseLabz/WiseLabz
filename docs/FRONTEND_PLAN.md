# WiseLabz — Frontend Plan

## Context

WiseLabz is a self-hosted, open-source homelab documentation tool. It connects to
services (Proxmox, Docker, pfSense, …) over their APIs, generates structured docs
from configurable templates, shows live data on a dashboard, and runs a diff engine
that detects infrastructure changes — routing them to AI-assisted doc updates or
alerts.

This document is the frontend blueprint to be built **before any application code**.
It defines pages, components, state, the WebSocket contract, routing, edge states,
and onboarding. No application code, no visual design.

**Stack (confirmed from `web/package.json`):** React 19 + Vite 7 + TypeScript,
Zustand 5, React Query (`@tanstack/react-query`) 5, axios, Tailwind v4.
**To be added during implementation (locked — see §8):** `react-router-dom` v6,
**orval** (OpenAPI → React Query hooks, generates `src/api/`), `useWebSocket` custom
hook, **CodeMirror 6** (doc editor), **react-grid-layout** (dashboard grid),
**react-i18next** (i18n `t()` layer from day one).

**Product decisions locked during planning (drive everything below):**
1. **Multi-user with roles** — accounts have roles (admin / editor / viewer).
   There is a Users management page and role-gated routes, not just logged-in/out.
2. **AI module is optional/toggleable** — UI must handle AI-on and AI-off states
   everywhere AI appears (alerts, doc editor). AI config lives in Settings.
3. **Connectors are category-grouped** — one connector model with type metadata,
   but rendered by a **category component family**: Virtualization, Containers/PaaS,
   Networking. Type-specific fields are schema-driven within each category.
4. **Doc editor is full freehand + AI assist** — generated docs are a starting
   point; user gets markdown/rich editing, AI fills gaps.
5. **Dashboard is customizable** — togglable, arrangeable widgets, layout persisted
   per user.
6. **Auth/OIDC — providers file-defined, app toggles only** *(amended 2026-06-25;
   supersedes the original in-app-CRUD decision)*. OIDC providers (issuer, clientId,
   secret) live in config/env and never transit the API. Settings → Auth is read-only
   provider status + safe toggles (enable/disable provider, local-login, token TTLs).
   Rationale + contract: `docs/ARCHITECTURE.md` (OIDC section), `docs/openapi.yaml`.
7. **Docs are hierarchical** — one top-level lab doc plus per-service child docs,
   navigable as a tree.
8. **Docs are versioned with history UI** — view/restore/diff past revisions.
9. **Notifications: in-app + Email (SMTP) + Webhook/chat** channels.

---

## 1. Page Inventory

> Convention: `Auth` column — **public** (no auth), **protected** (any logged-in
> user), or a role (**admin**). `redirect→login` means unauthenticated hits bounce
> to `/login`.

### Auth & entry

| Route            | Purpose                                                        | Auth   | Primary data source                                                    | Key actions                                                                            |
|------------------|----------------------------------------------------------------|--------|------------------------------------------------------------------------|----------------------------------------------------------------------------------------|
| `/login`         | Local username/password login + OIDC provider buttons.         | public | `GET /api/auth/providers` (which OIDC enabled), `POST /api/auth/login` | Submit credentials; click an OIDC provider; go to forgot-password (if local).          |
| `/auth/callback` | OIDC redirect landing; exchanges code for JWT, then redirects. | public | `POST /api/auth/oidc/callback`                                         | None (transient); on success → dashboard or onboarding, on failure → login with error. |

### Core app

| Route                   | Purpose                                                                                | Auth      | Primary data source                                                                                                | Key actions                                                                                                      |
|-------------------------|----------------------------------------------------------------------------------------|-----------|--------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------|
| `/` → `/dashboard`      | Customizable overview of service status, recent changes, alert summary, sync activity. | protected | `GET /api/dashboard/overview`, `GET /api/dashboard/layout`; WS: `service.status`, `sync.progress`, `alert.created` | Add/remove/rearrange widgets; trigger global sync; jump to a service/alert/change.                               |
| `/services`             | List of all connected services with live status.                                       | protected | `GET /api/connectors` (+ status); WS: `service.status`                                                             | Filter/search; open a service; trigger per-service sync; add connector → onboarding/connectors.                  |
| `/services/:id`         | One service's detail: live data, status, last sync, linked docs.                       | protected | `GET /api/connectors/:id`, `GET /api/connectors/:id/data`; WS: `service.status`, `sync.progress` (scoped to id)    | Trigger sync; open service docs; open service change history; edit/remove connector (role-gated).                |
| `/services/:id/docs`    | The service's generated/edited doc (child of lab doc).                                 | protected | `GET /api/docs/service/:id`                                                                                        | View; open editor (editor+); view version history.                                                               |
| `/services/:id/history` | Change history (diffs) scoped to this service.                                         | protected | `GET /api/changes?serviceId=:id`                                                                                   | Inspect a change; view doc diff; resolve/ack alert tied to a change.                                             |
| `/docs`                 | Documentation viewer — hierarchical lab doc tree (lab root + per-service children).    | protected | `GET /api/docs/tree`, `GET /api/docs/:docId`                                                                       | Navigate tree/TOC; open editor; view version history; export/print.                                              |
| `/docs/:docId`          | A specific rendered doc node within the tree.                                          | protected | `GET /api/docs/:docId`                                                                                             | Same as `/docs`, focused on node.                                                                                |
| `/docs/:docId/edit`     | Freehand + AI-assisted editor for a doc node.                                          | editor+   | `GET /api/docs/:docId`, `GET /api/templates` (insertable blocks); WS: `doc.generated`, `doc.ai_suggestion`         | Edit markdown/rich; insert template blocks; request AI fill; save (new version); discard; view/restore versions. |
| `/docs/:docId/history`  | Version history + doc-version diff for a node.                                         | protected | `GET /api/docs/:docId/versions`, `GET /api/docs/:docId/versions/:rev`                                              | View a revision; diff two revisions; restore (editor+).                                                          |
| `/templates`            | Template / generation config — define what sections/fields docs generate.              | editor+   | `GET /api/templates`                                                                                               | Create/edit/delete template; preview; assign template to category/service.                                       |
| `/templates/:id`        | Single template editor.                                                                | editor+   | `GET /api/templates/:id`                                                                                           | Edit fields/sections; save; test-generate.                                                                       |
| `/connectors`           | Manage service connections — add/edit/remove, grouped by category.                     | editor+   | `GET /api/connectors`, `GET /api/connectors/schema` (category field metadata)                                      | Add connector (category → type → schema-driven form); edit; test connection; remove; enable/disable.             |
| `/connectors/new`       | Add-connector flow (category picker → type → credentials form).                        | editor+   | `GET /api/connectors/schema`                                                                                       | Pick category/type; fill schema-driven fields; test; save.                                                       |
| `/connectors/:id/edit`  | Edit an existing connector.                                                            | editor+   | `GET /api/connectors/:id`                                                                                          | Edit fields; re-test; rotate credentials; save.                                                                  |
| `/changes`              | Global diff / changelog feed — every detected infra change.                            | protected | `GET /api/changes`; WS: `change.detected`                                                                          | Filter by service/type/date; open a change; view doc diff; ack/dismiss.                                          |
| `/changes/:id`          | Single change detail with before/after diff.                                           | protected | `GET /api/changes/:id`                                                                                             | View diff; trigger AI doc update (if AI on) or apply manual update; ack/dismiss; jump to related alert.          |
| `/alerts`               | Alert center — pending items flagged by the diff engine.                               | protected | `GET /api/alerts`; WS: `alert.created`, `alert.resolved`                                                           | Filter by severity/status; resolve/snooze/dismiss; bulk-resolve; open underlying change.                         |
| `/alerts/:id`           | Single alert detail.                                                                   | protected | `GET /api/alerts/:id`                                                                                              | Resolve/snooze/dismiss; trigger AI update (if AI on); open change/service.                                       |
| `/onboarding`           | First-run flow when no connectors exist.                                               | protected | `GET /api/connectors` (empty check), `GET /api/connectors/schema`                                                  | Add first connector; trigger first sync; finish → dashboard.                                                     |

### Settings (nested)

| Route                             | Purpose                                                                               | Auth      | Primary data source                                                                                  | Key actions                                                                                                                                                                             |
|-----------------------------------|---------------------------------------------------------------------------------------|-----------|------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `/settings` → `/settings/profile` | Settings shell with section nav.                                                      | protected | —                                                                                                    | Navigate sections.                                                                                                                                                                      |
| `/settings/profile`               | Current user's profile + password.                                                    | protected | `GET /api/me`                                                                                        | Edit display name/email; change password (local accounts); manage own sessions/tokens.                                                                                                  |
| `/settings/users`                 | User & role management.                                                               | admin     | `GET /api/users`                                                                                     | Invite/create user; assign role; disable/delete; reset password.                                                                                                                        |
| `/settings/auth`                  | Auth status — local policy + read-only OIDC provider list.                            | admin     | `GET /api/auth/config`; `PUT /api/auth/config` (toggles/TTLs); `PUT /api/auth/providers/:id/enabled` | View file-defined providers (issuer/clientId/`secretConfigured`, never the secret); enable/disable a provider; toggle local login; set token TTLs. No provider create/edit (file-only). |
| `/settings/ai`                    | AI module config.                                                                     | admin     | `GET /api/ai/config`                                                                                 | Enable/disable AI; set provider/model/keys; choose default mode (auto-update vs suggest-only); test.                                                                                    |
| `/settings/notifications`         | Notification channels & rules.                                                        | admin     | `GET /api/notifications/config`                                                                      | Configure in-app, SMTP, webhook/chat targets; per-event routing; test send.                                                                                                             |
| `/settings/system` *(added)*      | Instance/system info — version, WS/health status, sync schedule, integrations health. | admin     | `GET /api/system/info`, `GET /api/health`; WS: connection status                                     | View health; set global sync schedule; restart/refresh integrations.                                                                                                                    |

### Global / utility (added — implied by product)

| Route              | Purpose                                       | Auth             | Notes               |
|--------------------|-----------------------------------------------|------------------|---------------------|
| `*` (NotFound)     | 404 within app shell.                         | protected/public | Catch-all.          |
| `/forbidden` (403) | Shown when a user lacks the role for a route. | protected        | Role-guard landing. |

**Pages added beyond the brief, with justification:**
- `/settings/system` & `/health` surface — multi-user self-hosted tool needs an
  admin view of instance health, WS status, and sync scheduling (diff engine has to
  run on a cadence).
- `/forbidden` (403) — roles exist, so role-denied needs a destination.
- `/docs/:docId/history`, `/services/:id/history`, `/changes/:id`,
  `/alerts/:id`, `/templates/:id`, `/connectors/:id/edit` — detail/sub-views
  implied by versioning, hierarchical docs, and the diff engine.

---

## 2. Component Architecture

Legend: **[L]** Layout · **[S]** Shared (`src/components/`) · **[P]** Page-specific
(`src/pages/<page>/components/`).

### App-wide shells

```
AppRoot [L]
├── QueryClientProvider / ZustandProviders (no UI)
├── WebSocketProvider [L]            (mounts useWebSocket, feeds stores)
├── ProtectedLayout [L]
│   ├── Sidebar [L]                  (nav, role-filtered items)
│   │   └── NavItem [S] · AlertBadge [S] · WsStatusDot [S]
│   ├── Topbar [L]                   (global sync btn, user menu, alert bell)
│   │   ├── GlobalSyncButton [S]
│   │   ├── AlertBell [S]            (reads pendingAlerts store)
│   │   └── UserMenu [S]
│   └── PageContainer [L]            (outlet + page-level ErrorBoundary + Suspense)
└── PublicLayout [L]                 (centered shell for login/callback)
```

### Cross-cutting shared components `[S]`

`DataTable`, `EmptyState`, `ErrorState`, `LoadingSkeleton`, `Toast`/`Toaster`,
`ConfirmDialog`, `Modal`/`Drawer`, `StatusPill` (service status), `SeverityTag`
(alerts), `DiffViewer` (used by changes + doc history), `MarkdownRenderer`,
`SchemaForm` (renders fields from backend JSON schema — used by connectors, AI,
notifications, templates), `RoleGate` (renders children only if role allows),
`TimeAgo`, `SearchFilterBar`, `Pagination`.

### Per-page composition

**Login `/login`**
```
LoginPage
├── LocalLoginForm [P]
├── OidcProviderButtons [P]      (maps GET /auth/providers)
└── AuthErrorBanner [S]
```

**Dashboard `/dashboard`**
```
DashboardPage
├── DashboardToolbar [P]         (edit-layout toggle, add-widget, global sync)
├── WidgetGrid [P]               (drag/drop, reads dashboard layout store)
│   └── WidgetFrame [P]          (wrapper: header, remove, resize)
│       ├── ServiceStatusWidget [P]
│       ├── RecentChangesWidget [P]
│       ├── AlertSummaryWidget [P]
│       ├── SyncActivityWidget [P]
│       └── DocsHealthWidget [P]
├── AddWidgetPanel [P]
└── EmptyState [S] / ErrorState [S] / LoadingSkeleton [S]
```

**Services list `/services` & detail `/services/:id`**
```
ServicesListPage → SearchFilterBar [S] · ServiceCard [P] · DataTable [S]
ServiceDetailPage
├── ServiceHeader [P]            (StatusPill [S], sync btn, RoleGate edit/remove)
├── ServiceTabs [P]              (overview | docs | history)  ← nested routes
├── ServiceLiveData [P]          (category-specific renderer)
│   ├── VirtualizationDataView [P]
│   ├── ContainerDataView [P]
│   └── NetworkingDataView [P]
└── SyncStatusBanner [S]
```

**Docs viewer `/docs`, node `/docs/:docId`**
```
DocsPage
├── DocTree [P]                  (lab root + per-service children)
├── DocTocSidebar [P]            (in-doc section nav)
├── DocContent [P] → MarkdownRenderer [S]
├── DocActions [P]               (edit RoleGate, history, export)
└── EmptyState [S] (no docs generated)
```

**Doc editor `/docs/:docId/edit`**
```
DocEditorPage
├── EditorToolbar [P]            (save, discard, insert-template, request-AI)
├── RichMarkdownEditor [P]       (wraps editor lib; local draft state)
├── TemplateBlockPicker [P]      (GET /api/templates)
├── AiAssistPanel [P]            (only if AI enabled; consumes doc.ai_suggestion WS)
└── UnsavedChangesGuard [S]
```

**Doc history `/docs/:docId/history`**
```
DocHistoryPage → VersionList [P] · DiffViewer [S] · RestoreButton [P] (RoleGate)
```

**Templates `/templates`, `/templates/:id`**
```
TemplatesPage → TemplateList [P] · TemplateCard [P]
TemplateEditorPage → TemplateSectionEditor [P] · SchemaForm [S] · TemplatePreview [P]
```

**Connectors `/connectors`, `/connectors/new`, `/connectors/:id/edit`**
```
ConnectorsPage
├── ConnectorCategoryGroup [P]   (Virtualization | Containers/PaaS | Networking)
│   └── ConnectorRow [P]         (StatusPill [S], test, edit, enable/disable, remove)
ConnectorFormPage
├── CategoryPicker [P]           (new only)
├── ConnectorTypePicker [P]      (types within category)
├── ConnectorCredentialsForm [P] → SchemaForm [S]   (fields from /connectors/schema)
└── TestConnectionButton [P]
```

**Changes `/changes`, `/changes/:id`**
```
ChangesFeedPage → SearchFilterBar [S] · ChangeFeedItem [P] · Pagination [S]
ChangeDetailPage
├── ChangeMeta [P]               (service, type, time, SeverityTag [S])
├── DiffViewer [S]               (before/after infra state)
├── DocImpactPanel [P]           (which docs affected)
└── ChangeActions [P]            (AI update if enabled | manual update | ack/dismiss)
```

**Alerts `/alerts`, `/alerts/:id`**
```
AlertCenterPage → AlertFilters [P] · AlertListItem [P] · BulkActionBar [P]
AlertDetailPage → AlertMeta [P] · linked ChangeSummary [P] · AlertActions [P]
```

**Settings `/settings/*`**
```
SettingsLayout [L] → SettingsNav [P] (RoleGate per section) + Outlet
├── ProfileSettings [P]          (ProfileForm, PasswordForm, SessionList)
├── UsersSettings [P]            (UserTable, InviteUserModal, RoleSelect) [admin]
├── AuthSettings [P]             (OidcProviderList [read-only: status + EnableToggle], LocalAuthToggle, TokenTtlForm) [admin]
├── AiSettings [P]               (AiConfigForm→SchemaForm, AiModeSelect, TestButton) [admin]
├── NotificationSettings [P]     (ChannelList: InApp/Smtp/Webhook cards → SchemaForm, EventRoutingTable) [admin]
└── SystemSettings [P]           (HealthPanel, SyncScheduleForm, IntegrationsHealthList) [admin]
```

**Onboarding `/onboarding`**
```
OnboardingPage
├── OnboardingStepper [P]        (welcome → add connector → first sync → done)
├── CategoryPicker [P]           (reused from connectors)
├── ConnectorCredentialsForm [P] (reused)
├── FirstSyncProgress [P]        (consumes sync.progress WS)
└── OnboardingComplete [P]
```

---

## 3. State Architecture

### Zustand stores (`src/stores/`)

| Store                  | Holds                                                                                                                                                                 | Why global (not local)                                                                                                       |
|------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------|
| `authStore`            | JWT/token, current user (`id`, name, **role**), auth status, OIDC provider list, login/logout actions.                                                                | Read by router guards, `RoleGate`, axios interceptor, topbar — cross-cutting; must survive navigation.                       |
| `wsStore`              | WS connection status (`connecting`/`open`/`closed`/`reconnecting`), last event ts.                                                                                    | Many components show liveness; `useWebSocket` writes once, everyone reads.                                                   |
| `alertsStore`          | Pending alert count + lightweight pending list (for bell/badge).                                                                                                      | Topbar bell, sidebar badge, dashboard summary need a shared real-time count updated by WS independent of which page is open. |
| `dashboardLayoutStore` | Widget layout (positions/sizes/enabled), edit-mode flag. Persisted (per-user, hydrated from `GET /api/dashboard/layout`, mirrored to localStorage for instant paint). | Customizable dashboard; layout outlives remounts and is user-owned.                                                          |
| `uiStore`              | Sidebar collapsed, active modals/drawers, theme hook point, global toasts queue.                                                                                      | Pure cross-page UI ephemeral state.                                                                                          |
| `syncStore`            | In-flight sync jobs keyed by service id + global, progress %.                                                                                                         | `sync.progress` WS updates arrive anywhere; services list, detail, dashboard, onboarding all read.                           |

Server data does **not** go in Zustand — it lives in React Query. Stores hold
session, UI, real-time-push, and user-owned layout only.

### React Query keys (`src/api/queries/`)

| Key                                                                                     | Caches                       | Invalidated by                                                    |
|-----------------------------------------------------------------------------------------|------------------------------|-------------------------------------------------------------------|
| `['dashboard','overview']`                                                              | Dashboard aggregate.         | connector add/edit/remove, sync complete, alert resolve.          |
| `['dashboard','layout']`                                                                | Saved widget layout.         | layout save mutation.                                             |
| `['connectors']`                                                                        | Connector list + status.     | `POST/PUT/DELETE /connectors`, connector enable/disable.          |
| `['connector', id]`                                                                     | One connector.               | edit/test that connector.                                         |
| `['connector', id, 'data']`                                                             | Live service data.           | `sync.complete` for id (WS → invalidate).                         |
| `['connectorSchema']`                                                                   | Category/type field schemas. | rarely; on app load / version change.                             |
| `['docs','tree']`                                                                       | Doc hierarchy.               | doc create/delete, connector add/remove, `doc.generated`.         |
| `['doc', docId]`                                                                        | Rendered doc node.           | doc save (new version), `doc.generated` for node.                 |
| `['doc', docId, 'versions']`                                                            | Version list.                | doc save, restore.                                                |
| `['templates']` / `['template', id]`                                                    | Templates.                   | template create/edit/delete.                                      |
| `['changes', filters]`                                                                  | Change feed page.            | `change.detected` (WS → invalidate list).                         |
| `['change', id]`                                                                        | Change detail.               | ack/dismiss/AI-update.                                            |
| `['alerts', filters]`                                                                   | Alert list.                  | `alert.created`/`alert.resolved` (WS), resolve/dismiss mutations. |
| `['alert', id]`                                                                         | Alert detail.                | resolve/snooze/dismiss.                                           |
| `['users']`                                                                             | User list (admin).           | invite/create/role-change/delete.                                 |
| `['me']`                                                                                | Current user profile.        | profile/password mutation.                                        |
| `['authConfig']`,`['aiConfig']`,`['notificationsConfig']`,`['systemInfo']`,`['health']` | Settings configs.            | their respective save mutations; `health` also via WS status.     |

**Invalidation strategy (mutation → invalidates):**
- Add/edit/remove connector → `['connectors']`, `['dashboard','overview']`,
  `['docs','tree']` (service node may appear/disappear), `['connectorSchema']` n/a.
- Trigger sync → optimistic entry in `syncStore`; on `sync.complete` WS →
  invalidate `['connector', id, 'data']`, `['dashboard','overview']`,
  `['changes', …]` (sync may surface changes).
- Save doc → `['doc', docId]`, `['doc', docId,'versions']`, `['docs','tree']`.
- Resolve/dismiss alert → `['alerts', …]`, `['alert', id]`, `['dashboard','overview']`;
  also decrement `alertsStore` immediately (optimistic).
- AI/manual change-update → `['change', id]`, related `['doc', docId]`,
  `['alerts', …]` if it clears an alert.
- WS-driven invalidations (`change.detected`, `alert.created`, `doc.generated`)
  are wired centrally in `WebSocketProvider`, mapping event → `queryClient.invalidateQueries`.

### Notable local component state

- **`RichMarkdownEditor` / DocEditorPage** — draft content, dirty flag, AI-suggestion
  staging, undo buffer. Stays local; only commits to server on save.
- **Connector add flow** — multi-step (category → type → schema form → test): local
  wizard state machine; nothing global until saved.
- **`WidgetGrid` drag session** — transient drag positions are local; committed to
  `dashboardLayoutStore` (and persisted) on drop.
- **`SyncActivityWidget` / FirstSyncProgress** — small real-time progress buffer fed
  by `syncStore`; throttle/coalesce rapid `sync.progress` events locally.
- **`DiffViewer`** — expanded/collapsed hunks, view mode (split/unified): local.
- **Filter bars** (changes/alerts/services) — filter state local, reflected into the
  React Query key so caching works per-filter; optionally synced to URL search params.

---

## 4. WebSocket Event Plan

**WS contract the backend must implement at `/ws`.** Every live UI behavior traces
to an event here.

**Naming convention:** lowercase `domain.action`, dot-separated. Domains:
`service`, `sync`, `change`, `alert`, `doc`, `system`. Envelope is uniform:

```ts
interface WsEnvelope<T = unknown> {
  type: string;        // e.g. "service.status"
  ts: string;          // ISO-8601 server timestamp
  payload: T;
  id?: string;         // optional event id for dedupe
}
```

`useWebSocket` parses the envelope, updates `wsStore` liveness, and dispatches by
`type`. `WebSocketProvider` owns the `type → handler` map (store writes, toasts,
query invalidations).

### Events

```ts
// 1. service.status — a connector's reachability/health changed
interface ServiceStatusPayload {
  serviceId: string;
  status: 'online' | 'degraded' | 'offline' | 'unknown';
  message?: string;
  lastChecked: string; // ISO
}
// Consumers: ServiceStatusWidget, ServicesListPage, ServiceDetailPage, Sidebar/WsStatus(no)
// Reaction: patch ['connectors'] + ['connector', id] cache (setQueryData); StatusPill re-renders. Toast only on transition to offline/degraded.

// 2. sync.progress — progress of an in-flight sync job
interface SyncProgressPayload {
  serviceId: string | null; // null = global sync
  jobId: string;
  phase: 'queued' | 'fetching' | 'diffing' | 'generating' | 'done' | 'error';
  percent: number;          // 0–100
  message?: string;
}
// Consumers: SyncActivityWidget, SyncStatusBanner, FirstSyncProgress, ServiceDetailPage
// Reaction: update syncStore[jobId]. On phase 'error' → ErrorState/toast.

// 3. sync.complete — a sync finished (terminal, carries result summary)
interface SyncCompletePayload {
  serviceId: string | null;
  jobId: string;
  changesDetected: number;
  alertsRaised: number;
  durationMs: number;
}
// Consumers: WebSocketProvider (orchestration), SyncActivityWidget
// Reaction: clear syncStore[jobId]; invalidate ['connector',id,'data'], ['dashboard','overview'], ['changes']; toast "Sync complete — N changes".

// 4. change.detected — diff engine found an infrastructure change
interface ChangeDetectedPayload {
  changeId: string;
  serviceId: string;
  changeType: string;       // e.g. 'vm.created', 'firewall.rule.modified'
  severity: 'info' | 'warning' | 'critical';
  summary: string;
  willTriggerAi: boolean;   // diff engine routed to AI vs alert
}
// Consumers: RecentChangesWidget, ChangesFeedPage, ServiceDetailPage(history)
// Reaction: invalidate ['changes'] + ['dashboard','overview']; toast for warning/critical.

// 5. alert.created — diff engine raised an alert needing user action
interface AlertCreatedPayload {
  alertId: string;
  changeId?: string;
  serviceId: string;
  severity: 'info' | 'warning' | 'critical';
  title: string;
}
// Consumers: AlertBell, Sidebar AlertBadge, alertsStore, AlertCenterPage, AlertSummaryWidget
// Reaction: alertsStore.increment + prepend; invalidate ['alerts'], ['dashboard','overview']; toast by severity.

// 6. alert.resolved — an alert was resolved (by this or another user/session)
interface AlertResolvedPayload {
  alertId: string;
  resolvedBy: string;       // user id
  resolution: 'resolved' | 'dismissed' | 'snoozed';
}
// Consumers: alertsStore, AlertCenterPage, AlertDetailPage
// Reaction: alertsStore.decrement/remove; invalidate ['alerts'], ['alert',id], ['dashboard','overview']. Multi-user: keeps sessions in sync.

// 7. doc.generated — a doc node was (re)generated by the engine/AI
interface DocGeneratedPayload {
  docId: string;
  serviceId?: string;
  trigger: 'ai' | 'template' | 'manual';
  newVersion: number;
}
// Consumers: DocsPage, DocContent, DocHistoryPage, DocsHealthWidget
// Reaction: invalidate ['doc',docId], ['doc',docId,'versions'], ['docs','tree']. If user is editing that doc → non-destructive "newer version available" banner.

// 8. doc.ai_suggestion — streamed/affixed AI suggestion for the open editor
interface DocAiSuggestionPayload {
  docId: string;
  requestId: string;
  status: 'streaming' | 'complete' | 'error';
  contentDelta?: string;    // incremental text when streaming
  fullContent?: string;     // on complete
  error?: string;
}
// Consumers: AiAssistPanel (DocEditorPage) only
// Reaction: append delta to local AI staging buffer; on complete enable "accept/insert"; on error show inline error. Only active when AI module enabled.

// 9. system.health — backend/integration health + diff-engine heartbeat
interface SystemHealthPayload {
  status: 'ok' | 'degraded' | 'down';
  components: { name: string; status: 'ok' | 'degraded' | 'down'; detail?: string }[];
}
// Consumers: SystemSettings HealthPanel, WsStatusDot (indirect)
// Reaction: setQueryData(['health']); degraded/down → admin toast + dot color.

// 10. system.notice — server-pushed broadcast (maintenance, forced logout, config change)
interface SystemNoticePayload {
  level: 'info' | 'warning' | 'critical';
  message: string;
  action?: 'reauth' | 'reload' | 'none';
}
// Consumers: Toaster (global), AppRoot
// Reaction: toast; if action 'reauth' → clear authStore + redirect /login; 'reload' → prompt refresh. (Covers admin changing auth/AI config affecting active sessions.)
```

**Reconnect behavior:** `useWebSocket` auto-reconnects with backoff, updates
`wsStore`. On reconnect, `WebSocketProvider` invalidates volatile queries
(`['alerts']`, `['changes']`, `['connectors']`, `['dashboard','overview']`) to
recover any events missed while disconnected.

---

## 5. Routing & Navigation (React Router v6)

```
<Routes>
  {/* Public */}
  <Route element={<PublicLayout/>}>
    <Route path="/login" element={<LoginPage/>}/>
    <Route path="/auth/callback" element={<OidcCallbackPage/>}/>
  </Route>

  {/* Protected — RequireAuth guard reads authStore; redirects to /login */}
  <Route element={<RequireAuth/>}>
    <Route element={<RequireOnboarded/>}>          {/* empty-state gate */}
      <Route element={<ProtectedLayout/>}>          {/* sidebar/topbar shell */}
        <Route index element={<Navigate to="/dashboard" replace/>}/>
        <Route path="dashboard" element={<DashboardPage/>}/>

        <Route path="services">
          <Route index element={<ServicesListPage/>}/>
          <Route path=":id" element={<ServiceDetailPage/>}>   {/* nested tabs */}
            <Route index element={<ServiceOverviewTab/>}/>
            <Route path="docs" element={<ServiceDocsTab/>}/>
            <Route path="history" element={<ServiceHistoryTab/>}/>
          </Route>
        </Route>

        <Route path="docs">
          <Route index element={<DocsPage/>}/>
          <Route path=":docId" element={<DocsPage/>}/>
          <Route path=":docId/edit" element={<RequireRole role="editor"><DocEditorPage/></RequireRole>}/>
          <Route path=":docId/history" element={<DocHistoryPage/>}/>
        </Route>

        <Route path="templates" element={<RequireRole role="editor"><Outlet/></RequireRole>}>
          <Route index element={<TemplatesPage/>}/>
          <Route path=":id" element={<TemplateEditorPage/>}/>
        </Route>

        <Route path="connectors" element={<RequireRole role="editor"><Outlet/></RequireRole>}>
          <Route index element={<ConnectorsPage/>}/>
          <Route path="new" element={<ConnectorFormPage/>}/>
          <Route path=":id/edit" element={<ConnectorFormPage/>}/>
        </Route>

        <Route path="changes">
          <Route index element={<ChangesFeedPage/>}/>
          <Route path=":id" element={<ChangeDetailPage/>}/>
        </Route>

        <Route path="alerts">
          <Route index element={<AlertCenterPage/>}/>
          <Route path=":id" element={<AlertDetailPage/>}/>
        </Route>

        <Route path="settings" element={<SettingsLayout/>}>
          <Route index element={<Navigate to="profile" replace/>}/>
          <Route path="profile" element={<ProfileSettings/>}/>
          <Route path="users" element={<RequireRole role="admin"><UsersSettings/></RequireRole>}/>
          <Route path="auth" element={<RequireRole role="admin"><AuthSettings/></RequireRole>}/>
          <Route path="ai" element={<RequireRole role="admin"><AiSettings/></RequireRole>}/>
          <Route path="notifications" element={<RequireRole role="admin"><NotificationSettings/></RequireRole>}/>
          <Route path="system" element={<RequireRole role="admin"><SystemSettings/></RequireRole>}/>
        </Route>

        <Route path="forbidden" element={<ForbiddenPage/>}/>
        <Route path="*" element={<NotFoundPage/>}/>
      </Route>
    </Route>

    {/* Onboarding lives inside auth but OUTSIDE RequireOnboarded to avoid loop */}
    <Route path="onboarding" element={<OnboardingPage/>}/>
  </Route>
</Routes>
```

**Guards & redirects:**
- `RequireAuth` — no valid token → `<Navigate to="/login" state={{from}}/>`.
- `RequireOnboarded` — authed but `GET /api/connectors` empty → redirect
  `/onboarding`. Once ≥1 connector exists → normal app. Onboarding route itself is
  excluded from this gate to prevent redirect loops.
- `RequireRole` — authed but role insufficient → `<Navigate to="/forbidden"/>`;
  navigation items for forbidden sections are hidden via `RoleGate` in the sidebar.
- Post-login: `/login` success → original `state.from` or `/dashboard` (which then
  may bounce to `/onboarding` if empty).
- OIDC: `/auth/callback` success → same post-login logic; failure → `/login` with error.

**Lazy-load boundaries (`React.lazy` + Suspense in `PageContainer`):** split the
heavy, infrequently-used routes — `DocEditorPage` (editor lib), `TemplateEditorPage`,
all of `settings/*`, `DiffViewer`-heavy `ChangeDetailPage`/`DocHistoryPage`, and the
dashboard grid lib. Login, dashboard shell, services, alerts stay in the main chunk
(hot paths).

---

## 6. Empty States, Error Boundaries & Loading

| Page/section                   | Empty state                                                                                                                              | Error state                                                                                                       | Loading                                               |
|--------------------------------|------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------|
| Dashboard                      | No connectors → full-page onboarding CTA (also handled by route gate). Connectors but no data yet → per-widget "waiting for first sync". | Overview fetch fail → page `ErrorState` with retry; per-widget failures isolated to `WidgetFrame` error boundary. | Skeleton widget grid.                                 |
| Services list                  | "No services connected" + Add connector CTA (editor+).                                                                                   | `ErrorState` + retry.                                                                                             | Table/card skeletons.                                 |
| Service detail                 | Connected but never synced → "Run first sync" prompt.                                                                                    | Not found → 404; data fail → inline `ErrorState`.                                                                 | Header skeleton + data placeholder.                   |
| Docs viewer                    | "No documentation generated yet" + (editor+) "Generate from template" / "Create doc".                                                    | Tree fail → `ErrorState`; node fail → inline.                                                                     | Tree + content skeleton.                              |
| Doc editor                     | New/empty doc → blank editor with template-insert hint.                                                                                  | Save fail → keep draft, toast error, no data loss; AI error → inline in `AiAssistPanel`.                          | Editor skeleton; AI "thinking" indicator (streaming). |
| Doc history                    | "Only one version" → hide diff, show single revision.                                                                                    | `ErrorState`.                                                                                                     | Version list skeleton.                                |
| Templates                      | "No templates — start from default" CTA.                                                                                                 | `ErrorState`.                                                                                                     | Card skeleton.                                        |
| Connectors                     | "No connectors" → category picker CTA.                                                                                                   | List fail `ErrorState`; per-connector test errors inline on row.                                                  | Row skeletons.                                        |
| Connector form                 | —                                                                                                                                        | Test-connection failure shown inline with backend message; save validation inline.                                | Inline button spinner on test/save.                   |
| Changes feed                   | "No changes detected yet" (explain diff engine runs on sync).                                                                            | `ErrorState` + retry.                                                                                             | Feed skeleton.                                        |
| Change detail                  | —                                                                                                                                        | 404 / fetch fail `ErrorState`.                                                                                    | Diff skeleton.                                        |
| Alert center                   | "No pending alerts — all clear".                                                                                                         | `ErrorState`.                                                                                                     | List skeleton.                                        |
| Settings/users                 | "Only you so far" + invite CTA.                                                                                                          | `ErrorState`.                                                                                                     | Table skeleton.                                       |
| Settings/auth,ai,notifications | AI: "AI module disabled" toggle-on prompt. Notifications: "No channels configured".                                                      | Config fetch/save fail → `ErrorState`/inline; test-send shows result inline.                                      | Form skeleton.                                        |
| System                         | —                                                                                                                                        | Health fail → `down` indicator.                                                                                   | Panel skeleton.                                       |

**Loading strategy overall:** **skeletons** for list/detail/structured content
(matches layout, avoids spinner jank); **inline button spinners** for mutations
(test, save, sync trigger); **optimistic UI** for low-risk mutations with clear
rollback — alert resolve/dismiss (decrement `alertsStore`, rollback on error),
dashboard widget add/remove/move (apply locally, persist after), doc save shows
optimistic "saved" then reconciles version. Destructive/credential ops
(connector remove, user delete) are **not** optimistic — confirm dialog + spinner +
server confirmation.

**Error boundaries:** one app-level boundary in `AppRoot` (catastrophic),
one per-page boundary in `PageContainer` (isolates page crashes, keeps shell alive),
and granular boundaries around each `WidgetFrame` and the `RichMarkdownEditor`
(a third-party editor lib shouldn't take down the page). WS failure never throws —
it degrades to `wsStore.status='closed'`, shows a reconnecting indicator, and the UI
falls back to React Query polling/refetch for freshness.

---

## 7. First-Run / Onboarding Flow

**Trigger:** authed user, `GET /api/connectors` returns empty → `RequireOnboarded`
redirects to `/onboarding`. (Admins and editors can complete it; a fresh install's
first user is admin.)

**What the user sees on first login:** a focused, non-modal onboarding page (app
shell minimized to reduce noise) with a short stepper — not a wall of docs.

**Steps:**
1. **Welcome** — one line on what WiseLabz does + "Connect your first service".
   Skippable explainer; experienced users can jump straight to step 2.
2. **Add first connector** — `CategoryPicker` (Virtualization / Containers·PaaS /
   Networking) → type within category → `SchemaForm` credentials → **Test connection**
   (must pass to continue, but allow "save anyway"). Reuses the exact connector
   components from `/connectors/new` — no throwaway onboarding-only forms.
3. **First sync** — auto-trigger a sync; `FirstSyncProgress` consumes `sync.progress`
   /`sync.complete`. Shows what the engine found (services, initial doc generated).
4. **Done** — summary ("Connected Proxmox · 1 doc generated · dashboard ready") with
   primary CTA to `/dashboard` and secondary "Add another connector".

**Completion definition:** onboarding is complete as soon as **≥1 connector exists**
(the `RequireOnboarded` gate condition). The first sync and "done" screen are guidance,
not gates — a user can leave after the connector saves and the app is fully usable.
Subsequent logins skip onboarding entirely.

**Not patronizing experienced users:** stepper is linear but every step has a "skip"
/ "do this later"; the connector form is the real form (no dumbed-down wrapper);
no forced tour, tooltips, or modal gating after completion; the only hard gate is
the genuine one (the app is useless with zero connectors). A persistent "Add
connector" affordance remains everywhere so power users never feel funneled.

---

## 8. Resolved Decisions (formerly open questions)

All questions raised during planning have been answered by the team. These are
**locked** and bind the implementation.

1. **Roles — 3 fixed roles (admin / editor / viewer).** Guards wired around these
   three from day one. **admin** = settings/users/auth/AI/notifications;
   **editor** = connectors/templates/docs/sync; **viewer** = read dashboard/docs/
   changes/alerts. Drives every `RequireRole`/`RoleGate` boundary in §1/§5.

2. **`DiffViewer` — unified component with a `formatter` prop.** One component serves
   both infra-state diffs (changelog) and doc-version diffs (history). **Caveat:**
   validate that infra-diff and doc-diff payload shapes are compatible before building;
   if they diverge significantly, revisit (potential split).

3. **Concurrent doc editing — last-write-wins + "newer version available" banner.**
   On `doc.generated` mid-edit, show a non-destructive banner; saving creates a new
   version. Soft lock ("X is editing") is **deferred to v2**.

4. **AI suggestion transport — WebSocket streaming over `/ws`** (`doc.ai_suggestion`
   event, §4). Keeps a single live channel. A dedicated SSE endpoint is a **v2**
   candidate if multiplexing `/ws` becomes too complex.

5. **Dashboard layout persistence — per-user** via `GET /api/dashboard/layout` and
   `PUT /api/dashboard/layout` (localStorage mirror for instant paint). Any role
   (including viewer) customizes its own dashboard. Per-user-with-admin-default is
   **deferred to v2**.

6. **Tooling — locked:**
   - **OpenAPI generator: `orval`** — generates typed React Query hooks directly from
     the backend OpenAPI schema into `src/api/`. This auto-produces most query keys in
     §3; the §3 key table becomes the naming contract orval config targets.
   - **Markdown editor: CodeMirror 6** — backs `RichMarkdownEditor` in `DocEditorPage`.
   - **Dashboard grid: `react-grid-layout`** — backs `WidgetGrid`.

7. **Token storage & refresh — in-memory access token + httpOnly refresh cookie.**
   Backend sets the refresh cookie `HttpOnly; Secure; SameSite=Strict`. The axios
   interceptor performs **silent refresh** transparently on 401; re-auth UI
   (`system.notice` → `reauth`, §4) is shown **only** when the refresh token itself is
   expired/revoked. `authStore` holds the access token in memory only (never
   localStorage).

8. **Notification routing — full event-type × channel matrix in v1.**
   `NotificationSettings` exposes the complete routing table from the start (each
   event type × each channel = in-app / SMTP / webhook-chat). Schema must be
   extensible as new event types/channels are added. `EventRoutingTable` renders this
   matrix; severity is a per-cell/per-row threshold within it.

9. **i18n — `react-i18next` from day one.** All user-facing strings go through the
   `t()` layer. English is the only v1 locale, but the infrastructure is in place so
   community translations land without a retrofit. Affects every component with copy
   (empty/error states in §6, onboarding copy in §7, nav labels).

---

## v2 Backlog

The following are explicitly deferred. **Implementation deliverable:** create
`docs/v2-features.md` containing this table (the file does not yet exist).

| Feature                                             | Context                                                                                                                                   |
|-----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| Soft lock on doc editing ("X is currently editing") | Prevents conflicts when multiple editors open the same doc simultaneously. Replaces the v1 last-write-wins approach (decision §8.3).      |
| SSE endpoint for AI suggestions                     | Alternative to WS streaming if the `/ws` channel becomes too complex to multiplex. Evaluate after v1 AI module is stable (decision §8.4). |
| Per-user layout with admin-defined default          | Extends v1 per-user layout persistence. Admin sets a default that users can override (decision §8.5).                                     |

---

## Implementation Notes / Verification

This is a planning blueprint — no app code is produced here. When implementation
begins, the foundational order is:

1. Create `docs/v2-features.md` with the v2 backlog table above.
2. Install deps (locked, §8): `react-router-dom`, `orval`, `codemirror`/CM6 packages,
   `react-grid-layout`, `react-i18next` + `i18next`.
3. Configure orval against the backend OpenAPI schema → generate typed React Query
   hooks into `src/api/` (query-key naming aligned to §3); stand up `QueryClient`.
   Wire i18next provider so `t()` is available before any page copy is written.
4. Build `useWebSocket` + `WebSocketProvider` against the §4 contract (mockable).
5. Auth: `authStore` (in-memory access token), axios silent-refresh interceptor
   (§8.7), `RequireAuth`/`RequireRole`, login/OIDC, `system.notice→reauth` handling.
6. App shell (`ProtectedLayout`, sidebar/topbar) + routing tree (§5).
7. Pages in dependency order: onboarding → connectors → dashboard → services →
   docs/editor → changes → alerts → settings.

**Verification approach (per slice):**
- Type-safety/lint: `cd web && npm run lint && npm run build` (tsc) must pass.
- Auth: confirm access token lives only in memory (not localStorage), and a 401
  triggers a single silent refresh before any re-auth UI appears.
- WS contract: stand up a mock `/ws` emitting each §4 event; assert each consumer
  reacts (store update / toast / query invalidation) as specified.
- Route guards: test unauthenticated → `/login`, role-denied → `/forbidden`,
  empty-connectors → `/onboarding`, and the no-loop onboarding exclusion.
- Empty/error/loading: force each state (no data, 500, slow) via mocked API and
  confirm the §6 behavior renders.
- End-to-end smoke: fresh install → onboarding → add connector → first sync (mock WS)
  → land on dashboard → see generated doc in `/docs` tree.
