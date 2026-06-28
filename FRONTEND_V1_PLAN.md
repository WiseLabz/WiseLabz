# WiseLabz Frontend — V1 Build Plan

## Context

WiseLabz auto-generates and maintains documentation for a homelab: connects to each
service's API, renders docs from templates, runs a diff engine that surfaces drift,
and drafts AI-assisted updates or raises alerts. Docs are the center of gravity; the
dashboard is a command surface that routes into them.

A frontend base already exists (~9 mocked routes, full OKLCH theme engine, MSW +
MockWebSocket mock layer, bottom-dock shell, DiffViewer, custom Markdown, stores).
This plan closes the gap from that base to a **complete V1 frontend**, built
**mock-first** (no live backend — backend comes later). Auth is built as
**frontend-only against mocks**. Identity, motion, and a11y are locked in `PRODUCT.md`
and must be honored throughout.

**Scope confirmed with user (this session):** full doc editor, AI suggestions on two
surfaces, rich service-detail, templates CRUD, full settings hub incl. accessibility,
full onboarding, full command palette, full destructive-op protection, i18n wired now,
desktop-first responsive. Nothing was deferred except the items seeded into
`docs/MISSING.md` (below).

**Out of plan scope:** dependency installation (user runs it — see commands already
provided), and any backend work.

---

## Design rigor (applies to every phase)

Non-negotiable, from `PRODUCT.md` + impeccable **product** register:

- **Identity:** Blueprint palette + Geist, soft-dark chrome. Signal-orange on
  cool-slate, radii 6–16px, layered depth shadows. The locked default is the brand —
  the theme engine is a power surface, not a playground.
- **Shell:** single bottom dock (floating, solid, centered; amber pill under active).
  No shell switching. Already built — reuse `components/shell/*`.
- **Status is never color alone:** every state = dot/shape + word + color. Reuse
  `StatusDot` + label. Keep ok/warn/err/idle distinct from signal.
- **Motion (4 earned moments only):** diff reveal, sync pulse/heartbeat, drift
  surfacing, row-resolve on accept/reject. No page-load choreography. Every animation
  needs a reduced-motion alternative; motion is a **user setting**, not OS-forced.
- **A11y target WCAG 2.2 AA:** body ≥4.5:1, large ≥3:1, placeholders held to 4.5:1,
  visible non-color focus rings everywhere.
- **Anti-slop:** no big-number hero wall, no identical card grids, no per-section
  uppercase eyebrows, no gradient text, no side-stripe borders, no over-rounding.
  Density only where the user works (rosters, diffs, tables); quiet elsewhere.
- **Responsive:** desktop-first, mobile-tolerable down to ~768px (not phone-designed).
- **i18n:** every user-facing string through `t()` (react-i18next), English-only
  catalog for V1.
- **Mock-first:** every new entity gets MSW handlers + fixtures; every live behavior
  gets a MockWebSocket timeline event. Verify in-browser against mocks.

---

## Current base (reuse, do not rebuild)

| Area | Path | State |
|---|---|---|
| Shell + bottom dock | `web/src/components/shell/*` | done |
| Theme engine (OKLCH, 7 presets, 4 fonts) | `web/src/theme.ts`, `store/theme.ts` | done |
| UI primitives (Button, Panel, states, StatusDot) | `web/src/components/ui/*` | done |
| DiffViewer + diff engine | `components/diff/`, `lib/{docdiffmodel,linediff,worddiff}.ts` | done |
| Markdown renderer, DocTree | `components/docs/*` | done |
| Mock layer (MSW + MockWebSocket + fixtures) | `web/src/mocks/*`, `data/fixtures.ts` | done |
| WS provider + live store + ws types | `ws/`, `store/live.ts`, `types/ws.ts` | done |
| Pages: dashboard, services, services/new, docs, changes(+detail), alerts, settings | `web/src/features/*` | mocked, unaudited |
| `useRole` / `useCanMutate` | `web/src/hooks/useRole.ts` | done |
| Orval-generated API clients | `web/src/api/generated/*` | done |

**Absent today (the V1 gap):** any auth/login, route guards, onboarding, command-palette
actions, service detail, connector edit, doc editor, templates, settings sub-pages,
toast layer, i18n init, DESIGN.md.

---

## Phases (dependency order)

### Phase 0 — Foundations & design baseline
Goal: shared infra + a written design contract before building features.

- **`docs/DESIGN.md`** — codify the existing visual system (impeccable `document`):
  tokens, type scale, radii, shadows, motion curves, status grammar. This is the
  reference every later phase aligns to.
- **`docs/MISSING.md`** — create with the deferred-feature table (below).
- **i18n init** — `web/src/i18n/index.ts`, provider in `main.tsx`, `en` catalog;
  establish `t()` namespace convention. All new strings go through it.
- **Toast layer** — mount `sonner` `<Toaster/>` in the shell, themed to tokens;
  thin `lib/toast.ts` wrapper so call sites are token-/i18n-consistent.
