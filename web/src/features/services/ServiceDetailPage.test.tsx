import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ServiceDetailPage } from './ServiceDetailPage';

const {
  restart,
  start,
  stop,
  actionRun,
  health,
  release,
  managedBy,
  connectorRecord,
  snapshotEntities,
  configPush,
  configFields,
  syncRows,
  snapshotRows,
} = vi.hoisted(() => ({
  release: vi.fn(),
  // Overrides the mocked connector's managedBy for the config-managed tests (#500).
  managedBy: { value: undefined as string | undefined },
  restart: vi.fn(),
  start: vi.fn(),
  stop: vi.fn(),
  actionRun: vi.fn(),
  connectorRecord: {
    value: {
      id: 'svc-pve1',
      name: 'pve1',
      type: 'proxmox',
      url: 'https://pve1.example',
      enabled: true,
      status: 'online',
      scheduleSeconds: null,
      nextRunAt: '',
      lastSyncAt: '',
      managedBy: undefined as string | undefined,
      capabilities: { restart: true, start: true, stop: true, configPush: false, configRead: false, credentialRefresh: false },
      actions: [] as Array<{
        name: string;
        entityScope: boolean;
        entityKind?: string;
        label?: string;
        description?: string;
        downtimeSeconds: number;
      }>,
    },
  },
  snapshotEntities: {
    value: [
      { name: 'vm-100', kind: 'vm', externalId: '100' },
      { name: 'web-container', kind: 'container', externalId: 'container-1' },
    ],
  },
  health: vi.fn(),
  configPush: vi.fn(),
  configFields: vi.fn().mockReturnValue({ data: [] }),
  syncRows: vi.fn().mockReturnValue({ data: [] }),
  snapshotRows: vi.fn().mockReturnValue({ data: [] }),
}));

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsConnectorId: () => ({
    data: { ...connectorRecord.value, managedBy: managedBy.value },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useGetConnectorsConnectorIdData: () => ({
    data: { sections: [], fetchedAt: new Date().toISOString() },
  }),
  useGetConnectorsConnectorIdSyncs: syncRows,
  useGetConnectorsConnectorIdSnapshots: snapshotRows,
  useGetConnectorsConnectorIdSnapshotsSnapshotId: () => ({
    data: { entities: snapshotEntities.value },
  }),
  useGetConnectorsConnectorIdConfigFields: configFields,
  useGetConnectorsSchema: () => ({ data: [] }),
  postConnectorsConnectorIdRestart: restart,
  postConnectorsConnectorIdStart: start,
  postConnectorsConnectorIdStop: stop,
  postConnectorsConnectorIdActionsName: actionRun,
  postConnectorsConnectorIdConfigPush: configPush,
  postConnectorsConnectorIdHealth: health,
  postConnectorsConnectorIdRelease: release,
  putConnectorsConnectorIdEnabled: vi.fn(),
  putConnectorsConnectorId: vi.fn(),
  getGetConnectorsQueryKey: () => [],
}));

