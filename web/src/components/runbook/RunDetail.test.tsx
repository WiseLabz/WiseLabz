import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import type { RunbookRun } from '../../api/model';
import i18n from '../../i18n';
import { toast } from '../../lib/toast';
import { RunDetail } from './RunDetail';

type ElevationProps = {
  open: boolean;
  title: string;
  action: string;
  target?: string;
  confirmLabel?: string;
  isPending?: boolean;
  onClose: () => void;
  onConfirm: (token: string | null) => Promise<void> | void;
};

vi.mock('../manager/ElevationConfirm', () => ({
  ElevationConfirm: ({
    open,
    title,
    action,
    target,
    confirmLabel,
    isPending,
    onClose,
    onConfirm,
  }: ElevationProps) =>
    open ? (
      <div role="dialog" aria-label={title}>
        <p>{action}</p>
        <p>{target}</p>
        <button disabled={isPending} onClick={() => onConfirm('fresh-elevation-token')}>
          {confirmLabel}
        </button>
        <button onClick={onClose}>Close elevation</button>
      </div>
    ) : null,
}));

vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const server = setupServer();

const baseRun: RunbookRun = {
  id: 'run-1',
  runbookId: 'runbook-1',
  runbookTitle: 'Failover runbook',
  state: 'failed',
  reason: 'interrupted',
  startedBy: 'user-started',
  resumedBy: 'user-resumed',
  cancelledBy: 'user-cancelled',
  startedAt: '2026-10-01T10:00:00Z',
  updatedAt: '2026-10-01T10:10:00Z',
  finishedAt: '2026-10-01T10:10:00Z',
  steps: [
    {
      id: 'step-1',
      position: 0,
      kind: 'lifecycle',
      title: 'Restart primary',
      connectorId: 'connector-1',
      connectorName: 'primary',
      verb: 'restart',
      entityRef: 'vm:100',
      timeoutSeconds: 300,
      state: 'failed',
      startedAt: '2026-10-01T10:02:00Z',
      finishedAt: '2026-10-01T10:03:00Z',
      error: 'Connector refused the restart',
      confirmedBy: 'user-confirmed',
      redacted: false,
      canExecute: true,
    },
  ],
};

const unknownActionStep: RunbookRun['steps'][number] = {
  id: 'action-step',
  position: 0,
  kind: 'connector_action',
  title: 'Rescan the index',
  connectorId: 'connector-1',
  connectorName: 'primary',
  action: 'rescan',
  entityRef: '',
  state: 'unknown',
  redacted: false,
  canExecute: true,
};

let currentRun = structuredClone(baseRun);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
beforeEach(() => {
  vi.mocked(toast.error).mockClear();
  currentRun = structuredClone(baseRun);
  server.use(http.get('/api/runbook-runs/:runId', () => HttpResponse.json(currentRun)));
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

function renderDetail(runId = 'run-1') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <RunDetail runId={runId} />
    </QueryClientProvider>
  );
}

