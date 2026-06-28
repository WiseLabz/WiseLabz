# WiseLabz — Design Contract

The single design contract every page aligns to. Documents the system **as it
already exists** in `web/src/index.css`, `web/src/theme.ts`, and
`web/src/components/ui/*`. Do not invent tokens. If a value isn't here, it isn't
part of the system — add it to the source files first, then document it here.

Sources of truth (in precedence order):
1. `web/src/theme.ts` — runtime theme engine. `ACTIVE = { palette: 'blueprint', font: 'geist' }`.
2. `web/src/index.css` — `@theme` first-paint fallback (mirrors the blueprint preset), z-index, focus, motion.
3. `web/src/components/ui/*` — primitive conventions.
4. `PRODUCT.md` — locked identity, principles, anti-references, a11y, motion.

> The `@theme` block in `index.css` is the first-paint fallback. At runtime the
> theme engine overrides `--color-*` and `--font-*` on `:root`. The OKLCH values
> below are the **active blueprint + geist** values.

---

## 1. Identity

Blueprint palette + Geist superfamily, in **soft-dark chrome**: a cool-slate
near-black canvas (hue 250) with layered surfaces and real depth shadows, lit by
a **single** signal-orange accent (hue 52). It reads as a **Swiss instrument
panel** — mono-forward, hairline grid over rounded-but-tight cards (6–16px),
restrained color, status as shape + word. Brand register is Linear/Vercel:
precise, technical, calm; machine-honest; dense only where the user works, quiet
everywhere else. The default *is* the brand — the theme engine (presets / fonts /
OKLCH sliders) is a power surface behind Settings, not a marketing playground.

---

## 2. Color tokens

OKLCH = `oklch(L C H)` — L 0..1 dark→light · C 0..~0.37 gray→vivid · H hue angle.
Values are the active **blueprint** preset (`neutralHue 250`, `signalHue 52`,
`canvasL 0.135`).

### Surfaces — depth from lightness steps + shadow, never borders alone
| Token | OKLCH | Use |
|---|---|---|
| `--color-canvas` | `0.135 0.008 250` | Page background. The floor. |
| `--color-canvas-sunken` | `0.108 0.008 250` | Recessed wells — inset areas below the page plane. |
| `--color-surface` | `0.163 0.009 250` | Default card / panel fill (`Panel`). |
| `--color-surface-raised` | `0.193 0.010 250` | Elevated elements — skeletons, empty-state icon chips, hover layers. |
| `--color-surface-overlay` | `0.185 0.010 250` | Floating UI — dropdowns, popovers, the dock. |

### Lines — hairlines, three weights
| Token | OKLCH | Use |
|---|---|---|
| `--color-line-soft` | `0.255 0.008 250` | Default card border, header dividers, blueprint grid. |
| `--color-line` | `0.315 0.008 250` | Standard separators that need to read. |
| `--color-line-strong` | `0.44 0.010 250` | Input borders, secondary-button border, scrollbar thumb. |

### Ink — text, three weights
| Token | OKLCH | Use |
|---|---|---|
| `--color-ink` | `0.975 0.003 250` | Primary text, body, headings. |
| `--color-ink-muted` | `0.73 0.007 250` | Secondary text, labels, captions. |
| `--color-ink-faint` | `0.55 0.008 250` | Tertiary — count suffixes, placeholders, disabled-ish meta. |

### Signal — the ONE brand accent (interaction / selection only)
Signal is the **only** chrome accent. Reach for it for: the active state, focus
rings, selection, the primary button, the dock pill, links, the one rare
highlight. **Never** use it as a status color.
| Token | OKLCH | Use |
|---|---|---|
| `--color-signal` | `0.80 0.135 52` | Primary fill, active icon, accent text. |
| `--color-signal-bright` | `0.87 0.125 52` | Hover/active on signal, focus-ring outline. |
| `--color-signal-ink` | `0.17 0.03 70` | Text/icon drawn **on** a signal fill. |
| `--color-signal-soft` | `0.46 0.084 52` | `::selection` bg, hover border hint. |
| `--color-signal-tint` | `0.27 0.043 52` | Faint signal wash (selected-row bg, signal severity tint). |

