import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { getGetAlertsQueryKey } from '../api/generated/alerts/alerts';
import { getGetSearchQueryKey } from '../api/generated/search/search';
import { getGetChangesQueryKey } from '../api/generated/changes/changes';
import { getGetConnectorsQueryKey } from '../api/generated/connectors/connectors';
import { getGetDashboardOverviewQueryKey } from '../api/generated/dashboard/dashboard';
import { getGetNotificationsQueryKey } from '../api/generated/notifications/notifications';
import {
  getGetRunbookRunQueryKey,
  getListRunbookRunsQueryKey,
} from '../api/generated/runbooks/runbooks';
import { getGetDiscoveryScanQueryKey } from '../api/generated/discovery/discovery';
import i18n from '../i18n';
import { useAuth } from '../store/auth';
import { useDiscovery } from '../store/discovery';
import { useLive } from '../store/live';
import { WebSocketProvider } from './WebSocketProvider';

vi.mock('../api/axios-instance', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api/axios-instance')>()),
  customInstance: vi.fn().mockResolvedValue({ ticket: 'test-ticket' }),
}));

class TestWebSocket {
  static last: TestWebSocket | undefined;
  static urls: string[] = [];
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(url: string) {
    TestWebSocket.urls.push(url);
    TestWebSocket.last = this;
    queueMicrotask(() => this.onopen?.(new Event('open')));
  }

  close() {
    this.onclose?.(new Event('close'));
  }
}

