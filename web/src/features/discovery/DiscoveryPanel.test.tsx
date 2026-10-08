import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import type { DiscoveryCandidate, DiscoveryScan } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { useLive } from '../../store/live';
import { DiscoveryPanel } from './DiscoveryPanel';

const api = vi.hoisted(() => ({
  suggestions: { current: [] as { cidr: string; source: string }[] },
  read: { data: undefined as unknown, isError: false, isFetching: false },
  startDiscoveryScan: vi.fn(),
  cancelDiscoveryScan: vi.fn(),
  refetch: vi.fn(),
}));

vi.mock('../../api/generated/discovery/discovery', () => ({
  useGetDiscoverySuggestions: () => ({ data: { suggestions: api.suggestions.current } }),
  useGetDiscoveryScan: () => ({
    data: api.read.data,
    isError: api.read.isError,
    isFetching: api.read.isFetching,
    refetch: api.refetch,
  }),
  startDiscoveryScan: (...args: unknown[]) => api.startDiscoveryScan(...args),
  cancelDiscoveryScan: (...args: unknown[]) => api.cancelDiscoveryScan(...args),
}));

// The real dialog re-authenticates over the network; here it just hands back a token.
vi.mock('../../components/manager/StepUp', () => ({
  StepUp: ({ action, onElevated }: { action: string; onElevated: (token: string) => void }) => (
    <button onClick={() => onElevated('tok-1')}>{`elevate ${action}`}</button>
  ),
}));

const pve: DiscoveryCandidate = {
  type: 'proxmox',
  name: 'Proxmox VE',
  address: '10.0.0.5',
  port: 8006,
  url: 'https://10.0.0.5:8006/api2/json',
  urlField: 'url',
};
const ha: DiscoveryCandidate = {
  type: 'home_assistant',
  name: 'Home Assistant',
  address: '10.0.0.7',
  port: 8123,
  url: 'http://10.0.0.7:8123',
  urlField: 'url',
};
const docker: DiscoveryCandidate = {
  type: 'docker',
  name: 'Docker',
  address: '10.0.0.3',
  port: 2375,
  url: 'tcp://10.0.0.3:2375',
  urlField: 'host',
  connectorId: 'c-docker',
};

const scan = (over: Partial<DiscoveryScan> = {}): DiscoveryScan => ({
  id: 'scan-1',
  cidr: '10.0.0.0/24',
  state: 'completed',
  startedAt: '2026-10-07T12:00:00Z',
  endedAt: '2026-10-07T12:00:20Z',
  done: 254,
  total: 254,
  answered: 3,
  partial: false,
  candidates: [],
  ...over,
});

const axiosError = (status: number, data: unknown, headers: Record<string, string> = {}) =>
  Object.assign(new Error(`status ${status}`), { isAxiosError: true, response: { status, data, headers } });

function renderPanel(onConnect = vi.fn()) {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <DiscoveryPanel onConnect={onConnect} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return onConnect;
}

beforeEach(() => {
  useDiscovery.setState({ scan: null });
  api.suggestions.current = [];
  api.read.data = undefined;
  api.read.isError = false;
  api.read.isFetching = false;
});
afterEach(() => {
  vi.clearAllMocks();
});

