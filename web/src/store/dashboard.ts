/**
 * Dashboard widget layout — order + enabled + column span. Persisted per-user
 * via GET/PUT /api/dashboard/layout and reset via POST /api/dashboard/layout/reset
 * (issue #94). localStorage is kept only as an instant-paint cache before the
 * API hydrates — it is never the source of truth.
 */
import { create } from 'zustand';
import { useSearchParams } from 'react-router-dom';
import {
  getDashboardLayout,
  putDashboardLayout,
  postDashboardLayoutReset,
} from '../api/generated/dashboard/dashboard';
import {
  WidgetPlacementType,
  type DashboardLayout,
  type WidgetPlacement,
  type Severity,
} from '../api/model';

// Extends the spec's WidgetPlacement with the UI's enabled/disabled flag —
// see the note on WIDGET_TYPE below for why the two shapes coexist.
type WireWidget = WidgetPlacement & {
  enabled: boolean;
  severityFilter?: Severity | 'all';
  pollingEnabled?: boolean;
};

export type WidgetId = 'roster' | 'changes' | 'alerts' | 'sync' | 'docs' | 'attention';

export interface WidgetDef {
  id: WidgetId;
  enabled: boolean;
  /** column span in the 6-col content grid */
  span: 2 | 3 | 4 | 6;
  severityFilter?: Severity | 'all';
  pollingEnabled?: boolean;
}

// Header time-range control (?range= URL param) and the RecentChangesWidget's
// `days` query param share this lookup — no duration-string parsing needed.
export type RangePreset = '24h' | '7d' | '30d' | '90d';
export const RANGE_DAYS: Record<RangePreset, number> = { '24h': 1, '7d': 7, '30d': 30, '90d': 90 };
export const DEFAULT_RANGE: RangePreset = '7d';

/** Cadence for polling-enabled widgets' refetchInterval. */
const WIDGET_POLL_INTERVAL_MS = 30_000;

/** The header's ?range= URL param resolved to a day count, shared by every widget that windows by day. */
export function useRangeDays(): number {
  const [searchParams] = useSearchParams();
  return RANGE_DAYS[(searchParams.get('range') as RangePreset | null) ?? DEFAULT_RANGE];
}

export const DEFAULT_LAYOUT: WidgetDef[] = [
  { id: 'roster', enabled: true, span: 4 },
  { id: 'alerts', enabled: true, span: 2 },
  { id: 'changes', enabled: true, span: 3 },
  { id: 'sync', enabled: true, span: 3 },
  { id: 'docs', enabled: true, span: 6 },
  // disabled by default — existing users shouldn't get a new widget forced on
  { id: 'attention', enabled: false, span: 2 },
];

// The OpenAPI contract models widgets as position/size placements
// (id/type/x/y/w/h) rather than this UI's order/enabled/span model, and the
// Go handler just stores whatever JSON it's given — so the wire format here
// carries both: standard placement fields plus an `enabled` extension.
// ponytail: reconciling the two shapes fully is out of scope for #94.
const WIDGET_TYPE: Record<WidgetId, WidgetPlacementType> = {
  roster: WidgetPlacementType.service_status,
  alerts: WidgetPlacementType.alert_summary,
  changes: WidgetPlacementType.recent_changes,
  sync: WidgetPlacementType.sync_activity,
  docs: WidgetPlacementType.docs_health,
  attention: WidgetPlacementType.attention,
};

const STORAGE_KEY = 'wiselabz.dashboard.layout';

