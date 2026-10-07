import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
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
  useGetRunbooks: () => ({
    data: { items: runbooks },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
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

vi.mock('../../components/runbook/RunHistory', () => ({
  RunHistory: ({
    runbookId,
    onSelectRun,
  }: {
    runbookId: string;
    onSelectRun: (id: string) => void;
  }) => <button onClick={() => onSelectRun('saved-run')}>History for {runbookId}</button>,
}));
vi.mock('../../components/runbook/RunDetail', () => ({
  RunDetail: ({ runId }: { runId: string }) => <p>Detail for {runId}</p>,
}));

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({
    data: [{ id: 'conn-1', name: 'pve1', type: 'proxmox' }],
  }),
  useGetConnectorsSchema: () => ({
    data: [
      {
        type: 'proxmox',
        category: 'hypervisor',
        displayName: 'Proxmox',
        fields: [],
        lifecycleVerbs: ['restart', 'stop'],
      },
    ],
  }),
  useGetConnectorsConnectorIdConfigFields: () => ({
    data: [
      {
        key: 'restartPolicy',
        label: 'Restart Policy',
        type: 'select',
        entityScope: true,
        options: ['no', 'on-failure', 'always', 'unless-stopped'],
      },
      { key: 'cores', label: 'CPU Cores', type: 'number', entityScope: true },
      { key: 'enabled', label: 'Enabled', type: 'toggle', entityScope: false },
      { key: 'name', label: 'Name', type: 'text', entityScope: false },
    ],
  }),
  useGetConnectorsConnectorIdSnapshots: () => ({ data: [{ id: 'snap-1' }] }),
  useGetConnectorsConnectorIdSnapshotsSnapshotId: () => ({
    data: { entities: [{ name: 'vm-100', externalId: '100' }] },
  }),
}));

vi.mock('../../api/generated/compliance/compliance', () => ({
  useGetComplianceSchema: () => ({
    data: {
      attributes: {
        proxmox: {
          virtual_machine: [{ name: 'status', type: 'string', description: 'VM state' }],
        },
      },
      joinFields: [],
    },
  }),
}));

