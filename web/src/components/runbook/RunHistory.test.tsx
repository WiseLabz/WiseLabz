import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import type { RunbookRun, RunbookRunPage } from '../../api/model';
import '../../i18n';
import { RunHistory } from './RunHistory';

const server = setupServer();
const allRuns = Array.from({ length: 11 }, (_, index) => makeRun(index + 1));
let requestedPages: number[] = [];

function makeRun(number: number): RunbookRun {
  return {
    id: `run-${number}`,
    runbookId: 'runbook-1',
    runbookTitle: `Recovery ${number}`,
    state: number === 1 ? 'failed' : 'succeeded',
    startedBy: `operator-${number}`,
    startedAt: `2026-10-0${(number % 8) + 1}T10:00:00Z`,
    updatedAt: `2026-10-0${(number % 8) + 1}T10:01:00Z`,
    finishedAt: `2026-10-0${(number % 8) + 1}T10:01:00Z`,
    requiresApproval: false,
    steps: [],
  };
}

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
beforeEach(() => {
  requestedPages = [];
  server.use(
    http.get('/api/runbooks/:runbookId/runs', ({ request }) => {
      const page = Number(new URL(request.url).searchParams.get('page') ?? '1');
      const pageSize = Number(new URL(request.url).searchParams.get('pageSize') ?? '10');
      requestedPages.push(page);
      const response: RunbookRunPage = {
        items: allRuns.slice((page - 1) * pageSize, page * pageSize),
        total: allRuns.length,
        page,
        pageSize,
      };
      return HttpResponse.json(response);
    })
  );
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function renderHistory(onSelectRun = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return {
    onSelectRun,
    ...render(
      <QueryClientProvider client={client}>
        <RunHistory runbookId="runbook-1" onSelectRun={onSelectRun} />
      </QueryClientProvider>
    ),
  };
}

describe('RunHistory', () => {
  it.each([
    ['awaiting_approval', 'Awaiting approval'],
    ['rejected', 'Rejected'],
  ] as const)('labels %s in run history', async (state, label) => {
    server.use(
      http.get('/api/runbooks/:runbookId/runs', () =>
        HttpResponse.json({
          items: [{ ...makeRun(12), state, requiresApproval: true }],
          total: 1,
          page: 1,
          pageSize: 10,
        })
      )
    );

    renderHistory();
    expect(await screen.findByText(label)).toBeInTheDocument();
  });

  it('renders a deep link and preserves modified-click browser behavior', async () => {
    const { onSelectRun } = renderHistory();
    await screen.findByText('Recovery 1');
    const link = screen.getByText('Recovery 1').closest('a') as HTMLAnchorElement;

    expect(link).toHaveAttribute('href', '/runbook-runs/run-1');
    fireEvent.click(link, { ctrlKey: true });
    expect(onSelectRun).not.toHaveBeenCalled();
    fireEvent.click(link);
    expect(onSelectRun).toHaveBeenCalledWith('run-1');
  });

  it('loads the next history page with the generated pagination parameters', async () => {
    renderHistory();
    expect(await screen.findByText('Recovery 1')).toBeInTheDocument();
    expect(screen.queryByText('Recovery 11')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Next page' }));
    expect(await screen.findByText('Recovery 11')).toBeInTheDocument();
    await waitFor(() => expect(requestedPages).toContain(2));
    expect(screen.queryByText('Recovery 1')).not.toBeInTheDocument();
  });

  it('shows the empty state when a runbook has no history', async () => {
    server.use(
      http.get('/api/runbooks/:runbookId/runs', () =>
        HttpResponse.json({ items: [], total: 0, page: 1, pageSize: 10 })
      )
    );
    renderHistory();

    expect(await screen.findByText('No runs yet.')).toBeInTheDocument();
  });
});
