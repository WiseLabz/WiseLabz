import { render, screen } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import '../../i18n';
import { useAuth } from '../../store/auth';
import { queryClient } from '../../app/queryClient';
import type { RunbookRun } from '../../api/model';

vi.mock('../shell/AppShell', async () => {
  const { Outlet } = await import('react-router-dom');
  return { AppShell: () => <Outlet /> };
});
vi.mock('../MotionProvider', () => ({
  MotionProvider: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock('../../ws/WebSocketProvider', () => ({
  WebSocketProvider: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => false }));

const server = setupServer();
const run: RunbookRun = {
  id: 'run-route',
  runbookId: 'rb-1',
  runbookTitle: 'Recorded remediation',
  state: 'succeeded',
  startedBy: 'viewer',
  startedAt: '2026-10-06T12:00:00Z',
  updatedAt: '2026-10-06T12:00:00Z',
  steps: [],
  requiresApproval: false,
};

beforeAll(async () => {
  server.listen({ onUnhandledRequest: 'error' });
  // Load the route chunk before rendering so coverage instrumentation does not
  // consume the UI assertion's wait deadline.
  await import('./RunPage');
});
afterEach(() => {
  server.resetHandlers();
  queryClient.clear();
});
afterAll(() => server.close());

async function openRun(response: RunbookRun) {
  server.use(
    http.get('/api/runbook-runs/run-route', () => HttpResponse.json(response)),
    // RequireOnboarded lets the route through only once a connector exists.
    http.get('/api/connectors', () => HttpResponse.json([{ id: 'conn-1' }]))
  );
  useAuth.setState({
    status: 'authenticated',
    bootstrap: vi.fn().mockResolvedValue(undefined),
    user: {
      id: 'viewer',
      username: 'viewer',
      role: 'user',
      authSource: 'local',
      createdAt: '2026-01-01T00:00:00Z',
    },
  });
  window.history.replaceState({}, '', '/runbook-runs/run-route');
  const { default: App } = await import('../../App');
  render(<App />);
  await screen.findByText('Recorded remediation');
}

describe('run detail route', () => {
  it('lets an authenticated non-admin open a run directly', async () => {
    await openRun(run);
    expect(screen.getByRole('heading', { name: 'Run details', level: 1 })).toBeInTheDocument();
    expect(screen.getByText('Succeeded')).toBeInTheDocument();
  });

  it('opens an awaiting approval request with frozen pending steps and no execution controls', async () => {
    await openRun({
      ...run,
      state: 'awaiting_approval',
      requiresApproval: true,
      canApprove: false,
      approvalExpiresAt: '2026-10-07T12:00:00Z',
      steps: [
        {
          id: 'pending-step',
          position: 0,
          kind: 'manual',
          title: 'Review failover',
          connectorId: '',
          state: 'pending',
          redacted: false,
          canExecute: true,
        },
      ],
    });

    expect(screen.getByText('Awaiting approval')).toBeInTheDocument();
    expect(screen.getByText('Review failover')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel request' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Approve and start run' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Reject request' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Resume run' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Confirm step' })).not.toBeInTheDocument();
  });

  it('opens a deleted-runbook run and prevents resume', async () => {
    await openRun({ ...run, runbookId: undefined, state: 'failed' });
    expect(
      screen.getByText(
        'This runbook was deleted. This run can still be reviewed, but it cannot be resumed.'
      )
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Resume run' })).not.toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
