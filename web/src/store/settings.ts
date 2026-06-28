/**
 * Appearance settings — motion is a first-class, user-controlled preference.
 * OS `prefers-reduced-motion` only SEEDS the initial value; the user's explicit
 * choice (persisted) wins and is never silently overridden. Applied to
 * <html data-motion> which index.css reads.
 */
import { create } from 'zustand';

export type MotionPref = 'full' | 'reduced' | 'off';

const STORAGE_KEY = 'wiselabz.motion';

function seed(): MotionPref {
  if (typeof window === 'undefined') return 'full';
  const saved = localStorage.getItem(STORAGE_KEY) as MotionPref | null;
  if (saved === 'full' || saved === 'reduced' || saved === 'off') return saved;
  // First run: OS preference seeds, but full is the product default.
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
    ? 'reduced'
    : 'full';
}

function apply(pref: MotionPref) {
  if (typeof document !== 'undefined') {
    document.documentElement.dataset.motion = pref;
  }
}

interface SettingsState {
  motion: MotionPref;
  setMotion: (m: MotionPref) => void;
}

export const useSettings = create<SettingsState>((set) => {
  const initial = seed();
  apply(initial);
  return {
    motion: initial,
    setMotion: (motion) => {
      apply(motion);
      if (typeof window !== 'undefined') localStorage.setItem(STORAGE_KEY, motion);
      set({ motion });
    },
  };
});

/** For motion's MotionConfig: 'reduced' and 'off' both suppress framer springs. */
export function framerReducedMotion(pref: MotionPref): 'always' | 'never' {
  return pref === 'full' ? 'never' : 'always';
}
