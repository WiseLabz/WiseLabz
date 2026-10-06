import { fireEvent, render, screen, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguagePreference } from '../../i18n';
import { EntityOverridesPage } from './EntityOverridesPage';

const { overridesHook, deleteHook, toastMock, getEntitiesIdMock, usersHook } = vi.hoisted(() => ({
  overridesHook: vi.fn(),
  deleteHook: vi.fn(),
  toastMock: { success: vi.fn(), error: vi.fn() },
  getEntitiesIdMock: vi.fn(),
  usersHook: vi.fn(),
}));

function statusError(status: number) {
  const err = new Error(`Request failed with status ${status}`);
  // @ts-expect-error test helper
  err.isAxiosError = true;
  // @ts-expect-error test helper
  err.response = { status };
  return err;
}

vi.mock('../../api/generated/search/search', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/search/search')>()),
  useGetEntityOverrides: () => overridesHook(),
  getEntitiesId: (id: string) => getEntitiesIdMock(id),
  getGetEntitiesIdQueryOptions: (id: string) => ({
    queryKey: [`/entities/${id}`],
    queryFn: () => getEntitiesIdMock(id),
  }),
  useDeleteEntityOverridesId: (options?: {
    mutation?: { onSuccess?: (data: unknown) => void; onError?: (err: unknown) => void };
  }) => ({
    mutate: (vars: { id: string }) => {
      try {
        const res = deleteHook(vars.id);
        options?.mutation?.onSuccess?.(res);
      } catch (err) {
        options?.mutation?.onError?.(err);
      }
    },
    isPending: false,
  }),
  getGetEntityOverridesQueryKey: () => ['/entity-overrides'],
}));

vi.mock('../../api/generated/users/users', () => ({
  useGetUsers: () => usersHook(),
}));

vi.mock('../../lib/toast', () => ({
  toast: toastMock,
}));

