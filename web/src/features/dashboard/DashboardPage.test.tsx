import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import type { User } from '../../api/model';
import { useDashboard, DEFAULT_LAYOUT } from '../../store/dashboard';
import { DashboardPage } from './DashboardPage';

// Widget subcomponents make their own API calls this test doesn't care about;
// let those pass through to MSW's default "no handler" 404 instead of failing
// the whole suite like the stricter `onUnhandledRequest: 'error'` other pages use.
const server = setupServer();

const baseLayout = { widgets: DEFAULT_LAYOUT.map((w) => ({ id: w.id, type: 'service_status', x: 0, y: 0, w: w.span, h: 1, enabled: w.enabled })) };

beforeAll(() => server.listen({ onUnhandledRequest: 'bypass' }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
});
afterAll(() => server.close());

beforeEach(() => {
  localStorage.clear();
  useDashboard.setState({ layout: DEFAULT_LAYOUT, hydrated: false });
});

function renderDashboard(me: Partial<User>) {
  server.use(
    http.get('/api/me', () => HttpResponse.json(me)),
    http.get('/api/dashboard/overview', () =>
      HttpResponse.json({ statusCounts: {}, pendingAlerts: 0, recentChanges: [], lastSyncAt: null })
    ),
    http.get('/api/dashboard/layout', () => HttpResponse.json(baseLayout))
  );
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('DashboardPage', () => {
  it('hides the default-layout editor from a viewer', async () => {
    renderDashboard({ role: 'viewer', canManageDashboardDefaults: false });

    expect(await screen.findByRole('button', { name: 'Reset to default' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit default layout' })).not.toBeInTheDocument();
  });

  it('hides the default-layout editor from an operator without the permission', async () => {
    renderDashboard({ role: 'operator', canManageDashboardDefaults: false });

    await screen.findByRole('button', { name: 'Reset to default' });
    expect(screen.queryByRole('button', { name: 'Edit default layout' })).not.toBeInTheDocument();
  });

  it('shows the default-layout editor to a permitted operator', async () => {
    renderDashboard({ role: 'operator', canManageDashboardDefaults: true });

    expect(await screen.findByRole('button', { name: 'Edit default layout' })).toBeInTheDocument();
  });

  it('resets the layout via the reset button', async () => {
    let resetCalled = false;
    renderDashboard({ role: 'viewer', canManageDashboardDefaults: false });
    server.use(
      http.post('/api/dashboard/layout/reset', () => {
        resetCalled = true;
        return HttpResponse.json({
          widgets: [{ id: 'roster', type: 'service_status', x: 0, y: 0, w: 4, h: 1, enabled: true }],
        });
      })
    );

    const resetButton = await screen.findByRole('button', { name: 'Reset to default' });
    fireEvent.click(resetButton);

    await waitFor(() => expect(resetCalled).toBe(true));
  });
});
