import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import type { Connector, DiscoveryAddOn, DiscoveryCandidate, DiscoveryScan, DiscoveryTLSTarget } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { ConnectQueue } from './ConnectQueue';
import { DiscoveryFlow } from './DiscoveryFlow';

const mocks = vi.hoisted(() => ({
  postConnectors: vi.fn(),
  putConnector: vi.fn(),
  getConnectors: [] as { id: string; type: string; name: string; managedBy?: string; config?: Record<string, string> }[],
  connectorsLoading: false,
  connectorsError: false,
  detailLoading: false,
  detailError: false,
  refetchConnectors: vi.fn(),
  refetchDetail: vi.fn(),
  // When set, the detail read runs through the real query cache instead of the static fake.
  detailFetch: null as null | ((id: string) => Promise<unknown>),
  listOptions: [] as unknown[],
  getScan: vi.fn(),
}));
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
  {
    type: 'pfsense',
    category: 'networking',
    displayName: 'pfSense',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'pfSense URL', kind: 'text', required: true },
      { name: 'api_key', label: 'API Key', kind: 'password', required: true },
    ],
  },
  {
    type: 'dnsresolver',
    category: 'networking',
    displayName: 'DNS Resolver',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'DNS Resolver URL', kind: 'text', required: true },
      { name: 'api_key', label: 'API Key', kind: 'password', required: true },
    ],
  },
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
  useGetConnectorsSchema: () => ({ data: schemas, isLoading: false, isError: false, refetch: vi.fn() }),
  // Like GET /connectors, the list never carries a connector's config.
  useGetConnectors: (options?: unknown) => {
    mocks.listOptions.push(options);
    return {
      data: mocks.getConnectors.map((connector) => Object.fromEntries(Object.entries(connector).filter(([key]) => key !== 'config'))),
      isLoading: mocks.connectorsLoading,
      isError: mocks.connectorsError,
      refetch: mocks.refetchConnectors,
    };
  },
  // The detail read carries the config.
  useGetConnectorsConnectorId: (id: string, options?: { query?: object }) => {
    // eslint-disable-next-line react-hooks/rules-of-hooks -- fixed for the whole test
    if (mocks.detailFetch) return useQuery({ queryKey: ['/connectors', id], queryFn: () => mocks.detailFetch!(id), retry: false, ...options?.query });
    return {
      data: mocks.detailLoading || mocks.detailError ? undefined : mocks.getConnectors.find((connector) => connector.id === id),
      isLoading: mocks.detailLoading,
      isError: mocks.detailError,
      isFetchedAfterMount: !mocks.detailLoading && !mocks.detailError,
      dataUpdatedAt: Number.MAX_SAFE_INTEGER,
      refetch: mocks.refetchDetail,
    };
  },
  postConnectors: (...args: unknown[]) => postConnectors(...args),
  putConnectorsConnectorId: (...args: unknown[]) => mocks.putConnector(...args),
  postConnectorsConnectorIdTest: vi.fn(),
  usePreviewConnectorRecipe: () => ({ mutate: vi.fn(), isPending: false, isError: false }),
  getGetConnectorsQueryKey: () => [],
  getGetConnectorsConnectorIdQueryKey: (id: string) => ['/connectors', id],
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
vi.mock('../compliance/CertificateExpiryPackOffer', () => ({
  CertificateExpiryPackOffer: ({ requested, onFinished }: { requested: boolean; onFinished: () => void }) =>
    requested ? <button onClick={onFinished}>Finish certificate offer</button> : null,
}));
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

const resolverAddOn: DiscoveryAddOn = {
  type: 'dnsresolver',
  url: 'https://10.0.0.8:443/api/v2',
};
const pfsense: DiscoveryCandidate = {
  type: 'pfsense',
  name: 'pfSense',
  address: '10.0.0.8',
  port: 443,
  url: 'https://10.0.0.8:443/api/v2',
  urlField: 'url',
  addOns: [resolverAddOn],
};

