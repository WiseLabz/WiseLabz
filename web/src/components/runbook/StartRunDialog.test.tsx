import type { ReactNode } from 'react';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { delay, http, HttpResponse } from 'msw';
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
  requiresApproval: false,
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

function renderDialog(onStarted = vi.fn(), selectedRunbook: Runbook = runbook) {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <StartRunDialog runbook={selectedRunbook} open onClose={vi.fn()} onStarted={onStarted} />
    </QueryClientProvider>
  );
}

describe('StartRunDialog', () => {
  it('requests approval for an opted-in runbook and keeps runbook.run elevation', async () => {
    let receivedToken: string | null = null;
    server.use(
      http.post('/api/runbooks/rb-1/run', ({ request }) => {
        const url = new URL(request.url);
        if (url.searchParams.get('dryRun') === 'true') {
          return HttpResponse.json({
            id: 'rb-1',
            canStart: true,
            requiresApproval: true,
            approverAvailable: true,
            steps: [step()],
          });
        }
        receivedToken = request.headers.get('X-Elevation-Token');
        return HttpResponse.json({ id: 'run-awaiting' }, { status: 202 });
      })
    );
    const onStarted = vi.fn();
    renderDialog(onStarted, { ...runbook, requiresApproval: true });

    expect(await screen.findByText('Review the frozen steps. Nothing runs until another operator approves this request.')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: 'Request approval' }));
    expect(await screen.findByTestId('elevation-binding')).toHaveTextContent('runbook.run|rb-1');
    fireEvent.click(screen.getByRole('button', { name: 'Confirm elevation' }));

    await waitFor(() => expect(onStarted).toHaveBeenCalledWith('run-awaiting'));
    expect(receivedToken).toBe('elevation-token');
  });

  it('disables approval requests when no eligible approver is available', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: true,
          requiresApproval: true,
          approverAvailable: false,
          steps: [step()],
        })
      )
    );
    renderDialog(vi.fn(), { ...runbook, requiresApproval: true });

    expect(await screen.findByText('No other eligible operator is available to approve this runbook.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Request approval' })).toBeDisabled();
    expect(screen.queryByRole('dialog', { name: 'Start “Restart the search stack”' })).toBeInTheDocument();
  });

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

  it('shows the named action and the request it will send in the preview', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: true,
          steps: [
            step({
              id: 'action-1',
              kind: 'connector_action',
              title: 'Rescan the index',
              verb: undefined,
              action: 'rescan',
              preview: {
                userDefined: true,
                label: 'Rescan index',
                description: 'Rebuilds the search index.',
                targetService: 'search-worker',
                estimatedDowntimeSeconds: 0,
                dependentServices: [],
                request: {
                  method: 'POST',
                  url: 'https://search.example/api/rescan',
                  headers: { 'X-Mode': 'fast' },
                  body: { force: true },
                },
              },
            }),
          ],
        })
      )
    );

    renderDialog();

    expect(await screen.findByText('Rescan the index')).toBeInTheDocument();
    expect(screen.getByText('rescan')).toBeInTheDocument();
    expect(screen.getByText('Rescan index')).toBeInTheDocument();
    expect(screen.getByText('User-defined')).toBeInTheDocument();
    expect(screen.getByText('No downtime declared')).toBeInTheDocument();
    expect(screen.getByText('POST')).toBeInTheDocument();
    expect(screen.getByText('https://search.example/api/rescan')).toBeInTheDocument();
    expect(screen.getByText(/"X-Mode": "fast"/)).toBeInTheDocument();
    expect(screen.getByText(/"force": true/)).toBeInTheDocument();
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

  it('shows a known config-push value changing to its frozen target', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: true,
          steps: [
            step({
              kind: 'config_push',
              title: 'Set memory',
              fieldKey: 'memory',
              targetValue: '512',
              currentValue: 256,
              currentValueKnown: true,
              verb: undefined,
            }),
          ],
        })
      )
    );
    renderDialog();

    expect(await screen.findByText('Current: 256 → 512')).toBeInTheDocument();
  });

  it('states that the config-push current value is unknown', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: true,
          steps: [
            step({
              kind: 'config_push',
              title: 'Set memory',
              fieldKey: 'memory',
              targetValue: '512',
              currentValueKnown: false,
              verb: undefined,
            }),
          ],
        })
      )
    );
    renderDialog();

    expect(await screen.findByText('Current value unknown → 512')).toBeInTheDocument();
  });

  it('marks a withdrawn config-push field as not executable', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: false,
          steps: [
            step({
              kind: 'config_push',
              title: 'Set memory',
              fieldKey: 'memory',
              targetValue: '512',
              currentValueKnown: false,
              canExecute: false,
              executeBlockedReason: 'unsupported_field',
              verb: undefined,
            }),
          ],
        })
      )
    );
    renderDialog();

    expect(
      await screen.findByText('Not executable: this configuration field is no longer writable.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();
  });

  it('shows the wait-for-entity condition and timeout in the preview', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: true,
          steps: [
            step({
              kind: 'wait_for_entity',
              title: 'Wait for the VM',
              entityRef: 'vm-100',
              attribute: 'status',
              operator: 'eq',
              expectedValue: '"running"',
              timeoutSeconds: 300,
              verb: undefined,
            }),
          ],
        })
      )
    );
    renderDialog();

    expect(await screen.findByText('status eq running')).toBeInTheDocument();
    expect(screen.getByText('300 seconds')).toBeInTheDocument();
  });

  it('binds elevation to runbook.run and starts with the elevation token', async () => {
    let receivedToken: string | null = null;
    let dryRunToken: string | null = 'unset';
    server.use(
      http.post('/api/runbooks/rb-1/run', ({ request }) => {
        const url = new URL(request.url);
        if (url.searchParams.get('dryRun') === 'true') {
          dryRunToken = request.headers.get('X-Elevation-Token');
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
    expect(dryRunToken).toBeNull();
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

  it('keeps start disabled while the preview is loading', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', async () => {
        await delay(100);
        return HttpResponse.json({ id: 'rb-1', canStart: true, steps: [step()] });
      })
    );
    renderDialog();

    expect(screen.getByText('Loading preview…')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();
    expect(await screen.findByText('Restart the search worker')).toBeInTheDocument();
    expect(screen.queryByText('Loading preview…')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start run' })).toBeEnabled();
  });

  it('shows the preview error, keeps start disabled and recovers on retry', async () => {
    let requests = 0;
    server.use(
      http.post('/api/runbooks/rb-1/run', () => {
        requests += 1;
        if (requests === 1) return new HttpResponse(null, { status: 500 });
        return HttpResponse.json({ id: 'rb-1', canStart: true, steps: [step()] });
      })
    );
    renderDialog();

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not load the run preview. Try again.'
    );
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();

    fireEvent.click(screen.getByRole('button', { name: 'Retry preview' }));
    expect(await screen.findByText('Restart the search worker')).toBeInTheDocument();
    expect(requests).toBe(2);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start run' })).toBeEnabled();
  });

  it('explains an empty runbook and keeps start disabled', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({ id: 'rb-1', canStart: false, steps: [] })
      )
    );
    renderDialog();

    expect(
      await screen.findByText('Add steps to this runbook before starting a run.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();
  });

  it('renders a step missing its fields as restricted without printing undefined', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', () =>
        HttpResponse.json({
          id: 'rb-1',
          canStart: false,
          steps: [{ id: 'bare-1', position: 0, redacted: false, canExecute: true }],
        })
      )
    );
    renderDialog();

    expect(await screen.findByText('Restricted step')).toBeInTheDocument();
    expect(document.body.textContent).not.toContain('undefined');
  });

  it('shows the unavailable message when the server is shutting down', async () => {
    server.use(
      http.post('/api/runbooks/rb-1/run', ({ request }) => {
        if (new URL(request.url).searchParams.get('dryRun') === 'true') {
          return HttpResponse.json({ id: 'rb-1', canStart: true, steps: [step()] });
        }
        return new HttpResponse(null, { status: 503 });
      })
    );
    renderDialog();

    fireEvent.click(await screen.findByRole('button', { name: 'Start run' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm elevation' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The server is shutting down. Try again shortly.'
    );
    expect(
      screen.queryByText('Could not start this run. Review the preview and try again.')
    ).not.toBeInTheDocument();
  });
});
