import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorForm } from './ConnectorForm';

const { postConnectors, previewRecipe, roleState } = vi.hoisted(() => ({
  postConnectors: vi.fn().mockResolvedValue({ id: 'c1' }),
  previewRecipe: vi.fn(),
  roleState: { isAdmin: false },
}));

// Field names and kinds exactly as the backend emits them (see proxmox.go).
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
  {
    type: 'caddy',
    category: 'networking',
    displayName: 'Caddy',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'Caddy Admin API URL', kind: 'text', required: false },
      { name: 'config_json', label: 'Caddy JSON config', kind: 'secret', required: false },
    ],
  },
  {
    type: 'custom',
    category: 'virtualization',
    displayName: 'Custom HTTP',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'Endpoint URL', kind: 'text', required: true },
      { name: 'recipe', label: 'Recipe (YAML)', kind: 'textarea', required: false },
      { name: 'auth_token', label: 'Token', kind: 'password', required: false },
    ],
  },
  {
    type: 'traefik',
    category: 'networking',
    displayName: 'Traefik Proxy',
    isCredentialRefresher: false,
    fields: [
      { name: 'auth_mode', label: 'Auth mode', kind: 'select', required: false, options: ['none', 'basic', 'token'] },
    ],
  },
  {
    type: 'methodless',
    category: 'networking',
    displayName: 'Methodless HTTP',
    isCredentialRefresher: false,
    fields: [
      { name: 'method', label: 'Method', kind: 'select', required: false },
    ],
  },
];

vi.mock('./RecipeYamlEditor', () => ({
  RecipeYamlEditor: ({ value, onChange, label, readOnly, focusRequest }: { value: string; onChange: (value: string) => void; label: string; readOnly: boolean; focusRequest: number }) => <textarea data-testid="recipe-yaml-editor" ref={(node) => { if (focusRequest) node?.focus(); }} id="connector-field-recipe" className="font-mono" aria-label={label} value={value} readOnly={readOnly} onChange={(event) => onChange(event.target.value)} />,
}));

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsSchema: () => ({ data: schemas, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetConnectors: () => ({ data: [{ id: 'traefik-visible', name: 'Visible Traefik', type: 'traefik' }, { id: 'npm', name: 'NPM', type: 'npm' }] }),
  postConnectors: (...args: unknown[]) => postConnectors(...args),
  usePreviewConnectorRecipe: () => ({
    mutate: (...args: unknown[]) => previewRecipe(...args),
    isPending: false,
    isError: false,
  }),
  getGetConnectorsQueryKey: () => [],
}));

vi.mock('../compliance/CertificateExpiryPackOffer', () => ({
  CertificateExpiryPackOffer: ({ requested }: { requested: boolean }) => {
    return requested ? <div data-testid="certificate-expiry-pack-offer" /> : null;
  },
}));

vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => roleState.isAdmin }));

afterEach(() => {
  localStorage.clear();
  roleState.isAdmin = false;
  previewRecipe.mockClear();
});

async function fillAndSubmit(toggleTls: boolean) {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ConnectorForm onCreated={vi.fn()} />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByText('Proxmox VE'));
  fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'pve1' } });
  fireEvent.change(screen.getByLabelText(/api url/i), { target: { value: 'https://pve:8006' } });
  fireEvent.change(screen.getByLabelText(/api token secret/i), { target: { value: 's3cret' } });
  const tls = screen.getByRole('switch', { name: /verify tls/i });
  expect(tls).toHaveAttribute('aria-checked', 'true');
  if (toggleTls) fireEvent.click(tls);
  fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
  await waitFor(() => expect(postConnectors).toHaveBeenCalled());
  return postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
}

describe('ConnectorForm verify_tls (#613)', () => {
  it('renders the toggle as a switch that defaults on and sends verifyTls true', async () => {
    const body = await fillAndSubmit(false);
    expect(body.verifyTls).toBe(true);
    expect(body.config).not.toHaveProperty('verify_tls');
    expect(body.config).not.toHaveProperty('url');
  });

  it('sends verifyTls false when switched off, outside config', async () => {
    const body = await fillAndSubmit(true);
    expect(body.verifyTls).toBe(false);
    expect(body.config).toEqual({ token_secret: 's3cret' });
  });
});

