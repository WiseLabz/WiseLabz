import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorEditForm } from './ConnectorEditForm';

const { putConnectorsConnectorId, roleState } = vi.hoisted(() => ({
  putConnectorsConnectorId: vi.fn(),
  roleState: { isAdmin: true },
}));

const savedConnector = { id: 'c1', name: 'probe', type: 'tlsprobe' };
const connectorData = {
  ...savedConnector,
  owner: '',
  url: '',
  verifyTls: true,
  enabled: true,
  status: 'online',
  config: { targets: 'existing.example:443', import_connector_id: '', import_port: '443' },
};
const schemas = [
  {
    type: 'tlsprobe',
    category: 'monitoring',
    displayName: 'TLS Probe',
    isCredentialRefresher: false,
    fields: [
      { name: 'targets', label: 'Targets', kind: 'textarea', required: false },
      { name: 'import_connector_id', label: 'Import hosts from Traefik connector', kind: 'text', required: false },
      { name: 'import_port', label: 'Port for imported hosts', kind: 'number', required: false, default: '443' },
    ],
  },
];

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsConnectorId: () => ({ data: connectorData, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetConnectors: () => ({ data: [] }),
  useGetConnectorsSchema: () => ({ data: schemas }),
  putConnectorsConnectorId: (...args: unknown[]) => putConnectorsConnectorId(...args),
  postConnectorsConnectorIdTest: vi.fn(),
  usePreviewConnectorRecipe: () => ({ mutate: vi.fn(), isPending: false, isError: false }),
  getGetConnectorsQueryKey: () => [],
}));

vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => roleState.isAdmin }));

beforeEach(() => {
  putConnectorsConnectorId.mockReset().mockResolvedValue(savedConnector);
});

afterEach(() => {
  roleState.isAdmin = true;
});

describe('ConnectorEditForm', () => {
  it('applies overrides once after loading, so later edits survive parent rerenders', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const onSaved = vi.fn();
    const onCancel = vi.fn();
    const view = render(
      <QueryClientProvider client={queryClient}>
        <ConnectorEditForm
          connectorId="c1"
          overrides={{ targets: 'existing.example:443\nqueued.example:443' }}
          onSaved={onSaved}
          onCancel={onCancel}
        />
      </QueryClientProvider>,
    );

    const targets = screen.getByLabelText('Targets');
    await waitFor(() => expect(targets).toHaveValue('existing.example:443\nqueued.example:443'));
    fireEvent.change(targets, { target: { value: 'edited.example:443' } });

    view.rerender(
      <QueryClientProvider client={queryClient}>
        <ConnectorEditForm
          connectorId="c1"
          overrides={{ targets: 'existing.example:443\nqueued.example:443' }}
          onSaved={onSaved}
          onCancel={onCancel}
        />
      </QueryClientProvider>,
    );
    expect(targets).toHaveValue('edited.example:443');

    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await waitFor(() => expect(putConnectorsConnectorId).toHaveBeenCalled());
    expect(putConnectorsConnectorId.mock.calls[0][1].config).toEqual({ targets: 'edited.example:443' });
  });

  it('calls onSaved with the saved connector', async () => {
    const onSaved = vi.fn();
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <ConnectorEditForm connectorId="c1" onSaved={onSaved} onCancel={vi.fn()} />
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(savedConnector));
  });

  it('calls onCancel from the cancel button', () => {
    const onCancel = vi.fn();
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <ConnectorEditForm connectorId="c1" onSaved={vi.fn()} onCancel={onCancel} />
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledOnce();
  });
});