### Status — DATA markers, never chrome, never collapsed into signal
Four tones, each with `fg` + `tint`. These mark **state of the lab**, not UI
interaction. They stay distinct (green / lemon / red / gray) and are always
paired with a word (see §3). Warn (lemon, hue 96) is deliberately kept off
signal-orange (hue 52) so the two never blur.
| Token | OKLCH | Tone | Use |
|---|---|---|---|
| `--color-ok` / `--color-ok-tint` | `0.80 0.12 168` / `0.27 0.05 168` | ok (green) | online / healthy / accepted. |
| `--color-warn` / `--color-warn-tint` | `0.86 0.135 96` / `0.30 0.06 96` | warn (lemon) | degraded / warning / drift. |
| `--color-err` / `--color-err-tint` | `0.67 0.2 25` / `0.30 0.08 25` | err (red) | offline / critical / error / danger. |
| `--color-idle` / `--color-idle-tint` | `0.56 0.008 250` / `0.26 0.006 250` | idle (gray) | unknown / info / inert. |

**Rule:** signal ≠ status. Chrome uses signal. Data uses status. Never swap them.

---

## 3. Status grammar

> **Status is never color alone.** Every state reads through **shape + word +
> color** (square marker + mono label), so it survives color blindness,
> grayscale, and a glance. — `PRODUCT.md` Design Principle 3.

Implemented in `components/ui/StatusDot.tsx` + `status.ts`:
- **Marker** — a crisp `7px` (pill) / `6px` (tag) **square**, `backgroundColor: tone.fg`. No glow, no ping. Live status (`online`) gets `motion-safe:animate-pulse` only.
- **Label** — a mono word, colored `tone.fg`. Always present; the word is what survives grayscale.

Mappings (from `status.ts`):

| Service status | Label | Tone → color |
|---|---|---|
| `online` | `online` | ok (green), live pulse |
| `degraded` | `degraded` | warn (lemon) |
| `offline` | `offline` | err (red) |
| `unknown` | `unknown` | idle (gray) |

| Severity | Label | Tone → color |
|---|---|---|
| `info` | `info` | idle (gray) |
| `warning` | `warning` | warn (lemon) |
| `critical` | `critical` | err (red) |

`toneColor` is the only place tones resolve to CSS vars — `ok/warn/err/idle` map
to the status tokens; `signal` maps to `signal-bright/signal-tint` (brand-only).
New status surfaces import from `status.ts`; never hardcode a status color.

---

## 4. Typography

Geist superfamily. **Mono is the dominant voice of the interface** (status,
counts, timestamps, diffs, labels, buttons); **sans is for prose** (body copy,
descriptions). Body defaults to sans at `--text-base`.

| Family | Token | Stack | Use |
|---|---|---|---|
| Mono | `--font-mono` | `'Geist Mono Variable', ui-monospace, 'SF Mono', Menlo, monospace` | UI chrome, data, labels, buttons, headers. |
| Sans | `--font-sans` | `'Geist Variable', ui-sans-serif, system-ui, -apple-system, sans-serif` | Prose, descriptions, long-form. |

Feature settings:
- `.font-mono` → `font-feature-settings: 'zero' 1, 'ss02' 1` (slashed zero + stylistic set 02). Mono in the UI must carry these.
- `.nums` → `font-variant-numeric: tabular-nums; 'tnum' 1, 'zero' 1`. Use on any aligning numeric column (counts, metrics, timestamps).

Scale (fixed rem, instrument-large at the top end):

| Token | Size | Line-height | Typical use |
|---|---|---|---|
| `--text-2xs` | 0.6875rem | 1rem | Micro labels, panel-header eyebrow, severity tags. |
| `--text-xs` | 0.75rem | 1.15rem | Captions, meta, small buttons, status pills. |
| `--text-sm` | 0.8125rem | 1.3rem | Secondary body, md buttons. |
| `--text-base` | 0.875rem | 1.5rem | Body default. |
| `--text-lg` | 1.0625rem | 1.55rem | Lead text. |
| `--text-xl` | 1.375rem | 1.6rem | Sub-headings. |
| `--text-2xl` | 1.875rem | 2rem | Section titles. |
| `--text-3xl` | 2.75rem | 2.75rem | Page titles / display. |
| `--text-4xl` | 4rem | 3.9rem | Hero display (rare). |

Rules:
- **Line length** capped **65–75ch** for prose. Use a `max-w-*` that lands in range (existing states use `max-w-xs` for terse copy).
- **Display letter-spacing floor** `-0.04em` — do not track tighter than this on large/display text.
- Mono micro-labels (panel headers) use `tracking-[0.16em]` uppercase — this is the **only** sanctioned uppercase tracking, scoped to the `PanelHeader` eyebrow, not per-section.

