import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import { UptimePanel } from './UptimePanel';
import { sparkGeometry } from './sparkGeometry';
import type { UptimeHistory } from '../../api/model';

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
});
afterAll(() => server.close());

const win = (over: Record<string, number>) => ({
  windowStart: '2026-01-01T00:00:00Z',
  windowEnd: '2026-01-02T00:00:00Z',
  checkCount: 0,
  availabilityPct: 0,
  mttrSeconds: 0,
  outageCount: 0,
  ...over,
});

function renderPanel() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <UptimePanel id="c1" />
    </QueryClientProvider>
  );
}

describe('UptimePanel', () => {
  it('shows availability and MTTR, and "No data" for windows without checks', async () => {
    server.use(
      http.get('/api/connectors/c1/uptime', () =>
        HttpResponse.json({
          connectorId: 'c1',
          windows: {
            '24h': win({ checkCount: 10, availabilityPct: 99.5, mttrSeconds: 120, outageCount: 1 }),
            '7d': win({ checkCount: 0 }),
            '30d': win({ checkCount: 0 }),
          },
        })
      ),
      http.get('/api/connectors/c1/uptime/history', () =>
        HttpResponse.json({
          connectorId: 'c1',
          window: '24h',
          windowStart: '2026-01-01T00:00:00Z',
          windowEnd: '2026-01-02T00:00:00Z',
          bucketSeconds: 900,
          buckets: [
            { start: '2026-01-01T00:00:00Z', status: 'online', avgLatencyMs: 120, checkCount: 3 },
            { start: '2026-01-01T00:15:00Z', status: 'offline', checkCount: 1 },
          ],
        })
      )
    );
    renderPanel();

    expect(await screen.findByText('99.50%')).toBeInTheDocument();
    expect(screen.getAllByText('No data')).toHaveLength(2);
    expect(screen.queryByText('0.00%')).not.toBeInTheDocument();
    expect(await screen.findByRole('img')).toBeInTheDocument();
  });

  it('shows an empty history message when no buckets exist', async () => {
    server.use(
      http.get('/api/connectors/c1/uptime', () =>
        HttpResponse.json({
          connectorId: 'c1',
          windows: { '24h': win({}), '7d': win({}), '30d': win({}) },
        })
      ),
      http.get('/api/connectors/c1/uptime/history', () =>
        HttpResponse.json({
          connectorId: 'c1',
          window: '24h',
          windowStart: '2026-01-01T00:00:00Z',
          windowEnd: '2026-01-02T00:00:00Z',
          bucketSeconds: 900,
          buckets: [],
        })
      )
    );
    renderPanel();

    expect(await screen.findByText('No checks recorded in this window.')).toBeInTheDocument();
    expect(screen.getAllByText('No data')).toHaveLength(3);
  });
});

const hist = (buckets: UptimeHistory['buckets']): UptimeHistory => ({
  connectorId: 'c1',
  window: '24h',
  windowStart: '2026-01-01T00:00:00Z',
  windowEnd: '2026-01-02T00:00:00Z',
  bucketSeconds: 900,
  buckets,
});

describe('sparkGeometry', () => {
  it('places buckets by time so gaps stay visible', () => {
    const g = sparkGeometry(
      hist([
        { start: '2026-01-01T00:00:00Z', status: 'online', avgLatencyMs: 100, checkCount: 1 },
        { start: '2026-01-01T12:00:00Z', status: 'online', avgLatencyMs: 100, checkCount: 1 },
      ])
    );
    expect(g.x('2026-01-01T00:00:00Z')).toBe(0);
    expect(g.x('2026-01-01T12:00:00Z')).toBeCloseTo(140);
    // 12h gap between buckets: two separate segments, not one joined line.
    expect(g.segments).toHaveLength(2);
  });

  it('joins adjacent buckets and breaks the line at missing latency', () => {
    const g = sparkGeometry(
      hist([
        { start: '2026-01-01T00:00:00Z', status: 'online', avgLatencyMs: 100, checkCount: 1 },
        { start: '2026-01-01T00:15:00Z', status: 'online', avgLatencyMs: 200, checkCount: 1 },
        { start: '2026-01-01T00:30:00Z', status: 'offline', checkCount: 1 },
        { start: '2026-01-01T00:45:00Z', status: 'online', avgLatencyMs: 50, checkCount: 1 },
      ])
    );
    expect(g.segments.map((s) => s.length)).toEqual([2, 1]);
    expect(g.maxLatency).toBe(200);
  });

  it('renders a gap as no status cell and a lone point as a marker', async () => {
    server.use(
      http.get('/api/connectors/c1/uptime', () =>
        HttpResponse.json({ connectorId: 'c1', windows: { '24h': win({}), '7d': win({}), '30d': win({}) } })
      ),
      http.get('/api/connectors/c1/uptime/history', () =>
        HttpResponse.json(
          hist([
            { start: '2026-01-01T00:00:00Z', status: 'online', avgLatencyMs: 100, checkCount: 1 },
            { start: '2026-01-01T12:00:00Z', status: 'offline', checkCount: 1 },
          ])
        )
      )
    );
    const { container } = renderPanel();
    await screen.findByRole('img');
    expect(container.querySelectorAll('[data-testid="status-cell"]')).toHaveLength(2);
    expect(container.querySelectorAll('circle[data-testid="latency-segment"]')).toHaveLength(1);
    expect(container.querySelectorAll('polyline')).toHaveLength(0);
  });
});
