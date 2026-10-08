import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorForm } from './ConnectorForm';
import { ConnectorEditPage } from './ConnectorEditPage';

const { post, put, elevated } = vi.hoisted(() => ({ post: vi.fn(), put: vi.fn(), elevated: vi.fn() }));
const recipe = 'version: 1\ncategory: other\nauth: {mode: none}\nendpoints: []\nactions:\n  rescan: {method: POST, path: /rescan}';
const connector = { id: 'c1', name: 'Recipe service', type: 'custom', category: 'other', enabled: true, status: 'online', url: 'https://api.example', config: { recipe } };
const schema = { type: 'custom', category: 'other', displayName: 'Custom HTTP', fields: [{ name: 'url', label: 'Endpoint URL', kind: 'text', required: true }, { name: 'recipe', label: 'Recipe (YAML)', kind: 'textarea' }] };

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsSchema: () => ({ data: [schema], isLoading: false }),
  useGetConnectors: () => ({ data: [connector] }),
  useGetConnectorsConnectorId: () => ({ data: connector, isLoading: false }),
  postConnectors: (...args: unknown[]) => post(...args),
  putConnectorsConnectorId: (...args: unknown[]) => put(...args),
  postConnectorsConnectorIdTest: vi.fn(),
  getGetConnectorsQueryKey: () => [],
}));
vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => true, useConnectorRole: () => 'operator' }));
vi.mock('./TestRecipePanel', () => ({ TestRecipePanel: () => null }));
vi.mock('./ConnectorPermissionsTab', () => ({ ConnectorPermissionsTab: () => null }));
vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('../../components/manager/StepUp', () => ({
  StepUp: ({ action, target, onElevated }: { action: string; target?: string; onElevated: (token: string) => void }) => (
    <button onClick={() => { elevated(action, target); onElevated('elevated-token'); }}>Complete step-up</button>
  ),
}));

function renderFlow(edit: boolean, onCreated = vi.fn()) {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter initialEntries={['/connectors/c1/edit']}>
        {edit ? <Routes><Route path="/connectors/:id/edit" element={<ConnectorEditPage />} /><Route path="/services/:id" element={<p>Saved connector</p>} /></Routes> : <ConnectorForm initialType="custom" initialValues={{ name: connector.name, url: connector.url, recipe }} onCreated={onCreated} />}
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  post.mockReset();
  put.mockReset();
  const required = { isAxiosError: true, response: { status: 400, data: { code: 'elevation_required' } } };
  post.mockRejectedValueOnce(required).mockResolvedValue(connector);
  put.mockRejectedValueOnce(required).mockResolvedValue(connector);
});

describe('recipe action save retry', () => {
  it.each([false, true])('asks for scoped elevation and repeats the exact %s save', async (edit) => {
    const onCreated = vi.fn();
    renderFlow(edit, onCreated);
    fireEvent.click(screen.getByRole('button', { name: edit ? /^save$/i : /test & add connector/i }));
    const complete = await screen.findByRole('button', { name: 'Complete step-up' });
    const api = edit ? put : post;
    expect(api).toHaveBeenCalledTimes(1);
    const before = api.mock.calls[0];
    // Edits made while confirming must not change the request being approved.
    fireEvent.change(screen.getByLabelText(/name/i), { target: { value: 'Another name' } });
    fireEvent.click(complete);
    await waitFor(() => expect(api).toHaveBeenCalledTimes(2));
    expect(elevated).toHaveBeenCalledWith('connector.recipeActions', edit ? 'c1' : undefined);
    const after = api.mock.calls[1];
    expect(edit ? after.slice(0, 2) : after.slice(0, 1)).toEqual(before);
    expect(after[after.length - 1]).toEqual({ headers: { 'X-Elevation-Token': 'elevated-token' } });
    if (edit) await screen.findByText('Saved connector');
    else await waitFor(() => expect(onCreated).toHaveBeenCalledWith(connector));
  });
});
