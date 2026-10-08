import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import type { DiscoveryCandidate, DiscoveryScan } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { AddConnectorPage } from './AddConnectorPage';

const mocks = vi.hoisted(() => ({ isAdmin: { current: true }, postConnectors: vi.fn() }));

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
];

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsSchema: () => ({ data: schemas, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetConnectors: () => ({ data: [] }),
  postConnectors: (...args: unknown[]) => mocks.postConnectors(...args),
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
const scan: DiscoveryScan = {
  id: 'scan-1',
  cidr: '10.0.0.0/24',
  state: 'completed',
  startedAt: '2026-10-07T12:00:00Z',
  endedAt: '2026-10-07T12:00:20Z',
  done: 254,
  total: 254,
  answered: 1,
  partial: false,
  candidates: [pve],
};

function renderPage() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={['/services/new']}>
        <Routes>
          <Route path="/services/new" element={<AddConnectorPage />} />
          <Route path="/services" element={<div>services list</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mocks.isAdmin.current = true;
  useDiscovery.setState({ scan: null });
  mocks.postConnectors.mockResolvedValue({ id: 'c1', name: 'pve', type: 'proxmox' });
});
afterEach(() => vi.clearAllMocks());

describe('AddConnectorPage scan action', () => {
  it('is shown to an instance admin', () => {
    renderPage();
    expect(screen.getByRole('button', { name: /scan network/i })).toBeInTheDocument();
  });

  it('is hidden for a user who is not an instance admin', () => {
    mocks.isAdmin.current = false;
    renderPage();
    expect(screen.queryByRole('button', { name: /scan network/i })).toBeNull();
    expect(screen.getByRole('button', { name: 'Proxmox VE' })).toBeInTheDocument();
  });

  it('swaps the form for the scan and back', () => {
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /scan network/i }));
    expect(screen.getByRole('heading', { name: /scan your network/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Proxmox VE' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /add by hand/i }));
    expect(screen.getByRole('button', { name: 'Proxmox VE' })).toBeInTheDocument();
  });

  it('returns to the services list when the connect queue ends', async () => {
    useDiscovery.getState().setScan(scan);
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /scan network/i }));
    fireEvent.click(screen.getByRole('checkbox'));
    fireEvent.click(screen.getByRole('button', { name: /connect 1 selected/i }));
    fireEvent.change(screen.getByLabelText(/api token secret/i), { target: { value: 's3cret' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    expect(await screen.findByText('services list')).toBeInTheDocument();
    await waitFor(() => expect(mocks.postConnectors).toHaveBeenCalledTimes(1));
  });

  it('also returns to the services list when every candidate is skipped', async () => {
    useDiscovery.getState().setScan(scan);
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /scan network/i }));
    fireEvent.click(screen.getByRole('checkbox'));
    fireEvent.click(screen.getByRole('button', { name: /connect 1 selected/i }));
    fireEvent.click(screen.getByRole('button', { name: /^skip$/i }));
    expect(await screen.findByText('services list')).toBeInTheDocument();
    expect(mocks.postConnectors).not.toHaveBeenCalled();
  });
});
