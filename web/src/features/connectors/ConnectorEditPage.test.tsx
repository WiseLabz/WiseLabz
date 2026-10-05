import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorEditPage } from './ConnectorEditPage';

const { putConnectorsConnectorId, testMock, toastError } = vi.hoisted(() => ({
  toastError: vi.fn(),
  putConnectorsConnectorId: vi.fn().mockResolvedValue({}),
  testMock: vi.fn(),
}));

let connectorData: Record<string, unknown> = {
  id: 'c1',
  name: 'pve1',
  type: 'proxmox',
  url: 'https://pve1.example',
  verifyTls: true,
  enabled: true,
  status: 'online',
  secretRotatedAt: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000).toISOString(),
  userExpiresAt: '',
  rotationMaxAgeDays: null,
};

let schemas: Array<Record<string, unknown>> = [
  { type: 'proxmox', category: 'virtualization', displayName: 'Proxmox', fields: [], isCredentialRefresher: false },
];

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsConnectorId: () => ({ data: connectorData, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetConnectorsSchema: () => ({ data: schemas }),
  putConnectorsConnectorId: (...args: unknown[]) => putConnectorsConnectorId(...args),
  postConnectorsConnectorIdTest: (...args: unknown[]) => testMock(...args),
  getGetConnectorsQueryKey: () => [],
}));

vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: (...args: unknown[]) => toastError(...args) } }));

vi.mock('../../hooks/useRole', () => ({ useConnectorRole: () => 'operator' }));

vi.mock('./ConnectorPermissionsTab', () => ({ ConnectorPermissionsTab: () => null }));

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/connectors/c1/edit']}>
        <Routes>
          <Route path="/connectors/:id/edit" element={<ConnectorEditPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ConnectorEditPage rotation fields (#239 PR1)', () => {
  it('shows the "last rotated" text and rotation inputs for a non-refresher type', () => {
    renderPage();
    expect(screen.getByText(/secret last rotated 5 days ago/i)).toBeInTheDocument();
    expect(screen.getByText(/credential expires on/i)).toBeInTheDocument();
    expect(screen.getByText(/rotation reminder/i)).toBeInTheDocument();
  });

  it('hides rotation fields for a refresher connector type', () => {
    schemas = [
      { type: 'proxmox', category: 'virtualization', displayName: 'Proxmox', fields: [], isCredentialRefresher: true },
    ];
    renderPage();
    expect(screen.queryByText(/last rotated/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/credential expires on/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/rotation reminder/i)).not.toBeInTheDocument();
    schemas = [
      { type: 'proxmox', category: 'virtualization', displayName: 'Proxmox', fields: [], isCredentialRefresher: false },
    ];
  });

  it('submits userExpiresAt and rotationMaxAgeDays when set', async () => {
    renderPage();
    fireEvent.change(screen.getByLabelText(/credential expires on/i), { target: { value: '2027-01-01' } });
    fireEvent.change(screen.getByLabelText(/rotation reminder/i), { target: { value: '30' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await waitFor(() =>
      expect(putConnectorsConnectorId).toHaveBeenCalledWith(
        'c1',
        expect.objectContaining({ userExpiresAt: '2027-01-01T00:00:00Z', rotationMaxAgeDays: 30 }),
      ),
    );
  });
});

describe('ConnectorEditPage config-managed connectors (#500)', () => {
  it.each([
    ['config', /declared in config\.yaml/],
    ['config-orphaned', /removed from config\.yaml/],
  ])('replaces the form for a %s connector', (managedBy, message) => {
    const original = connectorData;
    connectorData = { ...connectorData, managedBy };
    try {
      renderPage();
      expect(screen.getByText('Managed by config')).toBeInTheDocument();
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /save/i })).not.toBeInTheDocument();
    } finally {
      connectorData = original;
    }
  });
});