describe('RunDetail', () => {
  it.each([
    ['running', 'Running'],
    ['waiting_manual', 'Waiting for manual confirmation'],
    ['failed', 'Failed'],
    ['succeeded', 'Succeeded'],
    ['cancelled', 'Cancelled'],
    ['expired', 'Expired'],
  ] as const)('labels the %s run state', async (state, label) => {
    currentRun = { ...baseRun, state };
    renderDetail();
    const detail = screen.getByRole('region', { name: 'Run details' });
    const stateLabels = await within(detail).findAllByText(label);
    expect(stateLabels.some((element) => element.closest('[aria-live="polite"]'))).toBe(true);
  });

  it.each([
    ['pending', 'Pending'],
    ['running', 'Running'],
    ['waiting', 'Waiting'],
    ['succeeded', 'Succeeded'],
    ['failed', 'Failed'],
    ['skipped', 'Skipped'],
    ['unknown', 'Unknown'],
  ] as const)('labels the %s step state', async (state, label) => {
    currentRun = {
      ...baseRun,
      state: 'running',
      steps: [{ ...baseRun.steps[0], state }],
    };
    renderDetail();
    const detail = screen.getByRole('region', { name: 'Run details' });
    const stateLabels = await within(detail).findAllByText(label);
    expect(stateLabels.some((element) => element.closest('[aria-live="polite"]'))).toBe(true);
  });

  it('shows frozen step data, reasons, times, and actor IDs', async () => {
    renderDetail();

    expect(await screen.findByText('Restart primary')).toBeInTheDocument();
    expect(screen.getByText('primary')).toBeInTheDocument();
    expect(screen.getByText('connector-1')).toBeInTheDocument();
    expect(screen.getByText('vm:100')).toBeInTheDocument();
    expect(screen.getByText('Connector refused the restart')).toBeInTheDocument();
    expect(screen.getByText('interrupted')).toBeInTheDocument();
    for (const actor of ['user-started', 'user-resumed', 'user-cancelled', 'user-confirmed']) {
      expect(screen.getByText(actor)).toBeInTheDocument();
    }
    expect(screen.getAllByText(/Oct 1, 2026/).length).toBeGreaterThan(0);
  });

  it('shows the current and target values for a config-push step', async () => {
    currentRun = {
      ...baseRun,
      state: 'running',
      steps: [
        {
          ...baseRun.steps[0],
          kind: 'config_push',
          title: 'Set VM memory',
          fieldKey: 'memory',
          targetValue: '512',
          currentValue: 256,
          currentValueKnown: true,
          verb: undefined,
        },
      ],
    };
    renderDetail();

    expect(await screen.findByText('Current: 256 → 512')).toBeInTheDocument();
    expect(screen.getByText('memory')).toBeInTheDocument();
  });

  it('states when a config-push current value is unknown', async () => {
    currentRun = {
      ...baseRun,
      state: 'running',
      steps: [
        {
          ...baseRun.steps[0],
          kind: 'config_push',
          title: 'Set VM memory',
          fieldKey: 'memory',
          targetValue: '512',
          currentValueKnown: false,
          verb: undefined,
        },
      ],
    };
    renderDetail();

    expect(await screen.findByText('Current value unknown → 512')).toBeInTheDocument();
  });

  it('shows only the target for a config-push step without current value information', async () => {
    currentRun = {
      ...baseRun,
      state: 'succeeded',
      steps: [
        {
          ...baseRun.steps[0],
          kind: 'config_push',
          title: 'Set VM memory',
          fieldKey: 'memory',
          targetValue: '512',
          verb: undefined,
        },
      ],
    };
    renderDetail();

    expect(await screen.findByText('512')).toBeInTheDocument();
    expect(screen.queryByText(/Current value unknown/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Current:/)).not.toBeInTheDocument();
  });

  it('marks a withdrawn config-push field as not executable', async () => {
    currentRun = {
      ...baseRun,
      steps: [
        {
          ...baseRun.steps[0],
          kind: 'config_push',
          fieldKey: 'memory',
          targetValue: '512',
          canExecute: false,
          executeBlockedReason: 'unsupported_field',
          verb: undefined,
        },
      ],
    };
    renderDetail();

    expect(
      await screen.findByText('Not executable: this configuration field is no longer writable.')
    ).toBeInTheDocument();
  });

  it('shows a wait condition and its timeout reason', async () => {
    currentRun = {
      ...baseRun,
      steps: [
        {
          ...baseRun.steps[0],
          kind: 'wait_for_entity',
          title: 'Wait for the VM',
          entityRef: 'vm-100',
          attribute: 'status',
          operator: 'eq',
          expectedValue: '"running"',
          timeoutSeconds: 300,
          error: 'Timed out waiting for status; last value was starting',
          verb: undefined,
        },
      ],
    };
    renderDetail();

    expect(await screen.findByText('status eq running')).toBeInTheDocument();
    const timeoutReason = screen.getByText(/Timed out waiting for status/);
    expect(timeoutReason.closest('p')).toHaveTextContent(
      'Step error: Timed out waiting for status; last value was starting'
    );
  });

  it('renders redacted steps as neutral restricted rows without exposing hidden fields', async () => {
    currentRun = {
      ...baseRun,
      steps: [
        {
          ...baseRun.steps[0],
          id: 'hidden-step',
          position: 1,
          redacted: true,
          title: 'SECRET step title',
          connectorId: 'secret-connector-id',
          connectorName: 'SECRET connector name',
          kind: 'lifecycle',
          entityRef: 'SECRET entity',
          error: 'SECRET step error',
        },
      ],
    };
    renderDetail();

    expect(await screen.findByText('Restricted step')).toBeInTheDocument();
    expect(screen.getByText('user-confirmed')).toBeInTheDocument();
    expect(document.body.textContent).not.toContain('SECRET');
  });

  it('confirms a waiting manual step without elevation', async () => {
    currentRun = {
      ...baseRun,
      state: 'waiting_manual',
      steps: [
        {
          ...baseRun.steps[0],
          id: 'manual-step',
          kind: 'manual',
          title: 'Verify failover',
          connectorId: undefined,
          connectorName: undefined,
          verb: undefined,
          state: 'waiting',
        },
      ],
    };
    let confirmed = false;
    let elevationHeader: string | null = 'unset';
    server.use(
      http.post('/api/runbook-runs/:runId/steps/:stepId/confirm', ({ params, request }) => {
        confirmed = params.runId === 'run-1' && params.stepId === 'manual-step';
        elevationHeader = request.headers.get('X-Elevation-Token');
        return new HttpResponse(null, { status: 204 });
      })
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Confirm step' }));
    await waitFor(() => expect(confirmed).toBe(true));
    expect(elevationHeader).toBeNull();
    expect(screen.queryByText('runbook.run')).not.toBeInTheDocument();
  });

  it('warns before elevation when the first unfinished step is unknown', async () => {
    currentRun = {
      ...baseRun,
      steps: [
        { ...baseRun.steps[0], state: 'succeeded' },
        { ...baseRun.steps[0], id: 'unknown-step', position: 1, state: 'unknown' },
      ],
    };
    let elevationToken = '';
    server.use(
      http.post('/api/runbook-runs/:runId/resume', ({ request }) => {
        elevationToken = request.headers.get('X-Elevation-Token') ?? '';
        return HttpResponse.json({ ...currentRun, state: 'running' }, { status: 202 });
      })
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Resume run' }));
    expect(
      await screen.findByText(i18n.t('runbooks.runs.resumeWarningUnknown'))
    ).toBeInTheDocument();
    expect(screen.queryByText('runbook.run')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Continue to approval' }));

    const elevation = await screen.findByRole('dialog', { name: 'Resume run' });
    expect(elevation).toHaveTextContent('runbook.run');
    expect(elevation).toHaveTextContent('runbook-1');
    fireEvent.click(within(elevation).getByRole('button', { name: 'Resume run' }));
    await waitFor(() => expect(elevationToken).toBe('fresh-elevation-token'));
  });

  it.each([
    ['Send the action again', 'resend'],
    ['Mark the step as done', 'mark_done'],
  ] as const)(
    'sends the "%s" decision when resuming past an unknown action step',
    async (label, decision) => {
      currentRun = { ...baseRun, steps: [unknownActionStep] };
      let body: unknown;
      let elevationToken = '';
      server.use(
        http.post('/api/runbook-runs/:runId/resume', async ({ request }) => {
          body = await request.json();
          elevationToken = request.headers.get('X-Elevation-Token') ?? '';
          return HttpResponse.json({ ...currentRun, state: 'running' }, { status: 202 });
        })
      );
      renderDetail();

      fireEvent.click(await screen.findByRole('button', { name: 'Resume run' }));
      const decisionDialog = await screen.findByRole('dialog', { name: 'Resume run' });
      expect(within(decisionDialog).getByRole('note')).toHaveTextContent(
        i18n.t('runbooks.runs.resumeDecisionWarning')
      );
      const continueButton = within(decisionDialog).getByRole('button', {
        name: 'Continue to approval',
      });
      expect(continueButton).toBeDisabled();

      fireEvent.click(within(decisionDialog).getByRole('radio', { name: new RegExp(`^${label}`) }));
      expect(continueButton).toBeEnabled();
      fireEvent.click(continueButton);

      const elevation = await screen.findByRole('dialog', { name: 'Resume run' });
      fireEvent.click(within(elevation).getByRole('button', { name: 'Resume run' }));
      await waitFor(() => expect(elevationToken).toBe('fresh-elevation-token'));
      expect(body).toEqual({ decision });
    }
  );

  it('shows a clear message when the server still needs a decision for the action step', async () => {
    currentRun = { ...baseRun, steps: [unknownActionStep] };
    server.use(
      http.post('/api/runbook-runs/:runId/resume', () =>
        HttpResponse.json(
          { code: 'unknown_step_decision_required', message: 'decision required' },
          { status: 409 }
        )
      )
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Resume run' }));
    const decisionDialog = await screen.findByRole('dialog', { name: 'Resume run' });
    fireEvent.click(within(decisionDialog).getByRole('radio', { name: /^Send the action again/ }));
    fireEvent.click(within(decisionDialog).getByRole('button', { name: 'Continue to approval' }));
    const elevation = await screen.findByRole('dialog', { name: 'Resume run' });
    fireEvent.click(within(elevation).getByRole('button', { name: 'Resume run' }));

    expect(
      await screen.findByText(i18n.t('runbooks.runs.resumeDecisionRequired'))
    ).toBeInTheDocument();
  });

  it('shows no decision choice and sends no body for an unknown lifecycle step', async () => {
    currentRun = {
      ...baseRun,
      steps: [{ ...baseRun.steps[0], id: 'unknown-step', state: 'unknown' }],
    };
    let body: string | undefined;
    let elevationToken = '';
    server.use(
      http.post('/api/runbook-runs/:runId/resume', async ({ request }) => {
        body = await request.text();
        elevationToken = request.headers.get('X-Elevation-Token') ?? '';
        return HttpResponse.json({ ...currentRun, state: 'running' }, { status: 202 });
      })
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Resume run' }));
    expect(
      await screen.findByText(i18n.t('runbooks.runs.resumeWarningUnknown'))
    ).toBeInTheDocument();
    expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Continue to approval' }));

    const elevation = await screen.findByRole('dialog', { name: 'Resume run' });
    fireEvent.click(within(elevation).getByRole('button', { name: 'Resume run' }));
    await waitFor(() => expect(elevationToken).toBe('fresh-elevation-token'));
    expect(body).toBe('');
  });

  it('shows the named action and the request a connector action step sends', async () => {
    currentRun = {
      ...baseRun,
      state: 'succeeded',
      steps: [
        {
          ...unknownActionStep,
          state: 'succeeded',
          preview: {
            userDefined: true,
            label: 'Rescan index',
            description: 'Rebuilds the search index.',
            targetService: 'primary',
            estimatedDowntimeSeconds: 0,
            dependentServices: [],
            request: {
              method: 'POST',
              url: 'https://primary.example/api/rescan',
              body: { force: true },
            },
          },
        },
      ],
    };
    renderDetail();

    const detail = screen.getByRole('region', { name: 'Run details' });
    expect(await within(detail).findByText(i18n.t('runbooks.runs.actionName'))).toBeInTheDocument();
    expect(within(detail).getByText('rescan')).toBeInTheDocument();
    expect(within(detail).getByText('User-defined')).toBeInTheDocument();
    expect(within(detail).getByText('https://primary.example/api/rescan')).toBeInTheDocument();
    expect(within(detail).getByText(/"force": true/)).toBeInTheDocument();
  });

  it('opens elevation directly when the first unfinished step is known', async () => {
    currentRun = {
      ...baseRun,
      steps: [{ ...baseRun.steps[0], state: 'failed' }],
    };
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Resume run' }));
    expect(await screen.findByText('runbook.run')).toBeInTheDocument();
    expect(screen.queryByText(/unknown result/i)).not.toBeInTheDocument();
  });

  it('shows the localised message when a deleted runbook rejects resume with 409', async () => {
    const apiMessage = 'The runbook no longer exists; this run cannot be resumed';
    server.use(
      http.post('/api/runbook-runs/:runId/resume', () =>
        HttpResponse.json({ code: 'runbook_deleted', message: apiMessage }, { status: 409 })
      )
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Resume run' }));
    const elevation = await screen.findByRole('dialog', { name: 'Resume run' });
    fireEvent.click(within(elevation).getByRole('button', { name: 'Resume run' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      i18n.t('runbooks.runs.resumeDeleted')
    );
    expect(screen.queryByText(apiMessage)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Resume run' })).not.toBeInTheDocument();
  });

  it('explains why a deleted runbook run cannot be resumed', async () => {
    currentRun = { ...baseRun, runbookId: undefined };
    renderDetail();

    expect(
      await screen.findByText(
        'This runbook was deleted. This run can still be reviewed, but it cannot be resumed.'
      )
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Resume run' })).not.toBeInTheDocument();
  });

  it('requires a separate confirmation before cancelling', async () => {
    let cancelled = false;
    server.use(
      http.post('/api/runbook-runs/:runId/cancel', () => {
        cancelled = true;
        return new HttpResponse(null, { status: 204 });
      })
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Cancel run' }));
    const confirmation = await screen.findByRole('dialog', { name: 'Cancel run' });
    expect(cancelled).toBe(false);
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Yes, cancel run' }));
    await waitFor(() => expect(cancelled).toBe(true));
  });

  it('disables run actions when any frozen step is not executable', async () => {
    currentRun = {
      ...baseRun,
      state: 'waiting_manual',
      steps: [{ ...baseRun.steps[0], kind: 'manual', state: 'waiting', canExecute: false }],
    };
    renderDetail();

    expect(await screen.findByRole('button', { name: 'Confirm step' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Cancel run' })).toBeEnabled();
    expect(
      screen.getByText(
        'You need operator access to every connector in this run, or to at least one connector when the run has none.'
      )
    ).toBeInTheDocument();
  });

  it('disables manual confirmation without connectorless-run access and shows cancellation errors', async () => {
    currentRun = {
      ...baseRun,
      state: 'waiting_manual',
      steps: [
        {
          ...baseRun.steps[0],
          kind: 'manual',
          connectorId: undefined,
          connectorName: undefined,
          state: 'waiting',
          canExecute: false,
        },
      ],
    };
    server.use(
      http.post('/api/runbook-runs/:runId/cancel', () =>
        HttpResponse.json(
          { code: 'forbidden', message: 'insufficient permissions' },
          { status: 403 }
        )
      )
    );
    renderDetail();

    expect(await screen.findByRole('button', { name: 'Confirm step' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel run' }));
    const confirmation = await screen.findByRole('dialog', { name: 'Cancel run' });
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Yes, cancel run' }));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
  });

  it('keeps a run with a deleted connector cancellable', async () => {
    currentRun = {
      ...baseRun,
      state: 'failed',
      steps: [
        {
          id: 'gone-step',
          position: 0,
          redacted: true,
          canExecute: false,
          state: 'failed',
        },
      ],
    };
    let cancelledRun = '';
    server.use(
      http.post('/api/runbook-runs/:runId/cancel', ({ params }) => {
        cancelledRun = String(params.runId);
        return new HttpResponse(null, { status: 204 });
      })
    );
    renderDetail();

    expect(await screen.findByText('Restricted step')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Resume run' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel run' }));
    const confirmation = await screen.findByRole('dialog', { name: 'Cancel run' });
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Yes, cancel run' }));
    await waitFor(() => expect(cancelledRun).toBe('run-1'));
  });

  it('renders steps missing their fields as neutral rows without printing undefined', async () => {
    currentRun = {
      ...baseRun,
      steps: [
        { id: 'bare-step', position: 0, redacted: false, canExecute: true },
        {
          id: 'no-connector-step',
          position: 1,
          redacted: false,
          canExecute: true,
          kind: 'lifecycle',
          title: 'Orphaned lifecycle step',
          state: 'pending',
        },
      ],
    };
    renderDetail();

    expect(await screen.findAllByText('Restricted step')).toHaveLength(2);
    expect(screen.queryByText('Orphaned lifecycle step')).not.toBeInTheDocument();
    expect(document.body.textContent).not.toContain('undefined');
  });

  it('shows the forbidden message when confirm is refused with 403', async () => {
    currentRun = {
      ...baseRun,
      state: 'waiting_manual',
      steps: [
        {
          ...baseRun.steps[0],
          id: 'manual-step',
          kind: 'manual',
          connectorId: undefined,
          connectorName: undefined,
          verb: undefined,
          state: 'waiting',
        },
      ],
    };
    server.use(
      http.post(
        '/api/runbook-runs/:runId/steps/:stepId/confirm',
        () => new HttpResponse(null, { status: 403 })
      )
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Confirm step' }));
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('You do not have permission to do this.')
    );
  });

  it('refetches the run when confirm conflicts with the current state', async () => {
    currentRun = {
      ...baseRun,
      state: 'waiting_manual',
      steps: [
        {
          ...baseRun.steps[0],
          id: 'manual-step',
          kind: 'manual',
          connectorId: undefined,
          connectorName: undefined,
          verb: undefined,
          state: 'waiting',
        },
      ],
    };
    let runRequests = 0;
    server.use(
      http.get('/api/runbook-runs/:runId', () => {
        runRequests += 1;
        return HttpResponse.json(currentRun);
      }),
      http.post(
        '/api/runbook-runs/:runId/steps/:stepId/confirm',
        () => new HttpResponse(null, { status: 409 })
      )
    );
    renderDetail();

    fireEvent.click(await screen.findByRole('button', { name: 'Confirm step' }));
    await waitFor(() => expect(runRequests).toBeGreaterThanOrEqual(2));
    expect(toast.error).toHaveBeenCalledWith(
      'This run or step is no longer in the required state. The latest state has been loaded.'
    );
  });
});
