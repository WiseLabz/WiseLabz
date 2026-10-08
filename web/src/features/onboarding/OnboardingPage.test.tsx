import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import type { DiscoveryCandidate, DiscoveryScan } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { useLive } from '../../store/live';
import { OnboardingPage } from './OnboardingPage';

const mocks = vi.hoisted(() => ({
  isAdmin: { current: true },
  postConnectors: vi.fn(),
  sync: vi.fn(),
}));

const schemas = [
  {
    type: 'proxmox',
    category: 'virtualization',
    displayName: 'Proxmox VE',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'API URL', kind: 'text', required: true },
      { name: 'token_secret', label: 'API Token Secret', kind: 'password', required: true },
    ],
  },
  {
    type: 'home_assistant',
    category: 'virtualization',
    displayName: 'Home Assistant',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'Home Assistant URL', kind: 'text', required: true },
      { name: 'access_token', label: 'Long-Lived Access Token', kind: 'password', required: true },
    ],
  },
];

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsSchema: () => ({ data: schemas, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetConnectors: () => ({ data: [] }),
  postConnectors: (...args: unknown[]) => mocks.postConnectors(...args),
  postConnectorsConnectorIdSync: (...args: unknown[]) => mocks.sync(...args),
  getGetConnectorsQueryKey: () => [],
}));
vi.mock('../../api/generated/discovery/discovery', () => ({
  useGetDiscoverySuggestions: () => ({ data: { suggestions: [] } }),
  useGetDiscoveryScan: () => ({ data: undefined, isError: false, isFetching: false, refetch: vi.fn() }),
  startDiscoveryScan: vi.fn(),
  cancelDiscoveryScan: vi.fn(),
}));
vi.mock('../compliance/CertificateExpiryPackOffer', () => ({ CertificateExpiryPackOffer: () => null }));
vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => mocks.isAdmin.current }));

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

const scanOf = (candidates: DiscoveryCandidate[]): DiscoveryScan => ({
  id: 'scan-1',
  cidr: '10.0.0.0/24',
  state: 'completed',
  startedAt: '2026-10-07T12:00:00Z',
  endedAt: '2026-10-07T12:00:20Z',
  done: 254,
  total: 254,
  answered: candidates.length,
  partial: false,
  candidates,
});

function renderOnboarding() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <OnboardingPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getAllByRole('button', { name: /connect a service/i })[0]);
}

const type = (label: RegExp, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } });
const save = () => fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
const finishSync = (id: string) =>
  act(() => useLive.getState().upsertJob({ jobId: `job-${id}`, serviceId: id, phase: 'done', percent: 100, startedAt: Date.now() }));

beforeEach(() => {
  mocks.isAdmin.current = true;
  useDiscovery.setState({ scan: null });
  useLive.setState({ jobs: {} });
  let n = 0;
  mocks.postConnectors.mockImplementation(async (body: { name: string; type: string }) => ({ id: `c${++n}`, name: body.name, type: body.type }));
  mocks.sync.mockResolvedValue(undefined);
});
afterEach(() => vi.clearAllMocks());

describe('OnboardingPage connect step', () => {
  it('offers both scanning the network and adding a connector by hand to an instance admin', () => {
    renderOnboarding();
    expect(screen.getByRole('heading', { name: /scan your network/i })).toBeInTheDocument();
    expect(screen.getByText(/add one by hand/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Proxmox VE' })).toBeInTheDocument();
  });

  it('shows only the manual form to someone who is not an instance admin', () => {
    mocks.isAdmin.current = false;
    renderOnboarding();
    expect(screen.queryByRole('heading', { name: /scan your network/i })).toBeNull();
    expect(screen.queryByText(/add one by hand/i)).toBeNull();
    expect(screen.getByRole('button', { name: 'Proxmox VE' })).toBeInTheDocument();
  });

  it('still reaches the sync step from the manual form, with one connector', async () => {
    renderOnboarding();
    fireEvent.click(screen.getByRole('button', { name: 'Proxmox VE' }));
    type(/display name/i, 'pve1');
    type(/api url/i, 'https://pve:8006');
    type(/api token secret/i, 's3cret');
    save();

    expect(await screen.findByRole('heading', { name: /running the first sync/i })).toBeInTheDocument();
    await waitFor(() => expect(mocks.sync).toHaveBeenCalledTimes(1));
    expect(mocks.sync).toHaveBeenCalledWith('c1');
    expect(screen.getAllByRole('progressbar')).toHaveLength(1);

    expect(screen.getByRole('button', { name: /^continue$/i })).toBeDisabled();
    finishSync('c1');
    expect(screen.getByRole('heading', { name: /first sync complete/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^continue$/i }));
    expect(await screen.findByRole('heading', { name: /you.re set up/i })).toBeInTheDocument();
  });
});

describe('OnboardingPage with a network scan', () => {
  it('syncs every connector created in the queue and shows each one’s progress', async () => {
    useDiscovery.getState().setScan(scanOf([pve, ha]));
    renderOnboarding();

    screen.getAllByRole('checkbox').forEach((box) => fireEvent.click(box));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));
    type(/api token secret/i, 's3cret');
    save();
    await screen.findByText('Candidate 2 of 2');
    type(/long-lived access token/i, 'tok');
    save();

    expect(await screen.findByRole('heading', { name: /first sync for 2 services/i })).toBeInTheDocument();
    await waitFor(() => expect(mocks.sync).toHaveBeenCalledTimes(2));
    expect(mocks.sync.mock.calls.map((c) => c[0]).sort()).toEqual(['c1', 'c2']);
    const bars = screen.getAllByRole('progressbar');
    expect(bars).toHaveLength(2);
    expect(bars.map((b) => b.getAttribute('aria-label'))).toEqual(['Proxmox VE (10.0.0.5)', 'Home Assistant (10.0.0.7)']);

    // Continue waits for both syncs.
    finishSync('c1');
    expect(screen.getByRole('button', { name: /^continue$/i })).toBeDisabled();
    finishSync('c2');
    expect(screen.getByRole('button', { name: /^continue$/i })).toBeEnabled();
  });

  it('stays on the connect step when every candidate is skipped', async () => {
    useDiscovery.getState().setScan(scanOf([pve, ha]));
    renderOnboarding();

    screen.getAllByRole('checkbox').forEach((box) => fireEvent.click(box));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    await screen.findByText('Candidate 2 of 2');
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));

    expect(await screen.findByRole('heading', { name: /connect your first service/i })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /scan your network/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /running the first sync/i })).toBeNull();
    expect(mocks.sync).not.toHaveBeenCalled();
    expect(mocks.postConnectors).not.toHaveBeenCalled();
  });

  it('offers to continue with what was connected after stopping the queue', async () => {
    useDiscovery.getState().setScan(scanOf([pve, ha]));
    renderOnboarding();

    screen.getAllByRole('checkbox').forEach((box) => fireEvent.click(box));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));
    type(/api token secret/i, 's3cret');
    save();
    await screen.findByText('Candidate 2 of 2');
    fireEvent.click(screen.getByRole('button', { name: /^stop$/i }));

    const cont = await screen.findByRole('button', { name: /continue with 1 connected service/i });
    fireEvent.click(cont);
    expect(await screen.findByRole('heading', { name: /running the first sync/i })).toBeInTheDocument();
    await waitFor(() => expect(mocks.sync).toHaveBeenCalledWith('c1'));
    expect(screen.getAllByRole('progressbar')).toHaveLength(1);
    expect(mocks.sync).toHaveBeenCalledTimes(1);
  });
});
