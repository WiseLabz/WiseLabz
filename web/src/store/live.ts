/**
 * Live runtime state fed by the WebSocket layer (mock timeline in dev).
 * Kept deliberately small: sync progress, ws connection health, pending alerts,
 * and a rolling activity feed the dashboard's sync widget renders.
 */
import { create } from 'zustand';
import type { ServiceStatus } from '../api/model';
import type { SyncPhase } from '../types/ws';

export type WsState = 'connecting' | 'open' | 'closed';

export interface SyncJob {
  jobId: string;
  serviceId: string | null;
  phase: SyncPhase;
  percent: number;
  message?: string;
  startedAt: number;
}

export interface ActivityEntry {
  id: string;
  kind: 'sync' | 'change' | 'alert' | 'doc' | 'status';
  label: string;
  detail?: string;
  at: string;
  tone: 'signal' | 'ok' | 'warn' | 'err' | 'idle';
}

interface LiveState {
  ws: WsState;
  setWs: (s: WsState) => void;

  /** active sync jobs keyed by serviceId ('global' for fleet-wide) */
  jobs: Record<string, SyncJob>;
  upsertJob: (j: SyncJob) => void;
  clearJob: (key: string) => void;

  /** transient per-service status overrides from service.status frames */
  statusOverrides: Record<string, ServiceStatus>;
  setStatus: (serviceId: string, status: ServiceStatus) => void;

  pendingAlerts: number;
  setPendingAlerts: (n: number) => void;
  bumpAlerts: (delta: number) => void;

  activity: ActivityEntry[];
  pushActivity: (e: ActivityEntry) => void;
}

const jobKey = (serviceId: string | null) => serviceId ?? 'global';

export const useLive = create<LiveState>((set) => ({
  ws: 'connecting',
  setWs: (ws) => set({ ws }),

  jobs: {},
  upsertJob: (j) =>
    set((s) => ({ jobs: { ...s.jobs, [jobKey(j.serviceId)]: j } })),
  clearJob: (key) =>
    set((s) => {
      const next = { ...s.jobs };
      delete next[key];
      return { jobs: next };
    }),

  statusOverrides: {},
  setStatus: (serviceId, status) =>
    set((s) => ({ statusOverrides: { ...s.statusOverrides, [serviceId]: status } })),

  pendingAlerts: 0,
  setPendingAlerts: (pendingAlerts) => set({ pendingAlerts }),
  bumpAlerts: (delta) =>
    set((s) => ({ pendingAlerts: Math.max(0, s.pendingAlerts + delta) })),

  activity: [],
  pushActivity: (e) => set((s) => ({ activity: [e, ...s.activity].slice(0, 40) })),
}));

export { jobKey };
