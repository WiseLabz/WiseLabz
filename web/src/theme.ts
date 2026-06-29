/* ============================================================================
 *  WiseLabz — THEME ENGINE
 *  ---------------------------------------------------------------------------
 *  Source of truth for palettes + fonts. You can drive it three ways:
 *    1. Live, in the app:  Settings → Theme  (basic presets or advanced knobs).
 *    2. Code default:      edit `ACTIVE` at the bottom.
 *    3. New presets/fonts: add to `PRESETS` / `FONT_SETS` below.
 *
 *  Colors are OKLCH:  oklch(Lightness  Chroma  Hue)
 *    L 0..1 (dark→light) · C 0..~0.37 (gray→vivid) · H 0..360 (the hue angle)
 * ========================================================================== */

/* ---- Font files (bundled so any set can be selected at runtime) ------------ */
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/500.css';
import '@fontsource/ibm-plex-mono/600.css';
import '@fontsource/ibm-plex-mono/700.css';
import '@fontsource/ibm-plex-sans/400.css';
import '@fontsource/ibm-plex-sans/500.css';
import '@fontsource/ibm-plex-sans/600.css';
import '@fontsource-variable/jetbrains-mono';
import '@fontsource-variable/inter-tight';
import '@fontsource-variable/space-grotesk';
import '@fontsource/space-mono/400.css';
import '@fontsource/space-mono/700.css';
import '@fontsource-variable/geist';
import '@fontsource-variable/geist-mono';

/* ===========================================================================
 *  FONT SETS  —  { mono, sans }.  Mono is the dominant UI voice; sans is prose.
 * ======================================================================== */
export const FONT_SETS = {
  plex: { label: 'IBM Plex', mono: "'IBM Plex Mono'", sans: "'IBM Plex Sans'" },
  jetbrains: { label: 'JetBrains', mono: "'JetBrains Mono Variable'", sans: "'Inter Tight Variable'" },
  space: { label: 'Space', mono: "'Space Mono'", sans: "'Space Grotesk Variable'" },
  geist: { label: 'Geist', mono: "'Geist Mono Variable'", sans: "'Geist Variable'" },
} as const;

export type FontSetName = keyof typeof FONT_SETS;

/* ===========================================================================
 *  PALETTE BUILDER  —  a neutral ramp + ONE signal accent + a status set.
 * ======================================================================== */
export type PaletteTokens = Record<string, string>;

export interface PaletteOpts {
  neutralHue: number; // hue of the near-black canvas & grays (250 = cool slate)
  neutralChroma: number; // how tinted the neutrals are (0 = pure gray)
  canvasL: number; // page background lightness (0.14 dark … 0.20 lighter)
  signalHue: number; // brand accent hue (52 orange · 152 green · 286 violet)
  signalChroma: number; // accent vividness (0.12 muted … 0.19 vivid)
  signalL: number; // accent lightness
  signalInkHue: number; // hue of text placed ON the accent
}

export const DEFAULT_OPTS: PaletteOpts = {
  neutralHue: 255,
  neutralChroma: 0.008,
  canvasL: 0.165,
  signalHue: 78,
  signalChroma: 0.135,
  signalL: 0.80,
  signalInkHue: 70,
};

/** Editable ranges for the advanced UI sliders. */
export const OPT_META: Record<keyof PaletteOpts, { label: string; min: number; max: number; step: number; hint: string }> = {
  neutralHue: { label: 'Neutral hue', min: 0, max: 360, step: 1, hint: 'tint of the dark base' },
  neutralChroma: { label: 'Neutral chroma', min: 0, max: 0.03, step: 0.001, hint: '0 = pure gray' },
  canvasL: { label: 'Background lightness', min: 0.1, max: 0.22, step: 0.005, hint: 'darker ← → lighter' },
  signalHue: { label: 'Accent hue', min: 0, max: 360, step: 1, hint: 'the brand color angle' },
  signalChroma: { label: 'Accent chroma', min: 0.05, max: 0.2, step: 0.005, hint: 'muted ← → vivid' },
  signalL: { label: 'Accent lightness', min: 0.5, max: 0.88, step: 0.01, hint: 'darker ← → brighter' },
  signalInkHue: { label: 'Accent-text hue', min: 0, max: 360, step: 1, hint: 'text drawn on the accent' },
};

