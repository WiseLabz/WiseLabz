/**
 * Live theme state. Drives the Settings → Theme switcher and persists the choice
 * per-browser. Every change recomputes tokens and writes them to :root so the
 * whole app re-skins immediately.
 *
 *   mode 'preset' → one of PRESETS
 *   mode 'custom' → full makePalette() opts, edited via the Advanced sliders
 */
import { create } from 'zustand';
import {
  ACTIVE,
  FontSetName,
  PaletteName,
  PaletteOpts,
  PRESETS,
  applyTokens,
  makePalette,
  presetOpts,
} from '../theme';

type ColorMode = 'preset' | 'custom';

interface Persisted {
  font: FontSetName;
  mode: ColorMode;
  preset: PaletteName;
  custom: PaletteOpts;
}

interface ThemeState extends Persisted {
  setFont: (font: FontSetName) => void;
  setMode: (mode: ColorMode) => void;
  setPreset: (preset: PaletteName) => void;
  setCustomOpt: <K extends keyof PaletteOpts>(key: K, value: PaletteOpts[K]) => void;
  /** copy the current preset's opts into custom + switch to custom mode */
  forkPresetToCustom: () => void;
  reset: () => void;
}

const STORAGE_KEY = 'wiselabz.theme.v3';

function load(): Persisted {
  const fallback: Persisted = {
    font: ACTIVE.font,
    mode: 'preset',
    preset: ACTIVE.palette,
    custom: presetOpts(ACTIVE.palette),
  };
  if (typeof window === 'undefined') return fallback;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return fallback;
    const p = JSON.parse(raw) as Partial<Persisted>;
    return {
      font: p.font && p.font in PRESETS_FONTS ? p.font : fallback.font,
      mode: p.mode === 'custom' ? 'custom' : 'preset',
      preset: p.preset && p.preset in PRESETS ? p.preset : fallback.preset,
      custom: { ...fallback.custom, ...(p.custom ?? {}) },
    };
  } catch {
    return fallback;
  }
}

// tiny guard set so a stale localStorage font name can't break the app
const PRESETS_FONTS: Record<string, true> = { plex: true, jetbrains: true, space: true, geist: true };

function tokensFor(s: Persisted) {
  return s.mode === 'custom' ? makePalette(s.custom) : makePalette(PRESETS[s.preset].opts);
}

function commit(s: Persisted) {
  applyTokens(tokensFor(s), s.font, s.mode === 'custom' ? 'custom' : s.preset);
  if (typeof window !== 'undefined') {
    const persisted: Persisted = { font: s.font, mode: s.mode, preset: s.preset, custom: s.custom };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(persisted));
  }
}

export const useTheme = create<ThemeState>((set, get) => {
  const initial = load();
  commit(initial); // apply persisted choice on first load

  const update = (patch: Partial<Persisted>) => {
    const next = { ...get(), ...patch } as Persisted;
    commit(next);
    set(patch as Partial<ThemeState>);
  };

  return {
    ...initial,
    setFont: (font) => update({ font }),
    setMode: (mode) => update({ mode }),
    setPreset: (preset) => update({ mode: 'preset', preset }),
    setCustomOpt: (key, value) =>
      update({ mode: 'custom', custom: { ...get().custom, [key]: value } }),
    forkPresetToCustom: () => update({ mode: 'custom', custom: presetOpts(get().preset) }),
    reset: () =>
      update({
        font: ACTIVE.font,
        mode: 'preset',
        preset: ACTIVE.palette,
        custom: presetOpts(ACTIVE.palette),
      }),
  };
});
