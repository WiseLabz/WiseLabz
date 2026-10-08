import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import type { DiscoveryCandidate, DiscoveryScan } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { ConnectQueue } from './ConnectQueue';
import { DiscoveryFlow } from './DiscoveryFlow';

const mocks = vi.hoisted(() => ({ postConnectors: vi.fn(), getScan: vi.fn() }));
const postConnectors = mocks.postConnectors;

// Field names and kinds as the backend emits them for these two types.
const schemas = [
  {
    type: 'proxmox',
    category: 'virtualization',
    displayName: 'Proxmox VE',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'API URL', kind: 'text', required: true },
      { name: 'token_secret', label: 'API Token Secret', kind: 'password', required: true },
      { name: 'verify_tls', label: 'Verify TLS', kind: 'toggle', required: false, default: 'true' },
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
  {
    type: 'docker',
    category: 'containers_paas',
    displayName: 'Docker',
    isCredentialRefresher: false,
    fields: [{ name: 'host', label: 'Docker Host', kind: 'text', required: true }],
  },
];

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsSchema: () => ({ data: schemas, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetConnectors: () => ({ data: [] }),
  postConnectors: (...args: unknown[]) => postConnectors(...args),
  getGetConnectorsQueryKey: () => [],
}));
vi.mock('../../api/generated/discovery/discovery', () => ({
  useGetDiscoverySuggestions: () => ({ data: { suggestions: [] } }),
  // The real query over an endpoint that never answers; tests that need it override `getScan`.
  useGetDiscoveryScan: (options?: { query?: object }) =>
    useQuery({ queryKey: ['/discovery/scan'], queryFn: () => mocks.getScan(), ...options?.query }),
  getGetDiscoveryScanQueryKey: () => ['/discovery/scan'],
  startDiscoveryScan: vi.fn(),
  cancelDiscoveryScan: vi.fn(),
}));
vi.mock('../compliance/CertificateExpiryPackOffer', () => ({ CertificateExpiryPackOffer: () => null }));
vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => true }));

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
};

const completedScan = (candidates: DiscoveryCandidate[]): DiscoveryScan => ({
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

function wrap(ui: React.ReactNode, client = new QueryClient()) {
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
}

const save = () => fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
const type = (label: RegExp, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } });

beforeEach(() => {
  useDiscovery.setState({ scan: null });
  mocks.getScan.mockReset();
  mocks.getScan.mockImplementation(() => new Promise(() => {}));
  let n = 0;
  postConnectors.mockImplementation(async (body: { name: string; type: string }) => ({ id: `c${++n}`, name: body.name, type: body.type }));
});
afterEach(() => vi.clearAllMocks());

describe('ConnectQueue', () => {
  it('opens each candidate in the standard form with the type chosen, the address filled in and its position shown', () => {
    render(wrap(<ConnectQueue candidates={[pve, ha]} onCreated={vi.fn()} onFinished={vi.fn()} onStop={vi.fn()} />));
    expect(screen.getByText('Candidate 1 of 2')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: /connect proxmox ve at 10\.0\.0\.5:8006/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/api url/i)).toHaveValue('https://10.0.0.5:8006/api2/json');
    expect(screen.getByRole('button', { name: 'Proxmox VE' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: /^skip$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^stop$/i })).toBeInTheDocument();
  });

  it('fills a type that keeps its endpoint in another field (Docker host)', async () => {
    render(wrap(<ConnectQueue candidates={[docker]} onCreated={vi.fn()} onFinished={vi.fn()} onStop={vi.fn()} />));
    expect(screen.getByLabelText(/docker host/i)).toHaveValue('tcp://10.0.0.3:2375');
    save();
    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    expect(postConnectors.mock.calls[0][0]).toMatchObject({ type: 'docker', config: { host: 'tcp://10.0.0.3:2375' } });
  });

  it('moves to the next candidate after a save and finishes after the last', async () => {
    const onCreated = vi.fn();
    const onFinished = vi.fn();
    render(wrap(<ConnectQueue candidates={[pve, ha]} onCreated={onCreated} onFinished={onFinished} onStop={vi.fn()} />));

    type(/api token secret/i, 's3cret');
    save();
    expect(await screen.findByText('Candidate 2 of 2')).toBeInTheDocument();
    expect(onCreated).toHaveBeenCalledTimes(1);
    expect(onCreated.mock.calls[0][1]).toBe(pve);
    expect(onFinished).not.toHaveBeenCalled();

    expect(screen.getByLabelText(/home assistant url/i)).toHaveValue('http://10.0.0.7:8123');
    type(/long-lived access token/i, 'tok');
    save();
    await waitFor(() => expect(onFinished).toHaveBeenCalledTimes(1));
    expect(onCreated).toHaveBeenCalledTimes(2);
    expect(postConnectors).toHaveBeenCalledTimes(2);
  });

  it('skips without creating anything', async () => {
    const onCreated = vi.fn();
    const onFinished = vi.fn();
    render(wrap(<ConnectQueue candidates={[pve, ha]} onCreated={onCreated} onFinished={onFinished} onStop={vi.fn()} />));
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    expect(await screen.findByText('Candidate 2 of 2')).toBeInTheDocument();
    expect(postConnectors).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    expect(onFinished).toHaveBeenCalledTimes(1);
    expect(onCreated).not.toHaveBeenCalled();
  });

  it('stays on the candidate when saving fails, shows the error, and lets the admin correct it', async () => {
    postConnectors.mockRejectedValueOnce(
      Object.assign(new Error('bad'), {
        isAxiosError: true,
        response: { status: 400, data: { code: 'invalid_request', message: 'bad token', details: [{ field: 'token_secret', msg: 'token rejected by the server' }] } },
      }),
    );
    const onCreated = vi.fn();
    render(wrap(<ConnectQueue candidates={[pve, ha]} onCreated={onCreated} onFinished={vi.fn()} onStop={vi.fn()} />));
    type(/api token secret/i, 'wrong');
    save();
    expect((await screen.findAllByText(/token rejected by the server/i)).length).toBeGreaterThan(0);
    expect(screen.getByText('Candidate 1 of 2')).toBeInTheDocument();
    expect(onCreated).not.toHaveBeenCalled();

    // Correct it and retry, or skip.
    type(/api token secret/i, 'right');
    save();
    expect(await screen.findByText('Candidate 2 of 2')).toBeInTheDocument();
    expect(onCreated).toHaveBeenCalledTimes(1);
  });

  it('hands control back when stopped', () => {
    const onStop = vi.fn();
    const onFinished = vi.fn();
    render(wrap(<ConnectQueue candidates={[pve, ha]} onCreated={vi.fn()} onFinished={onFinished} onStop={onStop} />));
    fireEvent.click(screen.getByRole('button', { name: /^stop$/i }));
    expect(onStop).toHaveBeenCalledTimes(1);
    expect(onFinished).not.toHaveBeenCalled();
  });
});