---

## 5. Radii & shadows

### Radii — rounded but tight. Soft-dark, not pill-everything.
| Token | Value | When |
|---|---|---|
| `--radius-sm` | 6px | Buttons, inputs, small tags, focus-ring radius. |
| `--radius-md` | 8px | Inputs, skeletons, mid controls. |
| `--radius-lg` | 12px | **Cards / panels** (default `Panel`). |
| `--radius-xl` | 16px | Largest containers, icon chips, modals. |

Cards cap at **12–16px**. No pills on chrome, no over-rounding past `xl`.

### Shadows — layered depth is part of the language.
| Token | Value | When |
|---|---|---|
| `--shadow-panel` | `0 1px 2px oklch(0 0 0 / .28)` | Resting cards / panels. |
| `--shadow-raised` | `0 2px 6px /.3, 0 8px 24px /.28` | Hover-lifted / elevated elements. |
| `--shadow-pop` | `0 16px 48px /.55, 0 2px 8px /.4` | Popovers, dropdowns, modals, the dock. |

Depth comes from surface-step + shadow together — not from heavier borders.

---

## 6. Motion

Motion is a **core differentiator**, built in from the start — but **meaning-bound**.
Only **four moments earn animation**; everything else is plain state feedback. No
page-load choreography.

| # | Earned moment | Behavior |
|---|---|---|
| 1 | Diff reveal | Add/remove lines animate in when a Change opens. |
| 2 | Sync pulse / heartbeat | Live status (`online`) quiet pulse; sync activity heartbeat. |
| 3 | Drift surfacing | New drift entering the view animates to draw the eye. |
| 4 | Row resolve | A row resolving on accept (Acknowledge) / reject (Dismiss). |

Easing — **ease-out only, no bounce on chrome**:
| Token | Curve | Use |
|---|---|---|
| `--ease-out-quart` | `cubic-bezier(0.25, 1, 0.5, 1)` | General settle. |
| `--ease-out-expo` | `cubic-bezier(0.16, 1, 0.3, 1)` | Larger reveals, longer travel. |
| `--ease-snap` | `cubic-bezier(0.2, 0.8, 0.2, 1)` | Button/control feedback (150ms). |

Reduced motion:
- Motion is a **user setting** (Settings → Appearance: Full / Reduced / Off), applied via `[data-motion='off']` on the root, which clamps `animation-duration`/`transition-duration` to `0.001ms` and `scroll-behavior: auto`.
- OS `prefers-reduced-motion` **only seeds the initial value** — it never silently forces motion off. The user decides and can override. Default is full motion.
- Every animation needs a reduced-motion alternative (instant state, no loss of meaning). Live pulses use `motion-safe:` so they self-disable under reduced motion.

---

## 7. Z-index scale

Use the **semantic** scale on `:root`. Never an arbitrary `999`.
| Token | Value | Layer |
|---|---|---|
| `--z-base` | 0 | Default flow. |
| `--z-sticky` | 100 | Sticky headers / toolbars. |
| `--z-dropdown` | 200 | Menus, popovers, the dock. |
| `--z-overlay` | 300 | Scrims / backdrops. |
| `--z-modal` | 400 | Dialogs. |
| `--z-toast` | 500 | Toasts / notifications. |
| `--z-tooltip` | 600 | Tooltips (top). |

---

## 8. Accessibility bar — WCAG 2.2 AA

- Body text **≥ 4.5:1** contrast; large text **≥ 3:1**; **placeholders held to 4.5:1** (do not drop placeholder ink below `--color-ink-faint` against the input fill).
- **Visible, non-color focus** on every interactive element: `:focus-visible` → `outline: 2px solid var(--color-signal-bright); outline-offset: 2px; border-radius: var(--radius-sm)`. Never remove it; `:focus:not(:focus-visible)` clears the ring for pointer users only.
- Status / severity **never color alone** — always square marker + mono word (§3).
- Dark-first, tuned for long sessions; the light theme is held to the same contrast bar.
- `color-scheme: dark` set on `html`; respect `-webkit-text-size-adjust: 100%`.

---

## 9. Anti-slop bans

From `PRODUCT.md` anti-references + the impeccable bar. Do **not** ship:

