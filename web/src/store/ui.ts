/** Ephemeral UI state: command palette, sidebar collapse, dashboard edit mode. */
import { create } from 'zustand';

/** Doc-diff rendering: unified (default) or side-by-side. Persisted per browser. */
export type DiffLayout = 'unified' | 'split';
const DIFF_KEY = 'wiselabz.diffLayout';
const loadDiffLayout = (): DiffLayout => {
  if (typeof window === 'undefined') return 'unified';
  return localStorage.getItem(DIFF_KEY) === 'split' ? 'split' : 'unified';
};

interface UiState {
  paletteOpen: boolean;
  setPalette: (open: boolean) => void;
  togglePalette: () => void;

  sidebarCollapsed: boolean;
  toggleSidebar: () => void;

  diffLayout: DiffLayout;
  setDiffLayout: (layout: DiffLayout) => void;

  editingDashboard: boolean;
  setEditingDashboard: (v: boolean) => void;
}

export const useUi = create<UiState>((set) => ({
  paletteOpen: false,
  setPalette: (paletteOpen) => set({ paletteOpen }),
  togglePalette: () => set((s) => ({ paletteOpen: !s.paletteOpen })),

  sidebarCollapsed: false,
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),

  diffLayout: loadDiffLayout(),
  setDiffLayout: (diffLayout) => {
    if (typeof window !== 'undefined') localStorage.setItem(DIFF_KEY, diffLayout);
    set({ diffLayout });
  },

  editingDashboard: false,
  setEditingDashboard: (editingDashboard) => set({ editingDashboard }),
}));
