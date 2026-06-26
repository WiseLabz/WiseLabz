/**
 * Scripted WS timeline — replays a realistic sequence of the 10 events from
 * WS_CONTRACT.md so every consumer can be exercised without a backend:
 *
 *   service status → a full sync (progress ramp → complete) → a detected change →
 *   an alert → a regenerated doc → a short AI-suggestion stream, plus a recurring
 *   health/status heartbeat.
 *
 * `startTimeline(emit)` returns a cleanup that clears all timers.
 */
import type { WsEnvelope, WsEventMap, WsEventType } from '../../types/ws';

const SERVICE_ID = 'svc-proxmox-1';
const JOB_ID = 'job-mock-1';
const DOC_ID = 'doc-lab';

const newId = (): string =>
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : Math.random().toString(36).slice(2);

/** Build a typed envelope with a fresh timestamp + id. */
export function env<K extends WsEventType>(
  type: K,
  payload: WsEventMap[K],
): WsEnvelope<WsEventMap[K]> {
  return { type, ts: new Date().toISOString(), payload, id: newId() };
}

type Emit = (e: WsEnvelope) => void;

interface ScheduledEvent {
  delayMs: number;
  build: () => WsEnvelope;
}

/** One-shot narrative, fired once after the socket opens. */
const scriptedEvents: ScheduledEvent[] = [
  {
    delayMs: 300,
    build: () =>
      env('service.status', {
        serviceId: SERVICE_ID,
        status: 'online',
        lastChecked: new Date().toISOString(),
      }),
  },
  { delayMs: 600, build: () => syncProgress('queued', 0) },
  { delayMs: 1000, build: () => syncProgress('fetching', 25) },
  { delayMs: 1400, build: () => syncProgress('diffing', 55) },
  { delayMs: 1800, build: () => syncProgress('generating', 80) },
  { delayMs: 2100, build: () => syncProgress('done', 100) },
  {
    delayMs: 2300,
    build: () =>
      env('sync.complete', {
        serviceId: SERVICE_ID,
        jobId: JOB_ID,
        changesDetected: 2,
        alertsRaised: 1,
        durationMs: 2000,
      }),
  },
  {
    delayMs: 2500,
    build: () =>
      env('change.detected', {
        changeId: 'chg-mock-1',
        serviceId: SERVICE_ID,
        changeType: 'vm.created',
        severity: 'warning',
        summary: 'New VM "k8s-worker-03" detected on node pve1',
        willTriggerAi: true,
      }),
  },
  {
    delayMs: 2700,
    build: () =>
      env('alert.created', {
        alertId: 'alt-mock-1',
        changeId: 'chg-mock-1',
        serviceId: SERVICE_ID,
        severity: 'warning',
        title: 'Undocumented VM created',
      }),
  },
  {
    delayMs: 2900,
    build: () =>
      env('doc.generated', {
        docId: DOC_ID,
        serviceId: SERVICE_ID,
        trigger: 'ai',
        newVersion: 4,
      }),
  },
  // Short AI-suggestion stream (only AiAssistPanel consumes it; harmless elsewhere).
  {
    delayMs: 3300,
    build: () =>
      env('doc.ai_suggestion', {
        docId: DOC_ID,
        requestId: 'req-mock-1',
        status: 'streaming',
        contentDelta: '## VMs\n\n| Name | Node |',
      }),
  },
  {
    delayMs: 3550,
    build: () =>
      env('doc.ai_suggestion', {
        docId: DOC_ID,
        requestId: 'req-mock-1',
        status: 'streaming',
        contentDelta: '\n| k8s-worker-03 | pve1 |',
      }),
  },
  {
    delayMs: 3800,
    build: () =>
      env('doc.ai_suggestion', {
        docId: DOC_ID,
        requestId: 'req-mock-1',
        status: 'complete',
        fullContent: '## VMs\n\n| Name | Node |\n| k8s-worker-03 | pve1 |',
      }),
  },
  {
    delayMs: 4200,
    build: () =>
      env('system.notice', {
        level: 'info',
        message: 'Mock mode — no backend connected. Data is fake.',
        action: 'none',
      }),
  },
];

function syncProgress(
  phase: WsEventMap['sync.progress']['phase'],
  percent: number,
): WsEnvelope {
  return env('sync.progress', {
    serviceId: SERVICE_ID,
    jobId: JOB_ID,
    phase,
    percent,
  });
}

/** Recurring heartbeat so liveness indicators keep ticking. */
const HEARTBEAT_MS = 8000;

function heartbeat(emit: Emit): void {
  emit(
    env('system.health', {
      status: 'ok',
      components: [
        { name: 'api', status: 'ok' },
        { name: 'diff-engine', status: 'ok' },
        { name: 'database', status: 'ok' },
      ],
    }),
  );
  emit(
    env('service.status', {
      serviceId: SERVICE_ID,
      status: 'online',
      lastChecked: new Date().toISOString(),
    }),
  );
}

export function startTimeline(emit: Emit): () => void {
  const timeouts = scriptedEvents.map((e) =>
    setTimeout(() => emit(e.build()), e.delayMs),
  );
  const interval = setInterval(() => heartbeat(emit), HEARTBEAT_MS);

  return () => {
    timeouts.forEach(clearTimeout);
    clearInterval(interval);
  };
}
