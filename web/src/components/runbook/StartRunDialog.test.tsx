import type { ReactNode } from 'react';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import '../../i18n';
import type { Runbook, RunbookRunStep } from '../../api/model';
import { StartRunDialog } from './StartRunDialog';

vi.mock('../ui/Dialog', () => ({
  Dialog: ({ open, title, children }: { open: boolean; title: string; children: ReactNode }) =>
    open ? (
      <div role="dialog" aria-label={title}>
        {children}
      </div>
    ) : null,
}));

vi.mock('../manager/ElevationConfirm', () => ({
  ElevationConfirm: ({
    open,
    action,
    target,
    title,
    onConfirm,
  }: {
    open: boolean;
    action: string;
    target?: string;
    title: string;
    onConfirm: (token: string | null) => void;
  }) =>
    open ? (
      <div role="dialog" aria-label={title}>
        <p data-testid="elevation-binding">
          {action}|{target}
        </p>
        <button type="button" onClick={() => onConfirm('elevation-token')}>
          Confirm elevation
        </button>
      </div>
    ) : null,
}));

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

const runbook: Runbook = {
  id: 'rb-1',
  title: 'Restart the search stack',
  body: 'Recover search after an outage.',
  targetType: 'change_type',
  targetValue: 'search.unavailable',
  steps: [],
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

function step(overrides: Partial<RunbookRunStep> = {}): RunbookRunStep {
  return {
    id: 'step-1',
    position: 0,
    kind: 'lifecycle',
    title: 'Restart the search worker',
    connectorId: 'conn-1',
    connectorName: 'pve1',
    verb: 'restart',
    entityRef: '',
    timeoutSeconds: 0,
    redacted: false,
    canExecute: true,
    executeBlockedReason: '',
    ...overrides,
  };
}

function renderDialog(onStarted = vi.fn()) {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <StartRunDialog runbook={runbook} open onClose={vi.fn()} onStarted={onStarted} />
    </QueryClientProvider>
  );
}

describe('StartRunDialog', () => {
  it('renders ordered lifecycle impact from the dry-run preview', async () => {
    let dryRun = false;
    server.use(
      http.post('/api/runbooks/rb-1/run', ({ request }) => {
        dryRun = new URL(request.url).searchParams.get('dryRun') === 'true';
        return HttpResponse.json({
          id: 'rb-1',
          canStart: true,
          steps: [
            step({
              id: 'manual-2',
              position: 1,
              kind: 'manual',
              title: 'Verify search results',
              connectorId: '',
              connectorName: '',
              verb: undefined,
            }),
            step({
              id: 'lifecycle-1',
              position: 0,
              preview: {
                targetService: 'search-worker',
                estimatedDowntimeSeconds: 20,
                affectedEntities: ['Indexer', 'Search API'],
                dependentServices: [{ name: 'Search API', kind: 'service' }],
              },
            }),
          ],
        });
      })
    );

    renderDialog();

    expect(await screen.findByText('Restart the search worker')).toBeInTheDocument();
    expect(screen.getByText('Verify search results')).toBeInTheDocument();
    expect(screen.getByText('search-worker')).toBeInTheDocument();
    expect(screen.getByText('20 seconds')).toBeInTheDocument();
    expect(screen.getByText('Indexer')).toBeInTheDocument();
    expect(screen.getAllByText('Search API')).toHaveLength(2);
    expect(dryRun).toBe(true);
  });

  it('shows a blocked reason and keeps start disabled', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: false,
          steps: [
            step({
              canExecute: false,
              executeBlockedReason: 'no_operator_grant',
            }),
          ],
        })
      )
    );

    renderDialog();

    expect(await screen.findByText('You need operator access to pve1.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();
  });

  it('binds elevation to runbook.run and starts with the elevation token', async () => {
    let receivedToken: string | null = null;
    server.use(
      http.post('/api/runbooks/rb-1/run', ({ request }) => {
        const url = new URL(request.url);
        if (url.searchParams.get('dryRun') === 'true') {
          return HttpResponse.json({ id: 'rb-1', canStart: true, steps: [step()] });
        }
        receivedToken = request.headers.get('X-Elevation-Token');
        return HttpResponse.json({ id: 'run-1' });
      })
    );
    const onStarted = vi.fn();
    renderDialog(onStarted);

    fireEvent.click(await screen.findByRole('button', { name: 'Start run' }));
    expect(await screen.findByTestId('elevation-binding')).toHaveTextContent('runbook.run|rb-1');
    fireEvent.click(screen.getByRole('button', { name: 'Confirm elevation' }));

    await waitFor(() => {
      expect(receivedToken).toBe('elevation-token');
      expect(onStarted).toHaveBeenCalledWith('run-1');
    });
  });

  it('shows the API conflict message and opens the existing run through its link', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', ({ request }) => {
        if (new URL(request.url).searchParams.get('dryRun') === 'true') {
          return HttpResponse.json({ id: 'rb-1', canStart: true, steps: [step()] });
        }
        return HttpResponse.json(
          { code: 'run_in_progress', message: 'A run is already active.', runId: 'run-existing' },
          { status: 409 }
        );
      })
    );
    const onStarted = vi.fn();
    renderDialog(onStarted);

    fireEvent.click(await screen.findByRole('button', { name: 'Start run' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm elevation' }));

    expect(await screen.findByText('A run is already active.')).toBeInTheDocument();
    const existingRunLink = screen.getByRole('link', { name: 'Open active run' });
    expect(existingRunLink).toHaveAttribute('href', '/runbook-runs/run-existing');
    fireEvent.click(existingRunLink);
    expect(onStarted).toHaveBeenCalledWith('run-existing');
  });

  it('renders a redacted preview step neutrally and disables start', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: false,
          steps: [
            {
              id: 'restricted-1',
              position: 0,
              redacted: true,
              canExecute: false,
              executeBlockedReason: 'no_viewer_grant',
            },
          ],
        })
      )
    );
    renderDialog();
    expect(await screen.findByText('Restricted step')).toBeInTheDocument();
    expect(screen.getByText('You do not have permission to view this step.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();
    expect(screen.queryByText('Lifecycle action')).not.toBeInTheDocument();
  });
});
