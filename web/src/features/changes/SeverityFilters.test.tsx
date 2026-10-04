import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, expect, it, vi } from 'vitest';
import '../../i18n';
import { ChangesPage } from '../changes/ChangesPage';
import { AlertsPage } from '../alerts/AlertsPage';

const { changes, alerts } = vi.hoisted(() => ({ changes: vi.fn(), alerts: vi.fn() }));
vi.mock('../../hooks/useRole', () => ({ useOperatorConnectorIds: () => new Set() }));
vi.mock('../../components/views/SavedViewsMenu', () => ({ SavedViewsMenu: () => null }));
vi.mock('../../api/generated/changes/changes', () => ({
  useGetChanges: changes,
  getGetChangesQueryKey: () => ['changes'],
  postChangesBulkResolve: vi.fn(),
}));
vi.mock('../../api/generated/alerts/alerts', () => ({
  useGetAlerts: alerts,
  getGetAlertsQueryKey: () => ['alerts'],
  postAlertsAlertIdResolve: vi.fn(),
  postAlertsAlertIdDismiss: vi.fn(),
  postAlertsAlertIdSnooze: vi.fn(),
  postAlertsBulkSnooze: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
  const page = {
    data: { items: [], total: 100, page: 1, pageSize: 20 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  };
  changes.mockReturnValue(page);
  alerts.mockReturnValue(page);
});
function mount(page: React.ReactNode, initial = '/') {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[initial]}>{page}</MemoryRouter>
    </QueryClientProvider>
  );
}
it('sends Changes severity to the server and resets pagination on filter changes', async () => {
  mount(<ChangesPage />, '/changes?page=3&severity=warning');
  expect(changes).toHaveBeenLastCalledWith({ page: 3, pageSize: 20, severity: 'warning' });
  fireEvent.click(screen.getByRole('tab', { name: 'Critical' }));
  await waitFor(() =>
    expect(changes).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, severity: 'critical' })
  );
  fireEvent.click(screen.getByRole('tab', { name: 'All' }));
  expect(changes).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, severity: undefined });
});
it('sends Alerts severity and pending status before server pagination', async () => {
  mount(<AlertsPage />);
  expect(alerts).toHaveBeenLastCalledWith({
    page: 1,
    pageSize: 20,
    status: 'pending',
    severity: undefined,
  });
  fireEvent.click(screen.getByRole('button', { name: 'Critical' }));
  await waitFor(() =>
    expect(alerts).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      status: 'pending',
      severity: 'critical',
    })
  );
  fireEvent.click(screen.getByRole('button', { name: 'All' }));
  expect(alerts).toHaveBeenLastCalledWith({
    page: 1,
    pageSize: 20,
    status: 'pending',
    severity: undefined,
  });
});
