import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorPermissionsTab } from './ConnectorPermissionsTab';

let isInstanceAdmin = true;
vi.mock('../../hooks/useRole', () => ({
  useIsInstanceAdmin: () => isInstanceAdmin,
}));

const { upsertMock, deleteMock } = vi.hoisted(() => ({
  upsertMock: vi.fn(),
  deleteMock: vi.fn(),
}));

const grants = [
  { id: 'g1', userId: 'u1', connectorId: 'c1', role: 'operator' as const, source: 'manual' as const, createdAt: '', updatedAt: '' },
];

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsConnectorIdPermissions: () => ({ data: grants, isLoading: false, isError: false, refetch: vi.fn() }),
  putConnectorsConnectorIdPermissionsUserId: upsertMock,
  deleteConnectorsConnectorIdPermissionsUserId: deleteMock,
  getGetConnectorsConnectorIdPermissionsQueryKey: (connectorId: string) => [`/connectors/${connectorId}/permissions`],
}));

vi.mock('../../api/generated/users/users', () => ({
  useGetUsers: () => ({
    data: [
      { id: 'u1', username: 'alice', displayName: 'Alice' },
      { id: 'u2', username: 'bob', displayName: 'Bob' },
    ],
  }),
}));

function renderTab() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ConnectorPermissionsTab connectorId="c1" />
    </QueryClientProvider>
  );
}

describe('ConnectorPermissionsTab (#240 PR1)', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders nothing for a non-instance-admin', () => {
    isInstanceAdmin = false;
    const { container } = renderTab();
    expect(container).toBeEmptyDOMElement();
    isInstanceAdmin = true;
  });

  it('lists existing grants and offers only ungranted users to add', () => {
    renderTab();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Bob' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Alice' })).not.toBeInTheDocument();
  });

  it('adding a user calls upsert with the default viewer role', async () => {
    upsertMock.mockResolvedValue({});
    renderTab();
    fireEvent.change(screen.getByDisplayValue(/add a user/i), { target: { value: 'u2' } });
    fireEvent.click(screen.getByRole('button', { name: /add/i }));
    await waitFor(() => expect(upsertMock).toHaveBeenCalledWith('c1', 'u2', { role: 'viewer' }));
  });

  it('changing a grant role calls upsert with the new role', async () => {
    upsertMock.mockResolvedValue({});
    renderTab();
    fireEvent.change(screen.getByDisplayValue('Operator'), { target: { value: 'viewer' } });
    await waitFor(() => expect(upsertMock).toHaveBeenCalledWith('c1', 'u1', { role: 'viewer' }));
  });

  it('revoking a grant calls delete', async () => {
    deleteMock.mockResolvedValue(undefined);
    renderTab();
    fireEvent.click(screen.getByRole('button', { name: /revoke/i }));
    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith('c1', 'u1'));
  });
});
