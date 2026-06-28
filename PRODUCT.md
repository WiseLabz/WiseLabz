# Product

## Register

product

## Users

Technical homelabbers — people who run their own infrastructure at home or in a
small lab: Proxmox VE, Docker/Portainer, pfSense/OPNsense, self-hosted apps.
They are comfortable with APIs, YAML config, Docker Compose, and the terminal.
Their context: managing a fleet of services that drift over time, where the
documentation is always stale and nobody enjoys writing it. They reach for
WiseLabz to stop hand-maintaining docs and to get an honest, live picture of
what changed since the last sync.

Primary jobs to be done:
- See live service status and what drifted at a glance.
- Read accurate, auto-generated documentation for each service.
- Review and accept/reject diffs the sync engine surfaces.
- Configure connectors, templates, auth, and AI providers without fighting the UI.

## Product Purpose

WiseLabz automatically generates and maintains documentation for a homelab. It
connects to each service's native API, fetches live data, renders structured docs
from configurable templates, and runs a diff engine that detects change since the
last sync — then either drafts an AI-assisted section update or raises an alert so
nothing drifts undocumented. The web dashboard shows live service data alongside
the generated docs.

Documentation is the heart of the product, but not the whole of it. WiseLabz is a
homelab *manager*, not only a documentation manager — the name is the thesis: it
makes your lab *wise*. The dashboard is a command surface, not a passive status
wall: you see live state and act on it, and everything you click routes you into
the docs that explain it. Docs stay the center of gravity; the manager layer is how
you reach and operate on them.

Success: the user trusts the docs without re-checking them, spends near-zero effort
keeping them current, notices drift through WiseLabz before it bites them, and runs
the lab from one place instead of jumping between service consoles.

## Product decisions (pre-planning, v1)

Locked before the planning session. These are product/UX scope calls; technical
enforcement and build details live in `docs/ARCHITECTURE.md`.

1. **IA spine — Docs is home; the dashboard is the entry/command surface.** The
   dashboard is a work-and-control surface (what drifted, what's stale, what needs
   review, what you can act on) that routes into docs. Never a big-number hero wall.
2. **Manager scope (v1) is deliberately narrow.** In scope: trigger sync
   (global + per-service), enable/disable a connector, and add/remove connectors —
   all via the UI. Out of scope for v1: lab-mutating operations (service
   start/stop/restart, config push). The manager grows from there, deliberately.
3. **AI drafts are hybrid.** Minor section updates surface inline in the doc with a
   provenance marker (drafted / human-confirmed / synced-raw); structural changes
   route through the Changes accept/reject loop. Nothing implies more confidence
   than the sync actually has.
4. **Blueprint is *the* identity, not one theme among many.** Swiss instrument
   panel — mono-forward, hairline grid over cards, sharp corners, one signal accent,
   status as shape + word. Ships with light/dark only; no theme playground.
5. **Motion is a meaning-bound differentiator.** Four moments earn animation: diff
   reveal, sync pulse (live heartbeat), drift surfacing, and state transitions
   (a row resolving on accept/reject). Everything else is plain state feedback; no
   page-load choreography.

**Open (decide during planning): how far "manager" goes past v1.** Lab-mutating
control (lifecycle, config push) is desired but deferred — it changes the risk
profile and is gated on the permission/confirmation model in `docs/ARCHITECTURE.md`.

## Locked frontend direction (planning session, 2026-06)

Refines the decisions above; supersedes the "sharp corners / light-dark only"
specifics of decision 4. Shell architecture and the theme flag live in
`docs/ARCHITECTURE.md`.

1. **Navigation — bottom dock.** A floating, centered, soft-dark dock (solid, not
   glass); one amber pill slides under the active item. A sidebar variant was built
   and rejected in comparison. The dock is the single shell; no shell switching.
2. **Identity — Blueprint palette + Geist, in soft-dark chrome.** Signal-orange on
   cool-slate, rounded-but-tight radii (6–16px), real layered depth shadows. This
   replaces the original sharp/flat Blueprint chrome while keeping the Swiss
   instrument-panel discipline (mono-forward, one signal accent, status as shape +
   word). Status colors stay distinct (green/lemon/red), never collapsed into the
   accent.
3. **Theme engine ships behind Settings → Theme** (presets / fonts / OKLCH sliders)
   for exploration, defaulting to the locked Blueprint + Geist combo. It is a power
   surface, not a marketing theme playground; the default is the brand.
4. **Changes read as a JetBrains-IDE git diff.** A doc Change renders the whole
   document with context — add/remove lines highlighted, word-level intra-line
   highlighting, foldable unchanged regions, unified (default) or side-by-side.
   Acknowledge = accept, Dismiss = reject; the row resolves with motion (moment #4
   above) on either.

## Brand Personality

**Precise, technical, calm.** Confident expert tooling in the Linear/Vercel
register: restrained, machine-honest, never shouting. Voice is direct and
unpadded — it states facts (status, counts, timestamps, diffs) plainly and
trusts the user to read them. No marketing gloss, no fake enthusiasm, no
hand-holding. The interface should feel like a well-built instrument: dense
where density earns its keep, quiet everywhere else.

## Anti-references

- **Generic AI dashboard slop** — identical card grids, the big-number hero-metric
  template, gradient accents, tiny uppercase tracked eyebrows over every section.
  (The v1 prototype was rejected for exactly this.)
- **Heavy enterprise consoles (Grafana / Datadog)** — chart-soup, cluttered chrome,
  gray-on-gray density with no hierarchy, the legacy ops-console feel.
- **Bootstrap / Material defaults** — off-the-shelf component-kit look with no identity.

## Design Principles

1. **Machine-honest.** Show real state — exact counts, timestamps, statuses, diffs.
   Never decorate data or imply more confidence than the sync actually has.
2. **Density that earns its keep.** Pack information where the user is working
   (rosters, diffs, tables); stay quiet and spacious everywhere else. Density is a
   tool, not a default.
3. **Status is never color alone.** Every state reads through shape + word + color
   (dot + label), so it survives color blindness, grayscale, and a glance.
4. **Restraint over reflex.** No card unless a card is the best affordance; no
   accent unless it carries meaning. Warmth comes from typography and one rare
   highlight, not from chrome.
5. **Expert-default, keyboard-friendly.** Strong defaults, fast paths (command
   palette, shortcuts), and an UI that respects that the user already knows their lab.

## Accessibility & Inclusion

Target **WCAG 2.2 AA**, plus committed extras:
- Body text ≥4.5:1 contrast; large text ≥3:1; placeholders held to the same bar.
- Visible, non-color focus indicators on every interactive element.
- Status and severity never communicated by color alone — always dot/icon + word.
- Dark-first theme designed for long sessions; light theme held to the same contrast bar.

**Motion is a default, not an afterthought.** Animation and motion are a core
differentiator for WiseLabz — the interface should be astonishingly beautiful, and
motion carries that. Build it in from the start, not as garnish. Reduced motion is
**not** a blanket suppression rule; it is a **user-controlled setting** in
Settings → Appearance (Motion: Full / Reduced / Off). The OS `prefers-reduced-motion`
signal seeds the initial value but never silently forces motion off — the user decides
and can override. Default is full motion.