export function widgetsFromWire(raw: unknown): WidgetDef[] {
  const known = new Set(DEFAULT_LAYOUT.map((w) => w.id));
  const kept: WidgetDef[] = [];
  if (Array.isArray(raw)) {
    for (const w of raw as Partial<WireWidget>[]) {
      const id = w?.id as WidgetId;
      if (!known.has(id)) continue;
      const fallbackSpan = DEFAULT_LAYOUT.find((d) => d.id === id)!.span;
      const span = [2, 3, 4, 6].includes(w.w as number) ? (w.w as WidgetDef['span']) : fallbackSpan;
      // ponytail: opaque JSON blob, no schema validation — field just needs to round-trip
      kept.push({
        id,
        enabled: typeof w.enabled === 'boolean' ? w.enabled : true,
        span,
        severityFilter: w.severityFilter,
        pollingEnabled: typeof w.pollingEnabled === 'boolean' ? w.pollingEnabled : undefined,
      });
    }
  }
  const missing = DEFAULT_LAYOUT.filter((d) => !kept.some((w) => w.id === d.id));
  return [...kept, ...missing];
}

export function widgetsToWire(layout: WidgetDef[]): DashboardLayout {
  const widgets: WireWidget[] = layout.map((w, i) => ({
    id: w.id,
    type: WIDGET_TYPE[w.id],
    x: 0,
    y: i,
    w: w.span,
    h: 1,
    enabled: w.enabled,
    severityFilter: w.severityFilter,
    pollingEnabled: w.pollingEnabled,
  }));
  return { widgets } as unknown as DashboardLayout;
}

function loadCache(): WidgetDef[] {
  if (typeof window === 'undefined') return DEFAULT_LAYOUT;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_LAYOUT;
    return widgetsFromWire(JSON.parse(raw));
  } catch {
    return DEFAULT_LAYOUT;
  }
}

function persistCache(layout: WidgetDef[]) {
  if (typeof window !== 'undefined') {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(widgetsToWire(layout).widgets));
  }
}

interface DashboardState {
  layout: WidgetDef[];
  hydrated: boolean;
  hydrate: () => Promise<void>;
  setOrder: (ids: WidgetId[]) => void;
  toggle: (id: WidgetId) => void;
  setWidgetOptions: (id: WidgetId, patch: Partial<Pick<WidgetDef, 'severityFilter' | 'pollingEnabled'>>) => void;
  reset: () => Promise<void>;
}

export const useDashboard = create<DashboardState>((set, get) => ({
  layout: loadCache(),
  hydrated: false,

  hydrate: async () => {
    try {
      const res = await getDashboardLayout();
      const layout = widgetsFromWire(res.widgets);
      persistCache(layout);
      set({ layout, hydrated: true });
    } catch {
      set({ hydrated: true });
    }
  },

  setOrder: (ids) => {
    const byId = new Map(get().layout.map((w) => [w.id, w]));
    const layout = ids.map((id) => byId.get(id)!).filter(Boolean);
    persistCache(layout);
    set({ layout });
    void putDashboardLayout(widgetsToWire(layout));
  },

  toggle: (id) => {
    const layout = get().layout.map((w) => (w.id === id ? { ...w, enabled: !w.enabled } : w));
    persistCache(layout);
    set({ layout });
    void putDashboardLayout(widgetsToWire(layout));
  },

  setWidgetOptions: (id, patch) => {
    const layout = get().layout.map((w) => (w.id === id ? { ...w, ...patch } : w));
    persistCache(layout);
    set({ layout });
    void putDashboardLayout(widgetsToWire(layout));
  },

  reset: async () => {
    const res = await postDashboardLayoutReset();
    const layout = widgetsFromWire(res.widgets);
    persistCache(layout);
    set({ layout });
  },
}));

/** A widget's polling state and the cadence it drives, plus the toggle. */
export function useWidgetPolling(id: WidgetId) {
  const widgetDef = useDashboard((s) => s.layout.find((w) => w.id === id));
  const setWidgetOptions = useDashboard((s) => s.setWidgetOptions);
  const pollingEnabled = widgetDef?.pollingEnabled ?? false;
  return {
    pollingEnabled,
    refetchInterval: pollingEnabled ? WIDGET_POLL_INTERVAL_MS : (false as const),
    togglePolling: () => setWidgetOptions(id, { pollingEnabled: !pollingEnabled }),
  };
}