vi.mock('../../api/generated/changes/changes', () => ({
  useGetChanges: () => ({ data: { items: [] } }),
}));
vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsServiceConnectorId: () => ({ isError: true }),
  getGetDocsServiceConnectorIdQueryKey: () => [],
  postDocsGenerate: vi.fn(),
}));
vi.mock('../../api/generated/templates/templates', () => ({
  useGetTemplates: () => ({ data: [] }),
}));
vi.mock('./UptimePanel', () => ({ UptimePanel: () => null }));
vi.mock('../../hooks/useRole', () => ({ useConnectorRole: () => 'operator' }));
vi.mock('../../store/live', () => ({
  useLive: (selector: (state: object) => unknown) =>
    selector({ statusOverrides: {}, activity: [] }),
}));
vi.mock('../../lib/runSync', () => ({ runSync: vi.fn() }));
const { toastWarning, toastSuccess } = vi.hoisted(() => ({
  toastWarning: vi.fn(),
  toastSuccess: vi.fn(),
}));
vi.mock('../../lib/toast', () => ({
  toast: { warning: toastWarning, success: toastSuccess, error: vi.fn() },
}));
vi.mock('../../components/manager/ConfirmDestructive', () => ({ ConfirmDestructive: () => null }));
vi.mock('../../components/manager/ElevationConfirm', () => ({
  ElevationConfirm: ({
    open,
    title,
    action,
    target,
    onConfirm,
  }: {
    open: boolean;
    title: string;
    action: string;
    target?: string;
    onConfirm: (token: string | null) => void;
  }) =>
    open ? (
      <div role="dialog" aria-label={title} data-action={action} data-target={target}>
        <button onClick={() => onConfirm(null)}>confirm-elevation</button>
        <button onClick={() => onConfirm('step-up-token')}>confirm-elevation-token</button>
      </div>
    ) : null,
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

const originalConnectorRecord = connectorRecord.value;

function renderPage() {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter initialEntries={['/services/svc-pve1']}>
        <Routes>
          <Route path="/services/:id" element={<ServiceDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  connectorRecord.value = originalConnectorRecord;
  managedBy.value = undefined;
  syncRows.mockReturnValue({ data: [] });
  snapshotRows.mockReturnValue({ data: [] });
});

it('links snapshot history and sync changes to the browser', () => {
  syncRows.mockReturnValue({ data: [{ id: 'sync-1', snapshotId: 'new', status: 'success', startedAt: '2026-09-26T10:00:00Z', durationMs: 100, attempt: 1 }] });
  snapshotRows.mockReturnValue({ data: [{ id: 'new' }, { id: 'old' }] });
  renderPage();
  expect(screen.getByRole('link', { name: 'History' })).toHaveAttribute('href', '/services/svc-pve1/snapshots');
  expect(screen.getByRole('link', { name: 'View changes' })).toHaveAttribute('href', '/services/svc-pve1/snapshots?a=old&b=new');
});

describe('ServiceDetailPage restart preview', () => {
  it('shows the read-only restart impact returned by the connector API', async () => {
    restart.mockResolvedValue({
      targetService: 'pve1',
      estimatedDowntimeSeconds: 30,
      dependentServices: [{ kind: 'vm', name: 'home-assistant' }],
    });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Restart' }));

    await waitFor(() =>
      expect(restart).toHaveBeenCalledWith('svc-pve1', { entityRef: undefined }, { dryRun: true })
    );
    expect(await screen.findByRole('dialog', { name: 'Restart impact' })).toHaveTextContent(
      'Review the impact below, then confirm to restart.'
    );
    expect(screen.getByText('30 seconds')).toBeInTheDocument();
    expect(screen.getByText('home-assistant')).toBeInTheDocument();
  });

  it('sends the picked entityRef for both the dry-run and the real restart', async () => {
    restart.mockResolvedValue({
      targetService: 'vm-100',
      estimatedDowntimeSeconds: 10,
      dependentServices: [],
    });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Restart' }));
    await waitFor(() =>
      expect(restart).toHaveBeenCalledWith('svc-pve1', { entityRef: undefined }, { dryRun: true })
    );

    fireEvent.change(await screen.findByRole('combobox', { name: 'Entity' }), {
      target: { value: '100' },
    });

    await waitFor(() =>
      expect(restart).toHaveBeenLastCalledWith('svc-pve1', { entityRef: '100' }, { dryRun: true })
    );

    fireEvent.click(screen.getByRole('button', { name: 'Restart now' }));
    fireEvent.click(await screen.findByRole('button', { name: 'confirm-elevation' }));

    await waitFor(() =>
      expect(restart).toHaveBeenLastCalledWith(
        'svc-pve1',
        { entityRef: '100' },
        { dryRun: false },
        undefined
      )
    );
  });
});

describe('ServiceDetailPage health check', () => {
  it('runs a health check and shows the result, disabling the button while pending', async () => {
    let resolveHealth: (value: { status: string; message: string; latencyMs: number }) => void =
      () => {};
    health.mockReturnValue(
      new Promise((resolve) => {
        resolveHealth = resolve;
      })
    );

    renderPage();
    const button = await screen.findByRole('button', { name: /Health check/ });
    fireEvent.click(button);

    await waitFor(() => expect(health).toHaveBeenCalledWith('svc-pve1'));
    expect(await screen.findByRole('button', { name: 'Checking…' })).toBeDisabled();

    resolveHealth({ status: 'degraded', message: 'Slow response', latencyMs: 820 });

    expect(await screen.findByRole('status')).toHaveTextContent('Slow response');
    expect(screen.getByRole('status')).toHaveTextContent('took 820ms');
  });

  it('shows an error message when the health check fails', async () => {
    health.mockRejectedValue(new Error('boom'));

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /Health check/ }));

    expect(await screen.findByRole('status')).toHaveTextContent(
      "Couldn't run the health check. Try again."
    );
  });
});

