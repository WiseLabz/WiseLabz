/**
 * Fires a scripted sync over the mock WS so the global-sync button feels live in
 * dev (progress ramp → complete → a detected change). Against a real backend this
 * would be a POST /sync; here we drive the same frames the contract defines, so
 * the dashboard's sync sweep + activity feed react exactly as they will in prod.
 */
import type { SyncPhase, WsEvent } from '../types/ws';

const RAMP: [SyncPhase, number, number][] = [
  ['queued', 0, 0],
  ['fetching', 28, 350],
  ['diffing', 60, 750],
  ['generating', 85, 1150],
  ['done', 100, 1500],
];

export function triggerMockSync(serviceId: string | null = null) {
  const w = window.__wsMock;
  if (!w) return false;
  const jobId = `job-${Date.now().toString(36)}`;

  for (const [phase, percent, at] of RAMP) {
    setTimeout(() => {
      w.emit(w.env('sync.progress', { serviceId, jobId, phase, percent }) as WsEvent);
    }, at);
  }
  setTimeout(() => {
    w.emit(
      w.env('sync.complete', {
        serviceId,
        jobId,
        changesDetected: 1,
        alertsRaised: 0,
        durationMs: 1500,
      }) as WsEvent,
    );
    w.emit(
      w.env('change.detected', {
        changeId: `chg-${Date.now().toString(36)}`,
        serviceId: serviceId ?? 'svc-pve1',
        changeType: 'config.reconciled',
        severity: 'info',
        summary: 'Configuration reconciled — documentation up to date',
        willTriggerAi: false,
      }) as WsEvent,
    );
  }, 1700);
  return true;
}
