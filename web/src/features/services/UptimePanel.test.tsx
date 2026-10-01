import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import { UptimePanel } from './UptimePanel';

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
        HttpResponse.json({ connectorId: 'c1', window: '24h', buckets: [] })
      )
    );
    renderPanel();

    expect(await screen.findByText('No checks recorded in this window.')).toBeInTheDocument();
    expect(screen.getAllByText('No data')).toHaveLength(3);
  });
});
