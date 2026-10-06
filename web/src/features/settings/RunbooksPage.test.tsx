import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { RunbooksPage } from './RunbooksPage';

const { postRunbooks, putRunbooksRunbookId, deleteRunbooksRunbookId } = vi.hoisted(() => ({
  postRunbooks: vi.fn().mockResolvedValue({}),
  putRunbooksRunbookId: vi.fn().mockResolvedValue({}),
  deleteRunbooksRunbookId: vi.fn().mockResolvedValue({}),
}));

let runbooks: unknown[] = [];

vi.mock('../../api/generated/runbooks/runbooks', () => ({
  useGetRunbooks: () => ({ data: { items: runbooks }, isLoading: false, isError: false, refetch: vi.fn() }),
  getGetRunbooksQueryKey: () => ['runbooks'],
  postRunbooks,
  putRunbooksRunbookId,
  deleteRunbooksRunbookId,
}));

vi.mock('../../components/ui/Dialog', () => ({
  Dialog: ({
    open,
    title,
    children,
  }: {
    open: boolean;
    title: string;
    children: React.ReactNode;
  }) =>
    open ? (
      <div role="dialog" aria-label={title}>
        {children}
      </div>
    ) : null,
}));

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({
    data: [{ id: 'conn-1', name: 'pve1', type: 'proxmox' }],
  }),
  useGetConnectorsSchema: () => ({
    data: [{ type: 'proxmox', category: 'hypervisor', displayName: 'Proxmox', fields: [], lifecycleVerbs: ['restart', 'stop'] }],
  }),
  useGetConnectorsConnectorIdSnapshots: () => ({ data: [{ id: 'snap-1' }] }),
  useGetConnectorsConnectorIdSnapshotsSnapshotId: () => ({
    data: { entities: [{ name: 'vm-100', externalId: '100' }] },
  }),
}));

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <RunbooksPage />
    </QueryClientProvider>
  );
}

