/**
 * Single WS connection for the app. In dev it transparently hits MockWebSocket
 * (installed by the mock layer), which replays the WS_CONTRACT timeline. Frames
 * update the live store (sync progress, status, alerts, activity feed) and
 * invalidate the relevant React Query caches so REST data stays fresh.
 */
import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { getGetDashboardOverviewQueryKey } from '../api/generated/dashboard/dashboard';
import { getGetChangesQueryKey } from '../api/generated/changes/changes';
import { getAlerts, getGetAlertsQueryKey } from '../api/generated/alerts/alerts';
import { getGetConnectorsQueryKey } from '../api/generated/connectors/connectors';
import { getGetFindingsQueryKey } from '../api/generated/findings/findings';
import { getGetNotificationsQueryKey } from '../api/generated/notifications/notifications';
import { getGetSearchQueryKey } from '../api/generated/search/search';
import { getGetDocsTreeQueryKey } from '../api/generated/docs/docs';
import {
  getGetRunbookRunQueryKey,
  getListRunbookRunsQueryKey,
} from '../api/generated/runbooks/runbooks';
import { useLive } from '../store/live';
import { toast } from '../lib/toast';
import { navigateTo } from '../lib/navigation';
import i18n from '../i18n';
import { refreshSession, useAuth } from '../store/auth';
import { customInstance } from '../api/axios-instance';
import type { WsEvent } from '../types/ws';

// A sync can emit hundreds of change/alert frames in a burst. Invalidations are
// coalesced per query key and toasts per kind, so a burst costs one refetch per
// list and one toast instead of N each.
const BURST_MS = 300;

const invalidateTimers = new Map<string, ReturnType<typeof setTimeout>>();
function invalidateDebounced(qc: ReturnType<typeof useQueryClient>, queryKey: readonly unknown[]) {
  const k = JSON.stringify(queryKey);
  clearTimeout(invalidateTimers.get(k));
  invalidateTimers.set(
    k,
    setTimeout(() => {
      invalidateTimers.delete(k);
      void qc.invalidateQueries({ queryKey });
    }, BURST_MS)
  );
}

type Burst = { count: number; first: () => void; summary: (n: number) => void };
const bursts = new Map<string, Burst & { timer: ReturnType<typeof setTimeout> }>();
// Buffers toasts of one kind; a lone event shows its own toast, a burst a summary.
function toastBurst(kind: string, first: () => void, summary: (n: number) => void) {
  const cur = bursts.get(kind);
  if (cur) {
    clearTimeout(cur.timer);
    cur.count++;
    cur.summary = summary;
  }
  const b = cur ?? { count: 1, first, summary, timer: undefined as never };
  b.timer = setTimeout(() => {
    bursts.delete(kind);
    if (b.count === 1) b.first();
    else b.summary(b.count);
  }, BURST_MS);
  bursts.set(kind, b);
}

const jump = (to: string) => ({
  label: i18n.t('notify.view'),
  onClick: () => navigateTo(to),
});

const wsUrl = (ticket: string) => {
  return `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/ws?ticket=${encodeURIComponent(ticket)}`;
};

// The socket is authorized by a one-time, short-lived ticket minted with the
// normal access token, so the refresh cookie is never the WS credential.
const fetchTicket = () =>
  customInstance<{ ticket: string }>({ url: '/ws/ticket', method: 'POST' }).then((r) => r.ticket);

// Recent event ids remembered for dedupe. Bounded so a long session can't grow it.
const SEEN_CAP = 500;

