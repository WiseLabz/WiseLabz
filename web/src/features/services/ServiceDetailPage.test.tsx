import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ServiceDetailPage } from './ServiceDetailPage';

const { restart } = vi.hoisted(() => ({ restart: vi.fn() }));

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsConnectorId: () => ({
    data: {
      id: 'svc-pve1',
      name: 'pve1',
      type: 'proxmox',
      url: 'https://pve1.example',
      enabled: true,
      status: 'online',
      scheduleSeconds: null,
      nextRunAt: null,
      lastSyncAt: null,
    },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useGetConnectorsConnectorIdData: () => ({
    data: { sections: [], fetchedAt: new Date().toISOString() },
  }),
  useGetConnectorsConnectorIdSyncs: () => ({ data: [] }),
  useGetConnectorsSchema: () => ({ data: [] }),
  postConnectorsConnectorIdRestart: restart,
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
vi.mock('../../hooks/useRole', () => ({ useCanMutate: () => true }));
vi.mock('../../store/live', () => ({
  useLive: (selector: (state: object) => unknown) =>
    selector({ statusOverrides: {}, activity: [] }),
}));
vi.mock('../../lib/runSync', () => ({ runSync: vi.fn() }));
vi.mock('../../components/manager/ConfirmDestructive', () => ({ ConfirmDestructive: () => null }));
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

describe('ServiceDetailPage restart preview', () => {
  it('shows the read-only restart impact returned by the connector API', async () => {
    restart.mockResolvedValue({
      targetService: 'pve1',
      estimatedDowntimeSeconds: 30,
      dependentServices: [{ kind: 'vm', name: 'home-assistant' }],
    });

    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Preview restart impact' }));

    await waitFor(() => expect(restart).toHaveBeenCalledWith('svc-pve1', { dryRun: true }));
    expect(await screen.findByRole('dialog', { name: 'Restart impact' })).toHaveTextContent(
      'Read-only preview. No service will be restarted.'
    );
    expect(screen.getByText('30 seconds')).toBeInTheDocument();
    expect(screen.getByText('home-assistant')).toBeInTheDocument();
  });
});