const tlsTarget = (address: string, port = 443): DiscoveryTLSTarget => ({
  address,
  port,
  source: 'range',
});

const tlsProbe = (id: string, name: string, targets: string, managedBy?: string): Connector => ({
  id,
  name,
  type: 'tlsprobe',
  category: 'monitoring',
  enabled: true,
  status: 'unknown',
  capabilities: { restart: false, start: false, stop: false, configPush: false, configRead: false, credentialRefresh: false },
  actions: [],
  managedBy: managedBy as Connector['managedBy'],
  config: { targets, import_connector_id: '', import_port: '443' },
});

const completedScan = (candidates: DiscoveryCandidate[], tlsTargets?: DiscoveryTLSTarget[]): DiscoveryScan => ({
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
  ...(tlsTargets ? { tlsTargets } : {}),
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
  mocks.getConnectors = [];
  mocks.connectorsLoading = false;
  mocks.connectorsError = false;
  mocks.detailLoading = false;
  mocks.detailError = false;
  mocks.detailFetch = null;
  mocks.listOptions = [];
  mocks.refetchConnectors.mockReset();
  mocks.refetchDetail.mockReset();
  let n = 0;
  postConnectors.mockImplementation(async (body: { name: string; type: string; config?: Record<string, string> }) => ({
    id: `c${++n}`,
    name: body.name,
    type: body.type,
    config: body.config,
  }));
  mocks.putConnector.mockImplementation(async (id: string, body: { config?: Record<string, string> }) => {
    const current = mocks.getConnectors.find((connector) => connector.id === id);
    return { ...current, ...body, config: { ...current?.config, ...body.config } };
  });
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

  it('offers pfSense DNS Resolver immediately after the candidate and saves it when selected by default', async () => {
    const onCreated = vi.fn();
    const onAddOnCreated = vi.fn();
    const onFinished = vi.fn();
    render(wrap(
      <ConnectQueue
        candidates={[pfsense]}
        onCreated={onCreated}
        onAddOnCreated={onAddOnCreated}
        onFinished={onFinished}
        onStop={vi.fn()}
      />,
    ));

    type(/api key/i, 'pfsense-secret');
    save();
    expect(await screen.findByRole('heading', { name: /connect dns resolver at 10\.0\.0\.8:443/i })).toBeInTheDocument();
    expect(onCreated).toHaveBeenCalledTimes(1);
    expect(onAddOnCreated).not.toHaveBeenCalled();

    type(/api key/i, 'resolver-secret');
    save();
    await waitFor(() => expect(onFinished).toHaveBeenCalledOnce());
    expect(postConnectors.mock.calls.map(([body]) => (body as { type: string }).type)).toEqual(['pfsense', 'dnsresolver']);
    expect(onAddOnCreated.mock.calls[0][1]).toBe(pfsense);
    expect(onAddOnCreated.mock.calls[0][2]).toBe(resolverAddOn);
  });

  it('keeps a selected add-on after the parent candidate is skipped, and lets the add-on be skipped', async () => {
    const onFinished = vi.fn();
    const onAddOnCreated = vi.fn();
    render(wrap(
      <ConnectQueue
        candidates={[pfsense]}
        addOns={[{ candidate: pfsense, addOn: resolverAddOn }]}
        onCreated={vi.fn()}
        onAddOnCreated={onAddOnCreated}
        onFinished={onFinished}
        onStop={vi.fn()}
      />,
    ));

    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    expect(await screen.findByRole('heading', { name: /connect dns resolver/i })).toBeInTheDocument();
    expect(postConnectors).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    expect(onFinished).toHaveBeenCalledOnce();
    expect(onAddOnCreated).not.toHaveBeenCalled();
  });

  it('stops from an add-on step without finishing the queue', async () => {
    const onStop = vi.fn();
    const onFinished = vi.fn();
    render(wrap(
      <ConnectQueue
        candidates={[pfsense]}
        addOns={[{ candidate: pfsense, addOn: resolverAddOn }]}
        onCreated={vi.fn()}
        onFinished={onFinished}
        onStop={onStop}
      />,
    ));
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    await screen.findByRole('heading', { name: /connect dns resolver/i });
    fireEvent.click(screen.getByRole('button', { name: /^stop$/i }));
    expect(onStop).toHaveBeenCalledOnce();
    expect(onFinished).not.toHaveBeenCalled();
  });

  it('queues an independently selected add-on even when its candidate is not selected', async () => {
    useDiscovery.getState().setScan(completedScan([pfsense]));
    render(wrap(<DiscoveryFlow />));

    fireEvent.click(screen.getByRole('checkbox', { name: /dns resolver https:\/\/10\.0\.0\.8/i }));
    expect(screen.getByRole('checkbox', { name: /pfsense.*10\.0\.0\.8:443/i })).not.toBeChecked();
    fireEvent.click(screen.getByRole('button', { name: /connect 1 selected/i }));
    expect(await screen.findByRole('heading', { name: /connect dns resolver at 10\.0\.0\.8:443/i })).toBeInTheDocument();
    type(/api key/i, 'resolver-secret');
    save();

    await waitFor(() => expect(postConnectors).toHaveBeenCalledOnce());
    expect(postConnectors.mock.calls[0][0]).toMatchObject({ type: 'dnsresolver' });
  });

  it('appends only two of four certificates to a 98-target probe and marks only targets still saved', async () => {
    const existing = Array.from({ length: 98 }, (_, index) => `existing-${index}.example:443`).join('\n');
    const probe = tlsProbe('probe-1', 'Existing probe', existing);
    mocks.getConnectors = [probe];
    const targets = ['10.0.0.20', '10.0.0.21', '10.0.0.22', '10.0.0.23'].map((address) => tlsTarget(address));
    const onTLSProbeSaved = vi.fn();
    const onFinished = vi.fn();
    render(wrap(
      <ConnectQueue
        candidates={[]}
        tlsTargets={targets}
        onCreated={vi.fn()}
        onTLSProbeSaved={onTLSProbeSaved}
        onFinished={onFinished}
        onStop={vi.fn()}
      />,
    ));

    const field = screen.getByLabelText('Targets');
    await waitFor(() => expect((field as HTMLTextAreaElement).value.split('\n')).toHaveLength(100));
    const initialTargets = (field as HTMLTextAreaElement).value.split('\n');
    expect(initialTargets).toHaveLength(100);
    expect(initialTargets.slice(-2)).toEqual(['10.0.0.20:443', '10.0.0.21:443']);
    expect(screen.getByText('Not added (100-target limit)')).toBeInTheDocument();
    expect(screen.getByText('10.0.0.22:443')).toBeInTheDocument();
    expect(screen.getByText('10.0.0.23:443')).toBeInTheDocument();

    fireEvent.change(field, {
      target: { value: initialTargets.filter((line) => line !== '10.0.0.20:443').join('\n') },
    });
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }));
    await waitFor(() => expect(onTLSProbeSaved).toHaveBeenCalledOnce());
    expect(mocks.putConnector.mock.calls[0][0]).toBe('probe-1');
    expect((mocks.putConnector.mock.calls[0][1] as { config: { targets: string } }).config.targets.split('\n')).toHaveLength(99);
    expect(onTLSProbeSaved.mock.calls[0][1]).toEqual([targets[1]]);
    expect(onFinished).toHaveBeenCalledOnce();
  });

  it('plans from the fresh probe detail, never from a stale cached entry', async () => {
    // A save leaves the cached detail with the pre-save targets for up to the cache lifetime.
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000 } } });
    client.setQueryData(['/connectors', 'probe-1'], tlsProbe('probe-1', 'Existing probe', 'old.example:443'));
    mocks.getConnectors = [tlsProbe('probe-1', 'Existing probe', 'old.example:443\nnewer.example:443')];
    let answer: (value: unknown) => void = () => {};
    mocks.detailFetch = vi.fn(() => new Promise((resolve) => { answer = resolve; }));
    render(wrap(
      <ConnectQueue candidates={[]} tlsTargets={[tlsTarget('10.0.0.20')]} onCreated={vi.fn()} onFinished={vi.fn()} onStop={vi.fn()} />,
      client,
    ));

    await waitFor(() => expect(mocks.detailFetch).toHaveBeenCalledOnce());
    // The form must not render from the stale entry while the fresh read is in flight.
    expect(screen.queryByLabelText('Targets')).toBeNull();
    answer(mocks.getConnectors[0]);
    await waitFor(() => expect(screen.getByLabelText('Targets')).toHaveValue('old.example:443\nnewer.example:443\n10.0.0.20:443'));
  });

  it('deduplicates compressed and expanded IPv6 targets', async () => {
    mocks.getConnectors = [tlsProbe('probe-v6', 'IPv6 probe', '[fd00:0:0:0:0:0:0:1]:443')];
    render(wrap(
      <ConnectQueue
        candidates={[]}
        tlsTargets={[tlsTarget('fd00::1')]}
        onCreated={vi.fn()}
        onFinished={vi.fn()}
        onStop={vi.fn()}
      />,
    ));

    await waitFor(() => expect(screen.getByLabelText('Targets')).toHaveValue('[fd00:0:0:0:0:0:0:1]:443'));
    expect((screen.getByLabelText('Targets') as HTMLTextAreaElement).value.split('\n')).toHaveLength(1);
  });

  it('waits for the connector list before choosing create, and offers retry on read failure', () => {
    mocks.connectorsLoading = true;
    const props = {
      candidates: [],
      tlsTargets: [tlsTarget('10.0.0.20')],
      onCreated: vi.fn(),
      onFinished: vi.fn(),
      onStop: vi.fn(),
    };
    const view = render(wrap(<ConnectQueue {...props} />));
    expect(screen.queryByRole('button', { name: 'TLS Probe' })).toBeNull();

    mocks.connectorsLoading = false;
    mocks.connectorsError = true;
    view.rerender(wrap(<ConnectQueue {...props} />));
    expect(screen.getByText(/could not load tls probes/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    expect(mocks.refetchConnectors).toHaveBeenCalledOnce();
    expect(screen.queryByRole('button', { name: 'TLS Probe' })).toBeNull();
  });

  it('holds the form until the probe detail loads, and offers retry when the detail read fails', async () => {
    mocks.getConnectors = [tlsProbe('probe-1', 'Existing probe', 'existing.example:443')];
    mocks.detailLoading = true;
    const props = {
      candidates: [],
      tlsTargets: [tlsTarget('10.0.0.20')],
      onCreated: vi.fn(),
      onFinished: vi.fn(),
      onStop: vi.fn(),
    };
    const view = render(wrap(<ConnectQueue {...props} />));
    expect(screen.queryByLabelText('Targets')).toBeNull();
    expect(screen.queryByText('Not added (100-target limit)')).toBeNull();

    mocks.detailLoading = false;
    mocks.detailError = true;
    view.rerender(wrap(<ConnectQueue {...props} />));
    expect(screen.getByText(/could not load tls probes/i)).toBeInTheDocument();
    expect(screen.queryByLabelText('Targets')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    expect(mocks.refetchDetail).toHaveBeenCalledOnce();

    mocks.detailError = false;
    view.rerender(wrap(<ConnectQueue {...props} />));
    await waitFor(() => expect(screen.getByLabelText('Targets')).toHaveValue('existing.example:443\n10.0.0.20:443'));
  });

  it('looks for an existing TLS Probe in the largest connector page', () => {
    render(wrap(
      <ConnectQueue
        candidates={[]}
        tlsTargets={[tlsTarget('10.0.0.20')]}
        onCreated={vi.fn()}
        onFinished={vi.fn()}
        onStop={vi.fn()}
      />,
    ));
    expect(mocks.listOptions).toContainEqual(expect.objectContaining({
      request: { params: { pageSize: 100 } },
      query: { queryKey: [{ pageSize: 100 }] },
    }));
  });

  it('creates a TLS Probe when none exists and skips the certificate offer before continuing', async () => {
    const target = tlsTarget('10.0.0.20');
    const onTLSProbeSaved = vi.fn();
    const onFinished = vi.fn();
    render(wrap(
      <ConnectQueue
        candidates={[]}
        tlsTargets={[target]}
        onCreated={vi.fn()}
        onTLSProbeSaved={onTLSProbeSaved}
        onFinished={onFinished}
        onStop={vi.fn()}
      />,
    ));

    expect(screen.getByRole('button', { name: 'TLS Probe' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByLabelText('Targets')).toHaveValue('10.0.0.20:443');
    save();
    fireEvent.click(await screen.findByRole('button', { name: 'Finish certificate offer' }));
    await waitFor(() => expect(onFinished).toHaveBeenCalledOnce());
    expect(postConnectors.mock.calls[0][0]).toMatchObject({ type: 'tlsprobe', config: { targets: '10.0.0.20:443' } });
    expect(onTLSProbeSaved.mock.calls[0][1]).toEqual([target]);
  });

  it('defaults to the newest TLS Probe and lets the admin choose another one', async () => {
    mocks.getConnectors = [
      tlsProbe('newest', 'Newest probe', 'new.example:443'),
      tlsProbe('older', 'Older probe', 'old.example:443'),
    ];
    render(wrap(
      <ConnectQueue
        candidates={[]}
        tlsTargets={[tlsTarget('10.0.0.20')]}
        onCreated={vi.fn()}
        onFinished={vi.fn()}
        onStop={vi.fn()}
      />,
    ));

    const chooser = screen.getByRole('combobox', { name: 'TLS Probe to update' });
    expect(chooser).toHaveValue('newest');
    await waitFor(() => expect(screen.getByLabelText('Targets')).toHaveValue('new.example:443\n10.0.0.20:443'));
    fireEvent.change(chooser, { target: { value: 'older' } });
    await waitFor(() => expect(screen.getByLabelText('Targets')).toHaveValue('old.example:443\n10.0.0.20:443'));
  });

  it('refetches on every probe switch under the app staleTime, so A to B to A never sticks on the skeleton', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000 } } });
    mocks.getConnectors = [tlsProbe('a', 'Probe A', 'a.example:443'), tlsProbe('b', 'Probe B', 'b.example:443')];
    mocks.detailFetch = vi.fn(async (id: string) => mocks.getConnectors.find((connector) => connector.id === id));
    render(wrap(
      <ConnectQueue candidates={[]} tlsTargets={[tlsTarget('10.0.0.20')]} onCreated={vi.fn()} onFinished={vi.fn()} onStop={vi.fn()} />,
      client,
    ));
    const chooser = screen.getByRole('combobox', { name: 'TLS Probe to update' });
    for (const [id, host] of [['a', 'a'], ['b', 'b'], ['a', 'a']]) {
      fireEvent.change(chooser, { target: { value: id } });
      await waitFor(() => expect(screen.getByLabelText('Targets')).toHaveValue(`${host}.example:443\n10.0.0.20:443`));
    }
    expect(mocks.detailFetch).toHaveBeenCalledTimes(3);
  });

  it('shows the error, not the form, when a stale cached detail is followed by a failed fresh read', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000, retry: false } } });
    client.setQueryData(['/connectors', 'a'], tlsProbe('a', 'Probe A', 'old.example:443'));
    mocks.getConnectors = [tlsProbe('a', 'Probe A', 'old.example:443')];
    mocks.detailFetch = vi.fn(() => Promise.reject(new Error('down')));
    render(wrap(
      <ConnectQueue candidates={[]} tlsTargets={[tlsTarget('10.0.0.20')]} onCreated={vi.fn()} onFinished={vi.fn()} onStop={vi.fn()} />,
      client,
    ));
    expect(await screen.findByText(/could not load tls probes/i)).toBeInTheDocument();
    expect(screen.queryByLabelText('Targets')).toBeNull();
  });

  it('keeps the form mounted when a background refetch fails after a good fresh read', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000, retry: false } } });
    mocks.getConnectors = [tlsProbe('a', 'Probe A', 'a.example:443')];
    mocks.detailFetch = vi.fn(async (id: string) => mocks.getConnectors.find((connector) => connector.id === id));
    render(wrap(
      <ConnectQueue candidates={[]} tlsTargets={[tlsTarget('10.0.0.20')]} onCreated={vi.fn()} onFinished={vi.fn()} onStop={vi.fn()} />,
      client,
    ));
    await waitFor(() => expect(screen.getByLabelText('Targets')).toHaveValue('a.example:443\n10.0.0.20:443'));
    mocks.detailFetch = vi.fn(() => Promise.reject(new Error('down')));
    await client.refetchQueries({ queryKey: ['/connectors', 'a'] });
    await waitFor(() => expect(client.getQueryState(['/connectors', 'a'])?.status).toBe('error'));
    expect(screen.getByLabelText('Targets')).toHaveValue('a.example:443\n10.0.0.20:443');
  });

  it('does not queue an add-on explicitly deselected in the discovery panel', async () => {
    useDiscovery.getState().setScan(completedScan([pfsense]));
    const onQueueFinished = vi.fn();
    render(wrap(<DiscoveryFlow onQueueFinished={onQueueFinished} />));

    fireEvent.click(screen.getByRole('checkbox', { name: /pfsense.*10\.0\.0\.8:443/i }));
    const addonCheckbox = screen.getByRole('checkbox', { name: /dns resolver https:\/\/10\.0\.0\.8/i });
    expect(addonCheckbox).toBeChecked();
    fireEvent.click(addonCheckbox);
    fireEvent.click(screen.getByRole('button', { name: /connect 1 selected/i }));
    type(/api key/i, 'pfsense-secret');
    save();

    await waitFor(() => expect(onQueueFinished).toHaveBeenCalledOnce());
    expect(postConnectors).toHaveBeenCalledTimes(1);
    expect(postConnectors.mock.calls[0][0]).toMatchObject({ type: 'pfsense' });
    expect(screen.queryByRole('heading', { name: /connect dns resolver/i })).toBeNull();
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

  it('reports a save that finishes after Stop without finishing the stopped queue', async () => {
    let complete: ((connector: { id: string; name: string; type: string }) => void) | undefined;
    postConnectors.mockImplementationOnce(() => new Promise((resolve) => { complete = resolve; }));
    const onCreated = vi.fn();
    const onFinished = vi.fn();
    const onStop = vi.fn();
    render(wrap(<ConnectQueue candidates={[pve]} onCreated={onCreated} onFinished={onFinished} onStop={onStop} />));
    type(/api token secret/i, 'slow-secret');
    save();
    await waitFor(() => expect(postConnectors).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByRole('button', { name: /^stop$/i }));
    complete?.({ id: 'late-candidate', name: 'late candidate', type: 'proxmox' });

    await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
    expect(onStop).toHaveBeenCalledOnce();
    expect(onFinished).not.toHaveBeenCalled();
  });

  it('reports a save that finishes after Skip without advancing the next step twice', async () => {
    let complete: ((connector: { id: string; name: string; type: string }) => void) | undefined;
    postConnectors.mockImplementationOnce(() => new Promise((resolve) => { complete = resolve; }));
    const onCreated = vi.fn();
    const onFinished = vi.fn();
    render(wrap(<ConnectQueue candidates={[pve, ha]} onCreated={onCreated} onFinished={onFinished} onStop={vi.fn()} />));
    type(/api token secret/i, 'slow-secret');
    save();
    await waitFor(() => expect(postConnectors).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    expect(await screen.findByText('Candidate 2 of 2')).toBeInTheDocument();
    complete?.({ id: 'late-candidate', name: 'late candidate', type: 'proxmox' });

    await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
    expect(screen.getByText('Candidate 2 of 2')).toBeInTheDocument();
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
