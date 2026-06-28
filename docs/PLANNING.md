# Frontend planning — handoff

Pre-planning refinement is done. This file is the durable record of what got
**locked** and what still needs a **planning decision**. Read alongside
`PRODUCT.md` (product/UX decisions) and `docs/ARCHITECTURE.md` (technical
decisions).

## Locked this session (frontend direction)

These are decided. Planning builds on them; don't relitigate unless a topic below
forces it.

1. **Navigation = bottom dock.** Floating, centered, soft-dark (solid, not glass).
   One amber pill slides under the active item (motion `layoutId`). Sidebar
   variant was built and rejected in comparison. Implemented:
   `web/src/components/shell/{ShellDock,Dock}.tsx`.
2. **Identity = Blueprint palette + Geist font, soft-dark chrome.** Signal-orange
   on cool-slate; rounded-but-tight radii (6–16px); real layered depth shadows.
   Replaces the original sharp/flat Blueprint chrome. Default set in
   `web/src/theme.ts` (`ACTIVE = { palette: 'blueprint', font: 'geist' }`);
   first-paint fallback in `web/src/index.css`.
3. **Theme engine stays behind the Settings → Theme surface** (presets / fonts /
   OKLCH sliders) for dev exploration; ships defaulting to the locked combo.
4. **Status colors stay distinct** (green/lemon/red), never collapsed into the
   accent — machine-honest status (dot + word).
5. Prior product decisions in `PRODUCT.md` (Docs-first IA, narrow manager scope,
   hybrid AI drafts, meaning-bound motion) still hold.

## Plan-session topics

### 1. Changes / diff view → JetBrains-IDE git-diff style  *(main new topic)*

Goal: a Change shouldn't render as today's stitched before/after hunks. It should
read like a JetBrains IDE git diff — **whole document with context**, add/remove
lines highlighted, ideally word-level intra-line highlighting.

What already exists: `web/src/components/diff/DiffViewer.tsx` + `lib/linediff.ts`
do a **unified LCS line diff** (full lines, `+`/`−` gutter, both line-number
columns, green-add/red-del tint). ~40% there.

**Blocker = data contract, not UI.** Today `Diff` in `docs/openapi.yaml` is
`hunks[]` (per-hunk before/after). JetBrains-style needs the **full both
revisions** (or hunks + surrounding context lines). Changing this touches:
`openapi.yaml` → orval regen → MSW mocks → diff engine (backend).

Decisions to make:
- Side-by-side two-pane (JetBrains default) vs unified, and which is default.
  Side-by-side wants width (fine on dock full-width, tight on mobile).
- Word-level intra-line diff: hand-rolled char-LCS vs a lib (`diff`,
  `diff-match-patch`).
- Backend sends full doc text vs context-windowed hunks (perf on large docs).
- Markdown render-aware diff vs raw-text diff.
- How this maps onto the accept/reject Changes loop (structural changes route
  through review per PRODUCT.md decision 3).

### 2. Page retune to the soft-dark register

Chrome (shells) is soft-dark; the **pages** (Dashboard, Services, Docs, Changes,
Alerts, Settings) still carry sharp-Blueprint component styling. Plan a pass to
bring them into the new register cohesively. Dashboard first (it's the command
surface). Components to revisit: `ui/Panel`, `ui/Button`, `WidgetFrame`, widgets,
DocTree, status tags.

### 3. Shell cleanup (decided, just needs doing)

- Remove `ShellSwitch` (temporary design-test toggle) and the orphaned
  `web/src/components/shell/Sidebar.tsx`.
- Fold locked decisions 1–2 above into `PRODUCT.md` (nav + identity) and
  `docs/ARCHITECTURE.md` (shell architecture + theme-flag).

### 4. Manager scope build-out (from existing open question)

`PRODUCT.md` leaves open "how far manager goes past v1." v1 = sync + connector
enable/disable + connector add/remove via UI, with role-based access + per-action
step-up (password/TOTP, disable-able in Settings) + confirm-with-blast-radius for
destructive ops (see `docs/ARCHITECTURE.md`). Plan the actual UI flows for these.

## Housekeeping

- `graphify update` currently **refuses** (node-count mismatch, 198 vs 278 in
  `graphify-out/graph.json`); needs `graphify rebuild .` to resync.
