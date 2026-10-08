import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorEditPage } from './ConnectorEditPage';

const { putConnectorsConnectorId, previewRecipe, roleState, testMock, toastError } = vi.hoisted(() => ({
  toastError: vi.fn(),
  putConnectorsConnectorId: vi.fn().mockResolvedValue({}),
  previewRecipe: vi.fn(),
  roleState: { isAdmin: false },
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

vi.mock('./RecipeYamlEditor', () => ({
  RecipeYamlEditor: ({ value, onChange, label, readOnly, focusRequest }: { value: string; onChange: (value: string) => void; label: string; readOnly: boolean; focusRequest: number }) => <textarea data-testid="recipe-yaml-editor" ref={(node) => { if (focusRequest) node?.focus(); }} id="connector-field-recipe" className="font-mono" aria-label={label} value={value} readOnly={readOnly} onChange={(event) => onChange(event.target.value)} />,
}));

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsConnectorId: () => ({ data: connectorData, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetConnectors: () => ({ data: [{ id: 'visible-traefik', name: 'Visible Traefik', type: 'traefik' }] }),
  useGetConnectorsSchema: () => ({ data: schemas }),
  putConnectorsConnectorId: (...args: unknown[]) => putConnectorsConnectorId(...args),
  postConnectorsConnectorIdTest: (...args: unknown[]) => testMock(...args),
  usePreviewConnectorRecipe: () => ({
    mutate: (...args: unknown[]) => previewRecipe(...args),
    isPending: false,
    isError: false,
  }),
  getGetConnectorsQueryKey: () => [],
}));

vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: (...args: unknown[]) => toastError(...args) } }));

vi.mock('../../hooks/useRole', () => ({ useConnectorRole: () => 'operator', useIsInstanceAdmin: () => roleState.isAdmin }));

vi.mock('./ConnectorPermissionsTab', () => ({ ConnectorPermissionsTab: () => null }));

afterEach(() => {
  localStorage.clear();
  roleState.isAdmin = false;
  previewRecipe.mockClear();
});

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

