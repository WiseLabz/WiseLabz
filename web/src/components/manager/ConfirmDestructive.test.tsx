import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { connectors } from '../../data/fixtures';
import { ConfirmDestructive } from './ConfirmDestructive';

const { removeConnector } = vi.hoisted(() => ({ removeConnector: vi.fn() }));

vi.mock('../../api/generated/connectors/connectors', () => ({
  deleteConnectorsConnectorId: removeConnector,
  getGetConnectorsQueryKey: () => ['/connectors'],
  useGetConnectorsConnectorIdRemovalImpact: () => ({
    data: { trackedServices: 1, docSections: 2, snapshots: 3 },
    isLoading: false,
  }),
}));

vi.mock('../../api/generated/settings/settings', () => ({
  useGetAuthConfig: () => ({ data: { stepUpForDestructive: false } }),
}));

function renderDialog(onClose = vi.fn(), onConfirmed = vi.fn()) {
  const connector = connectors[0];
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <ConfirmDestructive
        open
        connectorId={connector.id}
        connectorName={connector.name}
        onClose={onClose}
        onConfirmed={onConfirmed}
      />
    </QueryClientProvider>,
  );
  return { connector, onClose, onConfirmed };
}

describe('ConfirmDestructive', () => {
  afterEach(cleanup);

  beforeEach(() => {
    removeConnector.mockReset().mockResolvedValue(undefined);
  });

  it('does not mutate until the connector name is confirmed, and cancel only closes the dialog', () => {
    const { onClose, onConfirmed } = renderDialog();

    expect(screen.getByRole('button', { name: 'Remove connector' })).toBeDisabled();
    fireEvent.click(screen.getByText('Cancel', { selector: 'button' }));

    expect(onClose).toHaveBeenCalledOnce();
    expect(onConfirmed).not.toHaveBeenCalled();
    expect(removeConnector).not.toHaveBeenCalled();
  });

  it('removes the connector only after its exact name is entered', async () => {
    const { connector, onConfirmed } = renderDialog();

    fireEvent.change(screen.getByLabelText(`Type ${connector.name} to confirm`), {
      target: { value: connector.name },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Remove connector' }));

    await waitFor(() => expect(removeConnector).toHaveBeenCalledWith(connector.id, undefined));
    await waitFor(() => expect(onConfirmed).toHaveBeenCalledOnce());
  });
});