describe('ServiceDetailPage start/stop preview', () => {
  it('shows the read-only start impact and estimatedDowntimeSeconds', async () => {
    start.mockResolvedValue({
      targetService: 'pve1',
      estimatedDowntimeSeconds: 15,
      dependentServices: [],
    });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Start' }));

    await waitFor(() =>
      expect(start).toHaveBeenCalledWith('svc-pve1', { entityRef: undefined }, { dryRun: true })
    );
    expect(await screen.findByRole('dialog', { name: 'Start impact' })).toHaveTextContent('15 seconds');
  });

  it('shows an indefinite downtime label for stop (estimatedDowntimeSeconds: 0)', async () => {
    stop.mockResolvedValue({
      targetService: 'pve1',
      estimatedDowntimeSeconds: 0,
      dependentServices: [],
    });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Stop' }));

    await waitFor(() =>
      expect(stop).toHaveBeenCalledWith('svc-pve1', { entityRef: undefined }, { dryRun: true })
    );
    expect(await screen.findByRole('dialog', { name: 'Stop impact' })).toHaveTextContent(
      'Indefinite (until started again)'
    );
  });
});

describe('ServiceDetailPage config push', () => {
  it('pushes a field and shows a success toast', async () => {
    configFields.mockReturnValue({
      data: [{ key: 'memory', label: 'Memory (MB)', type: 'number', entityScope: true }],
    });
    configPush.mockResolvedValue({});

    renderPage();
    fireEvent.change(await screen.findByDisplayValue('This connector has no writable config fields.'), {
      target: { value: 'memory' },
    });
    fireEvent.change(screen.getByPlaceholderText('entityRef'), { target: { value: '100' } });
    fireEvent.change(screen.getByPlaceholderText('New value'), { target: { value: '4096' } });
    fireEvent.click(screen.getByRole('button', { name: 'Push' }));
    fireEvent.click(await screen.findByRole('button', { name: 'confirm-elevation' }));

    await waitFor(() =>
      expect(configPush).toHaveBeenCalledWith(
        'svc-pve1',
        expect.objectContaining({ entityRef: '100', fieldKey: 'memory', value: 4096 }),
        undefined
      )
    );
    expect(toastSuccess).toHaveBeenCalled();
  });

  it('shows a mismatch toast on a 409 response', async () => {
    configFields.mockReturnValue({
      data: [{ key: 'memory', label: 'Memory (MB)', type: 'number', entityScope: true }],
    });
    configPush.mockRejectedValue({ isAxiosError: true, response: { status: 409 } });

    renderPage();
    fireEvent.change(await screen.findByDisplayValue('This connector has no writable config fields.'), {
      target: { value: 'memory' },
    });
    fireEvent.change(screen.getByPlaceholderText('entityRef'), { target: { value: '100' } });
    fireEvent.change(screen.getByPlaceholderText('New value'), { target: { value: '4096' } });
    fireEvent.click(screen.getByRole('button', { name: 'Push' }));
    fireEvent.click(await screen.findByRole('button', { name: 'confirm-elevation' }));

    await waitFor(() => expect(toastWarning).toHaveBeenCalled());
  });
});

