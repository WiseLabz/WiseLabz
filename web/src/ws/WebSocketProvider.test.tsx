import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { getGetChangesQueryKey } from '../api/generated/changes/changes';
import { getGetDashboardOverviewQueryKey } from '../api/generated/dashboard/dashboard';
import { useAuth } from '../store/auth';
import { WebSocketProvider } from './WebSocketProvider';

class TestWebSocket {
  static last: TestWebSocket | undefined;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(_url: string) {
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
    window.WebSocket = originalWebSocket;
    useAuth.setState({ status: 'unknown', user: null });
    TestWebSocket.last = undefined;
  });

  it('invalidates the dashboard and changes caches after a completed sync', async () => {
    window.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    useAuth.setState({ status: 'authenticated' });
    const queryClient = new QueryClient();
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    render(
      <QueryClientProvider client={queryClient}>
        <WebSocketProvider><div /></WebSocketProvider>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(TestWebSocket.last).toBeDefined());
    act(() => TestWebSocket.last?.onmessage?.(new MessageEvent('message', {
      data: JSON.stringify({
        type: 'sync.complete',
        id: 'sync-1',
        ts: '2026-09-06T12:00:00Z',
        payload: { serviceId: 'svc-1', jobId: 'job-1', changesDetected: 0, alertsRaised: 0, durationMs: 1 },
      }),
    })));

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetDashboardOverviewQueryKey() });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: getGetChangesQueryKey() });
  });
});