- **Shared primitives gap-fill** (verify, build only if missing): `Modal`/`Dialog`
  (native `<dialog>` or portal — never absolute-in-overflow), `ConfirmDialog`,
  `Pagination`, `SeverityTag`, `TimeAgo` (wrap `lib/time.ts`), `RoleGate` (wrap
  `useCanMutate`). `ConfirmDestructive` + `StepUp` already exist — extend, don't dupe.
- **Audit baseline** — impeccable `critique` of the existing pages; record P0/P1 to
  feed Phase 2. (Signals show project never critiqued.)

Verify: `cd web && npm run lint && npm run build` clean; toast fires; `t()` renders.

### Phase 1 — Auth frontend (mock-backed)
Goal: real auth UX, zero backend. Token never leaves memory.

- **`store/auth.ts`** — in-memory access token + current user (id, name, role);
  `login/logout`, OIDC provider list. Never localStorage.
- **Mock auth** — extend `mocks/handlers.ts`: `/auth/providers`, `/auth/login`,
  `/auth/oidc/callback`, `/auth/refresh`, `/auth/logout`, `/auth/elevate`; `/me`.
  Seed a fixture operator user + one OIDC provider.
- **Axios silent-refresh interceptor** — on 401, one silent refresh, then re-auth UI
  only if refresh itself fails (drive via `system.notice→reauth` mock event).
- **`features/auth/`** — `PublicLayout` (centered, off-shell), `LoginPage` (local
  user/pass + OIDC provider buttons), `AuthCallbackPage`.
- **Guards** — `RequireAuth` (→ `/login`), `RequireRole` (→ `/forbidden`),
  `RequireOnboarded` (zero-connectors → `/onboarding`, no loop). `ForbiddenPage` 403.
- **Step-up modal** — password/TOTP form (mock-validated), issues scoped elevation
  token held in `auth` store; consumed by destructive ops (Phase 5).
- **Wire `App.tsx`** — split public vs protected route trees under the guards.

Verify: unauth → `/login`; bad role → `/forbidden`; token in memory only; 401 →
single silent refresh before any re-auth UI.

### Phase 2 — Polish existing pages
Goal: bring the 9 mocked pages to the Phase-0 DESIGN.md bar before extending them.

- Apply Phase-0 critique P0/P1: contrast, focus rings, status-as-shape+word, motion
  alternatives, slop removal, spacing rhythm.
- Route all existing strings through `t()`.
- **Topbar search → opens the command palette** (dedupe the two search affordances;
  one entry point).
- Confirm desktop-first reflow holds to ~768px on dense surfaces.

Verify: impeccable `polish` re-run shows P0/P1 cleared; lint/build clean.