describe('ConnectorEditPage TLS probe endpoint settings', () => {
  const tlsProbeSchema = {
    type: 'tlsprobe',
    category: 'monitoring',
    displayName: 'TLS Probe',
    isCredentialRefresher: false,
    fields: [
      { name: 'targets', label: 'Targets', kind: 'textarea', required: false },
      { name: 'import_connector_id', label: 'Import hosts from Traefik connector', kind: 'text', required: false },
      { name: 'import_port', label: 'Port for imported hosts', kind: 'number', required: false, default: '443' },
    ],
  };

  it('shows endpoint settings read-only to non-admins and omits them from an unrelated save', async () => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [tlsProbeSchema];
    connectorData = {
      ...connectorData,
      type: 'tlsprobe',
      url: '',
      config: { targets: 'nas.lab:443', import_connector_id: 'traefik-1', import_port: '8443' },
    };
    roleState.isAdmin = false;
    putConnectorsConnectorId.mockClear();
    try {
      renderPage();
      expect(screen.getByLabelText('Targets')).toHaveValue('nas.lab:443');
      expect(screen.getByLabelText('Targets')).toBeDisabled();
      expect(screen.getByLabelText('Import hosts from Traefik connector')).toHaveValue('traefik-1');
      expect(screen.getByLabelText('Import hosts from Traefik connector')).toBeDisabled();
      expect(screen.getByLabelText('Port for imported hosts')).toHaveValue(8443);
      expect(screen.getByLabelText('Port for imported hosts')).toBeDisabled();
      fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'renamed probe' } });
      fireEvent.click(screen.getByRole('button', { name: /save/i }));
      await waitFor(() => expect(putConnectorsConnectorId).toHaveBeenCalled());
      const body = putConnectorsConnectorId.mock.calls[0][1] as { config: Record<string, unknown> };
      expect(body.config).toEqual({});
    } finally {
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  });

  it('offers only viewable Traefik connectors to admins', () => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [tlsProbeSchema];
    connectorData = { ...connectorData, type: 'tlsprobe', url: '', config: { targets: '', import_connector_id: 'traefik-1', import_port: '443' } };
    roleState.isAdmin = true;
    try {
      renderPage();
      const picker = screen.getByLabelText('Import hosts from Traefik connector');
      expect(picker.tagName).toBe('SELECT');
      expect(picker).toHaveValue('traefik-1');
      expect(screen.getByRole('option', { name: /traefik-1/ })).toBeDisabled();
      expect(screen.getByRole('option', { name: 'Visible Traefik' })).toHaveValue('visible-traefik');
      expect(screen.queryByRole('option', { name: 'NPM' })).not.toBeInTheDocument();
    } finally {
      schemas = originalSchemas;
      connectorData = originalConnector;
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

describe('ConnectorEditPage recipe editing', () => {
  const recipe = [
    'version: 1',
    'category: media',
    'auth:',
    '  mode: none',
    'endpoints:',
    '  - name: shows',
  ].join('\n');
  const customSchema = {
    type: 'custom',
    category: 'virtualization',
    displayName: 'Custom HTTP',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'Target URL', kind: 'text', required: true },
      { name: 'recipe', label: 'Recipe (YAML)', kind: 'textarea', required: false },
      { name: 'auth_token', label: 'Token', kind: 'password', required: false },
    ],
  };

  it('shows and preserves the stored recipe when saving without editing it', async () => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [customSchema];
    connectorData = {
      ...connectorData,
      type: 'custom',
      config: { recipe },
    };
    putConnectorsConnectorId.mockClear();
    try {
      roleState.isAdmin = true;
      renderPage();
      await openYaml();
      expect(screen.getByRole('textbox', { name: 'Recipe (YAML)' })).toHaveValue(recipe);
      expect(await screen.findByText('Media')).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: /save/i }));
      await waitFor(() => expect(putConnectorsConnectorId).toHaveBeenCalled());
      const body = putConnectorsConnectorId.mock.calls[0][1] as { category?: string; config: Record<string, unknown> };
      expect(body.config.recipe).toBe(recipe);
      expect(body).not.toHaveProperty('category');
    } finally {
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  });

  it('derives a changed category from the edited recipe and shows its located validation error', async () => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [customSchema];
    connectorData = { ...connectorData, type: 'custom', config: { recipe } };
    putConnectorsConnectorId.mockRejectedValueOnce(
      Object.assign(new Error('invalid recipe'), {
        isAxiosError: true,
        response: { status: 400, data: { details: [{ field: 'config.recipe.endpoints[0].entity.external_id', msg: 'is required' }] } },
      }),
    );
    try {
      roleState.isAdmin = true;
      renderPage();
      await openYaml();
      fireEvent.change(screen.getByRole('textbox', { name: 'Recipe (YAML)' }), { target: { value: recipe.replace('category: media', 'category: monitoring') } });
      expect(screen.getByText('Monitoring')).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: /save/i }));
      await waitFor(() => expect(screen.getByText('config.recipe.endpoints[0].entity.external_id')).toBeInTheDocument());
      await waitFor(() => expect(screen.getByRole('textbox', { name: 'Recipe (YAML)' })).toHaveFocus());
      const body = putConnectorsConnectorId.mock.calls[0][1] as { category?: string; config: Record<string, unknown> };
      expect(body.config.recipe).toContain('category: monitoring');
      expect(body).not.toHaveProperty('category');
    } finally {
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  });

  it('previews the current edited recipe with the connector id and stored URL', async () => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [customSchema];
    connectorData = {
      ...connectorData,
      type: 'custom',
      url: 'https://media.example',
      verifyTls: false,
      config: { recipe },
    };
    roleState.isAdmin = true;
    try {
      roleState.isAdmin = true;
      renderPage();
      await openYaml();
      fireEvent.change(screen.getByRole('textbox', { name: /recipe/i }), {
        target: { value: recipe.replace('name: shows', 'name: movies') },
      });
      fireEvent.change(screen.getByLabelText(/token/i), { target: { value: 'new-token' } });
      fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
      expect(previewRecipe).toHaveBeenCalledWith({
        data: {
          connectorId: 'c1',
          url: 'https://media.example',
          verifyTls: false,
          config: {
            recipe: recipe.replace('name: shows', 'name: movies'),
            auth_token: 'new-token',
          },
        },
      }, expect.anything());
    } finally {
      roleState.isAdmin = false;
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  });
});

