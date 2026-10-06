import { fireEvent, render, screen, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguagePreference } from '../../i18n';
import { EntityOverridesPage } from './EntityOverridesPage';

const { overridesHook, deleteHook, toastMock } = vi.hoisted(() => ({
  overridesHook: vi.fn(),
  deleteHook: vi.fn(),
  toastMock: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('../../api/generated/search/search', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/search/search')>()),
  useGetEntityOverrides: () => overridesHook(),
  useDeleteEntityOverridesId: (options?: {
    mutation?: { onSuccess?: () => void; onError?: () => void };
  }) => ({
    mutate: (vars: { id: string }) => {
      deleteHook(vars.id);
      options?.mutation?.onSuccess?.();
    },
    isPending: false,
  }),
  getGetEntityOverridesQueryKey: () => ['/entity-overrides'],
}));

vi.mock('sonner', () => ({
  toast: toastMock,
}));

function mount() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
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
    createdBy: 'admin-user',
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
    createdBy: 'admin-user',
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
    overridesHook.mockReturnValue({
      data: mockOverrides,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
  });

  afterEach(async () => {
    await setLanguagePreference('en');
  });

  it('lists overrides with action, note, creator, and state', () => {
    mount();
    expect(screen.getByRole('heading', { level: 1, name: 'Identity overrides' })).toBeInTheDocument();
    expect(screen.getByText('Separated for maintenance')).toBeInTheDocument();
    expect(screen.getByText('Manual switch clustering')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Dormant')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'router' })).toHaveAttribute('href', '/entities/entity-1');
  });

  it('removes an override after confirmation', async () => {
    mount();
    const removeButtons = screen.getAllByRole('button', { name: 'Remove' });
    fireEvent.click(removeButtons[0]);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText('Remove override')).toBeInTheDocument();

    const confirmButton = within(dialog).getByRole('button', { name: 'Remove' });
    fireEvent.click(confirmButton);

    expect(deleteHook).toHaveBeenCalledWith('ov-1');
    expect(toastMock.success).toHaveBeenCalledWith('Override removed.');
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