describe('WebSocketProvider', () => {
  const originalWebSocket = window.WebSocket;

  afterEach(() => {
    vi.useRealTimers();
    window.WebSocket = originalWebSocket;
    useAuth.setState({ status: 'unknown', user: null });
    useLive.setState({ activity: [], jobs: {}, pendingAlerts: 0, statusOverrides: {} });
    TestWebSocket.last = undefined;
    TestWebSocket.urls = [];
  });

  it('invalidates the dashboard and changes caches after a completed sync', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
    const translate = vi.spyOn(i18n, 't');

    render(
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );

    await waitFor(() => expect(TestWebSocket.last).toBeDefined());
    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'sync.complete',
            id: 'sync-1',
            ts: '2026-09-06T12:00:00Z',
            payload: {
              serviceId: 'svc-1',
              jobId: 'job-1',
              changesDetected: 0,
              alertsRaised: 0,
              durationMs: 1,
            },
          }),
        })
      )
    );

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetDashboardOverviewQueryKey() });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetChangesQueryKey() });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetSearchQueryKey() });
    expect(useLive.getState().activity[0]).toMatchObject({
      label: 'Sync complete',
      detail: '0 change(s) · 0 alert(s)',
    });
    expect(translate).toHaveBeenCalledWith('notify.syncCompleteTitle');
    expect(translate).toHaveBeenCalledWith('notify.syncCompleteDetail', { changes: 0, alerts: 0 });
  });

  it('stores localized activity copy for alert and documentation notices', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    const translate = vi.spyOn(i18n, 't');
    render(
      <QueryClientProvider client={new QueryClient()}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );
    await waitFor(() => expect(TestWebSocket.last).toBeDefined());

    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'alert.created',
            ts: '2026-09-06T12:00:00Z',
            payload: {
              alertId: 'alert-1',
              serviceId: 'svc-1',
              severity: 'warning',
              title: 'Disk space low',
            },
          }),
        })
      )
    );
    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'doc.generated',
            ts: '2026-09-06T12:00:00Z',
            payload: { docId: 'doc-1', trigger: 'template', newVersion: 3 },
          }),
        })
      )
    );

    expect(useLive.getState().activity).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ label: 'Disk space low', detail: 'New alert' }),
        expect.objectContaining({
          label: 'Documentation regenerated',
          detail: 'template · v3',
        }),
      ])
    );
    expect(translate).toHaveBeenCalledWith('notify.newAlert');
    expect(translate).toHaveBeenCalledWith('notify.documentationRegenerated');
    expect(translate).toHaveBeenCalledWith('notify.documentationRegeneratedDetail', {
      trigger: 'template',
      version: 3,
    });
  });

  const renderProvider = async (queryClient = new QueryClient()) => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    render(
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );
    await waitFor(() => expect(TestWebSocket.last).toBeDefined());
  };

  const send = (frame: Record<string, unknown>) =>
    act(() =>
      TestWebSocket.last?.onmessage?.(new MessageEvent('message', { data: JSON.stringify(frame) }))
    );

  const alertFrame = (extra: Record<string, unknown>, alertId: string) => ({
    type: 'alert.created',
    payload: { alertId, serviceId: 'svc-1', severity: 'warning', title: 'Disk space low' },
    ...extra,
  });

  it('refetches run detail and history for every run update without caching the event payload', async () => {
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
    const refetch = vi
      .fn()
      .mockResolvedValue({
        id: 'run-1',
        state: 'succeeded',
        steps: [{ title: 'Server-redacted detail' }],
      });
    await renderProvider(queryClient);
    const { QueryObserver } = await import('@tanstack/react-query');
    const observer = new QueryObserver(queryClient, {
      queryKey: getGetRunbookRunQueryKey('run-1'),
      queryFn: refetch,
    });
    const unsubscribe = observer.subscribe(() => {});
    await waitFor(() => expect(refetch).toHaveBeenCalledTimes(1));

    for (const id of ['run-update-1', 'run-update-2']) {
      send({
        type: 'runbook.run.updated',
        id,
        payload: {
          runId: 'run-1',
          runbookId: 'rb-1',
          state: 'running',
          step: { id: 'step-1', state: 'running' },
        },
      });
      await waitFor(() => expect(refetch).toHaveBeenCalledTimes(id === 'run-update-1' ? 2 : 3));
    }
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetRunbookRunQueryKey('run-1') });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: getListRunbookRunsQueryKey('rb-1'),
    });
    expect(queryClient.getQueryData(getGetRunbookRunQueryKey('run-1'))).toMatchObject({
      state: 'succeeded',
      steps: [{ title: 'Server-redacted detail' }],
    });
    unsubscribe();
  });

  it('refetches only the run named by the update', async () => {
    const queryClient = new QueryClient();
    await renderProvider(queryClient);
    const { QueryObserver } = await import('@tanstack/react-query');
    const refetchRun1 = vi.fn().mockResolvedValue({ id: 'run-1' });
    const refetchRun2 = vi.fn().mockResolvedValue({ id: 'run-2' });
    const unsubscribe = [
      new QueryObserver(queryClient, {
        queryKey: getGetRunbookRunQueryKey('run-1'),
        queryFn: refetchRun1,
      }),
      new QueryObserver(queryClient, {
        queryKey: getGetRunbookRunQueryKey('run-2'),
        queryFn: refetchRun2,
      }),
    ].map((observer) => observer.subscribe(() => {}));
    await waitFor(() => {
      expect(refetchRun1).toHaveBeenCalledTimes(1);
      expect(refetchRun2).toHaveBeenCalledTimes(1);
    });

    send({
      type: 'runbook.run.updated',
      id: 'run-2-update',
      payload: { runId: 'run-2', runbookId: 'rb-1', state: 'running' },
    });

    await waitFor(() => expect(refetchRun2).toHaveBeenCalledTimes(2));
    expect(refetchRun1).toHaveBeenCalledTimes(1);
    unsubscribe.forEach((stop) => stop());
  });

  it.each(['runbook.run_failed', 'runbook.run_waiting'])(
    'refreshes the notification bell on %s',
    async (type) => {
      const queryClient = new QueryClient();
      const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
      await renderProvider(queryClient);
      send({
        type,
        id: type,
        payload: { alertId: '', title: 'Run update', message: 'Step needs attention' },
      });
      expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetNotificationsQueryKey() });
    }
  );

  it('handles a frame with a repeated id only once', async () => {
    await renderProvider();
    const frame = alertFrame({ id: 'evt-1', ts: '2026-09-06T12:00:00Z' }, 'alert-1');

    send(frame);
    send(frame);

    expect(useLive.getState().pendingAlerts).toBe(1);
    expect(useLive.getState().activity).toHaveLength(1);
  });

  it('coalesces an alert burst into one refetch of each list', async () => {
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
    await renderProvider(queryClient);

    for (let i = 0; i < 5; i++) send(alertFrame({ id: `burst-${i}` }, `alert-${i}`));

    // Counters and the activity feed stay per-event; only refetches are deferred.
    expect(useLive.getState().pendingAlerts).toBe(5);
    expect(invalidateQueries).not.toHaveBeenCalled();
    await waitFor(() => expect(invalidateQueries).toHaveBeenCalledTimes(2));
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetAlertsQueryKey() });
  });

  it('handles a connector-scoped frame carrying connectorId', async () => {
    await renderProvider();

    send(alertFrame({ id: 'evt-3', ts: '2026-09-06T12:00:00Z', connectorId: 'svc-1' }, 'alert-1'));

    expect(useLive.getState().pendingAlerts).toBe(1);
    expect(useLive.getState().activity).toHaveLength(1);
  });

  it('still handles frames without an id, each one', async () => {
    await renderProvider();

    send(alertFrame({ ts: '2026-09-06T12:00:00Z' }, 'alert-1'));
    send(alertFrame({ ts: '2026-09-06T12:00:00Z' }, 'alert-2'));

    expect(useLive.getState().pendingAlerts).toBe(2);
  });

  it('fills a missing ts with a valid ISO timestamp', async () => {
    await renderProvider();

    send(alertFrame({ id: 'evt-2' }, 'alert-1'));

    const at = useLive.getState().activity[0]?.at;
    expect(at).toBeDefined();
    expect(new Date(at as string).toISOString()).toBe(at);
  });

  it('never puts the access token in the WebSocket URL', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );
    await waitFor(() => expect(TestWebSocket.urls).toHaveLength(1));
    expect(TestWebSocket.urls[0]).toMatch(/\/api\/ws\?ticket=test-ticket$/);
  });

  it('updates docLocks store on doc lock events', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );
    await waitFor(() => expect(TestWebSocket.last).toBeDefined());

    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'doc.lock.acquired',
            ts: '2026-09-06T12:00:00Z',
            payload: {
              docId: 'doc-1',
              userId: 'user-1',
              userName: 'Ada',
              acquiredAt: '2026-09-06T12:00:00Z',
              expiresAt: '2026-09-06T12:05:00Z',
            },
          }),
        })
      )
    );

    expect(useLive.getState().docLocks['doc-1']).toEqual({
      userId: 'user-1',
      userName: 'Ada',
      expiresAt: '2026-09-06T12:05:00Z',
    });

    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'doc.lock.released',
            ts: '2026-09-06T12:01:00Z',
            payload: { docId: 'doc-1', userId: 'user-1' },
          }),
        })
      )
    );

    expect(useLive.getState().docLocks['doc-1']).toBeUndefined();

    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'doc.lock.acquired',
            ts: '2026-09-06T12:02:00Z',
            payload: {
              docId: 'doc-2',
              userId: 'user-2',
              acquiredAt: '2026-09-06T12:02:00Z',
              expiresAt: '2026-09-06T12:07:00Z',
            },
          }),
        })
      )
    );

    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({
            type: 'doc.lock.expired',
            ts: '2026-09-06T12:07:30Z',
            payload: { docId: 'doc-2', userId: 'user-2' },
          }),
        })
      )
    );

    expect(useLive.getState().docLocks['doc-2']).toBeUndefined();
  });

  it('gracefully drops malformed messages without crashing or mutating state', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    render(
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );

    await waitFor(() => expect(TestWebSocket.last).toBeDefined());

    act(() =>
      TestWebSocket.last?.onmessage?.(new MessageEvent('message', { data: 'not valid json{{{' }))
    );

    expect(useLive.getState().activity).toEqual([]);
    expect(invalidateQueries).not.toHaveBeenCalled();
  });

  it('reconnects with exponential backoff after socket closes', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });

    const { unmount } = render(
      <QueryClientProvider client={new QueryClient()}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );

    // Wait for initial connection with real timers
    vi.useRealTimers();
    await waitFor(() => expect(TestWebSocket.last).toBeDefined());
    vi.useFakeTimers({ shouldAdvanceTime: true });

    const initialUrlCount = TestWebSocket.urls.length;

    act(() => TestWebSocket.last?.onclose?.(new Event('close')));

    expect(useLive.getState().ws).toBe('closed');
    expect(TestWebSocket.urls.length).toBe(initialUrlCount);

    await vi.advanceTimersByTimeAsync(1000);

    expect(TestWebSocket.urls.length).toBe(initialUrlCount + 1);

    unmount();
  });

  const volatileKeys = () => [
    getGetAlertsQueryKey(),
    getGetSearchQueryKey(),
    getGetChangesQueryKey(),
    getGetConnectorsQueryKey(),
    getGetDashboardOverviewQueryKey(),
    getGetNotificationsQueryKey(),
  ];

  const invalidatedKeys = (spy: { mock: { calls: unknown[][] } }) =>
    spy.mock.calls
      .filter((c) => 'queryKey' in (c[0] as object))
      .map((c) => (c[0] as { queryKey: unknown }).queryKey);

  it('does not resync on the first open', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    render(
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );

    await waitFor(() => expect(useLive.getState().ws).toBe('open'));
    expect(invalidateQueries).not.toHaveBeenCalled();
  });

  it('refetches volatile queries and clears sync jobs after a reconnect', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    const { unmount } = render(
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );

    await vi.advanceTimersByTimeAsync(0);
    expect(useLive.getState().ws).toBe('open');
    expect(invalidateQueries).not.toHaveBeenCalled();

    useLive.getState().upsertJob({
      jobId: 'job-1',
      serviceId: 'svc-1',
      phase: 'fetching',
      percent: 10,
      startedAt: 0,
    });

    act(() => TestWebSocket.last?.onclose?.(new Event('close')));
    await vi.advanceTimersByTimeAsync(1000);

    expect(useLive.getState().ws).toBe('open');
    expect(invalidatedKeys(invalidateQueries)).toEqual(volatileKeys());
    expect(useLive.getState().jobs).toEqual({});

    unmount();
  });

  it('refetches volatile queries and clears sync jobs on system.resync', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');
    queryClient.setQueryData(getGetRunbookRunQueryKey('missed-run'), { state: 'running' });
    queryClient.setQueryData(getListRunbookRunsQueryKey('rb-1', { page: 2 }), { items: [] });

    render(
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider>
          <div />
        </WebSocketProvider>
      </QueryClientProvider>
    );

    await waitFor(() => expect(useLive.getState().ws).toBe('open'));
    useLive.getState().upsertJob({
      jobId: 'job-1',
      serviceId: null,
      phase: 'fetching',
      percent: 10,
      startedAt: 0,
    });

    act(() =>
      TestWebSocket.last?.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({ type: 'system.resync', ts: '2026-09-06T12:00:00Z', payload: {} }),
        })
      )
    );

    expect(invalidatedKeys(invalidateQueries)).toEqual(volatileKeys());
    expect(useLive.getState().jobs).toEqual({});
    expect(queryClient.getQueryState(getGetRunbookRunQueryKey('missed-run'))?.isInvalidated).toBe(
      true
    );
    expect(
      queryClient.getQueryState(getListRunbookRunsQueryKey('rb-1', { page: 2 }))?.isInvalidated
    ).toBe(true);
  });
  it('delivers AI results through the cache keyed by document and request', async () => {
    const client = new QueryClient();
    await renderProvider(client);
    const payload = {
      docId: 'doc-1',
      requestId: 'req-1',
      status: 'complete',
      fullContent: '# Server result',
    };
    send({ type: 'doc.ai_suggestion', id: 'ai-1', payload });
    expect(client.getQueryData(['doc-ai-suggestion', 'doc-1', 'req-1'])).toEqual(payload);
    expect(client.getQueryData(['doc-ai-suggestion', 'doc-2', 'req-1'])).toBeUndefined();
    expect(client.getQueryData(['doc-ai-suggestion', 'doc-1', 'req-2'])).toBeUndefined();
  });

  it('feeds network discovery frames into the discovery store and refetches the scan when it completes', async () => {
    const client = new QueryClient();
    const invalidateQueries = vi.spyOn(client, 'invalidateQueries');
    await renderProvider(client);
    useDiscovery.setState({
      scan: {
        id: 'scan-1',
        cidr: '10.0.0.0/24',
        state: 'running',
        startedAt: '2026-10-07T12:00:00Z',
        done: 0,
        total: 254,
        answered: 0,
        partial: false,
        candidates: [],
      },
    });
    const candidate = {
      type: 'proxmox',
      name: 'Proxmox VE',
      address: '10.0.0.5',
      port: 8006,
      url: 'https://10.0.0.5:8006/api2/json',
      urlField: 'url',
    };

    send({ type: 'discovery.progress', id: 'd-1', payload: { scanId: 'scan-1', done: 80, total: 254, answered: 1 } });
    send({ type: 'discovery.candidate', id: 'd-2', payload: { scanId: 'scan-1', candidate } });
    // A frame of an earlier scan is dropped.
    send({ type: 'discovery.candidate', id: 'd-3', payload: { scanId: 'old', candidate: { ...candidate, port: 1 } } });
    expect(useDiscovery.getState().scan).toMatchObject({ done: 80, answered: 1, candidates: [candidate] });
    expect(invalidateQueries).not.toHaveBeenCalledWith({ queryKey: getGetDiscoveryScanQueryKey() });

    send({ type: 'discovery.complete', id: 'd-4', payload: { scanId: 'scan-1', state: 'completed', partial: false } });
    expect(useDiscovery.getState().scan?.state).toBe('completed');
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetDiscoveryScanQueryKey() });
    useDiscovery.setState({ scan: null });
  });
});
