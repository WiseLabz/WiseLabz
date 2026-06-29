/**
 * Accessibility / appearance preferences (Phase 8, Settings → Appearance).
 *
 * Sibling to store/settings.ts (which owns the Motion preference). Motion is left
 * there untouched; this store adds the rest of the accessibility hub: contrast
 * boost, text size, density, reduce-transparency, and focus-ring weight.
 *
 * Each preference is persisted to localStorage and applied LIVE to <html> via:
 *   1. a `data-*` attribute (so any future CSS can hook in), and
 *   2. an injected <style> element carrying the rules that have a real, immediate
 *      effect today — without editing the global index.css.
 *
 * OS preferences only ever SEED the initial value; the user's explicit choice
 * wins and is never silently overridden (mirrors the motion contract).
 */
import { create } from 'zustand';

export type Contrast = 'normal' | 'boost';
export type TextSize = 'sm' | 'base' | 'lg';
export type Density = 'comfortable' | 'compact';
export type FocusRing = 'standard' | 'bold';

export interface AppearanceState {
  contrast: Contrast;
  textSize: TextSize;
  density: Density;
  reduceTransparency: boolean;
  focusRing: FocusRing;
  setContrast: (v: Contrast) => void;
  setTextSize: (v: TextSize) => void;
  setDensity: (v: Density) => void;
  setReduceTransparency: (v: boolean) => void;
  setFocusRing: (v: FocusRing) => void;
  reset: () => void;
}

const STORAGE_KEY = 'wiselabz.appearance';
const STYLE_ID = 'wiselabz-appearance';

type Prefs = Pick<
  AppearanceState,
  'contrast' | 'textSize' | 'density' | 'reduceTransparency' | 'focusRing'
>;

const DEFAULTS: Prefs = {
  contrast: 'normal',
  textSize: 'base',
  density: 'comfortable',
  reduceTransparency: false,
  focusRing: 'standard',
};

function seed(): Prefs {
  if (typeof window === 'undefined') return { ...DEFAULTS };
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? 'null');
    if (saved && typeof saved === 'object') return { ...DEFAULTS, ...saved };
  } catch {
    /* ignore malformed storage */
  }
  // First run: OS high-contrast preference seeds contrast boost; the user can override.
  const prefersContrast =
    typeof window.matchMedia === 'function' && window.matchMedia('(prefers-contrast: more)').matches;
  return { ...DEFAULTS, contrast: prefersContrast ? 'boost' : 'normal' };
}

/** CSS that takes effect immediately, scoped to the data-* attributes below. */
function css(p: Prefs): string {
  const rules: string[] = [];

  if (p.contrast === 'boost') {
    // Default ink already clears AA; boost pushes secondary text toward AAA
    // (muted ≈10.7:1, faint ≈8.2:1) for low-vision / bright-ambient use.
    rules.push(
      ":root[data-contrast='boost']{--color-ink-muted:oklch(0.90 0.006 250);--color-ink-faint:oklch(0.86 0.008 250);}",
    );
  }
  if (p.textSize === 'sm') rules.push(":root[data-text-size='sm']{font-size:15px;}");
  if (p.textSize === 'lg') rules.push(":root[data-text-size='lg']{font-size:17.5px;}");
  if (p.density === 'compact') {
    // Tailwind v4 derives every spacing + box-size utility from --spacing
    // (calc(var(--spacing) * N)). Tightening that one token scales the whole
    // UI's padding, gaps, and control heights ~12% denser, proportionally.
    rules.push(":root[data-density='compact']{--spacing:0.22rem;}");
  }
  if (p.focusRing === 'bold') {
    rules.push(
      ":root[data-focus-ring='bold'] :focus-visible{outline-width:3px !important;outline-offset:3px !important;}",
    );
  }
  if (p.reduceTransparency) {
    // Solidify the native modal backdrop, and kill every frosted-glass blur
    // (topbar, dock, scrims) — the blur, not the slight tint, is what hurts
    // legibility for users who ask to reduce transparency.
    rules.push(
      ":root[data-reduce-transparency='true'] ::backdrop{background:var(--color-canvas) !important;}",
      ":root[data-reduce-transparency='true'] *{backdrop-filter:none !important;-webkit-backdrop-filter:none !important;}",
    );
  }
  return rules.join('\n');
}

function apply(p: Prefs) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.dataset.contrast = p.contrast;
  root.dataset.textSize = p.textSize;
  root.dataset.density = p.density;
  root.dataset.reduceTransparency = String(p.reduceTransparency);
  root.dataset.focusRing = p.focusRing;

  let style = document.getElementById(STYLE_ID) as HTMLStyleElement | null;
  if (!style) {
    style = document.createElement('style');
    style.id = STYLE_ID;
    document.head.appendChild(style);
  }
  style.textContent = css(p);
}

export const useAppearance = create<AppearanceState>((set, get) => {
  const initial = seed();
  apply(initial);

  const persist = (next: Prefs) => {
    apply(next);
    if (typeof window !== 'undefined') localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  };
  const update = (patch: Partial<Prefs>) => {
    const { contrast, textSize, density, reduceTransparency, focusRing } = get();
    const next: Prefs = { contrast, textSize, density, reduceTransparency, focusRing, ...patch };
    persist(next);
    set(patch);
  };

  return {
    ...initial,
    setContrast: (contrast) => update({ contrast }),
    setTextSize: (textSize) => update({ textSize }),
    setDensity: (density) => update({ density }),
    setReduceTransparency: (reduceTransparency) => update({ reduceTransparency }),
    setFocusRing: (focusRing) => update({ focusRing }),
    reset: () => update({ ...DEFAULTS }),
  };
});
