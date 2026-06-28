/**
 * Dashboard widget layout — order + enabled + column span. Persisted per-browser
 * (mirrors the planned GET/PUT /api/dashboard/layout; localStorage gives instant
 * paint before hydration). Reorder happens in edit mode via motion's Reorder.
 */
import { create } from 'zustand';

export type WidgetId = 'roster' | 'changes' | 'alerts' | 'sync' | 'docs';

export interface WidgetDef {
  id: WidgetId;
  enabled: boolean;
  /** column span in the 6-col content grid */
  span: 2 | 3 | 4 | 6;
}

const DEFAULT_LAYOUT: WidgetDef[] = [
  { id: 'roster', enabled: true, span: 4 },
  { id: 'alerts', enabled: true, span: 2 },
  { id: 'changes', enabled: true, span: 3 },
  { id: 'sync', enabled: true, span: 3 },
  { id: 'docs', enabled: true, span: 6 },
];

const STORAGE_KEY = 'wiselabz.dashboard.layout';

function load(): WidgetDef[] {
  if (typeof window === 'undefined') return DEFAULT_LAYOUT;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_LAYOUT;
    const parsed = JSON.parse(raw) as WidgetDef[];
    // Drop unknown ids, re-add any new defaults missing from saved layout.
    const known = new Set(DEFAULT_LAYOUT.map((w) => w.id));
    const kept = parsed.filter((w) => known.has(w.id));
    const missing = DEFAULT_LAYOUT.filter(
      (d) => !kept.some((w) => w.id === d.id),
    );
    return [...kept, ...missing];
  } catch {
    return DEFAULT_LAYOUT;
  }
}

function persist(layout: WidgetDef[]) {
  if (typeof window !== 'undefined') {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(layout));
  }
}

interface DashboardState {
  layout: WidgetDef[];
  setOrder: (ids: WidgetId[]) => void;
  toggle: (id: WidgetId) => void;
  reset: () => void;
}

export const useDashboard = create<DashboardState>((set) => ({
  layout: load(),
  setOrder: (ids) =>
    set((s) => {
      const byId = new Map(s.layout.map((w) => [w.id, w]));
      const layout = ids.map((id) => byId.get(id)!).filter(Boolean);
      persist(layout);
      return { layout };
    }),
  toggle: (id) =>
    set((s) => {
      const layout = s.layout.map((w) =>
        w.id === id ? { ...w, enabled: !w.enabled } : w,
      );
      persist(layout);
      return { layout };
    }),
  reset: () => {
    persist(DEFAULT_LAYOUT);
    return set({ layout: DEFAULT_LAYOUT });
  },
}));
