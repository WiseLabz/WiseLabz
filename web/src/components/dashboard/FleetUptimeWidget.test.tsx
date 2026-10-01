import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import { FleetUptimeWidget } from './widgets';
import { widgetsFromWire } from '../../store/dashboard';

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
});
afterAll(() => server.close());

function renderWidget() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <FleetUptimeWidget title="Fleet uptime" icon={null} />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('FleetUptimeWidget', () => {
  it('lists availability per connector and "No data" when there are no checks', async () => {
    server.use(
      http.get('/api/uptime', () =>
        HttpResponse.json({
          window: '7d',
          connectors: [
            { connectorId: 'a', name: 'alpha', checkCount: 5, availabilityPct: 99.123, mttrSeconds: 0, outageCount: 0 },
            { connectorId: 'b', name: 'beta', checkCount: 0, availabilityPct: 0, mttrSeconds: 0, outageCount: 0 },
          ],
        })
      )
    );
    renderWidget();

    expect(await screen.findByText('99.12%')).toBeInTheDocument();
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('No data')).toBeInTheDocument();
    expect(screen.queryByText('0.00%')).not.toBeInTheDocument();
  });
});

describe('uptime widget default', () => {
  it('is appended disabled to an existing saved layout that predates it', () => {
    const layout = widgetsFromWire([
      { id: 'roster', type: 'service_status', x: 0, y: 0, w: 4, h: 1, enabled: true },
    ]);
    expect(layout.find((w) => w.id === 'uptime')?.enabled).toBe(false);
    expect(layout.find((w) => w.id === 'roster')?.enabled).toBe(true);
  });
});