describe('ConnectorEditPage verify_tls (#613)', () => {
  it('shows the stored verifyTls on the switch and sends changes top-level, not in config', async () => {
    const original = schemas;
    schemas = [
      {
        type: 'proxmox',
        category: 'virtualization',
        displayName: 'Proxmox',
        isCredentialRefresher: false,
        fields: [
          { name: 'url', label: 'API URL', kind: 'text', required: true },
          { name: 'verify_tls', label: 'Verify TLS', kind: 'toggle', required: false },
        ],
      },
    ];
    try {
      putConnectorsConnectorId.mockClear();
      renderPage();
      const tls = screen.getByRole('switch', { name: /verify tls/i });
      expect(tls).toHaveAttribute('aria-checked', 'true');
      fireEvent.click(tls);
      fireEvent.click(screen.getByRole('button', { name: /save/i }));
      await waitFor(() => expect(putConnectorsConnectorId).toHaveBeenCalled());
      const body = putConnectorsConnectorId.mock.calls[0][1] as { verifyTls: boolean; config: object };
      expect(body.verifyTls).toBe(false);
      expect(body.config).not.toHaveProperty('verify_tls');
    } finally {
      schemas = original;
    }
  });
});

describe('ConnectorEditPage Caddy input mode (pasted JSON vs url)', () => {
  const caddySchema = {
    type: 'caddy',
    category: 'networking',
    displayName: 'Caddy',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'Caddy Admin API URL', kind: 'text', required: false },
      { name: 'config_json', label: 'Caddy JSON config', kind: 'secret', required: false },
      { name: 'bearer_token', label: 'Bearer token', kind: 'password', required: false },
      { name: 'verify_tls', label: 'Verify TLS', kind: 'toggle', required: false },
    ],
  };

  async function run(stored: { url: string }, edit: () => void) {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [caddySchema];
    connectorData = { ...connectorData, type: 'caddy', name: 'caddy1', url: stored.url };
    try {
      putConnectorsConnectorId.mockClear();
      renderPage();
      edit();
      fireEvent.click(screen.getByRole('button', { name: /save/i }));
      await waitFor(() => expect(putConnectorsConnectorId).toHaveBeenCalled());
      return putConnectorsConnectorId.mock.calls[0][1] as { url?: string; config: Record<string, unknown> };
    } finally {
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  }

  it('renaming a pasted-mode connector sends no config_json so the stored value is kept', async () => {
    const body = await run({ url: '' }, () =>
      fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'renamed' } }),
    );
    expect(body.url).toBe('');
    expect(body.config).not.toHaveProperty('config_json');
  });

  it('switching from url mode to pasted mode sends the json and an empty url', async () => {
    const body = await run({ url: 'http://caddy.example.com:2019' }, () => {
      fireEvent.change(screen.getByLabelText(/caddy admin api url/i), { target: { value: '' } });
      fireEvent.change(screen.getByLabelText(/caddy json config/i), { target: { value: '{"apps":{}}' } });
    });
    expect(body.url).toBe('');
    expect(body.config.config_json).toBe('{"apps":{}}');
  });

  it('switching from pasted mode back to url mode clears the stored json', async () => {
    const body = await run({ url: '' }, () =>
      fireEvent.change(screen.getByLabelText(/caddy admin api url/i), {
        target: { value: 'http://caddy.example.com:2019' },
      }),
    );
    expect(body.url).toBe('http://caddy.example.com:2019');
    expect(body.config.config_json).toBe('');
  });

  it('does not clear the stored json when editing a url-mode connector', async () => {
    const body = await run({ url: 'http://caddy.example.com:2019' }, () =>
      fireEvent.change(screen.getByLabelText(/caddy admin api url/i), {
        target: { value: 'http://caddy2.example.com:2019' },
      }),
    );
    expect(body.config).not.toHaveProperty('config_json');
  });

  it('shows the server field message when the save is rejected', async () => {
    const originalSchemas = schemas;
    schemas = [caddySchema];
    putConnectorsConnectorId.mockRejectedValueOnce(
      Object.assign(new Error('bad'), {
        isAxiosError: true,
        response: { status: 400, data: { details: [{ field: 'url', msg: 'set either url or config_json' }] } },
      }),
    );
    try {
      renderPage();
      fireEvent.click(screen.getByRole('button', { name: /save/i }));
      await waitFor(() => expect(toastError).toHaveBeenCalledWith(expect.stringContaining('set either url or config_json')));
    } finally {
      schemas = originalSchemas;
    }
  });
});