describe('ConnectorEditPage save and preview feedback race', () => {
  const recipe = 'version: 1\ncategory: media\nauth: {mode: none}\nendpoints:\n  - name: items\n    path: /api/items\n    method: GET\n    items: items\n    entity: {kind: media_item, name: name, external_id: id}\n';
  const customSchema = {
    type: 'custom',
    category: 'virtualization',
    displayName: 'Custom HTTP',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'Target URL', kind: 'text', required: true },
      { name: 'recipe', label: 'Recipe (YAML)', kind: 'textarea', required: false },
      { name: 'auth_token', label: 'Token', kind: 'password', required: false },
    ],
  };

  it('keeps a failed save error when a preview of the same recipe finishes after the save', async () => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [customSchema];
    connectorData = { ...connectorData, type: 'custom', url: 'https://media.example', config: { recipe } };
    let finishPreview: ((result: unknown) => void) | undefined;
    previewRecipe.mockImplementationOnce((_data, callbacks: { onSuccess: (result: unknown) => void }) => {
      finishPreview = (result) => callbacks.onSuccess(result);
    });
    putConnectorsConnectorId.mockRejectedValueOnce(
      Object.assign(new Error('invalid mapping'), {
        isAxiosError: true,
        response: { status: 400, data: { details: [{ field: 'config.recipe.endpoints[0].entity.external_id', msg: 'mapping must select an ID' }] } },
      }),
    );
    try {
      roleState.isAdmin = true;
      renderPage();
      await screen.findByLabelText('External ID');
      fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
      fireEvent.click(screen.getByRole('button', { name: /save/i }));
      await waitFor(() => expect(screen.getByLabelText('External ID')).toHaveAttribute('aria-invalid', 'true'));

      act(() => finishPreview?.({ endpoints: [], dependencies: [], errors: [] }));

      expect(screen.getByLabelText('External ID')).toHaveAttribute('aria-invalid', 'true');
      expect(screen.getByLabelText('External ID')).toHaveAccessibleDescription('mapping must select an ID');
    } finally {
      roleState.isAdmin = false;
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  });
});

describe('recipe editor dirty guard and rights', () => {
  const recipe = 'version: 1\ncategory: media\nauth: {mode: none}\nendpoints:\n  - name: items\n    path: /api/items\n    method: GET\n    items: items\n    entity: {kind: media_item, name: name, external_id: id}\n';
  it.each(['Form', 'YAML'])('warns before leaving after an edit in %s', async (tab) => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [{ type: 'custom', fields: [{ name: 'recipe', label: 'Recipe (YAML)', kind: 'textarea' }] }];
    connectorData = { ...connectorData, type: 'custom', config: { recipe } };
    roleState.isAdmin = true;
    try {
      renderPage();
      fireEvent.click(await screen.findByRole('tab', { name: tab }));
      if (tab === 'YAML') await screen.findByTestId('recipe-yaml-editor');
      if (tab === 'Form') fireEvent.change(await screen.findByLabelText('Path'), { target: { value: '/new' } });
      else fireEvent.change(await screen.findByLabelText('Recipe (YAML)'), { target: { value: recipe + '# edit\n' } });
      await waitFor(() => {
        const event = new Event('beforeunload', { cancelable: true });
        window.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(true);
      });
    } finally {
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  });
  it('shows recipe values without controls to a non-admin operator', async () => {
    const originalSchemas = schemas;
    const originalConnector = connectorData;
    schemas = [{ type: 'custom', fields: [{ name: 'recipe', label: 'Recipe (YAML)', kind: 'textarea' }] }];
    connectorData = { ...connectorData, type: 'custom', config: { recipe } };
    try {
      renderPage();
      expect(await screen.findByText('/api/items')).toBeInTheDocument();
      expect(screen.queryByLabelText('Path')).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole('tab', { name: 'YAML' }));
      expect(await screen.findByLabelText('Recipe (YAML)')).toHaveAttribute('readonly');
    } finally {
      schemas = originalSchemas;
      connectorData = originalConnector;
    }
  });
});

async function openYaml() {
  fireEvent.click(await screen.findByRole('tab', { name: 'YAML' }));
  return screen.findByTestId('recipe-yaml-editor');
}