// crypto.randomUUID is undefined on non-secure (plain-HTTP LAN) origins.
const newId = () =>
  typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `local-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;

// Alert badge drifts if frames are missed; recount from the API this often.
const ALERT_RESYNC_MS = 60_000;

// Older servers may omit id/ts; fill them locally. A fallback id is never seen
// twice, so it is never deduped.
function normalize(raw: WsEvent): WsEvent {
  return {
    ...raw,
    id: raw.id || newId(),
    ts: raw.ts || new Date().toISOString(),
  } as WsEvent;
}

// The server never replays missed frames, so after a reconnect (or a server
// system.resync) recover by refetching the volatile queries over REST. Sync jobs
// are cleared too: a running one re-adds itself on its next sync.progress, so a
// missed sync.complete can't leave a stuck progress bar.
function resync(qc: ReturnType<typeof useQueryClient>) {
  useLive.getState().resetJobs();
  qc.invalidateQueries({ queryKey: getGetAlertsQueryKey() });
  qc.invalidateQueries({ queryKey: getGetSearchQueryKey() });
  qc.invalidateQueries({ queryKey: getGetChangesQueryKey() });
  qc.invalidateQueries({ queryKey: getGetConnectorsQueryKey() });
  qc.invalidateQueries({ queryKey: getGetDashboardOverviewQueryKey() });
  qc.invalidateQueries({ queryKey: getGetNotificationsQueryKey() });
  qc.invalidateQueries({
    predicate: (query) => {
      const path = query.queryKey[0];
      return (
        typeof path === 'string' &&
        (path.startsWith('/runbook-runs/') || /^\/runbooks\/[^/]+\/runs$/.test(path))
      );
    },
  });
}

export function WebSocketProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient();
  const reconnects = useRef(0);
  // Provider-level so dedupe survives reconnects: Set for lookup, array for FIFO eviction.
  const seen = useRef({ ids: new Set<string>(), order: [] as string[] }).current;
  const status = useAuth((s) => s.status);

  useEffect(() => {
    if (status !== 'authenticated') {
      useLive.getState().setWs('closed');
      return;
    }

    let socket: WebSocket | null = null;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let closed = false;
    const syncAlertCount = async () => {
      try {
        const page = await getAlerts({ status: 'pending', pageSize: 1 });
        if (!closed && typeof page?.total === 'number')
          useLive.getState().setPendingAlerts(page.total);
      } catch {
        // keep the current count; the next tick retries
      }
    };
    let hasOpened = false; // first open is a fresh load; later opens are reconnects

    const connect = async () => {
      useLive.getState().setWs('connecting');
      let ticket: string;
      try {
        ticket = await fetchTicket();
      } catch {
        if (closed) return;
        const delay = Math.min(1000 * 2 ** reconnects.current++, 15000);
        retry = setTimeout(connect, delay);
        return;
      }
      if (closed) return;
      socket = new WebSocket(wsUrl(ticket));

      socket.onopen = () => {
        reconnects.current = 0;
        useLive.getState().setWs('open');
        if (hasOpened) resync(qc);
        void syncAlertCount();
        hasOpened = true;
      };

      socket.onclose = () => {
        // A stale socket (after logout→login) must not clobber the new one's state.
        if (closed) return;
        useLive.getState().setWs('closed');
        const delay = Math.min(1000 * 2 ** reconnects.current++, 15000);
        retry = setTimeout(connect, delay);
      };

      socket.onmessage = (ev) => {
        let frame: WsEvent;
        try {
          frame = normalize(JSON.parse(ev.data as string) as WsEvent);
        } catch {
          return;
        }
        if (seen.ids.has(frame.id)) return;
        seen.ids.add(frame.id);
        seen.order.push(frame.id);
        if (seen.order.length > SEEN_CAP) seen.ids.delete(seen.order.shift()!);
        handle(frame, qc);
      };
    };

    void connect();
    const alertTimer = setInterval(() => void syncAlertCount(), ALERT_RESYNC_MS);
    return () => {
      closed = true;
      clearInterval(alertTimer);
      if (retry) clearTimeout(retry);
      socket?.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status]);

  return <>{children}</>;
}

function handle(frame: WsEvent, qc: ReturnType<typeof useQueryClient>) {
  const s = useLive.getState();
  switch (frame.type) {
    case 'service.status': {
      const p = frame.payload;
      s.setStatus(p.serviceId, p.status);
      qc.invalidateQueries({ queryKey: getGetDashboardOverviewQueryKey() });
      break;
    }
    case 'sync.progress': {
      const p = frame.payload;
      s.upsertJob({
        jobId: p.jobId,
        serviceId: p.serviceId,
        phase: p.phase,
        percent: p.percent,
        message: p.message,
        startedAt: Date.now(),
      });
      break;
    }
    case 'sync.complete': {
      const p = frame.payload;
      s.clearJob(p.serviceId ?? 'global');
      s.pushActivity({
        id: frame.id,
        kind: 'sync',
        label: i18n.t('notify.syncCompleteTitle'),
        detail: i18n.t('notify.syncCompleteDetail', {
          changes: p.changesDetected,
          alerts: p.alertsRaised,
        }),
        at: frame.ts,
        tone: p.alertsRaised > 0 ? 'warn' : 'ok',
      });
      qc.invalidateQueries({ queryKey: getGetDashboardOverviewQueryKey() });
      qc.invalidateQueries({ queryKey: getGetSearchQueryKey() });
      qc.invalidateQueries({ queryKey: getGetChangesQueryKey() });
      {
        const msg = i18n.t('notify.syncComplete', {
          changes: p.changesDetected,
          alerts: p.alertsRaised,
        });
        const opts = p.changesDetected > 0 ? { action: jump('/changes') } : undefined;
        if (p.alertsRaised > 0) toast.warning(msg, opts);
        else toast.success(msg, opts);
      }
      break;
    }
    case 'change.detected': {
      const p = frame.payload;
      s.pushActivity({
        id: p.changeId,
        kind: 'change',
        label: p.summary,
        detail: p.changeType,
        at: frame.ts,
        tone: p.severity === 'critical' ? 'err' : p.severity === 'warning' ? 'warn' : 'signal',
      });
      invalidateDebounced(qc, getGetChangesQueryKey());
      invalidateDebounced(qc, getGetDashboardOverviewQueryKey());
      toastBurst(
        'change.detected',
        () => {
          const opts = { action: jump(`/changes/${p.changeId}`) };
          if (p.severity === 'critical') toast.error(p.summary, opts);
          else if (p.severity === 'warning') toast.warning(p.summary, opts);
          else toast.info(p.summary, opts);
        },
        (n) =>
          toast.info(i18n.t('notify.changesDetected', { count: n }), { action: jump('/changes') })
      );
      break;
    }
    case 'alert.created': {
      const p = frame.payload;
      s.bumpAlerts(1);
      s.pushActivity({
        id: p.alertId,
        kind: 'alert',
        label: p.title,
        detail: i18n.t('notify.newAlert'),
        at: frame.ts,
        tone: p.severity === 'critical' ? 'err' : 'warn',
      });
      invalidateDebounced(qc, getGetAlertsQueryKey());
      invalidateDebounced(qc, getGetNotificationsQueryKey());
      toastBurst(
        'alert.created',
        () => {
          const opts = { action: jump('/alerts') };
          if (p.severity === 'critical') toast.error(p.title, opts);
          else toast.warning(p.title, opts);
        },
        (n) =>
          toast.warning(i18n.t('notify.alertsCreated', { count: n }), { action: jump('/alerts') })
      );
      break;
    }
    case 'alert.resolved': {
      s.bumpAlerts(-1);
      qc.invalidateQueries({ queryKey: getGetAlertsQueryKey() });
      qc.invalidateQueries({ queryKey: getGetNotificationsQueryKey() });
      break;
    }
    case 'quality.finding.created':
    case 'quality.findings.changed': {
      qc.invalidateQueries({ queryKey: getGetFindingsQueryKey() });
      break;
    }
    case 'finding.created':
    case 'system.job_failed':
    case 'runbook.run_failed':
    case 'runbook.run_waiting': {
      // Per-user notification dispatch (Dispatcher.NotifyFindingCreated /
      // NotifySystemEvent), not the broadcast-to-everyone
      // 'quality.finding.created' above — refresh the notification bell the
      // same way 'alert.resolved' does.
      qc.invalidateQueries({ queryKey: getGetNotificationsQueryKey() });
      break;
    }
    case 'runbook.run.updated': {
      // Events carry identifiers and state only; REST applies step redaction.
      void qc.invalidateQueries({ queryKey: getGetRunbookRunQueryKey(frame.payload.runId) });
      if (frame.payload.runbookId) {
        void qc.invalidateQueries({
          queryKey: getListRunbookRunsQueryKey(frame.payload.runbookId),
        });
      }
      break;
    }
    case 'doc.generated': {
      s.pushActivity({
        id: frame.id,
        kind: 'doc',
        label: i18n.t('notify.documentationRegenerated'),
        detail: i18n.t('notify.documentationRegeneratedDetail', {
          trigger: frame.payload.trigger,
          version: frame.payload.newVersion,
        }),
        at: frame.ts,
        tone: 'signal',
      });
      qc.invalidateQueries({ queryKey: getGetDocsTreeQueryKey() });
      qc.invalidateQueries({ queryKey: getGetSearchQueryKey() });
      break;
    }
    case 'doc.lock.acquired': {
      const p = frame.payload;
      s.setDocLock(p.docId, { userId: p.userId, userName: p.userName, expiresAt: p.expiresAt });
      break;
    }
    case 'doc.lock.released': {
      const p = frame.payload;
      s.setDocLock(p.docId, undefined);
      break;
    }
    case 'doc.lock.expired': {
      const p = frame.payload;
      s.setDocLock(p.docId, undefined);
      break;
    }
    case 'system.resync': {
      resync(qc);
      break;
    }
    case 'system.notice': {
      const p = frame.payload;
      const tone =
        p.level === 'critical' ? toast.error : p.level === 'warning' ? toast.warning : toast.info;
      if (p.action === 'reload')
        tone(p.message, {
          action: { label: i18n.t('notify.reload'), onClick: () => location.reload() },
        });
      else tone(p.message);
      if (p.action === 'reauth') void refreshSession();
      break;
    }
    case 'system.health': {
      qc.invalidateQueries({ queryKey: getGetDashboardOverviewQueryKey() });
      if (frame.payload.status !== 'ok') toast.warning(i18n.t('notify.healthDegraded'));
      break;
    }
    case 'doc.ai_suggestion': {
      const p = frame.payload;
      // Cache by both ids: a result may arrive before the POST response.
      qc.setQueryData(['doc-ai-suggestion', p.docId, p.requestId], p);
      break;
    }
    default:
      break;
  }
}
