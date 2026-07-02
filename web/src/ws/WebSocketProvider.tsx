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
import { getGetAlertsQueryKey } from '../api/generated/alerts/alerts';
import { getGetDocsTreeQueryKey } from '../api/generated/docs/docs';
import { useLive } from '../store/live';
import { toast } from '../lib/toast';
import { navigateTo } from '../lib/navigation';
import i18n from '../i18n';
import { useAuth } from '../store/auth';
import { getAccessToken } from '../api/axios-instance';
import type { WsEvent } from '../types/ws';

const jump = (to: string) => ({
  label: i18n.t('notify.view'),
  onClick: () => navigateTo(to),
});

const wsUrl = () => {
  const base = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/ws`;
  const token = getAccessToken();
  return token ? `${base}?access_token=${encodeURIComponent(token)}` : base;
};

export function WebSocketProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient();
  const live = useRef(useLive.getState()).current; // stable store handle
  const reconnects = useRef(0);
  const status = useAuth((s) => s.status);

  useEffect(() => {
    if (status !== 'authenticated') {
      useLive.getState().setWs('closed');
      return;
    }

    let socket: WebSocket | null = null;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let closed = false;

    const connect = () => {
      useLive.getState().setWs('connecting');
      socket = new WebSocket(wsUrl());

      socket.onopen = () => {
        reconnects.current = 0;
        useLive.getState().setWs('open');
      };

      socket.onclose = () => {
        useLive.getState().setWs('closed');
        if (closed) return;
        const delay = Math.min(1000 * 2 ** reconnects.current++, 15000);
        retry = setTimeout(connect, delay);
      };

      socket.onmessage = (ev) => {
        let frame: WsEvent;
        try {
          frame = JSON.parse(ev.data as string) as WsEvent;
        } catch {
          return;
        }
        handle(frame, qc);
      };
    };

    connect();
    return () => {
      closed = true;
      if (retry) clearTimeout(retry);
      socket?.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status]);

  // touch `live` so the ref isn't flagged unused under noUnusedLocals
  void live;
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
        id: frame.id ?? crypto.randomUUID(),
        kind: 'sync',
        label: 'Sync complete',
        detail: `${p.changesDetected} change${p.changesDetected === 1 ? '' : 's'} · ${p.alertsRaised} alert${p.alertsRaised === 1 ? '' : 's'}`,
        at: frame.ts,
        tone: p.alertsRaised > 0 ? 'warn' : 'ok',
      });
      qc.invalidateQueries({ queryKey: getGetDashboardOverviewQueryKey() });
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
      qc.invalidateQueries({ queryKey: getGetChangesQueryKey() });
      qc.invalidateQueries({ queryKey: getGetDashboardOverviewQueryKey() });
      {
        const opts = { action: jump(`/changes/${p.changeId}`) };
        if (p.severity === 'critical') toast.error(p.summary, opts);
        else if (p.severity === 'warning') toast.warning(p.summary, opts);
        else toast.info(p.summary, opts);
      }
      break;
    }
    case 'alert.created': {
      const p = frame.payload;
      s.bumpAlerts(1);
      s.pushActivity({
        id: p.alertId,
        kind: 'alert',
        label: p.title,
        detail: 'New alert',
        at: frame.ts,
        tone: p.severity === 'critical' ? 'err' : 'warn',
      });
      qc.invalidateQueries({ queryKey: getGetAlertsQueryKey() });
      {
        const opts = { action: jump('/alerts') };
        if (p.severity === 'critical') toast.error(p.title, opts);
        else toast.warning(p.title, opts);
      }
      break;
    }
    case 'alert.resolved': {
      s.bumpAlerts(-1);
      qc.invalidateQueries({ queryKey: getGetAlertsQueryKey() });
      break;
    }
    case 'doc.generated': {
      s.pushActivity({
        id: frame.id ?? crypto.randomUUID(),
        kind: 'doc',
        label: 'Documentation regenerated',
        detail: `${frame.payload.trigger} · v${frame.payload.newVersion}`,
        at: frame.ts,
        tone: 'signal',
      });
      qc.invalidateQueries({ queryKey: getGetDocsTreeQueryKey() });
      break;
    }
    default:
      break;
  }
}
