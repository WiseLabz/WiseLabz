import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, it, vi } from 'vitest';
import '../../i18n';
import { NotificationsPage } from './NotificationsPage';

const { put, config } = vi.hoisted(() => ({
  put: vi.fn(),
  config: {
    channels: [{ type: 'slack', enabled: true, config: { url: 'https://hooks.slack.test/x', channelId: 'C123ABC', botTokenSet: true } }],
    routing: [],
  },
}));
vi.mock('../../api/generated/settings/settings', () => ({
  useGetNotificationsConfig: () => ({
    data: config,
    isLoading: false,
    isError: false,
  }),
  putNotificationsConfig: put,
  postNotificationsConfigTest: vi.fn(),
  getGetNotificationsConfigQueryKey: () => ['notifications-config'],
}));
vi.mock('../../api/generated/notifications/notifications', () => ({
  useGetNotificationsDeliveries: () => ({
    data: { items: [], total: 0, page: 1, pageSize: 25 },
    isLoading: false,
    isError: false,
  }),
}));
vi.mock('../../api/generated/connectors/connectors', () => ({ useGetConnectors: () => ({ data: [] }) }));

it('lets admins set and clear a write-only Slack bot token', async () => {
  put.mockResolvedValue({});
  render(
    <QueryClientProvider client={new QueryClient()}>
      <NotificationsPage />
    </QueryClientProvider>
  );

  expect(screen.getByPlaceholderText('Token is set')).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText('Slack bot token'), { target: { value: 'xoxb-new' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save routing' }));
  await waitFor(() => expect(put).toHaveBeenCalledWith(expect.objectContaining({
    channels: [expect.objectContaining({ config: expect.objectContaining({ botToken: 'xoxb-new', channelId: 'C123ABC' }) })],
  })));

  fireEvent.click(screen.getByRole('button', { name: 'Clear bot token' }));
  expect(screen.getByLabelText('Slack channel ID')).toHaveValue('');
  fireEvent.click(screen.getByRole('button', { name: 'Save routing' }));
  await waitFor(() => expect(put).toHaveBeenLastCalledWith(expect.objectContaining({
    channels: [expect.objectContaining({ config: expect.objectContaining({ botToken: '' }) })],
  })));
});