describe('DiscoveryPanel', () => {
  it('prefills the range with the admin’s own network and lets a chip replace it', () => {
    api.suggestions.current = [
      { cidr: '192.168.1.0/24', source: 'client' },
      { cidr: '172.18.0.0/24', source: 'server' },
    ];
    renderPanel();
    const input = screen.getByLabelText(/range to scan/i);
    expect(input).toHaveValue('192.168.1.0/24');
    expect(screen.getByRole('button', { name: /172\.18\.0\.0\/24/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /172\.18\.0\.0\/24/ }));
    expect(input).toHaveValue('172.18.0.0/24');
  });

  it('leaves the field empty when no private range is suggested, and never overwrites typing', () => {
    renderPanel();
    expect(screen.getByLabelText(/range to scan/i)).toHaveValue('');
    expect(screen.getByRole('button', { name: /^scan$/i })).toBeDisabled();
  });

  it('starts a scan and shows it running with its progress', async () => {
    api.suggestions.current = [{ cidr: '192.168.1.0/24', source: 'client' }];
    api.startDiscoveryScan.mockResolvedValue({
      scan: scan({ state: 'running', cidr: '192.168.1.0/24', done: 0, answered: 0, endedAt: undefined }),
    });
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /^scan$/i }));
    await waitFor(() => expect(api.startDiscoveryScan).toHaveBeenCalled());
    expect(api.startDiscoveryScan.mock.calls[0][0]).toEqual({ cidr: '192.168.1.0/24' });
    expect(api.startDiscoveryScan.mock.calls[0][1]).toBeUndefined();
    expect(await screen.findByText(/scanning 192\.168\.1\.0\/24/i)).toBeInTheDocument();

    act(() => useDiscovery.getState().applyProgress({ scanId: 'scan-1', done: 127, total: 254, answered: 1 }));
    expect(screen.getByText(/127 of 254 addresses checked/)).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50');
    // The range cannot be edited, nor a second scan started, while one runs.
    expect(screen.getByLabelText(/range to scan/i)).toBeDisabled();
  });

  it('retries with an elevation token when the server asks for step-up', async () => {
    api.suggestions.current = [{ cidr: '192.168.1.0/24', source: 'client' }];
    api.startDiscoveryScan
      .mockRejectedValueOnce(axiosError(400, { code: 'elevation_required' }))
      .mockResolvedValueOnce({ scan: scan({ state: 'running', endedAt: undefined }) });
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /^scan$/i }));
    fireEvent.click(await screen.findByRole('button', { name: 'elevate discovery.scan' }));
    await waitFor(() => expect(api.startDiscoveryScan).toHaveBeenCalledTimes(2));
    expect(api.startDiscoveryScan.mock.calls[1][1]).toEqual({ headers: { 'X-Elevation-Token': 'tok-1' } });
    expect(await screen.findByRole('progressbar')).toBeInTheDocument();
  });

  it('cancels a running scan and shows it cancelled with what was found', async () => {
    useDiscovery.getState().setScan(scan({ state: 'running', done: 40, endedAt: undefined, candidates: [pve] }));
    api.cancelDiscoveryScan.mockResolvedValue({ scan: scan({ state: 'cancelled', done: 40, candidates: [pve] }) });
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /cancel scan/i }));
    await waitFor(() => expect(api.cancelDiscoveryScan).toHaveBeenCalled());
    expect(await screen.findByText('Cancelled')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
    expect(screen.queryByRole('progressbar')).toBeNull();
  });

  it('lists candidates with selection, marks connected ones and offers only the rest', () => {
    useDiscovery.getState().setScan(scan({ candidates: [ha, docker, pve] }));
    const onConnect = renderPanel();

    // Ordered by address, not by arrival.
    const rows = screen.getAllByRole('listitem');
    expect(rows.map((r) => within(r).getByText(/^10\.0\.0\.\d:/).textContent)).toEqual([
      '10.0.0.3:2375',
      '10.0.0.5:8006',
      '10.0.0.7:8123',
    ]);

    const connectedRow = rows[0];
    expect(within(connectedRow).getByText('Already connected')).toBeInTheDocument();
    expect(within(connectedRow).getByRole('checkbox')).toBeDisabled();
    expect(within(connectedRow).getByRole('link', { name: /view connector/i })).toHaveAttribute('href', '/services/c-docker');

    const connect = screen.getByRole('button', { name: /connect 0 selected/i });
    expect(connect).toBeDisabled();
    fireEvent.click(within(rows[1]).getByRole('checkbox'));
    fireEvent.click(within(rows[2]).getByRole('checkbox'));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));
    expect(onConnect).toHaveBeenCalledWith([pve, ha]);
  });

  it('shows the nothing-found state with the answered count', () => {
    useDiscovery.getState().setScan(scan({ answered: 7 }));
    renderPanel();
    expect(screen.getByText('No known products found')).toBeInTheDocument();
    expect(screen.getByText(/7 of 254 addresses answered/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /connect/i })).toBeNull();
  });

  it('flags a partial result', () => {
    useDiscovery.getState().setScan(scan({ partial: true, candidates: [pve] }));
    renderPanel();
    expect(screen.getByText(/60 second limit/)).toBeInTheDocument();
  });

  it('treats a 404 from the read endpoint as no scan', () => {
    useDiscovery.getState().setScan(scan({ candidates: [pve] }));
    api.read.isError = true;
    renderPanel();
    expect(useDiscovery.getState().scan).toBeNull();
    expect(screen.queryByText('Proxmox VE')).toBeNull();
  });

  it('keeps a scan it just started while a stale 404 is being refetched', () => {
    useDiscovery.getState().setScan(scan({ state: 'running', endedAt: undefined }));
    api.read.isError = true;
    api.read.isFetching = true;
    renderPanel();
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('reads the scan again when the socket reconnects', () => {
    useLive.setState({ ws: 'closed' });
    renderPanel();
    expect(api.refetch).not.toHaveBeenCalled();
    act(() => useLive.getState().setWs('open'));
    expect(api.refetch).toHaveBeenCalledTimes(1);
    // Staying open does not read again.
    act(() => useLive.getState().setWs('open'));
    expect(api.refetch).toHaveBeenCalledTimes(1);
  });

  it('reads the held scan on mount (reload while running)', () => {
    api.read.data = { scan: scan({ state: 'running', done: 90, endedAt: undefined, candidates: [pve] }) };
    renderPanel();
    expect(screen.getByText(/90 of 254 addresses checked/)).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
  });

  it('shows the field error for a rejected range', async () => {
    api.suggestions.current = [{ cidr: '8.8.8.0/24', source: 'client' }];
    api.startDiscoveryScan.mockRejectedValue(
      axiosError(400, { code: 'invalid_request', message: 'x', details: [{ field: 'cidr', msg: 'only private ranges are accepted' }] }),
    );
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /^scan$/i }));
    expect(await screen.findByText('only private ranges are accepted')).toBeInTheDocument();
    expect(screen.getByLabelText(/range to scan/i)).toHaveAttribute('aria-invalid', 'true');
  });

  it('explains a conflict by showing the running scan', async () => {
    api.suggestions.current = [{ cidr: '192.168.1.0/24', source: 'client' }];
    api.startDiscoveryScan.mockRejectedValue(axiosError(409, { code: 'scan_in_progress', details: { scan: scan({ state: 'running' }) } }));
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /^scan$/i }));
    expect(await screen.findByText(/a scan is already running/i)).toBeInTheDocument();
    await waitFor(() => expect(api.refetch).toHaveBeenCalled());
  });

  it('tells the admin how long to wait when the hourly limit is reached', async () => {
    api.suggestions.current = [{ cidr: '192.168.1.0/24', source: 'client' }];
    api.startDiscoveryScan.mockRejectedValue(
      axiosError(429, { code: 'rate_limited', details: { retryAfterSeconds: 1500 } }, { 'retry-after': '1500' }),
    );
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /^scan$/i }));
    expect(await screen.findByText(/try again in 25 min/i)).toBeInTheDocument();
  });

  it('shows a generic message for an unexpected failure', async () => {
    api.suggestions.current = [{ cidr: '192.168.1.0/24', source: 'client' }];
    api.startDiscoveryScan.mockRejectedValue(axiosError(500, {}));
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /^scan$/i }));
    expect(await screen.findByText(/could not start the scan/i)).toBeInTheDocument();
  });
});
