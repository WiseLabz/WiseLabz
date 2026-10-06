import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError } from 'axios';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguagePreference } from '../../i18n';
import { EntityDetailPage } from './EntityDetailPage';

const {
  entityHook,
  isInstanceAdminMock,
  postOverrideMutate,
  getEntitiesIdMock,
  connectorsHook,
  snapshotsHook,
  snapshotDetailHook,
  toastMock,
} = vi.hoisted(() => ({
  entityHook: vi.fn(),
  isInstanceAdminMock: vi.fn(),
  postOverrideMutate: vi.fn(),
  getEntitiesIdMock: vi.fn(),
  connectorsHook: vi.fn(),
  snapshotsHook: vi.fn(),
  snapshotDetailHook: vi.fn(),
  toastMock: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('../../api/generated/search/search', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/search/search')>()),
  useGetEntitiesId: (id: string) => entityHook(id),
  getEntitiesId: (id: string) => getEntitiesIdMock(id),
  usePostEntityOverrides: (options?: {
    mutation?: { onError?: (err: unknown) => void };
  }) => ({
    mutate: (vars: { data: unknown }, callbacks?: { onSuccess?: (res: unknown) => void }) => {
      postOverrideMutate(vars.data);
      try {
        const results = postOverrideMutate.mock.results;
        const res = results[results.length - 1]?.value;
        if (res instanceof Error) {
          options?.mutation?.onError?.(res);
        } else if (res) {
          callbacks?.onSuccess?.(res);
        }
      } catch (err) {
        options?.mutation?.onError?.(err);
      }
    },
    isPending: false,
  }),
}));

vi.mock('../../hooks/useRole', () => ({
  useIsInstanceAdmin: () => isInstanceAdminMock(),
}));

vi.mock('../../api/generated/connectors/connectors', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/connectors/connectors')>()),
  useGetConnectors: () => connectorsHook(),
  useGetConnectorsConnectorIdSnapshots: () => snapshotsHook(),
  useGetConnectorsConnectorIdSnapshotsSnapshotId: () => snapshotDetailHook(),
}));

vi.mock('sonner', () => ({
  toast: toastMock,
}));

