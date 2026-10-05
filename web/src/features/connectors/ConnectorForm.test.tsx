import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorForm } from './ConnectorForm';

const { postConnectors } = vi.hoisted(() => ({ postConnectors: vi.fn().mockResolvedValue({ id: 'c1' }) }));

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
    type: 'caddy',
    category: 'networking',
    displayName: 'Caddy',
    isCredentialRefresher: false,
    fields: [
      { name: 'url', label: 'Caddy Admin API URL', kind: 'text', required: false },
      { name: 'config_json', label: 'Caddy JSON config', kind: 'secret', required: false },
    ],
  },
];

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsSchema: () => ({ data: schemas, isLoading: false, isError: false, refetch: vi.fn() }),
  postConnectors: (...args: unknown[]) => postConnectors(...args),
  getGetConnectorsQueryKey: () => [],
}));

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
