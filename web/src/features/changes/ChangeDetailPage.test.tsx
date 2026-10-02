import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import '../../i18n';
import { curatedHandlers } from '../../mocks/curated';
import { ChangeDetailPage } from './ChangeDetailPage';

const server = setupServer(...curatedHandlers);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

const docConflict = {
  id: 'chg-doc-1',
  serviceId: 'svc-1',
  serviceName: 'Node one',
  changeType: 'doc_conflict',
  severity: 'info',
  summary: 'Doc "Node one": edited section "snap.status" also changed upstream',
  willTriggerAi: false,
  detectedAt: '2026-10-01T12:00:00Z',
  status: 'new',
  diff: { format: 'doc', baseText: 'healthy (UPS note)', headText: 'degraded', language: 'md' },
  affectedDocIds: ['doc-1'],
  narration: '',
};

function renderPage() {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/changes/chg-doc-1']}>
        <Routes>
          <Route path="/changes/:changeId" element={<ChangeDetailPage />} />
          <Route path="/changes" element={<p>feed</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ChangeDetailPage doc reviews', () => {
  it('offers keep/accept instead of ack/dismiss and posts the chosen action', async () => {
    let posted: unknown;
    server.use(
      http.get('*/changes/:changeId', () => HttpResponse.json(docConflict)),
      http.post('*/changes/:changeId/resolve-doc', async ({ request }) => {
        posted = await request.json();
        return HttpResponse.json({ ...docConflict, status: 'acknowledged' });
      }),
    );
    renderPage();

    const accept = await screen.findByRole('button', { name: 'Accept generated' });
    expect(screen.getByRole('button', { name: 'Keep mine' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Acknowledge' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'AI update' })).not.toBeInTheDocument();

    fireEvent.click(accept);
    await waitFor(() => expect(posted).toEqual({ action: 'accept' }));
  });

  it('keeps the regular actions for infrastructure changes', async () => {
    server.use(
      http.get('*/changes/:changeId', () =>
        HttpResponse.json({ ...docConflict, changeType: 'vm.created', diff: { format: 'infra', hunks: [] } }),
      ),
    );
    renderPage();
    expect(await screen.findByRole('button', { name: 'Acknowledge' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Accept generated' })).not.toBeInTheDocument();
  });
});