describe('ServiceDetailPage config-managed connectors (#500)', () => {
  it('offers every control for a UI-managed connector', () => {
    renderPage();
    for (const name of ['Edit', 'Remove', 'Disable', 'Sync']) {
      expect(screen.getByRole('button', { name })).toBeInTheDocument();
    }
    expect(screen.queryByText('config')).not.toBeInTheDocument();
  });

  it('locks settings but keeps operational actions for a config-managed connector', () => {
    managedBy.value = 'config';
    renderPage();
    expect(screen.getByText('config')).toBeInTheDocument();
    expect(screen.getByText(/Managed by config\.yaml/)).toBeInTheDocument();
    for (const name of ['Edit', 'Remove', 'Disable']) {
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument();
    }
    expect(screen.getByRole('button', { name: 'Sync' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /health/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Release to UI' })).not.toBeInTheDocument();
  });

  it('offers only release and remove for an orphaned connector', async () => {
    managedBy.value = 'config-orphaned';
    release.mockResolvedValue({});
    renderPage();
    expect(screen.getByText('orphaned')).toBeInTheDocument();
    for (const name of ['Edit', 'Disable', 'Enable', 'Sync']) {
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument();
    }
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Release to UI' }));
    await waitFor(() => expect(release).toHaveBeenCalledWith('svc-pve1'));
  });
});

describe('ServiceDetailPage recipe actions', () => {
  const rescanPreview = {
    userDefined: true,
    label: 'Rescan library',
    description: 'Rebuild the library index',
    request: {
      method: 'POST',
      url: 'https://svc.example/api/rescan',
      headers: {},
      body: { force: false },
    },
    targetService: 'pve1',
    estimatedDowntimeSeconds: 0,
    dependentServices: [],
    affectedEntities: [],
  };

  beforeEach(() => {
    actionRun.mockReset();
  });

  it('shows the restart and rescan controls for a custom connector, but no Start or Stop', async () => {
    connectorRecord.value = {
      ...originalConnectorRecord,
      type: 'custom',
      actions: [
        { name: 'restart', entityScope: false, downtimeSeconds: 0 },
        { name: 'rescan', entityScope: false, label: 'Rescan library', downtimeSeconds: 0 },
      ],
    };

    renderPage();

    expect(await screen.findByRole('button', { name: 'Restart' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Rescan library' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Start' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Stop' })).not.toBeInTheDocument();
  });

  it('keeps Start, Stop and Restart for a built-in proxmox connector', async () => {
    connectorRecord.value = {
      ...originalConnectorRecord,
      type: 'proxmox',
      capabilities: {
        restart: true,
        start: true,
        stop: true,
        configPush: true,
        configRead: true,
        credentialRefresh: true,
      },
      actions: [],
    };

    renderPage();

    expect(await screen.findByRole('button', { name: 'Start' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Restart' })).toBeInTheDocument();
  });

  it('runs the dry-run preview, then the elevated action, and shows the result', async () => {
    connectorRecord.value = {
      ...originalConnectorRecord,
      type: 'custom',
      actions: [
        { name: 'rescan', entityScope: false, label: 'Rescan library', description: 'Rebuild the library index', downtimeSeconds: 0 },
      ],
    };
    actionRun.mockImplementation((_id: string, _name: string, _body: unknown, params: { dryRun?: boolean }) =>
      Promise.resolve(params.dryRun ? rescanPreview : { status: 200, excerpt: 'OK' })
    );

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Rescan library' }));

    await waitFor(() =>
      expect(actionRun).toHaveBeenCalledWith('svc-pve1', 'rescan', { entityRef: undefined }, { dryRun: true }, undefined)
    );
    expect(actionRun).toHaveBeenCalledTimes(1);

    const previewDialog = await screen.findByRole('dialog', { name: 'Preview Rescan library' });
    expect(previewDialog).toHaveTextContent('Recipe-defined action');
    expect(previewDialog).toHaveTextContent('Action: Rescan library');
    expect(previewDialog).toHaveTextContent('Description: Rebuild the library index');
    expect(within(previewDialog).getByText('POST')).toBeInTheDocument();
    expect(within(previewDialog).getByText('https://svc.example/api/rescan')).toBeInTheDocument();
    expect(previewDialog).toHaveTextContent('"force": false');

    expect(screen.queryByRole('button', { name: 'confirm-elevation-token' })).not.toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: 'Run Rescan library' })).not.toBeInTheDocument();

    fireEvent.click(within(previewDialog).getByRole('button', { name: 'Run action' }));

    const elevation = await screen.findByRole('dialog', { name: 'Run Rescan library' });
    expect(elevation).toHaveAttribute('data-action', 'connector.action');
    expect(elevation).toHaveAttribute('data-target', 'svc-pve1:rescan');
    expect(actionRun).toHaveBeenCalledTimes(1);

    fireEvent.click(within(elevation).getByRole('button', { name: 'confirm-elevation-token' }));

    await waitFor(() =>
      expect(actionRun).toHaveBeenLastCalledWith(
        'svc-pve1',
        'rescan',
        { entityRef: undefined },
        { dryRun: false },
        { headers: { 'X-Elevation-Token': 'step-up-token' } }
      )
    );

    const resultDialog = await screen.findByRole('dialog', { name: 'Action result' });
    expect(resultDialog).toHaveTextContent('Response status: 200');
    expect(within(resultDialog).getByText('OK')).toBeInTheDocument();
  });

  it('requests the dry-run for an entity-scoped action only after an entity is picked', async () => {
    connectorRecord.value = {
      ...originalConnectorRecord,
      type: 'custom',
      actions: [
        { name: 'rescan', entityScope: true, entityKind: 'container', label: 'Rescan container', downtimeSeconds: 0 },
      ],
    };
    actionRun.mockResolvedValue({ ...rescanPreview, targetService: 'web-container' });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Rescan container (container)' }));

    const previewDialog = await screen.findByRole('dialog', { name: 'Preview Rescan container' });
    expect(within(previewDialog).getByText('Choose an entity to preview this action.')).toBeInTheDocument();
    expect(actionRun).not.toHaveBeenCalled();

    fireEvent.change(within(previewDialog).getByRole('combobox', { name: 'Target container' }), {
      target: { value: 'container-1' },
    });

    await waitFor(() =>
      expect(actionRun).toHaveBeenCalledWith('svc-pve1', 'rescan', { entityRef: 'container-1' }, { dryRun: true }, undefined)
    );
    expect(actionRun).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole('button', { name: 'Run action' })).toBeInTheDocument();
  });
});