function mount(queryClient = new QueryClient()) {
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/entities/overrides']}>
        <Routes>
          <Route path="/entities/overrides" element={<EntityOverridesPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const mockOverrides = [
  {
    id: 'ov-1',
    action: 'detach' as const,
    note: 'Separated for maintenance',
    createdBy: 'admin-user-id',
    createdAt: '2026-09-01T00:00:00Z',
    state: 'active' as const,
    members: [
      {
        connectorId: 'c1',
        connectorName: 'Proxmox lab',
        kind: 'vm',
        ref: '42',
        name: 'router',
        entityId: 'entity-1',
      },
    ],
  },
  {
    id: 'ov-2',
    action: 'merge' as const,
    note: 'Manual switch clustering',
    createdBy: 'admin-user-id',
    createdAt: '2026-09-02T00:00:00Z',
    state: 'dormant' as const,
    members: [
      {
        connectorId: 'c1',
        connectorName: 'Proxmox lab',
        kind: 'vm',
        ref: '43',
        name: 'switch-a',
        entityId: 'entity-2',
      },
      {
        connectorId: 'c2',
        connectorName: 'OPNsense',
        kind: 'vm',
        ref: '44',
        name: 'switch-b',
        entityId: 'entity-2',
      },
    ],
  },
];

describe('EntityOverridesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void setLanguagePreference('en');
    usersHook.mockReturnValue({
      data: [{ id: 'admin-user-id', username: 'admin', displayName: 'Admin User' }],
    });
    overridesHook.mockReturnValue({
      data: mockOverrides,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    deleteHook.mockImplementation((id: string) => {
      const found = mockOverrides.find((o) => o.id === id);
      return found ?? { id, action: 'detach', members: [] };
    });
    getEntitiesIdMock.mockResolvedValue({ id: 'entity-1', name: 'router', members: [] });
  });

  afterEach(async () => {
    await setLanguagePreference('en');
  });

  it('lists overrides with action, note, resolved creator name, and state', () => {
    mount();
    expect(screen.getByRole('heading', { level: 1, name: 'Identity overrides' })).toBeInTheDocument();
    expect(screen.getByText('Separated for maintenance')).toBeInTheDocument();
    expect(screen.getByText('Manual switch clustering')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Dormant')).toBeInTheDocument();
    expect(screen.getAllByText('Created by Admin User')).toHaveLength(2);
    expect(screen.getByRole('link', { name: 'router' })).toHaveAttribute('href', '/entities/entity-1');
  });

  it('provides accessible aria-labels on remove buttons incorporating member names', () => {
    mount();
    expect(screen.getByRole('button', { name: 'Remove override for router' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove override for switch-a, switch-b' })).toBeInTheDocument();
  });

  it('removes an override, probes visibility when visible, closes dialog, and invalidates caches', async () => {
    const qc = new QueryClient();
    qc.setQueryData(['/entities/entity-1'], { id: 'entity-1' });
    qc.setQueryData(['/entity-overrides'], []);
    mount(qc);

    const removeBtn = screen.getByRole('button', { name: 'Remove override for router' });
    fireEvent.click(removeBtn);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText('Remove override')).toBeInTheDocument();

    const panel = dialog.querySelector('[class*="transition-"]');
    await vi.waitFor(() => {
      expect(panel).toHaveClass('opacity-100');
    });

    const confirmButton = within(dialog).getByRole('button', { name: 'Remove' });
    fireEvent.click(confirmButton);

    expect(deleteHook).toHaveBeenCalledWith('ov-1');
    await vi.waitFor(() => {
      expect(getEntitiesIdMock).toHaveBeenCalledWith('entity-1');
      expect(toastMock.success).toHaveBeenCalledWith(
        'Override removed.',
        expect.objectContaining({
          action: expect.objectContaining({
            label: 'View identity',
          }),
        })
      );
      expect(qc.getQueryState(['/entities/entity-1'])?.isInvalidated).toBe(false); // invalidated, then refetched by the visibility probe
      expect(qc.getQueryState(['/entity-overrides'])?.isInvalidated).toBe(true);
      expect(panel).toHaveClass('opacity-0');
    });
  });

  it('stays on page and shows not visible message when returned identity gives 404', async () => {
    getEntitiesIdMock.mockRejectedValue(statusError(404));
    mount();

    const removeBtn = screen.getByRole('button', { name: 'Remove override for router' });
    fireEvent.click(removeBtn);

    const dialog = await screen.findByRole('dialog');
    const panel = dialog.querySelector('[class*="transition-"]');
    await vi.waitFor(() => {
      expect(panel).toHaveClass('opacity-100');
    });

    const confirmButton = within(dialog).getByRole('button', { name: 'Remove' });
    fireEvent.click(confirmButton);

    await vi.waitFor(() => {
      expect(deleteHook).toHaveBeenCalledWith('ov-1');
      expect(getEntitiesIdMock).toHaveBeenCalledWith('entity-1');
      expect(toastMock.success).toHaveBeenCalledWith(
        'Override removed. The resulting identity is not visible with your current permissions.'
      );
      expect(panel).toHaveClass('opacity-0');
    });
  });

  it('shows plain toast without action when removing override whose members have no entityId', async () => {
    deleteHook.mockReturnValue({
      id: 'ov-no-id',
      action: 'detach',
      members: [{ connectorId: 'c1', kind: 'vm', ref: '42', name: 'router' }],
    });
    mount();

    const removeBtn = screen.getByRole('button', { name: 'Remove override for router' });
    fireEvent.click(removeBtn);

    const dialog = await screen.findByRole('dialog');
    const confirmButton = within(dialog).getByRole('button', { name: 'Remove' });
    fireEvent.click(confirmButton);

    await vi.waitFor(() => {
      expect(toastMock.success).toHaveBeenCalledWith('Override removed.');
    });
    expect(getEntitiesIdMock).not.toHaveBeenCalled();
  });

  it('shows plain toast without action when removing merge override whose members split into different entities', async () => {
    deleteHook.mockReturnValue({
      id: 'ov-2',
      action: 'merge',
      members: [
        { connectorId: 'c1', kind: 'vm', ref: '43', name: 'switch-a', entityId: 'entity-a' },
        { connectorId: 'c2', kind: 'vm', ref: '44', name: 'switch-b', entityId: 'entity-b' },
      ],
    });
    mount();

    const removeBtn = screen.getByRole('button', { name: 'Remove override for switch-a, switch-b' });
    fireEvent.click(removeBtn);

    const dialog = await screen.findByRole('dialog');
    const confirmButton = within(dialog).getByRole('button', { name: 'Remove' });
    fireEvent.click(confirmButton);

    await vi.waitFor(() => {
      expect(toastMock.success).toHaveBeenCalledWith('Override removed.');
    });
    expect(getEntitiesIdMock).not.toHaveBeenCalled();
  });

  it('translates view identity toast action in pt-BR', async () => {
    await setLanguagePreference('pt-BR');
    mount();

    const removeBtn = screen.getByRole('button', { name: /remover substituição para router/i });
    fireEvent.click(removeBtn);

    const dialog = await screen.findByRole('dialog');
    const confirmButton = within(dialog).getByRole('button', { name: 'Remover' });
    fireEvent.click(confirmButton);

    await vi.waitFor(() => {
      expect(toastMock.success).toHaveBeenCalledWith(
        'Substituição removida.',
        expect.objectContaining({
          action: expect.objectContaining({
            label: 'Ver identidade',
          }),
        })
      );
    });
  });

  it('handles delete failure with error toast', async () => {
    deleteHook.mockImplementation(() => {
      throw statusError(500);
    });
    mount();

    const removeBtn = screen.getByRole('button', { name: 'Remove override for router' });
    fireEvent.click(removeBtn);

    const dialog = await screen.findByRole('dialog');
    const confirmButton = within(dialog).getByRole('button', { name: 'Remove' });
    fireEvent.click(confirmButton);

    await vi.waitFor(() => {
      expect(toastMock.error).toHaveBeenCalledWith('Could not remove override.');
    });
  });

  it.each([500, 404])('resyncs and closes the dialog when delete fails with %i', async (status) => {
    deleteHook.mockImplementation(() => {
      throw statusError(status);
    });
    const qc = new QueryClient();
    qc.setQueryData(['/entities/entity-1'], { id: 'entity-1' });
    qc.setQueryData(['/entity-overrides'], []);
    mount(qc);

    fireEvent.click(screen.getByRole('button', { name: 'Remove override for router' }));
    const dialog = await screen.findByRole('dialog');
    const panel = dialog.querySelector('[class*="transition-"]');
    await vi.waitFor(() => {
      expect(panel).toHaveClass('opacity-100');
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Remove' }));

    await vi.waitFor(() => {
      expect(toastMock.error).toHaveBeenCalledWith('Could not remove override.');
      expect(qc.getQueryState(['/entity-overrides'])?.isInvalidated).toBe(true);
      expect(qc.getQueryState(['/entities/entity-1'])?.isInvalidated).toBe(true);
      expect(panel).toHaveClass('opacity-0');
    });
  });

  it('keeps the dialog open when delete gets no response', async () => {
    deleteHook.mockImplementation(() => {
      throw new Error('Network Error');
    });
    mount();

    fireEvent.click(screen.getByRole('button', { name: 'Remove override for router' }));
    const dialog = await screen.findByRole('dialog');
    const panel = dialog.querySelector('[class*="transition-"]');
    await vi.waitFor(() => {
      expect(panel).toHaveClass('opacity-100');
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Remove' }));

    await vi.waitFor(() => {
      expect(toastMock.error).toHaveBeenCalledWith('Could not remove override.');
    });
    expect(panel).toHaveClass('opacity-100');
  });

  it('never shows the raw creator id for unknown users or while users load', () => {
    usersHook.mockReturnValue({ data: [], isLoading: false });
    const { unmount } = mount();
    expect(screen.getAllByText('Created by Unknown user')).toHaveLength(2);
    expect(screen.queryByText(/admin-user-id/)).not.toBeInTheDocument();
    unmount();

    usersHook.mockReturnValue({ data: undefined, isLoading: true });
    mount();
    expect(screen.getAllByText('Created by …')).toHaveLength(2);
  });

  it('shows empty state when no overrides exist', () => {
    overridesHook.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    mount();
    expect(screen.getByText('No overrides')).toBeInTheDocument();
  });

  it('shows loading skeleton', () => {
    overridesHook.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: vi.fn(),
    });
    mount();
    expect(screen.getByLabelText('Loading')).toBeInTheDocument();
  });

  it('shows error state with working retry', () => {
    const refetch = vi.fn();
    overridesHook.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch,
    });
    mount();
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load overrides');
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetch).toHaveBeenCalledOnce();
  });
});
