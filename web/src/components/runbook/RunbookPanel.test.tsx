import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import type { RunbookPage } from '../../api/model';
import { RunbookPanel } from './RunbookPanel';

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function renderPanel(response: RunbookPage) {
  server.use(http.get('/api/runbooks', () => HttpResponse.json(response)));
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <RunbookPanel changeType="vm.created" />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('RunbookPanel', () => {
  it('renders nothing when no runbook matches the target', async () => {
    const { container } = renderPanel({ items: [], total: 0, page: 1, pageSize: 20 });

    // Let the query settle before asserting the still-empty DOM.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(container).toBeEmptyDOMElement();
  });

  it('renders the title and body when a runbook is found', async () => {
    renderPanel({
      items: [
        {
          id: 'rb-1',
          title: 'Restart the hung agent',
          body: 'Step 1: SSH in.\nStep 2: restart the service.',
          targetType: 'change_type',
          targetValue: 'vm.created',
          createdAt: '2026-01-01T00:00:00Z',
          updatedAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    });

    expect(await screen.findByText('Restart the hung agent')).toBeInTheDocument();
    expect(screen.getByText('Step 1: SSH in.', { exact: false })).toBeInTheDocument();
  });
});