| Banned | Instead |
|---|---|
| Big-number hero-metric wall | The dashboard is a work/command surface — what drifted, what's stale, what needs review. |
| Identical card grids | Vary by affordance; a card only when a card is the best affordance. |
| Per-section uppercase tracked eyebrows | The only uppercase eyebrow is the `PanelHeader` mono `2xs` label. |
| Gradient text / gradient accents | Solid signal, single hue. |
| Side-stripe borders (colored `border-left`/`border-right` accents) | Status via marker + word; emphasis via fill/tint. |
| Over-rounding | Cards cap **12–16px**; nothing past `--radius-xl`. |
| Glassmorphism by default | Solid surfaces. The dock is **solid, not glass**. |
| Chart-soup / gray-on-gray density everywhere | **Density only where the user works** (rosters, diffs, tables); quiet + spacious elsewhere. |
| Off-the-shelf Bootstrap/Material look | The system primitives below; one signal accent; mono-forward. |

Restraint over reflex: no accent unless it carries meaning. Warmth comes from
typography + one rare highlight, not from chrome.

---

## 10. Component conventions

New components must match these patterns (`components/ui/*`).

### Button (`Button.tsx`)
- `forwardRef`, extends `ButtonHTMLAttributes`. Props: `variant`, `size`, plus native.
- **Base:** `inline-flex items-center justify-center gap-2 rounded-sm font-mono font-medium`, `transition-[…] duration-150 ease-[var(--ease-snap)]`, `active:translate-y-px`, `disabled:pointer-events-none disabled:opacity-40 select-none`. Mono label, 6px corners, single solid fill **or** single hairline border — never border + glow.
- **Variants:**
  - `primary` — `bg-signal text-signal-ink`, hover `bg-signal-bright`.
  - `secondary` (default) — transparent, `border-line-strong`, hover `border-signal-soft` + `bg-surface`.
  - `ghost` — `text-ink-muted`, hover `text-ink` + `bg-surface`, no border.
  - `danger` — transparent, `border-err/40` + `text-err`, hover `bg-err-tint`.
- **Sizes:** `sm` = `h-7 px-2.5 text-xs`; `md` (default) = `h-9 px-3.5 text-sm`.
- `IconButton` — requires `label` (sets `aria-label` + `title`); `h-8 w-8 rounded-sm`, ghost-style hover.

### Panel (`Panel.tsx`)
- `Panel` — `flex flex-col overflow-hidden rounded-lg border-line-soft bg-surface shadow-[var(--shadow-panel)]`. Spreads native div attrs.
- `PanelHeader({ title, icon?, count?, action? })` — divider-bottom row; `icon` in `text-signal`; `title` as mono `2xs uppercase tracking-[0.16em] text-ink-muted`; optional Swiss `/ NN` count in `text-ink-faint`; optional `action` slot right-aligned. Never a marketing eyebrow.

### States (`states.tsx`) — every surface covers loading / empty / error
- `Skeleton({ className })` — `animate-pulse rounded-md bg-surface-raised`, `animationDuration: 1.4s`.
- `SkeletonRows({ rows = 4 })` — staggered-width shimmer lines for list/table loading.
- `EmptyState({ icon?, title, description?, action? })` — centered; optional `11×11 rounded-xl` icon chip on `surface-raised`; mono-less prose, terse copy capped `max-w-xs`.
- `ErrorState({ title?, description?, onRetry? })` — centered; `err-tint` icon chip with `err` warning glyph; renders a `secondary` `sm` **Retry** Button when `onRetry` is given.

### Status (`StatusDot.tsx` + `status.ts`)
- `StatusPill({ status })` — mono `text-xs`, square `Marker` + label, colored via `toneColor[tone].fg`; live pulse on `online`.
- `SeverityTag({ severity })` — mono `2xs` chip, `rounded-sm`, `fg` text on `tint` bg + square marker.
- `Marker` (exported as `StatusMarker`) — `7px` square, `tone.fg`, `motion-safe:animate-pulse` when `live`.
- All tone→color resolution goes through `toneColor` in `status.ts`. Add states there, not inline.

### Shared conventions for new primitives
- Use `cn()` (`lib/cn`) to merge classes; accept `className` last so callers can override.
- `forwardRef` + extend the native element's attribute type for interactive primitives.
- Reference tokens as `var(--color-*)` / scale tokens — never raw hex/OKLCH literals in components.
- Mono for any data/label/control; sans only for prose.
- Cover hover, active, `focus-visible`, and disabled on every interactive element.