describe('RunbooksPage steps editor', () => {
  afterEach(() => {
    runbooks = [];
    vi.clearAllMocks();
  });

  it('creates a runbook with a step', async () => {
    renderPage();

    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Restart worker' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByPlaceholderText('e.g. Restart the sync worker'), {
      target: { value: 'Restart the worker' },
    });

    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
    fireEvent.change(screen.getByLabelText('Action'), { target: { value: 'stop' } });
    fireEvent.change(screen.getByLabelText('Entity'), { target: { value: '100' } });

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(postRunbooks).toHaveBeenCalledWith(
        expect.objectContaining({
          steps: [
            expect.objectContaining({
              title: 'Restart the worker',
              connectorId: 'conn-1',
              verb: 'stop',
              entityRef: '100',
            }),
          ],
        })
      )
    );
  });

  it('surfaces a field error for an invalid step', async () => {
    postRunbooks.mockRejectedValueOnce({
      isAxiosError: true,
      response: { status: 400, data: { code: 'invalid_request', message: 'bad', details: [{ field: 'steps[0].verb', msg: 'unsupported verb' }] } },
    });

    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Restart worker' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('unsupported verb')).toBeInTheDocument();
  });

  it('creates a runbook with a step of each kind', async () => {
    renderPage();

    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Multi step runbook' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });

    // 1. Lifecycle step
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    const titleInputs = screen.getAllByLabelText('Title');
    fireEvent.change(titleInputs[1], { target: { value: 'Restart worker' } });
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
    fireEvent.change(screen.getByLabelText('Action'), { target: { value: 'stop' } });
    fireEvent.change(screen.getByLabelText('Entity'), { target: { value: '100' } });

    // 2. Sync and wait step
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    const kindSelects = screen.getAllByLabelText('Kind');
    fireEvent.change(kindSelects[1], { target: { value: 'sync_and_wait' } });
    const allTitleInputs2 = screen.getAllByLabelText('Title');
    fireEvent.change(allTitleInputs2[2], { target: { value: 'Sync connectors' } });
    const connectorSelects = screen.getAllByLabelText('Connector');
    fireEvent.change(connectorSelects[1], { target: { value: 'conn-1' } });
    const timeoutInput = screen.getByLabelText('Timeout (seconds)');
    fireEvent.change(timeoutInput, { target: { value: '600' } });

    // 3. Wait until healthy step
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    const kindSelects3 = screen.getAllByLabelText('Kind');
    fireEvent.change(kindSelects3[2], { target: { value: 'wait_until_healthy' } });
    const allTitleInputs3 = screen.getAllByLabelText('Title');
    fireEvent.change(allTitleInputs3[3], { target: { value: 'Wait for health' } });
    const connectorSelects3 = screen.getAllByLabelText('Connector');
    fireEvent.change(connectorSelects3[2], { target: { value: 'conn-1' } });

    // 4. Manual step
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    const kindSelects4 = screen.getAllByLabelText('Kind');
    fireEvent.change(kindSelects4[3], { target: { value: 'manual' } });
    const allTitleInputs4 = screen.getAllByLabelText('Title');
    fireEvent.change(allTitleInputs4[4], { target: { value: 'Confirm manually' } });

    // Verify manual step has no connector or action or timeout
    expect(screen.getAllByLabelText('Connector')).toHaveLength(3);
    expect(screen.getAllByLabelText('Action')).toHaveLength(1);

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(postRunbooks).toHaveBeenCalledWith(
        expect.objectContaining({
          steps: [
            expect.objectContaining({
              kind: 'lifecycle',
              title: 'Restart worker',
              connectorId: 'conn-1',
              verb: 'stop',
              entityRef: '100',
            }),
            expect.objectContaining({
              kind: 'sync_and_wait',
              title: 'Sync connectors',
              connectorId: 'conn-1',
              timeoutSeconds: 600,
            }),
            expect.objectContaining({
              kind: 'wait_until_healthy',
              title: 'Wait for health',
              connectorId: 'conn-1',
            }),
            expect.objectContaining({
              kind: 'manual',
              title: 'Confirm manually',
            }),
          ],
        })
      )
    );
  });

  it('surfaces a field error for an invalid step timeout', async () => {
    postRunbooks.mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 400,
        data: {
          code: 'invalid_request',
          message: 'bad',
          details: [{ field: 'steps[0].timeoutSeconds', msg: 'must be between 10 and 1800 seconds' }],
        },
      },
    });

    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Timeout test' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'sync_and_wait' } });
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('must be between 10 and 1800 seconds')).toBeInTheDocument();
  });

  it('keeps a redacted step read-only and sends it back untouched', async () => {
    runbooks = [
      {
        id: 'rb-1',
        title: 'Runbook with redacted step',
        targetType: 'change_type',
        targetValue: 'vm.created',
        steps: [
          {
            id: 'step-secret',
            position: 0,
            title: 'Restricted step',
            connectorId: '',
            connectorName: '',
            verb: '',
            entityRef: '',
            timeoutSeconds: 0,
            canExecute: false,
            executeBlockedReason: 'no_viewer_grant',
          },
        ],
      },
    ];

    renderPage();

    fireEvent.click(screen.getByRole('button', { name: 'Edit runbook' }));

    // Redacted step title should be disabled/read-only
    const stepTitle = screen.getByDisplayValue('Restricted step');
    expect(stepTitle).toBeDisabled();
    expect(screen.getByText('This step targets a connector you do not have permission to view.')).toBeInTheDocument();
    // Kind selector should not be displayed for redacted step
    expect(screen.queryByLabelText('Kind')).not.toBeInTheDocument();

    // Edit runbook title
    const runbookTitle = screen.getByDisplayValue('Runbook with redacted step');
    fireEvent.change(runbookTitle, { target: { value: 'Updated runbook' } });

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(putRunbooksRunbookId).toHaveBeenCalledWith(
        'rb-1',
        expect.objectContaining({
          title: 'Updated runbook',
          steps: [
            expect.objectContaining({
              id: 'step-secret',
              title: 'Restricted step',
            }),
          ],
        })
      )
    );

    // Assert kind was NOT set on the redacted step input payload
    const payload = putRunbooksRunbookId.mock.calls[0][1];
    expect(payload.steps[0]).not.toHaveProperty('kind');
  });
});
