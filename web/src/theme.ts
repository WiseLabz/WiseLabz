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
import '@fontsource-variable/big-shoulders-text';
import '@fontsource-variable/martian-mono';

/* ===========================================================================
 *  FONT SETS  —  { mono, sans }.  Mono is the dominant UI voice; sans is prose.
 * ======================================================================== */
export const FONT_SETS = {
  rack: { label: 'Rack', mono: "'Martian Mono Variable'", sans: "'Big Shoulders Text Variable'" },
  plex: { label: 'IBM Plex', mono: "'IBM Plex Mono'", sans: "'IBM Plex Sans'" },
  jetbrains: {
    label: 'JetBrains',
    mono: "'JetBrains Mono Variable'",
    sans: "'Inter Tight Variable'",
  },
  space: { label: 'Space', mono: "'Space Mono'", sans: "'Space Grotesk Variable'" },
  geist: { label: 'Geist', mono: "'Geist Mono Variable'", sans: "'Geist Variable'" },
} as const;

export type FontSetName = keyof typeof FONT_SETS;

/* ===========================================================================
 *  PALETTE BUILDER  —  a neutral ramp + ONE signal accent + a status set.
 * ======================================================================== */
export type PaletteTokens = Record<string, string>;

export interface PaletteOpts {
  neutralHue: number; // hue of the near-black canvas & grays (85 = warm steel)
  neutralChroma: number; // how tinted the neutrals are (0 = pure gray)
  canvasL: number; // page background lightness (0.14 dark … 0.20 lighter)
  accentPrimaryHue: number; // primary accent hue (45 = safety-orange)
  accentPrimaryChroma: number; // primary accent vividness (0.12 muted … 0.19 vivid)
  accentPrimaryL: number; // primary accent lightness
  accentPrimaryInkHue: number; // hue of text placed ON the primary accent
  accentSecondaryHue: number; // secondary accent hue (240 = cable-blue)
  accentSecondaryChroma: number; // secondary accent vividness
  accentSecondaryL: number; // secondary accent lightness
}

export const DEFAULT_OPTS: PaletteOpts = {
  neutralHue: 85,
  neutralChroma: 0.006,
  canvasL: 0.16,
  accentPrimaryHue: 45,
  accentPrimaryChroma: 0.18,
  accentPrimaryL: 0.74,
  accentPrimaryInkHue: 55,
  accentSecondaryHue: 240,
  accentSecondaryChroma: 0.11,
  accentSecondaryL: 0.64,
};

/** Editable ranges for the advanced UI sliders. */
export const OPT_META: Record<
  keyof PaletteOpts,
  { label: string; min: number; max: number; step: number; hint: string }
> = {
  neutralHue: { label: 'Neutral hue', min: 0, max: 360, step: 1, hint: 'tint of the dark base' },
  neutralChroma: { label: 'Neutral chroma', min: 0, max: 0.03, step: 0.001, hint: '0 = pure gray' },
  canvasL: {
    label: 'Background lightness',
    min: 0.1,
    max: 0.22,
    step: 0.005,
    hint: 'darker ← → lighter',
  },
  accentPrimaryHue: {
    label: 'Primary accent hue',
    min: 0,
    max: 360,
    step: 1,
    hint: 'main action / selection color',
  },
  accentPrimaryChroma: {
    label: 'Primary accent chroma',
    min: 0.05,
    max: 0.2,
    step: 0.005,
    hint: 'muted ← → vivid',
  },
  accentPrimaryL: {
    label: 'Primary accent lightness',
    min: 0.5,
    max: 0.88,
    step: 0.01,
    hint: 'darker ← → brighter',
  },
  accentPrimaryInkHue: {
    label: 'Primary-accent-text hue',
    min: 0,
    max: 360,
    step: 1,
    hint: 'text drawn on the primary accent',
  },
  accentSecondaryHue: {
    label: 'Secondary accent hue',
    min: 0,
    max: 360,
    step: 1,
    hint: 'links / secondary emphasis color',
  },
  accentSecondaryChroma: {
    label: 'Secondary accent chroma',
    min: 0.05,
    max: 0.2,
    step: 0.005,
    hint: 'muted ← → vivid',
  },
  accentSecondaryL: {
    label: 'Secondary accent lightness',
    min: 0.5,
    max: 0.88,
    step: 0.01,
    hint: 'darker ← → brighter',
  },
};

export function makePalette(opts: Partial<PaletteOpts>): PaletteTokens {
  const o = { ...DEFAULT_OPTS, ...opts };
  const N = (l: number, c = o.neutralChroma) => `oklch(${l} ${c} ${o.neutralHue})`;
  const P = (l: number, c: number) => `oklch(${l} ${c} ${o.accentPrimaryHue})`;
  const S = (l: number, c: number) => `oklch(${l} ${c} ${o.accentSecondaryHue})`;
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
    // Primary accent — safety-orange. Drives primary actions / selection.
    '--color-accent-primary': P(o.accentPrimaryL, o.accentPrimaryChroma),
    '--color-accent-primary-bright': P(o.accentPrimaryL + 0.07, o.accentPrimaryChroma - 0.01),
    '--color-accent-primary-ink': `oklch(0.17 0.03 ${o.accentPrimaryInkHue})`,
    '--color-accent-primary-soft': P(0.46, o.accentPrimaryChroma * 0.62),
    '--color-accent-primary-tint': P(0.27, o.accentPrimaryChroma * 0.32),
    // Secondary accent — cable-blue. Secondary emphasis / links / info, never
    // collapsed into the primary accent or into a status color.
    '--color-accent-secondary': S(o.accentSecondaryL, o.accentSecondaryChroma),
    '--color-accent-secondary-bright': S(o.accentSecondaryL + 0.07, o.accentSecondaryChroma - 0.01),
    '--color-accent-secondary-soft': S(0.46, o.accentSecondaryChroma * 0.62),
    '--color-accent-secondary-tint': S(0.27, o.accentSecondaryChroma * 0.32),
    '--color-ok': 'oklch(0.8 0.12 145)',
    '--color-ok-tint': 'oklch(0.27 0.05 145)',
    '--color-warn': 'oklch(0.84 0.14 80)',
    '--color-warn-tint': 'oklch(0.3 0.06 80)',
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
  rack: { label: 'Rack', desc: 'safety-orange + cable-blue · warm steel', opts: {} },
  phosphor: {
    label: 'Phosphor',
    desc: 'terminal green + amber · true black',
    opts: {
      neutralHue: 140,
      neutralChroma: 0.004,
      canvasL: 0.135,
      accentPrimaryHue: 145,
      accentPrimaryChroma: 0.16,
      accentPrimaryL: 0.8,
      accentSecondaryHue: 80,
      accentSecondaryChroma: 0.13,
      accentSecondaryL: 0.78,
    },
  },
  'blueprint-classic': {
    label: 'Blueprint (classic)',
    desc: 'orange · cool slate — the original identity',
    opts: {
      neutralHue: 250,
      neutralChroma: 0.008,
      canvasL: 0.135,
      accentPrimaryHue: 52,
      accentPrimaryChroma: 0.135,
      accentPrimaryL: 0.8,
      accentPrimaryInkHue: 70,
      accentSecondaryHue: 232,
      accentSecondaryChroma: 0.14,
      accentSecondaryL: 0.74,
    },
  },
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
  palette: 'rack',
  font: 'space',
};