describe('ConnectorForm TLS probe', () => {
  it('saves targets, the selected viewable Traefik connector and import port without a URL', async () => {
    roleState.isAdmin = true;
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('TLS Probe'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'tls-check' } });
    fireEvent.change(screen.getByLabelText('Targets'), { target: { value: 'nas.lab:443\nrouter.lab:8443' } });
    fireEvent.change(screen.getByLabelText('Import hosts from Traefik connector'), { target: { value: 'traefik-visible' } });
    fireEvent.change(screen.getByLabelText('Port for imported hosts'), { target: { value: '8443' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));

    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    const body = postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
    expect(body).not.toHaveProperty('url');
    expect(body.config).toEqual({ targets: 'nas.lab:443\nrouter.lab:8443', import_connector_id: 'traefik-visible', import_port: '8443' });
  });

  it('requests the certificate pack offer after creating a TLS probe', async () => {
    roleState.isAdmin = true;
    postConnectors.mockResolvedValueOnce({ id: 'tls1', type: 'tlsprobe' });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('TLS Probe'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'tls-check' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));

    expect(await screen.findByTestId('certificate-expiry-pack-offer')).toBeInTheDocument();
  });

  it('shows server field errors on the target list', async () => {
    roleState.isAdmin = true;
    postConnectors.mockRejectedValueOnce(Object.assign(new Error('invalid target'), {
      response: { data: { details: [{ field: 'config.targets', msg: 'must include a valid port' }] } },
    }));
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('TLS Probe'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'tls-check' } });
    fireEvent.change(screen.getByLabelText('Targets'), { target: { value: 'bad-target' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));

    expect(await screen.findByText('must include a valid port')).toBeInTheDocument();
    expect(screen.getByLabelText('Targets')).toHaveFocus();
  });

  it('keeps TLS probe endpoint settings disabled for non-admin connector creation', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('TLS Probe'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'tls-check' } });
    expect(screen.getByLabelText('Targets')).toBeDisabled();
    expect(screen.getByLabelText('Import hosts from Traefik connector')).toBeDisabled();
    expect(screen.getByLabelText('Port for imported hosts')).toBeDisabled();
    expect(screen.getAllByText(/only an instance administrator can change/i)).toHaveLength(3);
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));

    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    const body = postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
    expect(body.config).toEqual({});
  });
});

describe('ConnectorForm select fields', () => {
  it('renders string options, lets the user choose one and sends it in the body', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Traefik Proxy'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'edge' } });
    const select = screen.getByLabelText('Auth mode');
    expect(select.tagName).toBe('SELECT');
    expect(Array.from((select as HTMLSelectElement).options).map((o) => o.value)).toEqual(['', 'none', 'basic', 'token']);
    fireEvent.change(select, { target: { value: 'basic' } });
    postConnectors.mockClear();
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    const body = postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
    expect(body.config).toEqual({ auth_mode: 'basic' });
  });

  it('renders a select field without options as a text input', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Methodless HTTP'));
    expect(screen.getByLabelText('Method').tagName).toBe('INPUT');
  });
});

