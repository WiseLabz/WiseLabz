/**
 * Network discovery scan state, fed by `discovery.*` WebSocket frames and
 * hydrated from GET /discovery/scan (on mount, after a reconnect and when a
 * scan completes). The server keeps one scan at a time, so the store holds the
 * current one; frames for any other scan id belong to an earlier scan and are
 * dropped.
 */
import { create } from 'zustand';
import type {
  DiscoveryCandidate,
  DiscoveryCandidateEvent,
  DiscoveryCompleteEvent,
  DiscoveryProgressEvent,
  DiscoveryScan,
} from '../api/model';

interface DiscoveryState {
  /** The current or most recent scan; null when none is held on the server. */
  scan: DiscoveryScan | null;
  /** Replace the scan, e.g. with the server's answer (or null for "no scan"). */
  setScan: (scan: DiscoveryScan | null) => void;
  applyProgress: (e: DiscoveryProgressEvent) => void;
  applyCandidate: (e: DiscoveryCandidateEvent) => void;
  applyComplete: (e: DiscoveryCompleteEvent) => void;
  /** Mark candidates as already connected, e.g. after the queue created them. */
  markConnected: (connected: { candidate: DiscoveryCandidate; connectorId: string }[]) => void;
}

const sameCandidate = (a: DiscoveryCandidate, b: DiscoveryCandidate) =>
  a.type === b.type && a.address === b.address && a.port === b.port;

export const useDiscovery = create<DiscoveryState>((set) => ({
  scan: null,
  setScan: (scan) => set({ scan }),
  applyProgress: (e) =>
    set((s) => {
      if (!s.scan || s.scan.id !== e.scanId || s.scan.state !== 'running') return s;
      return {
        scan: {
          ...s.scan,
          // Frames are throttled but not reordered by the server; never go back.
          done: Math.max(s.scan.done, e.done),
          total: e.total,
          answered: Math.max(s.scan.answered, e.answered),
        },
      };
    }),
  applyCandidate: (e) =>
    set((s) => {
      if (!s.scan || s.scan.id !== e.scanId || s.scan.state !== 'running') return s;
      if (s.scan.candidates.some((c) => sameCandidate(c, e.candidate))) return s;
      return { scan: { ...s.scan, candidates: [...s.scan.candidates, e.candidate] } };
    }),
  applyComplete: (e) =>
    set((s) => {
      if (!s.scan || s.scan.id !== e.scanId) return s;
      return {
        scan: {
          ...s.scan,
          state: e.state,
          partial: e.partial,
          endedAt: s.scan.endedAt ?? new Date().toISOString(),
        },
      };
    }),
  markConnected: (connected) =>
    set((s) => {
      if (!s.scan) return s;
      return {
        scan: {
          ...s.scan,
          candidates: s.scan.candidates.map((c) => {
            const hit = connected.find((x) => sameCandidate(x.candidate, c));
            return hit ? { ...c, connectorId: hit.connectorId } : c;
          }),
        },
      };
    }),
}));