describe('DiscoveryFlow (panel and queue together)', () => {
  const rows = () => screen.getAllByRole('listitem');

  it('two selected and saved: both connectors exist and both candidates show as already connected', async () => {
    useDiscovery.getState().setScan(completedScan([pve, ha]));
    const onConnectorCreated = vi.fn();
    const onQueueFinished = vi.fn();
    render(wrap(<DiscoveryFlow onConnectorCreated={onConnectorCreated} onQueueFinished={onQueueFinished} />));

    rows().forEach((r) => fireEvent.click(within(r).getByRole('checkbox')));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));

    type(/api token secret/i, 's3cret');
    save();
    await screen.findByText('Candidate 2 of 2');
    type(/long-lived access token/i, 'tok');
    save();

    await waitFor(() => expect(onQueueFinished).toHaveBeenCalledTimes(1));
    expect(postConnectors).toHaveBeenCalledTimes(2);
    expect(onConnectorCreated).toHaveBeenCalledTimes(2);
    // Back on the result list, both are connected and none can be selected.
    await waitFor(() => expect(screen.getAllByText('Already connected')).toHaveLength(2));
    rows().forEach((r) => expect(within(r).getByRole('checkbox')).toBeDisabled());
  });

  it('skipping the first and saving the second leaves the first selectable', async () => {
    useDiscovery.getState().setScan(completedScan([pve, ha]));
    const onQueueFinished = vi.fn();
    render(wrap(<DiscoveryFlow onQueueFinished={onQueueFinished} />));

    rows().forEach((r) => fireEvent.click(within(r).getByRole('checkbox')));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    await screen.findByText('Candidate 2 of 2');
    type(/long-lived access token/i, 'tok');
    save();

    await waitFor(() => expect(onQueueFinished).toHaveBeenCalledTimes(1));
    expect(postConnectors).toHaveBeenCalledTimes(1);
    expect(postConnectors.mock.calls[0][0]).toMatchObject({ type: 'home_assistant' });
    const [first, second] = rows();
    expect(within(first).getByRole('checkbox')).toBeEnabled();
    expect(within(first).queryByText('Already connected')).toBeNull();
    expect(within(second).getByText('Already connected')).toBeInTheDocument();
  });

  it('stopping returns to the list without finishing the queue, keeping what was connected', async () => {
    useDiscovery.getState().setScan(completedScan([pve, ha]));
    const onQueueFinished = vi.fn();
    const onConnectorCreated = vi.fn();
    render(wrap(<DiscoveryFlow onQueueFinished={onQueueFinished} onConnectorCreated={onConnectorCreated} />));

    rows().forEach((r) => fireEvent.click(within(r).getByRole('checkbox')));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));
    type(/api token secret/i, 's3cret');
    save();
    await screen.findByText('Candidate 2 of 2');
    fireEvent.click(screen.getByRole('button', { name: /^stop$/i }));

    expect(await screen.findByText('Already connected')).toBeInTheDocument();
    expect(onConnectorCreated).toHaveBeenCalledTimes(1);
    expect(onQueueFinished).not.toHaveBeenCalled();
  });

  it('returning within the query staleTime still shows the candidate just connected as connected', async () => {
    // The app keeps reads fresh for 30 s, so the panel remounting after the queue
    // would otherwise hydrate the store from the cached scan taken before the save.
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000 } } });
    const server = completedScan([pve, ha]);
    mocks.getScan.mockResolvedValueOnce({ scan: server });
    useDiscovery.getState().setScan(server);
    render(wrap(<DiscoveryFlow />, client));
    await waitFor(() => expect(client.getQueryState(['/discovery/scan'])?.status).toBe('success'));

    rows().forEach((r) => fireEvent.click(within(r).getByRole('checkbox')));
    fireEvent.click(screen.getByRole('button', { name: /connect 2 selected/i }));
    type(/api token secret/i, 's3cret');
    save();
    await screen.findByText('Candidate 2 of 2');
    fireEvent.click(screen.getByRole('button', { name: /^stop$/i }));

    // The refetch of the server's view has not landed (the mock never answers it).
    const [first, second] = await screen.findAllByRole('listitem');
    expect(within(first).getByText('Already connected')).toBeInTheDocument();
    expect(within(first).getByRole('checkbox')).toBeDisabled();
    expect(within(second).queryByText('Already connected')).toBeNull();
    expect(within(second).getByRole('checkbox')).toBeEnabled();
    // The cache was invalidated, so the remount asked the server again.
    await waitFor(() => expect(mocks.getScan).toHaveBeenCalledTimes(2));
  });
});
