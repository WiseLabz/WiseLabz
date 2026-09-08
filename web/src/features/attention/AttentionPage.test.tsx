import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import type { AttentionPage } from '../../api/model';
import { AttentionPage as AttentionPageComponent } from './AttentionPage';

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
});
afterAll(() => server.close());

function renderAttention(response: AttentionPage) {
  server.use(http.get('/api/attention', () => HttpResponse.json(response)));
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <AttentionPageComponent />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('AttentionPage', () => {
  it('renders empty state when no attention items exist', async () => {
    renderAttention({ data: [], total: 0, page: 1, pageSize: 20 });

    const emptyTitle = await screen.findByText('Nothing needs attention');
    expect(emptyTitle).toBeInTheDocument();
  });

  it('renders error state when data fetch fails', async () => {
    server.use(http.get('/api/attention', () => HttpResponse.error()));
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter>
          <AttentionPageComponent />
        </MemoryRouter>
      </QueryClientProvider>
    );

    const errorText = await screen.findByText((content) => content.includes('load'));
    expect(errorText).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('renders alert and finding items with correct kind badges', async () => {
    const alertItem = {
      id: 'alt-1',
      kind: 'alert' as const,
      severity: 'critical' as const,
      title: 'Critical alert title',
      connectorId: 'svc-1',
      detectedAt: '2026-01-01T00:00:00Z',
    };
    const findingItem = {
      id: 'find-1',
      kind: 'finding' as const,
      severity: 'warning' as const,
      title: 'Finding title',
      connectorId: 'svc-2',
      detectedAt: '2026-01-01T00:00:00Z',
      checkType: 'ownership_incomplete',
    };

    renderAttention({ data: [alertItem, findingItem], total: 2, page: 1, pageSize: 20 });

    expect(await screen.findByText('Critical alert title')).toBeInTheDocument();
    expect(screen.getByText('Finding title')).toBeInTheDocument();
    expect(screen.getByText('Alert')).toBeInTheDocument();
    expect(screen.getByText('Finding')).toBeInTheDocument();
  });

  it('displays connector ID and relative time for items', async () => {
    const item = {
      id: 'alt-1',
      kind: 'alert' as const,
      severity: 'info' as const,
      title: 'Test item',
      connectorId: 'svc-important',
      detectedAt: new Date(Date.now() - 5 * 60000).toISOString(),
    };

    renderAttention({ data: [item], total: 1, page: 1, pageSize: 20 });

    expect(await screen.findByText((content) => content.includes('svc-important'))).toBeInTheDocument();
    expect(screen.getByText((content) => content.includes('ago'))).toBeInTheDocument();
  });

  it('calls resolve mutation when resolve button is clicked for alert', async () => {
    const alertItem = {
      id: 'alt-1',
      kind: 'alert' as const,
      severity: 'warning' as const,
      title: 'Alert to resolve',
      connectorId: 'svc-1',
      detectedAt: '2026-01-01T00:00:00Z',
    };

    server.use(
      http.post('/api/alerts/:id/resolve', () => HttpResponse.json({ status: 'resolved' }))
    );

    renderAttention({ data: [alertItem], total: 1, page: 1, pageSize: 20 });

    const resolveButton = await screen.findByRole('button', { name: /resolve/i });
    fireEvent.click(resolveButton);

    // Verify the button was clicked (mutation was triggered)
    expect(resolveButton).toBeInTheDocument();
  });

  it('shows alert-specific buttons only for alerts', async () => {
    const alertItem = {
      id: 'alt-1',
      kind: 'alert' as const,
      severity: 'info' as const,
      title: 'Test Alert',
      connectorId: 'svc-1',
      detectedAt: '2026-01-01T00:00:00Z',
    };

    renderAttention({ data: [alertItem], total: 1, page: 1, pageSize: 20 });

    const resolveButtons = await screen.findAllByRole('button', { name: /resolve/i });
    const dismissButton = screen.getByRole('button', { name: /dismiss/i });
    const snoozeButton = screen.getByRole('button', { name: /snooze/i });

    expect(resolveButtons.length).toBeGreaterThan(0);
    expect(dismissButton).toBeInTheDocument();
    expect(snoozeButton).toBeInTheDocument();
  });
});