### Phase 3 — Command palette (full: nav + actions)
Goal: keyboard-first control surface (brand principle #5).

- Extend `components/command/CommandPalette.tsx`: fuzzy nav (pages, services, docs,
  changes) + **actions** (trigger global/per-service sync, toggle connector, switch
  theme, jump to settings). Actions respect `useCanMutate`.
- Registry pattern so later phases register their own commands.

Verify: ⌘K opens; nav jumps; each action invokes its mock + reflects via toast/feed.

### Phase 4 — Onboarding (full stepper)
Goal: first-run flow gated on zero connectors.

- **`features/onboarding/`** — stepper: welcome → add first connector (reuses the
  Phase-5 schema form) → trigger first sync (MockWebSocket timeline drives progress)
  → done → dashboard. Skippable, not patronizing. `RequireOnboarded` routes into it.

Verify: fresh mock state (no connectors) → onboarding; completing it → dashboard;
returning users never see it.

### Phase 5 — Services detail + connectors + destructive ops
Goal: the "live picture + operate on it" half.

- **`features/services/ServiceDetailPage` (`/services/:id`)** with all four panels:
  live raw-data snapshot viewer (status + last sync), linked doc + service-scoped
  change history, inline connector controls (sync/enable/disable/edit/remove),
  per-service activity timeline (from `live` store + mock).
- **Connector edit** (`/connectors/:id/edit`) — reuse `AddConnectorPage` schema form.
- **Destructive remove — full protection:** blast-radius (mock
  `/connectors/:id/removal-impact` → concrete dependents) → `StepUp` re-auth →
  type-connector-name-to-confirm → row resolves with motion (moment #4).
- Mock: `removal-impact`, `connectors/:id/data` (raw snapshot), service-scoped changes.

Verify: detail renders all panels live; remove flow blocks without correct
blast-radius confirm + step-up token.

### Phase 6 — Docs editor + AI suggestions
Goal: docs become editable; AI assists without streaming infra.

- **`features/docs/DocEditorPage` (`/docs/:docId/edit`)** — CodeMirror 6
  (`@uiw/react-codemirror` + `@codemirror/lang-markdown`), live preview via existing
  `Markdown.tsx`, save → new version. Last-write-wins + "newer version available"
  banner. Operator-gated.
- **`/docs/:docId/history`** route — versions list + restore, reusing DiffViewer.
- **AI suggestion — two surfaces (no streaming):**
  1. *In-editor batched review-diff:* "Suggest update" → one mock request → spinner →
     full proposed revision rendered as a diff in a review panel → Accept (applies to
     editor, sets provenance marker) / Reject. Reuses DiffViewer.
  2. *Changes-feed entry:* structural AI drafts land in `/changes` as `kind: ai-draft`,
     reviewed via the existing accept/reject loop (PRODUCT decision #3).
- Provenance markers in rendered docs: drafted / human-confirmed / synced-raw.
- Mock: `/docs/:docId/ai-suggest`, `/changes/:id/ai-update`; AI-draft fixtures.

Verify: edit→save→version; suggest→review-diff→accept writes editor + marker;
structural draft appears in Changes and resolves on accept/reject.

### Phase 7 — Templates (full CRUD)
Goal: control how docs render. Operator-only.

- **`features/templates/`** — list, editor (fields/sections), assign to connector
  categories/types, preview-generate. Reuse schema-form + DiffViewer (preview).
- Mock: `/templates` CRUD + `/templates/:id/preview`; template fixtures.

Verify: create/edit/delete persist in mock; preview renders a generated doc.

### Phase 8 — Settings hub (all sub-pages)
Goal: replace single SettingsPage with a nested hub.

- **`/settings/profile`** — profile, change password, active sessions list + revoke.
- **`/settings/users`** — operator-only user CRUD, roles, invite/reset.
- **`/settings/auth`** — provider enable/disable toggles + token TTLs; OIDC provider
  metadata **read-only** (never the secret; `secretConfigured` flag only).
- **`/settings/ai`** — AI provider config + test.
- **`/settings/notifications`** — full event-type × channel routing matrix
  (`EventRoutingTable`), per-cell severity threshold.
- **`/settings/system`** — instance health, WS status, sync schedule, version.
- **`/settings/appearance`** (accessibility hub) — Motion (Full/Reduced/Off; OS
  seeds, never forces), contrast boost, text-size/density, reduce-transparency,
  focus-ring style; keeps the existing theme presets/fonts/OKLCH surface.
- Mock + fixtures for users, sessions, auth/ai/notifications config, system info.

Verify: each sub-page reads/writes its mock; role-gating hides operator pages from
viewer; appearance controls take effect live (motion off kills animation).

### Phase 9 — Cross-cutting finish
Goal: live wiring + final quality sweep.

- **Live notifications:** WS events surface via **toasts** (sonner, click-to-jump),
  the **activity feed** (existing `live` store — as dashboard widget + Topbar
  dropdown), and **dashboard motion** (sync pulse, row-resolve). No separate
  notification-center bell (deferred — see MISSING.md).
- **Motion pass:** confirm the 4 earned moments only; reduced-motion alternatives
  everywhere; honor the appearance setting.
- **A11y sweep:** impeccable `audit` for AA contrast, focus, keyboard paths.
- **i18n completeness:** no hardcoded strings remain.
- **Final polish:** impeccable `polish` across the app.

Verify: full smoke (below) passes; `audit` clean; lint/build clean.

---

## `docs/MISSING.md` seed (deferred / future)

Create in Phase 0; capture anything raised later here instead of expanding V1:

- Phone-grade responsive (<768px) for dense surfaces.
- Topbar notification-center bell/dropdown (this round chose toasts + feed + motion).
- Soft-lock on concurrent doc editing ("X is editing") — replaces v1 last-write-wins.
- SSE endpoint for AI (if WS multiplexing gets complex) — N/A while mock-only.
- Per-user dashboard layout with admin-defined default.
- AI-suggestion token *streaming* (intentionally dropped for batched review-diff).
- Lab-mutating manager ops (service start/stop/restart, config push) — gated on
  permission/confirmation model.

---

## Mock-first conventions (all phases)

- New REST → add to `mocks/handlers.ts`, back with curated `data/fixtures.ts` (not
  faker) for believable, deterministic data.
- New live behavior → add a `mocks/ws/timeline.ts` event matching `types/ws.ts`;
  extend `WsEventType` if a new event is needed.
- Keep orval-generated types as the contract so a later live-backend swap is config-only.

## End-to-end verification (V1 done)

1. `cd web && npm run lint && npm run build` — clean (tsc + lint).
2. Fresh mock state → `/login` → log in (local + OIDC mock) → onboarding stepper →
   add connector → first sync (mock WS progress) → land on dashboard.
3. Dashboard: live sync pulse + activity feed + toast on sync complete.
4. Service detail: raw data, linked doc, history, activity; remove → blast-radius +
   step-up + type-to-confirm; row resolves with motion.
5. Docs: edit in CodeMirror → save version; "Suggest update" → review-diff → accept
   (provenance marker); structural draft appears in Changes, resolves on accept/reject.
6. Templates: create → preview → assign.
7. Settings: each sub-page reads/writes mock; appearance Motion=Off kills animation;
   viewer role can't see operator pages.
8. impeccable `audit` (AA contrast, focus, keyboard) + `polish` clean.
