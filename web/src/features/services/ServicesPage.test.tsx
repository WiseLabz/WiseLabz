import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguagePreference } from '../../i18n';
import { ServicesPage } from './ServicesPage';
import type { Connector } from '../../api/model';

let connectorsList: Connector[] = [
  { id: 'c1', name: 'PVE Cluster', category: 'virtualization', type: 'proxmox', url: 'https://pve.local', enabled: true, status: 'online' },
  { id: 'c2', name: 'Docker Host', category: 'containers_paas', type: 'docker', url: 'https://docker.local', enabled: true, status: 'online' },
  { id: 'c3', name: 'Core Switch', category: 'networking', type: 'opnsense', url: 'https://switch.local', enabled: true, status: 'online' },
  { id: 'c4', name: 'AdGuard Home', category: 'dns', type: 'adguardhome', url: 'https://dns.local', enabled: true, status: 'online' },
  { id: 'c5', name: 'TrueNAS Core', category: 'storage', type: 'custom', url: 'https://nas.local', enabled: true, status: 'online' },
  { id: 'c6', name: 'Prometheus', category: 'monitoring', type: 'custom', url: 'https://prom.local', enabled: true, status: 'online' },
  { id: 'c7', name: 'Jellyfin Media', category: 'media', type: 'custom', url: 'https://jellyfin.local', enabled: true, status: 'online' },
  { id: 'c8', name: 'Home Assistant', category: 'other', type: 'custom', url: 'https://hass.local', enabled: true, status: 'online' },
];

vi.mock('../../api/generated/connectors/connectors', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/generated/connectors/connectors')>();
  return {
    ...actual,
    useGetConnectors: () => ({
      data: connectorsList,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    }),
    useGetConnectorsMaintenanceWindows: () => ({ data: [] }),
    putConnectorsConnectorIdEnabled: vi.fn(),
    postConnectorsBulkSync: vi.fn(),
    postConnectorsBulkReauth: vi.fn(),
    postConnectorsBulkRestart: vi.fn(),
  };
});

vi.mock('../../hooks/useRole', () => ({
  useIsInstanceAdmin: () => true,
  useOperatorConnectorIds: () => new Set<string>(),
}));

describe('ServicesPage category presentation (#513)', () => {
  let queryClient: QueryClient;

  beforeEach(async () => {
    queryClient = new QueryClient();
    await setLanguagePreference('en');
  });

  afterEach(async () => {
    await setLanguagePreference('en');
  });

  it('renders all connector categories with English labels', async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ServicesPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByText('TrueNAS Core')).toBeInTheDocument();
    expect(screen.getByText('Storage')).toBeInTheDocument();
    expect(screen.getByText('Monitoring')).toBeInTheDocument();
    expect(screen.getByText('Media')).toBeInTheDocument();
    expect(screen.getByText('Other')).toBeInTheDocument();
    expect(screen.getByText('Virtualization')).toBeInTheDocument();
    expect(screen.getByText('Containers')).toBeInTheDocument();
    expect(screen.getByText('Networking')).toBeInTheDocument();
    expect(screen.getByText('DNS')).toBeInTheDocument();
  });

  it('renders all connector categories with Portuguese (Brazil) labels', async () => {
    await setLanguagePreference('pt-BR');

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ServicesPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByText('TrueNAS Core')).toBeInTheDocument();
    expect(screen.getByText('Armazenamento')).toBeInTheDocument();
    expect(screen.getByText('Monitoramento')).toBeInTheDocument();
    expect(screen.getByText('Mídia')).toBeInTheDocument();
    expect(screen.getByText('Outros')).toBeInTheDocument();
    expect(screen.getByText('Virtualização')).toBeInTheDocument();
    expect(screen.getByText('Contêineres')).toBeInTheDocument();
    expect(screen.getByText('Rede')).toBeInTheDocument();
    expect(screen.getByText('DNS')).toBeInTheDocument();
  });

  it('renders a connector with a category this bundle does not know', () => {
    const original = connectorsList;
    connectorsList = [
      ...original,
      {
        id: 'c9',
        name: 'Arcade Cabinet',
        category: 'gaming' as unknown as Connector['category'],
        type: 'custom',
        url: 'https://arcade.local',
        enabled: true,
        status: 'online',
      },
    ];
    try {
      render(
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <ServicesPage />
          </MemoryRouter>
        </QueryClientProvider>,
      );

      expect(screen.getByText('Arcade Cabinet')).toBeInTheDocument();
      expect(screen.getByText('gaming')).toBeInTheDocument();
    } finally {
      connectorsList = original;
    }
  });
});