describe('ConnectorForm optional url (pasted-JSON mode)', () => {
  it('omits url from the request when the type does not require it and it is empty', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Caddy'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'caddy1' } });
    fireEvent.change(screen.getByLabelText(/caddy json config/i), { target: { value: '{"apps":{}}' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    const body = postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
    expect(body).not.toHaveProperty('url');
    expect(body.config).toEqual({ config_json: '{"apps":{}}' });
  });
});

describe('ConnectorForm Caddy mode switch', () => {
  it('sends no url after the url is typed and cleared for pasted mode', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Caddy'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'caddy2' } });
    fireEvent.change(screen.getByLabelText(/caddy admin api url/i), { target: { value: 'http://caddy:2019' } });
    fireEvent.change(screen.getByLabelText(/caddy admin api url/i), { target: { value: '' } });
    fireEvent.change(screen.getByLabelText(/caddy json config/i), { target: { value: '{"apps":{}}' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    const body = postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
    expect(body).not.toHaveProperty('url');
    expect(body.config).toEqual({ config_json: '{"apps":{}}' });
  });
});

describe('ConnectorForm derived category', () => {
  async function submitCustom(recipe: string) {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Custom HTTP'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'library' } });
    fireEvent.change(screen.getByLabelText(/endpoint url/i), { target: { value: 'https://library.example' } });
    await openYaml();
    if (recipe) fireEvent.change(screen.getByRole('textbox', { name: 'Recipe (YAML)' }), { target: { value: recipe } });
    postConnectors.mockClear();
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    return postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
  }

  it('omits category when a recipe is typed so the server derives it', async () => {
    const body = await submitCustom('version: 1\ncategory: media\n');
    expect(body).not.toHaveProperty('category');
    expect(body.config).toEqual({ recipe: 'version: 1\ncategory: media\n', auth_token: '' });
  });

  it('sends the schema category without a recipe', async () => {
    const body = await submitCustom('');
    expect(body.category).toBe('virtualization');
  });

  it('shows the category from the recipe as read-only and follows edits', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Custom HTTP'));
    await openYaml();
    const recipe = await screen.findByRole('textbox', { name: 'Recipe (YAML)' });
    expect(recipe).toHaveClass('font-mono');
    fireEvent.change(recipe, { target: { value: '---\nversion: 1\n"category": \'media\' # from recipe\n' } });
    expect(await screen.findByText('Media')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: /category/i })).not.toBeInTheDocument();
    fireEvent.change(recipe, { target: { value: '{version: 1, category: monitoring, auth: {mode: none}}' } });
    expect(screen.getByText('Monitoring')).toBeInTheDocument();
  });

  it('shows located recipe validation errors returned when saving', async () => {
    postConnectors.mockRejectedValueOnce(
      Object.assign(new Error('invalid recipe'), {
        response: {
          data: {
            details: [{ field: 'config.recipe.endpoints[0].entity.name', msg: 'is required' }],
          },
        },
      }),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Custom HTTP'));
    fireEvent.change(screen.getByLabelText(/display name/i), { target: { value: 'library' } });
    fireEvent.change(screen.getByLabelText(/endpoint url/i), { target: { value: 'https://library.example' } });
    await openYaml();
    fireEvent.change(screen.getByRole('textbox', { name: 'Recipe (YAML)' }), { target: { value: 'version: 1\ncategory: media\n' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(screen.getByText('config.recipe.endpoints[0].entity.name')).toBeInTheDocument());
    expect(screen.getAllByText(/is required/).length).toBeGreaterThan(0);
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Recipe (YAML)' })).toHaveFocus());
  });

  it('previews the current unsaved form values', async () => {
    roleState.isAdmin = true;
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Custom HTTP'));
    fireEvent.change(screen.getByLabelText(/endpoint url/i), { target: { value: 'https://media.example' } });
    await openYaml();
    fireEvent.change(screen.getByRole('textbox', { name: /recipe/i }), { target: { value: 'version: 1\ncategory: media' } });
    fireEvent.change(screen.getByLabelText(/token/i), { target: { value: 'current-token' } });
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(previewRecipe.mock.calls[0][0]).toEqual({
      data: {
        url: 'https://media.example',
        verifyTls: true,
        config: { recipe: 'version: 1\ncategory: media', auth_token: 'current-token' },
      },
    });
    roleState.isAdmin = false;
  });
});

describe('ConnectorForm initial type and values', () => {
  it('opens with the type chosen and the address prefilled, and saves it', async () => {
    postConnectors.mockClear();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm
          onCreated={vi.fn()}
          initialType="proxmox"
          initialValues={{ url: 'https://10.0.0.5:8006/api2/json', name: 'Proxmox VE (10.0.0.5)' }}
        />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('button', { name: /proxmox ve/i })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByLabelText(/api url/i)).toHaveValue('https://10.0.0.5:8006/api2/json');
    expect(screen.getByLabelText(/display name/i)).toHaveValue('Proxmox VE (10.0.0.5)');
    // The type's own defaults still apply to fields that were not prefilled.
    expect(screen.getByRole('switch', { name: /verify tls/i })).toHaveAttribute('aria-checked', 'true');

    fireEvent.change(screen.getByLabelText(/api token secret/i), { target: { value: 's3cret' } });
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    const body = postConnectors.mock.calls[postConnectors.mock.calls.length - 1][0] as Record<string, unknown>;
    expect(body).toMatchObject({
      name: 'Proxmox VE (10.0.0.5)',
      type: 'proxmox',
      url: 'https://10.0.0.5:8006/api2/json',
      verifyTls: true,
    });
    expect(body.config).toEqual({ token_secret: 's3cret' });
  });

  it('opens on the type picker with nothing chosen when no initial type is given', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('button', { name: /proxmox ve/i })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.queryByLabelText(/api url/i)).toBeNull();
  });

  it('lets the user change the type after opening with one, dropping the prefilled values', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConnectorForm onCreated={vi.fn()} initialType="proxmox" initialValues={{ url: 'https://10.0.0.5:8006/api2/json' }} />
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText('Custom HTTP'));
    expect(screen.getByLabelText(/endpoint url/i)).toHaveValue('');
  });
});

