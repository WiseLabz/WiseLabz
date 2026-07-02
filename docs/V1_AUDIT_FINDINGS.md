# V1 Pre-Release Audit — Contract Drift, Dead Actions, Dead Code

Generated 2026-07-02, this session. Three parallel audits (backend↔spec parity,
backend dead code, frontend dead code/hook-shape) run against the state of the
repo after the fixes listed in "Already fixed" below. Source of truth for all
shape/path comparisons is `docs/openapi.yaml` (orval generates the frontend
client from it).

Merged with `docs/V1_INTEGRATION.md` (deleted — its hard blockers #1-3 were
already done in PR #4/production-deployment and are just listed as done below
for history; its still-open items #4/#6/#7 live on as section E).

Use this as the seed for a fix-plan branch. Don't re-discover — re-verify and fix.

---

## Already fixed this session (don't redo)

- `GET /api/ws` — frontend never sent the `access_token` query param the
  backend requires → constant 401s, WS never connected.
  Fixed in `web/src/ws/WebSocketProvider.tsx`.
- `GET /api/connectors` — backend wrapped in `{data, pagination}`, spec wants
  bare `Connector[]`. Fixed in `backend/internal/api/connectors/handlers.go`.
- `GET /api/templates` — same bare-array fix, `backend/internal/api/templates/handlers.go`.
- `GET /api/users` — same bare-array fix, `backend/internal/api/auth/handlers.go` (`ListUsers`).
- `GET /api/me/sessions` — was `{"sessions": [...]}`, spec wants bare array.
  Uncaught crash here took down the *entire* Settings route (no error
  boundary around `SessionsSection`). Fixed in `backend/internal/api/auth/handlers.go`.
- `GET /api/alerts`, `GET /api/changes` — shared `httputil.WritePaginated`
  fixed at the root: now emits `{items, total, page, pageSize}` matching the
  `AlertPage`/`ChangePage` schemas (was `{data, pagination:{...totalPages}}`).
- Deleted dead `GET /api/docs` (flat list, no spec entry, no frontend
  caller — tree nav covered it) + orphaned `Store.ListDocs`. **Note:** user
  now wants this rebuilt properly as a real feature — see "New feature" below.
  Don't re-delete it.

## Already done before this session (from the old V1_INTEGRATION.md, verified, don't redo)

- **Admin/first-user seeding + bcrypt** — `backend/internal/store/store.go`
  seeds an operator from `WISELABZ_ADMIN_PASSWORD` via `auth.HashPassword` on
  first boot, idempotent (refuses to start rather than silently no-op'ing).
- **SPA embed** — `backend/internal/web/embed.go` (`//go:embed all:dist`),
  single binary serves API + app.
- **Docker/deploy artifacts** — root `Dockerfile`, `docker-compose.yml` +
  `docker-compose.sqlite.yml`, `deploy/config.example.yaml`,
  `deploy/setup-env.sh`, `deploy/wiselabz.service`, CI workflows
  (`ci.yml`, `codeql.yml`, `release.yml`) all exist and landed via
  `9b0c79d feat(deploy): production deployment packaging` +
  `e540046 fix(store): make PostgreSQL driver actually work`.

---

## Decisions (resolved — act on these directly, no re-litigating)

1. **Settings/AI/System paths → rename backend to match spec.** Spec is
   source of truth. Rename `/settings/auth` → `/auth/config`,
   `/settings/ai` → `/ai/config`, add `POST /ai/config/test`,
   `/notifications/config` (+ test), `/system/info`. Regenerate frontend
   client after.
2. **`internal/ai` package → wire it up, don't delete.** `docs/MISSING.md`
   already scopes batched (non-streaming) AI-suggest as V1; only streaming
   was deliberately deferred. Most plumbing already exists (Settings AI page
   config fields, encrypted API key storage, `GetDecryptedAPIKey` written but
   uncalled). Work: implement one real `ai.Provider` (OpenAI-compatible
   endpoint covers most cloud + self-hosted options), route
   `docs.AISuggest` through `GetDecryptedAPIKey` → `Registry.Get(provider).Suggest(...)`
   instead of its current hardcoded stub string.
3. **Docs list+search → open to all authenticated users, not admin-only.**
   Every existing docs *read* route (`/docs/tree`, `/docs/{id}`,
   `/docs/{id}/versions`) is already open to viewers; only mutations
   (generate/save/restore/ai-suggest) are operator-gated. A flat searchable
   list is read-only and reaches the same docs a viewer can already browse to
   via tree — gating search-only behind operator would be an arbitrary UX
   inconsistency with no security benefit. Same permission tier as existing
   doc reads.

---

## A. Actions that silently do nothing (P0 — worst class: looks fine, does nothing)

| Where                                                             | What's wrong                                                                                                                                                                                                                                                       |
|-------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `web/src/features/alerts/AlertsPage.tsx:64-72`                    | Resolve/Dismiss/Snooze buttons have no `onClick`, no mutation wired. Zero call sites anywhere for `postAlertsAlertIdResolve`/`Dismiss`/`Snooze`. Feature is decorative.                                                                                            |
| `backend/internal/api/connectors/handlers.go:346` (`SyncAll`)     | Route wired at `POST /api/sync`, but body is a stub comment — never calls `sync.Engine.RunSyncAll` (`internal/sync/engine.go:152`, itself otherwise uncalled). "Sync all" button does nothing server-side.                                                         |
| `backend/internal/api/docs/handlers.go:198` (`AISuggest`)         | Hardcoded stub string, doesn't touch `internal/ai` at all (see open decision #2).                                                                                                                                                                                  |
| `backend/internal/notifications/dispatcher.go:24` (`NotifyAlert`) | Constructed in `main.go:102`, passed to `runAlertExpirer`, but `main.go:203` takes it as `_ *notifications.Dispatcher` — explicitly discarded, never called. In-app notification-on-alert path is dead. `createInApp` (dispatcher.go:44) is transitively dead too. |
| `web/src/components/command/CommandPalette.tsx:18,81-122`         | "Services"/"docs" command groups built from `web/src/data/fixtures.ts` (static dev data), not `useGetConnectors()`/`useGetDocsTree()`. Always lists the same 6 fictional connectors in prod; acting on them will 404 against the real backend.                     |
| `web/src/features/services/ServiceDetailPage.tsx:24,78`           | `linkedDoc` computed from the same static fixture instead of calling `getDocsServiceConnectorId` (exists, `web/src/api/generated/docs/docs.ts:155`, never called). Real connectors not in the fixture always show "no linked doc."                                 |

---

## B. Backend ↔ spec mismatches (34 total from parity audit)

### B1. Route-level — spec promises an endpoint the backend doesn't implement at that path/method (breaks frontend, orval calls land nowhere)

1. `GET/PUT /ai/config`, `POST /ai/config/test` — backend has `/settings/ai` instead, no test-connection endpoint at all. (See open decision #1.)
2. `GET/PUT /notifications/config`, `POST /notifications/config/test` — not implemented anywhere.
3. `GET/PUT /auth/config` — backend has `/settings/auth` instead. (Open decision #1.)
4. `PUT /auth/providers/{providerId}/enabled` — no route/handler anywhere.
5. `GET /system/info` — only `Health`/`Version` exist in `system/handlers.go`.
6. `GET /connectors/{connectorId}/data` — `ServiceSnapshot` endpoint entirely missing.
7. `POST /connectors/{connectorId}/test` — backend has collection-level `POST /connectors/test` instead (tests arbitrary unsaved config, not an existing connector by id).
8. `GET /docs/service/{connectorId}` — no route/handler.
9. `GET /docs/{docId}/versions/{rev}` — only list-versions exists, not get-single-version.
10. `POST /docs/{docId}/versions/{rev}/restore` — backend has `POST /docs/{id}/restore` with `rev` in the JSON body instead of the URL path.
11. `POST /changes/{changeId}/ack` — backend registers `/acknowledge`. Trivial rename, still 404s.
12. `POST /changes/{changeId}/ai-update` — not implemented.
13. `PUT /templates/{templateId}` — backend only registers `PATCH`.
14. `POST /templates/{templateId}/preview` — backend has collection-level `POST /templates/preview` instead (id + connector id expected in body).

### B2. Response/request shape — route exists but body doesn't match schema (breaks frontend at render/decode time)

1. **Docs `id` vs `docId`** — `store.DocRecord` and the ad-hoc `TreeNode` in `docs/handlers.go:35-40` emit `"id"`; spec's `Doc`/`DocNode` require `"docId"`. Hits the whole docs module (`GET /docs/tree`, `GET/PUT /docs/{id}`, `POST /docs/{id}/restore`). **Fix this while rebuilding the docs-list feature.**
2. `GET /dashboard/overview` — backend returns `{connectors, alertsPending, changesNew, docsTotal, latestChanges, lastSyncAt}`; spec's `DashboardOverview` wants `{statusCounts, pendingAlerts, recentChanges, lastSyncAt}`. Almost no field overlap. **Needs a live re-check** — the dashboard rendered counts fine in the user's log, so something may already reconcile this; verify before assuming it's fully broken.
3. `GET/PATCH /templates/{id}` — returns `{"template": t, "sections": sections}`; spec's `Template` is flat with `sections` as a direct property.
4. **Template `ord` vs `order`** — `store.TemplateSectionRecord` and the request-decode structs (`templates/handlers.go:70,115`) use `json:"ord"`; spec requires `"order"`. Wrong in both request and response.
5. `GET /connectors/{id}/removal-impact` — returns `{snapshots, docs, changes, alerts, hasData}`; spec's `RemovalImpact` wants `{trackedServices, docSections, snapshots, items}`. Only `snapshots` overlaps.
6. `GET /connectors/schema` — returns `{"schemas": [...]}`; spec wants a bare array.
7. `POST /connectors/{id}/sync` — returns `{message, connectorId}`, missing required `jobId` from `SyncJobRef`.
8. `GET /docs/{docId}/versions` — returns `{"versions": versions}`; spec wants a bare array.
9. `POST /alerts/{id}/resolve|dismiss|snooze` — all return `204`; spec wants `200` + full `Alert` body. (Moot until action A's buttons are wired, but blocks it working end-to-end once they are.)
10. `POST /alerts/{id}/snooze` request body — handler expects `{durationMinutes: int}`; spec wants `{until: date-time}`. Wrong both directions.
11. `POST /changes/{id}/acknowledge|dismiss` — return `204`; spec wants `200` + `ChangeDetail`.
12. `POST /auth/elevate` — returns `{"elevationToken": ...}`; spec's `ElevationToken` requires key `"token"`. Breaks step-up auth before destructive actions.
13. `GET /api/health` — returns `{status, healthy}`; spec wants `{status, components[]}`.
14. `GET /me/sessions` — `sanitizeSessions` omits the required `current` boolean from spec's `Session` schema.
15. `PUT /dashboard/layout` — returns `204`; spec wants `200` + `DashboardLayout` body.
16. Settings `accessTokenTTL`/`refreshTokenTTL` casing vs spec's `accessTokenTtl`/`refreshTokenTtl` — moot until #3/#1 is resolved, but will still be wrong once the path is fixed.
17. `GET /api/version`, `POST /api/docs/generate` — registered in router with no spec entry (dead/undocumented, low severity — orval simply never generates a client for these).
18. `POST /auth/oidc/callback` — spec's `OidcCallbackRequest` requires `state` (CSRF), handler never reads it; handler also accepts an undocumented `redirectUri`. Spec-drift plus a possible CSRF gap — flag for security review, not just a shape fix.
19. `GET /dashboard/layout` — includes an extra undocumented `userId` field. Harmless, spec-drift-only.
20. Alerts/Changes pages never paginate — `AlertsPage`/`ChangesPage` fetch `{items,total,page,pageSize}` but only ever render `items` in full; `total`/`page`/`pageSize` are never read. Silent truncation once a dataset exceeds one page. (`web/src/components/ui/Pagination.tsx` exists, fully built, and is never imported — this is its intended use.)

---

## C. Dead code (mechanical cleanup, low risk — do after B is triaged)

### Backend

- `backend/internal/store/sqlc/` — entire directory (12 `.sql` files, no generated `.go`, no `sqlc.yaml` anywhere in repo, nothing imports it). Abandoned codegen scaffold. Safe to delete.
- `backend/internal/ai/` — entire package, never imported anywhere (see open decision #2).
- `backend/internal/notifications/dispatcher.go` — `NotifyAlert` + `createInApp` (see section A).
- `backend/internal/sync/engine.go:152` — `RunSyncAll`, only caller is the dead `SyncAll` stub (section A).
- `backend/internal/doc/engine.go:133` — `GenerateFromSnapshot`, no caller (only `GenerateFromTemplate` is used).
- `backend/internal/ws/ws.go:135` — `Broadcast` (all-clients), no caller.
- `backend/internal/ws/ws.go:155` — `ClientCount`, no caller.
- `backend/internal/store/user.go:312` — `UpdateSessionLastSeen`, no caller anywhere including tests.
- `backend/internal/store/doc.go:126` — `DeleteDoc`, no caller (no delete route exists).
- `backend/internal/store/connector.go:268` — `GetSnapshotsByConnector`, no caller.
- `backend/internal/store/change.go:311` — `CountAlerts`, no caller (dashboard uses `CountAlertsPending`).
- `backend/internal/store/change.go:146` — `CountChanges`, no caller (dashboard uses `CountChangesNew`).
- `backend/internal/store/user.go:214` — `CountUsers`, no caller.
- `backend/internal/api/settings/handlers.go:226` — `GetDecryptedAPIKey`, no caller anywhere.

### Frontend

- `web/src/components/shell/Dock.tsx` — superseded nav component, `AppShell.tsx` only uses `ShellDock.tsx`. Zero references.
- `web/src/features/settings/SettingsPage.tsx` — one-line back-compat re-export shim, nothing imports `SettingsPage` anymore (all call sites use `SettingsLayout` directly).
- `web/src/components/ui/Pagination.tsx` — unused, but see B34: this is a real gap to fill, not just dead code to delete.
- `web/src/api/generated/alerts/alerts.ts:151` (`getAlertsAlertId`/`useGetAlertsAlertId`) — no alert-detail page/route exists.
- `web/src/api/generated/dashboard/dashboard.ts:147,239` (`getDashboardLayout`/`putDashboardLayout`) — dashboard layout is persisted only in local Zustand, never synced server-side. Confirms these two backend endpoints are currently unused from the frontend — decide whether to wire them up (cross-device layout sync) or drop them.
- `web/src/api/generated/changes/changes.ts:428` (`postChangesChangeIdAiUpdate`) — zero call sites; lower confidence, might be intended as a backend-only automatic trigger rather than a frontend action. Confirm before deleting.

---

## New feature (not a bug — user request)

**Docs list + search screen, open to all authenticated users** (see decision
#3). Rebuild `GET /api/docs` (deleted this session) as a real feature:
paginated flat list of all docs with search (likely by title), proper `docId`
field naming (fixes B15 as a side effect), spec entry (`DocPage` schema
matching the `AlertPage`/`ChangePage` pattern), regenerated frontend client,
and a real screen — route/nav placement still open (own nav item vs a view
toggle on the existing `/docs` tree page). Scoped as a medium/big feature, not
a quick add-on.

---

## E. Remaining integration & release checklist (merged from V1_INTEGRATION.md)

Known-good seams (still true, no action needed): frontend mocks gate behind
`VITE_USE_MOCKS` (`web/src/config/env.ts`, prod build drops them);
`web/src/api/axios-instance.ts` uses `baseURL: '/api'` + single silent-refresh
on 401; `web/vite.config.ts` dev-proxies `/api` and `/ws` (now `/api/ws`, see
"Already fixed") to `localhost:8080`; WS event names match both sides
(`service.status`, `sync.progress`, `sync.complete`, `change.detected`,
`alert.created`) per `web/src/types/ws.ts` vs `backend/internal/ws/ws.go` —
payload *field shapes* inside those frames are still unverified, worth a pass
once section B is fixed.

### E1. Run the app against the real backend, mocks off
Per the user's original report this session, this has now happened at least
once (local full-stack + Postgres) — and immediately surfaced the WS/roster/
alerts/settings bugs already fixed. Re-run once section B lands:

- [ ] `VITE_USE_MOCKS=false`, backend on `:8080`, walk the E2 smoke flow below.
- [ ] Confirm the silent-refresh interceptor fires correctly against a real 401.
- [ ] Confirm OIDC redirect/callback works against a real provider (or a mock IdP).
- [ ] Confirm step-up elevation token issuance + consumption on connector
      removal (blocked today by B26 — `elevationToken` vs `token` key mismatch).

### E2. End-to-end smoke against the real stack (mocks off)
1. `cd web && npm run lint && npm run build`; backend `go build`/`go test` clean.
2. Fresh DB → seeded admin → `/login` (local + OIDC) → onboarding → add connector
   → first sync (real WS progress) → dashboard. (Blocked today by A's dead
   `SyncAll` stub — first sync won't actually run.)
3. Dashboard: live sync pulse + activity feed + toast on sync complete.
4. Service detail: raw data, linked doc, history; remove → blast-radius +
   step-up + type-to-confirm; row resolves with motion. (Linked-doc blocked
   today by the fixture-data bug in section A.)
5. Docs: edit → save version; AI suggest → review-diff → accept (provenance);
   structural draft → Changes → resolves on accept/reject. (AI suggest is a
   hardcoded stub today, see A/open-decision #2.)
6. Templates: create → preview → assign. (Blocked today by B17/B18 —
   `ord`/`order` mismatch + wrapped response.)
7. Settings: each sub-page reads/writes; appearance Motion=Off kills
   animation; viewer role can't see operator pages. (AI/Notifications/System
   sub-pages blocked today by open decision #1.)
8. Single built binary (or `docker compose up`) serves all of the above —
   already verified working (see "Already done before this session").

### E3. CI green
- [ ] Lint (Frontend), Lint (Backend), Test (Backend), CodeQL (go + js-ts) all pass.

---

## Audit methodology notes (for whoever picks this up)

- Backend parity audit read every path+method in `docs/openapi.yaml` against
  `backend/internal/api/router.go` and all `*/handlers.go`, both directions.
- Backend dead-code audit: `go build ./...` and `go vet ./...` clean baseline;
  `staticcheck -checks U1000` found nothing (it can't prove methods unused —
  only free functions/types), so dead *methods* above are from manual
  grep-based reference counting across all 65 `Store` methods, 50 `Handler`
  methods, and 36 methods across `ai`/`notifications`/`sync`/`ws`/`doc`.
  None of the findings above are test-only usages (none existed in this
  sweep — every method with test usage also had a real caller).
- Frontend audit: `npx tsc --noEmit` from `web/` is clean, confirming any
  remaining shape mismatches (there were none beyond what's already fixed)
  would've been hidden behind loose inference, not caught by the compiler —
  don't trust `tsc` alone for this class of bug going forward.
- All 4 connector implementations (`custom`, `docker`, `pfsense`, `proxmox`)
  are alive via `_`-import + `init()` registration in `router.go:31-34` —
  not flagged, included here only so it isn't re-checked.