function mount() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={['/entities/entity-1']}>
        <Routes>
          <Route path="/entities/:id" element={<EntityDetailPage />} />
          <Route path="/entities/overrides" element={<div data-testid="overrides-page">Overrides</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const finding = (id: string, severity: string, status: string) => ({
  id,
  connectorId: 'c1',
  checkType: 'compliance',
  severity,
  title: `finding ${id}`,
  description: '',
  status,
  detectedCount: 1,
  firstDetectedAt: '2026-09-01T00:00:00Z',
  lastSeenAt: '2026-09-01T00:00:00Z',
});

const base = {
  id: 'entity-1',
  kind: 'vm',
  name: 'router',
  gone: false,
  members: [
    {
      connectorId: 'c1',
      connectorName: 'Proxmox lab',
      docId: 'doc-1',
      kind: 'vm',
      ref: '42',
      name: 'router',
    },
  ],
  relatedByIp: [],
  neighbors: [],
  history: [],
  findings: [],
  onReportingConnectors: [],
  runbooks: [],
};

const statusError = (status: number) =>
  new AxiosError('failed', 'ERR_BAD_REQUEST', undefined, undefined, { status } as never);

describe('EntityDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void setLanguagePreference('en');
    isInstanceAdminMock.mockReturnValue(true);
    entityHook.mockReturnValue({
      data: { ...base, gone: true },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    connectorsHook.mockReturnValue({
      data: [
        { id: 'c1', name: 'Proxmox lab' },
        { id: 'c2', name: 'OPNsense' },
      ],
      isLoading: false,
    });
    snapshotsHook.mockReturnValue({
      data: [{ id: 's1' }],
    });
    snapshotDetailHook.mockReturnValue({
      data: {
        entities: [
          { externalId: '42', kind: 'vm', name: 'router' },
          { externalId: '43', kind: 'vm', name: 'switch-target' },
          { externalId: '99', kind: 'container', name: 'container-target' },
        ],
      },
    });
    postOverrideMutate.mockImplementation((data: Record<string, unknown>) => {
      if (data.action === 'detach') {
        return {
          id: 'ov-detach-1',
          action: 'detach',
          note: '',
          createdBy: 'admin',
          createdAt: '2026-09-01T00:00:00Z',
          state: 'active',
          members: [{ ...base.members[0], entityId: 'entity-new' }],
        };
      }
      if (data.action === 'merge') {
        return {
          id: 'ov-merge-1',
          action: 'merge',
          note: data.note || '',
          createdBy: 'admin',
          createdAt: '2026-09-01T00:00:00Z',
          state: 'active',
          members: [
            { ...base.members[0], entityId: 'entity-merged' },
            {
              connectorId: data.otherConnectorId,
              kind: data.otherKind,
              ref: data.otherRef,
              name: 'other',
              entityId: 'entity-merged',
            },
          ],
        };
      }
      return {};
    });
  });

  afterEach(async () => {
    await setLanguagePreference('en');
  });

  it('shows the no-longer-observed banner and the visible member', () => {
    mount();
    expect(screen.getByRole('status')).toHaveTextContent('This entity is no longer observed.');
    expect(screen.getByRole('heading', { level: 1, name: 'router' })).toBeInTheDocument();
    expect(entityHook).toHaveBeenCalledWith('entity-1');
  });

  it('shows the loading skeleton', () => {
    entityHook.mockReturnValue({ data: undefined, isLoading: true, isError: false, refetch: vi.fn() });
    mount();
    expect(screen.getByLabelText('Loading')).toBeInTheDocument();
  });

  it('shows not-found without a retry button on 404', () => {
    const refetch = vi.fn();
    entityHook.mockReturnValue({ data: undefined, isLoading: false, isError: true, error: statusError(404), refetch });
    mount();
    expect(screen.getByRole('alert')).toHaveTextContent('Entity not found');
    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
  });

  it('shows the load error with a working retry on other failures', () => {
    const refetch = vi.fn();
    entityHook.mockReturnValue({ data: undefined, isLoading: false, isError: true, error: statusError(500), refetch });
    mount();
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load this entity');
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetch).toHaveBeenCalledOnce();
  });

  it('names connectors and links each member to its connector and doc', () => {
    mount();
    expect(screen.getByRole('link', { name: 'Proxmox lab' })).toHaveAttribute('href', '/services/c1');
    expect(screen.getByRole('link', { name: 'Connector doc' })).toHaveAttribute('href', '/docs/doc-1');
    expect(screen.queryByText('c1')).not.toBeInTheDocument();
  });

  it('shows when a departed member was last observed', () => {
    entityHook.mockReturnValue({
      data: { ...base, members: [{ ...base.members[0], goneAt: '2026-09-01T00:00:00Z' }] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    mount();
    expect(screen.getByText(/No longer observed since/)).toBeInTheDocument();
  });

  it('links neighbours and IP links to other visible entities only', () => {
    const self = { connectorId: 'c1', kind: 'vm', name: 'router', ref: '42' };
    entityHook.mockReturnValue({
      data: {
        ...base,
        neighbors: [
          {
            kind: 'dependency',
            from: self,
            to: { connectorId: 'c1', kind: 'vm', name: 'switch', ref: '7', entityId: 'entity-2' },
          },
        ],
        relatedByIp: [
          { reason: 'IP address', from: self, to: { connectorId: 'c1', kind: 'vm', name: 'orphan', ref: '8' } },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    mount();
    expect(screen.getByRole('link', { name: 'switch' })).toHaveAttribute('href', '/entities/entity-2');
    expect(screen.queryByRole('link', { name: 'orphan' })).not.toBeInTheDocument();
    expect(screen.getByText('orphan', { exact: false })).toBeInTheDocument();
  });

  it.each([
    ['en', 'critical · open'],
    ['pt-BR', 'crítico · aberto'],
  ])('translates finding severity and status (%s)', async (lng, label) => {
    await setLanguagePreference(lng as 'en' | 'pt-BR');
    entityHook.mockReturnValue({
      data: { ...base, findings: [finding('f1', 'critical', 'open')] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    mount();
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it('does not render detach, merge or overrides link for non-admins', () => {
    isInstanceAdminMock.mockReturnValue(false);
    mount();
    expect(screen.queryByRole('button', { name: 'Detach' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Merge' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /manage overrides/i })).not.toBeInTheDocument();
  });

  it('renders overrides link for instance admins', () => {
    mount();
    const link = screen.getByRole('link', { name: /manage overrides/i });
    expect(link).toHaveAttribute('href', '/entities/overrides');
  });

  it('detaches a member and navigates to the new identity when visible', async () => {
    getEntitiesIdMock.mockResolvedValue({ id: 'entity-new', name: 'new', members: [] });
    mount();

    const detachBtn = screen.getByRole('button', { name: 'Detach' });
    fireEvent.click(detachBtn);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText(/Are you sure you want to detach router/)).toBeInTheDocument();

    const confirmBtn = within(dialog).getByRole('button', { name: 'Detach' });
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(postOverrideMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          action: 'detach',
          connectorId: 'c1',
          kind: 'vm',
          ref: '42',
        })
      );
      expect(getEntitiesIdMock).toHaveBeenCalledWith('entity-new');
      expect(entityHook).toHaveBeenCalledWith('entity-new');
    });
    expect(toastMock.success).toHaveBeenCalledWith('Member detached.');
  });

  it('detaches a member and stays on the page with confirmation when not visible (404)', async () => {
    getEntitiesIdMock.mockRejectedValue(statusError(404));
    mount();

    const detachBtn = screen.getByRole('button', { name: 'Detach' });
    fireEvent.click(detachBtn);

    const dialog = await screen.findByRole('dialog');
    const confirmBtn = within(dialog).getByRole('button', { name: 'Detach' });
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(postOverrideMutate).toHaveBeenCalled();
      expect(getEntitiesIdMock).toHaveBeenCalledWith('entity-new');
      expect(toastMock.success).toHaveBeenCalledWith(
        'Member detached. The new identity is not visible with your current permissions.'
      );
    });
    expect(entityHook).not.toHaveBeenCalledWith('entity-new');
  });

  it('merges a member and navigates to the merged identity when visible', async () => {
    getEntitiesIdMock.mockResolvedValue({ id: 'entity-merged', name: 'merged', members: [] });
    mount();

    const mergeBtn = screen.getByRole('button', { name: 'Merge' });
    fireEvent.click(mergeBtn);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toBeInTheDocument();
    expect(
      within(dialog).getByText(/Choose another member of the same kind to merge with router/)
    ).toBeInTheDocument();

    const pickerSelect = within(dialog).getByLabelText('Target entity');
    fireEvent.change(pickerSelect, { target: { value: '43' } });

    const submitBtn = within(dialog).getByRole('button', { name: 'Merge' });
    expect(submitBtn).toBeEnabled();
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(postOverrideMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          action: 'merge',
          connectorId: 'c1',
          kind: 'vm',
          ref: '42',
          otherRef: '43',
          otherKind: 'vm',
        })
      );
      expect(getEntitiesIdMock).toHaveBeenCalledWith('entity-merged');
      expect(entityHook).toHaveBeenCalledWith('entity-merged');
    });
    expect(toastMock.success).toHaveBeenCalledWith('Members merged.');
  });

  it('merges a member and stays on page with confirmation when not visible (404)', async () => {
    getEntitiesIdMock.mockRejectedValue(statusError(404));
    mount();

    const mergeBtn = screen.getByRole('button', { name: 'Merge' });
    fireEvent.click(mergeBtn);

    const dialog = await screen.findByRole('dialog');
    const pickerSelect = within(dialog).getByLabelText('Target entity');
    fireEvent.change(pickerSelect, { target: { value: '43' } });

    const submitBtn = within(dialog).getByRole('button', { name: 'Merge' });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(postOverrideMutate).toHaveBeenCalled();
      expect(getEntitiesIdMock).toHaveBeenCalledWith('entity-merged');
      expect(toastMock.success).toHaveBeenCalledWith(
        'Members merged. The identity is not visible with your current permissions.'
      );
    });
    expect(entityHook).not.toHaveBeenCalledWith('entity-merged');
  });

  it('prevents merging a member with itself in the merge dialog', async () => {
    mount();
    const mergeBtn = screen.getByRole('button', { name: 'Merge' });
    fireEvent.click(mergeBtn);

    const dialog = await screen.findByRole('dialog');
    const pickerSelect = within(dialog).getByLabelText('Target entity');
    fireEvent.change(pickerSelect, { target: { value: '42' } });

    expect(within(dialog).getByRole('alert')).toHaveTextContent('Cannot merge a member with itself.');
    expect(within(dialog).getByRole('button', { name: 'Merge' })).toBeDisabled();
  });
});