function renderPage(entry = '/settings/runbooks') {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter initialEntries={[entry]}>
        <RunbooksPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('RunbooksPage steps editor', () => {
  afterEach(() => {
    runbooks = [];
    vi.clearAllMocks();
  });

  it('opens history from a runbook and then its run detail', () => {
    runbooks = [
      {
        id: 'rb-history',
        title: 'Recorded runbook',
        body: '',
        targetType: 'change_type',
        targetValue: 'vm.created',
        steps: [],
      },
    ];
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Run history for Recorded runbook' }));
    fireEvent.click(screen.getByRole('button', { name: 'History for rb-history' }));
    expect(screen.getByText('Detail for saved-run')).toBeInTheDocument();
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

    await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
    // toStrictEqual proves absent fields (timeoutSeconds) are really absent.
    expect(postRunbooks.mock.calls[0][0].steps).toStrictEqual([
      {
        kind: 'lifecycle',
        title: 'Restart the worker',
        connectorId: 'conn-1',
        verb: 'stop',
        entityRef: '100',
      },
    ]);
  });

  it('surfaces a field error for an invalid step', async () => {
    postRunbooks.mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 400,
        data: {
          code: 'invalid_request',
          message: 'bad',
          details: [{ field: 'steps[0].verb', msg: 'unsupported verb' }],
        },
      },
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

    await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
    expect(postRunbooks.mock.calls[0][0].steps).toStrictEqual([
      {
        kind: 'lifecycle',
        title: 'Restart worker',
        connectorId: 'conn-1',
        verb: 'stop',
        entityRef: '100',
      },
      {
        kind: 'sync_and_wait',
        title: 'Sync connectors',
        connectorId: 'conn-1',
        timeoutSeconds: 600,
      },
      {
        kind: 'wait_until_healthy',
        title: 'Wait for health',
        connectorId: 'conn-1',
      },
      {
        kind: 'manual',
        title: 'Confirm manually',
      },
    ]);
  });

  it('saves a config-push step with its writable field, typed value and entity', async () => {
    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Update policy' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByPlaceholderText('e.g. Restart the sync worker'), {
      target: { value: 'Change restart policy' },
    });
    fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'config_push' } });
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
    fireEvent.change(screen.getByLabelText('Writable field'), {
      target: { value: 'restartPolicy' },
    });
    fireEvent.change(screen.getByLabelText('Target value'), { target: { value: 'always' } });
    fireEvent.change(screen.getByLabelText('Entity'), { target: { value: '100' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
    expect(postRunbooks.mock.calls[0][0].steps).toStrictEqual([
      {
        kind: 'config_push',
        title: 'Change restart policy',
        connectorId: 'conn-1',
        fieldKey: 'restartPolicy',
        targetValue: 'always',
        entityRef: '100',
      },
    ]);
  });

  it.each([
    ['cores', '4', 4],
    ['name', 'worker-1', 'worker-1'],
  ])('keeps %s config-push values in their declared type', async (fieldKey, input, expected) => {
    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Change setting' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByPlaceholderText('e.g. Restart the sync worker'), {
      target: { value: 'Push setting' },
    });
    fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'config_push' } });
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
    fireEvent.change(screen.getByLabelText('Writable field'), { target: { value: fieldKey } });
    fireEvent.change(screen.getByLabelText('Target value'), { target: { value: input } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
    expect(postRunbooks.mock.calls[0][0].steps[0].targetValue).toBe(expected);
  });

  it('saves toggle config-push values as booleans', async () => {
    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Enable service' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByPlaceholderText('e.g. Restart the sync worker'), {
      target: { value: 'Enable service' },
    });
    fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'config_push' } });
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
    fireEvent.change(screen.getByLabelText('Writable field'), { target: { value: 'enabled' } });
    fireEvent.click(screen.getByLabelText('Target value'));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
    expect(postRunbooks.mock.calls[0][0].steps[0].targetValue).toBe(true);
  });

  it('saves a wait-for-entity step with the compliance attribute and timeout', async () => {
    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Wait for VM' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByPlaceholderText('e.g. Restart the sync worker'), {
      target: { value: 'Wait until running' },
    });
    fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'wait_for_entity' } });
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
    fireEvent.change(screen.getByLabelText('Entity'), { target: { value: '100' } });
    fireEvent.change(screen.getByLabelText('Attribute'), { target: { value: 'status' } });
    fireEvent.change(screen.getByLabelText('Operator'), { target: { value: 'eq' } });
    fireEvent.change(screen.getByLabelText('Expected value'), { target: { value: 'running' } });
    fireEvent.change(screen.getByLabelText('Timeout (minutes)'), { target: { value: '7' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
    expect(postRunbooks.mock.calls[0][0].steps).toStrictEqual([
      {
        kind: 'wait_for_entity',
        title: 'Wait until running',
        connectorId: 'conn-1',
        entityRef: '100',
        attribute: 'status',
        operator: 'eq',
        expectedValue: 'running',
        timeoutSeconds: 420,
      },
    ]);
  });

  it('shows server field errors for a wait step', async () => {
    postRunbooks.mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 400,
        data: {
          code: 'invalid_request',
          message: 'bad',
          details: [{ field: 'steps[0].attribute', msg: 'attribute is required' }],
        },
      },
    });
    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Wait for VM' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'wait_for_entity' } });
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    const fieldError = await screen.findByText('attribute is required');
    expect(fieldError.closest('p')).toHaveTextContent('Attribute: attribute is required');
    expect(fieldError.closest('p')).toHaveAttribute('role', 'alert');
  });

  describe('switching step kind', () => {
    const STEP_TITLE_PLACEHOLDER = 'e.g. Restart the sync worker';

    function startWithFilledLifecycleStep() {
      renderPage();
      fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
      fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Kind switch' } });
      fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
        target: { value: 'vm.created' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
      fireEvent.change(screen.getByPlaceholderText(STEP_TITLE_PLACEHOLDER), {
        target: { value: 'Switch me' },
      });
      fireEvent.change(screen.getByLabelText('Connector'), { target: { value: 'conn-1' } });
      fireEvent.change(screen.getByLabelText('Action'), { target: { value: 'stop' } });
      fireEvent.change(screen.getByLabelText('Entity'), { target: { value: '100' } });
    }

    it('drops connector, verb, entity and timeout when switching to manual', async () => {
      startWithFilledLifecycleStep();
      fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'sync_and_wait' } });
      fireEvent.change(screen.getByLabelText('Timeout (seconds)'), { target: { value: '600' } });
      fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'manual' } });

      fireEvent.click(screen.getByRole('button', { name: 'Save' }));

      await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
      expect(postRunbooks.mock.calls[0][0].steps).toStrictEqual([
        { kind: 'manual', title: 'Switch me' },
      ]);
    });

    it('drops verb and entity when switching a lifecycle step to sync_and_wait', async () => {
      startWithFilledLifecycleStep();
      fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'sync_and_wait' } });

      fireEvent.click(screen.getByRole('button', { name: 'Save' }));

      await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
      expect(postRunbooks.mock.calls[0][0].steps).toStrictEqual([
        { kind: 'sync_and_wait', title: 'Switch me', connectorId: 'conn-1' },
      ]);
    });

    it('drops the timeout when switching a sync_and_wait step back to lifecycle', async () => {
      startWithFilledLifecycleStep();
      fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'sync_and_wait' } });
      fireEvent.change(screen.getByLabelText('Timeout (seconds)'), { target: { value: '600' } });
      fireEvent.change(screen.getByLabelText('Kind'), { target: { value: 'lifecycle' } });

      fireEvent.click(screen.getByRole('button', { name: 'Save' }));

      await waitFor(() => expect(postRunbooks).toHaveBeenCalledTimes(1));
      expect(postRunbooks.mock.calls[0][0].steps).toStrictEqual([
        { kind: 'lifecycle', title: 'Switch me', connectorId: 'conn-1', verb: 'restart' },
      ]);
    });
  });

  it('sends a legacy lifecycle step back unchanged when saved without edits', async () => {
    runbooks = [
      {
        id: 'rb-legacy',
        title: 'Legacy runbook',
        body: '',
        targetType: 'change_type',
        targetValue: 'vm.created',
        steps: [
          {
            id: 'step-legacy',
            position: 0,
            kind: 'lifecycle',
            title: 'Restart the VM',
            connectorId: 'conn-1',
            connectorName: 'pve1',
            verb: 'restart',
            entityRef: '100',
            timeoutSeconds: 0,
            canExecute: true,
          },
        ],
      },
    ];

    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Edit runbook' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(putRunbooksRunbookId).toHaveBeenCalledTimes(1));
    expect(putRunbooksRunbookId.mock.calls[0][0]).toBe('rb-legacy');
    expect(putRunbooksRunbookId.mock.calls[0][1].steps).toStrictEqual([
      {
        id: 'step-legacy',
        kind: 'lifecycle',
        title: 'Restart the VM',
        connectorId: 'conn-1',
        verb: 'restart',
        entityRef: '100',
      },
    ]);
  });

  it('surfaces a field error for an invalid step timeout', async () => {
    postRunbooks.mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 400,
        data: {
          code: 'invalid_request',
          message: 'bad',
          details: [
            { field: 'steps[0].timeoutSeconds', msg: 'must be between 10 and 1800 seconds' },
          ],
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

  it('shows every server error for a step with its field label and clears them when steps change', async () => {
    const stepErrorBody = {
      isAxiosError: true,
      response: {
        status: 400,
        data: {
          code: 'invalid_request',
          message: 'bad',
          details: [
            { field: 'steps[0].title', msg: 'must not be empty' },
            { field: 'steps[0].connectorId', msg: 'is required' },
          ],
        },
      },
    };
    postRunbooks.mockRejectedValueOnce(stepErrorBody).mockRejectedValueOnce(stepErrorBody);

    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Two errors' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    const titleError = await screen.findByText('must not be empty');
    expect(titleError.closest('p')).toHaveTextContent('Title: must not be empty');
    expect(titleError.closest('p')).toHaveAttribute('role', 'alert');
    const connectorError = screen.getByText('is required');
    expect(connectorError.closest('p')).toHaveTextContent('Connector: is required');
    expect(connectorError.closest('p')).toHaveAttribute('role', 'alert');

    // Adding a step clears the index-keyed errors.
    fireEvent.click(screen.getByRole('button', { name: 'Add step' }));
    expect(screen.queryByText('must not be empty')).not.toBeInTheDocument();
    expect(screen.queryByText('is required')).not.toBeInTheDocument();

    // Moving a step clears them too.
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByText('must not be empty')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: 'Move step down' })[0]);
    expect(screen.queryByText('must not be empty')).not.toBeInTheDocument();
    expect(screen.queryByText('is required')).not.toBeInTheDocument();
  });

  it('keeps non-step server errors visible', async () => {
    postRunbooks.mockRejectedValueOnce({
      isAxiosError: true,
      response: {
        status: 400,
        data: {
          code: 'invalid_request',
          message: 'bad',
          details: [{ field: 'steps', msg: 'too many steps' }],
        },
      },
    });

    renderPage();
    fireEvent.click(screen.getAllByRole('button', { name: 'New runbook' })[0]);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Too many' } });
    fireEvent.change(screen.getByLabelText('Target', { exact: false }), {
      target: { value: 'vm.created' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('too many steps')).toBeInTheDocument();
  });

  describe('runbook with a redacted step', () => {
    const lockedHint =
      'This runbook has steps on connectors you cannot view, so its steps cannot be edited here. Your other changes are saved and the steps stay as they are.';

    const redactedStep = {
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
    };

    it('locks the steps and omits them from the saved payload', async () => {
      runbooks = [
        {
          id: 'rb-1',
          title: 'Runbook with redacted step',
          targetType: 'change_type',
          targetValue: 'vm.created',
          steps: [redactedStep],
        },
      ];

      renderPage();

      fireEvent.click(screen.getByRole('button', { name: 'Edit runbook' }));

      expect(screen.getByDisplayValue('Restricted step')).toBeDisabled();
      expect(
        screen.getByText('This step targets a connector you do not have permission to view.')
      ).toBeInTheDocument();
      expect(screen.getByText(lockedHint)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Add step' })).toBeDisabled();
      // Kind selector should not be displayed for redacted step
      expect(screen.queryByLabelText('Kind')).not.toBeInTheDocument();

      // Other fields stay editable.
      fireEvent.change(screen.getByDisplayValue('Runbook with redacted step'), {
        target: { value: 'Updated runbook' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'Save' }));

      await waitFor(() => expect(putRunbooksRunbookId).toHaveBeenCalledTimes(1));
      expect(putRunbooksRunbookId.mock.calls[0][0]).toBe('rb-1');
      const payload = putRunbooksRunbookId.mock.calls[0][1];
      expect(payload.title).toBe('Updated runbook');
      // An absent steps key tells the server to keep every stored step.
      expect(payload).not.toHaveProperty('steps');
    });

    it('also makes the visible steps read-only', () => {
      runbooks = [
        {
          id: 'rb-2',
          title: 'Mixed runbook',
          targetType: 'change_type',
          targetValue: 'vm.created',
          steps: [
            redactedStep,
            {
              id: 'step-visible',
              position: 1,
              kind: 'lifecycle',
              title: 'Restart the VM',
              connectorId: 'conn-1',
              connectorName: 'pve1',
              verb: 'restart',
              entityRef: '100',
              timeoutSeconds: 0,
              canExecute: true,
            },
          ],
        },
      ];

      renderPage();
      fireEvent.click(screen.getByRole('button', { name: 'Edit runbook' }));

      expect(screen.getByText(lockedHint)).toBeInTheDocument();
      expect(screen.getByDisplayValue('Restart the VM')).toBeDisabled();
      expect(screen.getByLabelText('Kind')).toBeDisabled();
      expect(screen.getByLabelText('Connector')).toBeDisabled();
      expect(screen.getByLabelText('Action')).toBeDisabled();
      expect(screen.getByLabelText('Entity')).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Move step up' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Move step down' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Remove step' })).toBeDisabled();
      expect(screen.getByRole('button', { name: 'Add step' })).toBeDisabled();
    });

    it('does not show the locked hint for a runbook without redacted steps', () => {
      runbooks = [
        {
          id: 'rb-3',
          title: 'Plain runbook',
          targetType: 'change_type',
          targetValue: 'vm.created',
          steps: [],
        },
      ];

      renderPage();
      fireEvent.click(screen.getByRole('button', { name: 'Edit runbook' }));

      expect(screen.queryByText(lockedHint)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Add step' })).toBeEnabled();
    });
  });
});