describe('recipe builder integration', () => {
  const recipe = 'version: 1\ncategory: media\nauth: {mode: none}\nendpoints:\n  - name: items\n    path: /api/items\n    method: GET\n    items: items\n    entity: {kind: media_item, name: name, external_id: id}\n';

  it('previews and saves the text edited in the Form view', async () => {
    roleState.isAdmin = true;
    postConnectors.mockClear();
    render(<QueryClientProvider client={new QueryClient()}><ConnectorForm onCreated={vi.fn()} initialType="custom" initialValues={{ name: 'library', url: 'https://media.example', recipe }} /></QueryClientProvider>);
    const path = await screen.findByLabelText('Path');
    fireEvent.change(path, { target: { value: '/api/new-items' } });
    const expected = recipe.replace('/api/items', '/api/new-items');
    previewRecipe.mockImplementationOnce((_variables, callbacks) => callbacks.onSuccess({ endpoints: [{ name: 'items', items: 0, count: 0, skipped: 0, samples: [], dependencies: [], error: 'request failed' }], dependencies: [], errors: [] }));
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(previewRecipe.mock.calls[0][0].data.config.recipe).toBe(expected);
    expect(screen.getByText('Failed')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(postConnectors).toHaveBeenCalled());
    expect(postConnectors.mock.calls[0][0].config.recipe).toBe(expected);
  });

  it('shows and focuses a failed save on its nested Form field, then marks errors stale on edit', async () => {
    roleState.isAdmin = true;
    postConnectors.mockRejectedValueOnce(Object.assign(new Error('invalid mapping'), { response: { data: { details: [{ field: 'config.recipe.endpoints[0].entity.external_id', msg: 'mapping must select an ID' }] } } }));
    render(<QueryClientProvider client={new QueryClient()}><ConnectorForm onCreated={vi.fn()} initialType="custom" initialValues={{ name: 'library', url: 'https://media.example', recipe }} /></QueryClientProvider>);
    const externalId = await screen.findByLabelText('External ID');
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(externalId).toHaveAttribute('aria-invalid', 'true'));
    expect(externalId).toHaveAccessibleDescription('mapping must select an ID');
    await waitFor(() => expect(externalId).toHaveFocus());
    fireEvent.change(externalId, { target: { value: 'identifier' } });
    expect(externalId).not.toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByText(/these errors refer to an earlier recipe/i)).toBeInTheDocument();
  });

  function renderMappingRecipe() {
    render(<QueryClientProvider client={new QueryClient()}><ConnectorForm onCreated={vi.fn()} initialType="custom" initialValues={{ name: 'library', url: 'https://media.example', recipe }} /></QueryClientProvider>);
  }

  function failSaveOnExternalId() {
    postConnectors.mockRejectedValueOnce(Object.assign(new Error('invalid mapping'), { response: { data: { details: [{ field: 'config.recipe.endpoints[0].entity.external_id', msg: 'mapping must select an ID' }] } } }));
  }

  it('keeps a failed save error when a preview of the same recipe finishes after the save', async () => {
    roleState.isAdmin = true;
    let finishPreview: ((result: unknown) => void) | undefined;
    previewRecipe.mockImplementationOnce((_variables, callbacks) => {
      finishPreview = (result) => callbacks.onSuccess(result);
    });
    failSaveOnExternalId();
    renderMappingRecipe();
    await screen.findByLabelText('External ID');
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(screen.getByLabelText('External ID')).toHaveAttribute('aria-invalid', 'true'));

    act(() => finishPreview?.({ endpoints: [], dependencies: [], errors: [] }));

    expect(screen.getByLabelText('External ID')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByLabelText('External ID')).toHaveAccessibleDescription('mapping must select an ID');
  });

  it('keeps a failed save error when a preview of the same recipe succeeds after the save failed', async () => {
    roleState.isAdmin = true;
    previewRecipe.mockImplementationOnce((_variables, callbacks) => callbacks.onSuccess({ endpoints: [], dependencies: [], errors: [] }));
    failSaveOnExternalId();
    renderMappingRecipe();
    await screen.findByLabelText('External ID');
    fireEvent.click(screen.getByRole('button', { name: /test & add/i }));
    await waitFor(() => expect(screen.getByLabelText('External ID')).toHaveAttribute('aria-invalid', 'true'));

    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));

    expect(screen.getByLabelText('External ID')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByLabelText('External ID')).toHaveAccessibleDescription('mapping must select an ID');
  });
});

async function openYaml() {
  fireEvent.click(await screen.findByRole('tab', { name: 'YAML' }));
  return screen.findByTestId('recipe-yaml-editor');
}