export function makePalette(opts: Partial<PaletteOpts>): PaletteTokens {
  const o = { ...DEFAULT_OPTS, ...opts };
  const N = (l: number, c = o.neutralChroma) => `oklch(${l} ${c} ${o.neutralHue})`;
  const S = (l: number, c: number) => `oklch(${l} ${c} ${o.signalHue})`;
  const cL = o.canvasL;

  return {
    '--color-canvas': N(cL),
    '--color-canvas-sunken': N(cL - 0.027),
    '--color-surface': N(cL + 0.028, o.neutralChroma + 0.001),
    '--color-surface-raised': N(cL + 0.058, o.neutralChroma + 0.002),
    '--color-surface-overlay': N(cL + 0.05, o.neutralChroma + 0.002),
    '--color-line-soft': N(0.255),
    '--color-line': N(0.315),
    '--color-line-strong': N(0.44, o.neutralChroma + 0.002),
    '--color-ink': N(0.975, 0.003),
    // Ink ramp tuned to clear WCAG 2.2 AA on every preset's surfaces (PRODUCT.md
    // bar: body + secondary text + placeholders all ≥4.5:1). muted ≈6.2:1,
    // faint ≈4.8:1 small-text; hierarchy carried by L-step + weight, not by
    // dropping below legibility.
    '--color-ink-muted': N(0.82, 0.007),
    '--color-ink-faint': N(0.78, 0.008),
    '--color-signal': S(o.signalL, o.signalChroma),
    '--color-signal-bright': S(o.signalL + 0.07, o.signalChroma - 0.01),
    '--color-signal-ink': `oklch(0.17 0.03 ${o.signalInkHue})`,
    '--color-signal-soft': S(0.46, o.signalChroma * 0.62),
    '--color-signal-tint': S(0.27, o.signalChroma * 0.32),
    '--color-ok': 'oklch(0.8 0.12 168)',
    '--color-ok-tint': 'oklch(0.27 0.05 168)',
    '--color-warn': 'oklch(0.86 0.135 96)',
    '--color-warn-tint': 'oklch(0.3 0.06 96)',
    '--color-err': 'oklch(0.67 0.2 25)',
    '--color-err-tint': 'oklch(0.3 0.08 25)',
    // Idle stays a desaturated gray (its "inactive" semantic), but bright enough
    // that idle text on its tint chip clears AA (≈4.7:1) and the marker matches
    // its ok/warn/err siblings' brightness.
    '--color-idle': N(0.78, 0.008),
    '--color-idle-tint': N(0.26),
  };
}

/* ===========================================================================
 *  PRESETS  —  each is a name + a one-line set of builder opts.
 *  Selecting one in the UI also seeds Advanced mode with these exact opts.
 * ======================================================================== */
export const PRESETS = {
  wise: { label: 'Wise', desc: 'amber · cool slate', opts: {} },
  blueprint: { label: 'Blueprint', desc: 'orange · cool slate', opts: { neutralHue: 250, signalHue: 52, canvasL: 0.135 } },
  oxide: { label: 'Oxide', desc: 'copper · warm charcoal', opts: { neutralHue: 58, neutralChroma: 0.012, canvasL: 0.16, signalHue: 42, signalChroma: 0.15 } },
  viridian: { label: 'Viridian', desc: 'terminal green', opts: { neutralHue: 200, neutralChroma: 0.004, signalHue: 152, signalChroma: 0.15, signalL: 0.78 } },
  arctic: { label: 'Arctic', desc: 'ice blue', opts: { neutralHue: 244, signalHue: 232, signalChroma: 0.14, signalL: 0.74 } },
  ultraviolet: { label: 'Ultraviolet', desc: 'violet · graphite', opts: { neutralHue: 285, signalHue: 286, signalChroma: 0.17 } },
  noir: { label: 'Noir', desc: 'magenta · true black', opts: { neutralHue: 0, neutralChroma: 0, canvasL: 0.145, signalHue: 350, signalChroma: 0.17 } },
} satisfies Record<string, { label: string; desc: string; opts: Partial<PaletteOpts> }>;

export type PaletteName = keyof typeof PRESETS;

/** Fill a preset's partial opts to a complete set (for seeding Advanced mode). */
export function presetOpts(name: PaletteName): PaletteOpts {
  return { ...DEFAULT_OPTS, ...PRESETS[name].opts };
}

/* ---- Low-level apply: write tokens + fonts onto :root ---------------------- */
export function applyTokens(tokens: PaletteTokens, font: FontSetName, paletteName?: string): void {
  const root = document.documentElement;
  for (const [key, value] of Object.entries(tokens)) root.style.setProperty(key, value);
  const f = FONT_SETS[font];
  root.style.setProperty('--font-mono', `${f.mono}, ui-monospace, 'SF Mono', Menlo, monospace`);
  root.style.setProperty('--font-sans', `${f.sans}, ui-sans-serif, system-ui, sans-serif`);
  if (paletteName) root.dataset.palette = paletteName;
}

/* ===========================================================================
 *  ▼▼▼  CODE DEFAULT  ▼▼▼  used on first load before the user picks anything.
 * ======================================================================== */
export const ACTIVE: { palette: PaletteName; font: FontSetName } = {
  palette: 'blueprint',
  font: 'geist',
};
