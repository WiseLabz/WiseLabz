import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ConnectorForm } from './ConnectorForm';
import { ConnectorEditPage } from './ConnectorEditPage';

const { post, put, elevated } = vi.hoisted(() => ({
  post: vi.fn(),
  put: vi.fn(),
  elevated: vi.fn(),
}));
const recipe =
  'version: 1\ncategory: other\nauth: {mode: none}\nendpoints: []\nactions:\n  rescan: {method: POST, path: /rescan}';
const connector = {
  id: 'c1',
  name: 'Recipe service',
  type: 'custom',
  category: 'other',
  enabled: true,
  status: 'online',
  url: 'https://api.example',
  config: { recipe },
};
const schema = {
  type: 'custom',
  category: 'other',
  displayName: 'Custom HTTP',
  fields: [
    { name: 'url', label: 'Endpoint URL', kind: 'text', required: true },
    { name: 'recipe', label: 'Recipe (YAML)', kind: 'textarea' },
    { name: 'auth_token', label: 'API token', kind: 'secret' },
  ],
};

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsSchema: () => ({ data: [schema], isLoading: false }),
  useGetConnectors: () => ({ data: [connector] }),
  useGetConnectorsConnectorId: () => ({ data: connector, isLoading: false }),
  postConnectors: (...args: unknown[]) => post(...args),
  putConnectorsConnectorId: (...args: unknown[]) => put(...args),
  postConnectorsConnectorIdTest: vi.fn(),
  getGetConnectorsQueryKey: () => [],
}));
vi.mock('../../hooks/useRole', () => ({
  useIsInstanceAdmin: () => true,
  useConnectorRole: () => 'operator',
}));
vi.mock('./RecipeYamlEditor', () => ({
  RecipeYamlEditor: ({ value, onChange, label, readOnly }: { value: string; onChange: (value: string) => void; label: string; readOnly: boolean }) => (
    <textarea data-testid="recipe-yaml-editor" aria-label={label} value={value} readOnly={readOnly} onChange={(event) => onChange(event.target.value)} />
  ),
}));
vi.mock('./TestRecipePanel', () => ({ TestRecipePanel: () => null }));
vi.mock('./ConnectorPermissionsTab', () => ({ ConnectorPermissionsTab: () => null }));
vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('../../components/ui/Dialog', () => ({
  Dialog: ({
    open,
    onClose,
    children,
  }: {
    open: boolean;
    onClose: () => void;
    children: React.ReactNode;
  }) =>
    open ? (
      <div role="dialog">
        <button onClick={onClose}>Cancel step-up</button>
        {children}
      </div>
    ) : null,
}));
vi.mock('../../components/manager/StepUp', () => ({
  StepUp: ({
    action,
    target,
    onElevated,
  }: {
    action: string;
    target?: string;
    onElevated: (token: string) => void;
  }) => (
    <button
      onClick={() => {
        elevated(action, target);
        onElevated('elevated-token');
      }}
    >
      Complete step-up
    </button>
  ),
}));

function renderFlow(edit: boolean, onCreated = vi.fn()) {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter initialEntries={['/connectors/c1/edit']}>
        {edit ? (
          <Routes>
            <Route path="/connectors/:id/edit" element={<ConnectorEditPage />} />
            <Route path="/services/:id" element={<p>Saved connector</p>} />
          </Routes>
        ) : (
          <ConnectorForm
            initialType="custom"
            initialValues={{ name: connector.name, url: connector.url, recipe }}
            onCreated={onCreated}
          />
        )}
      </MemoryRouter>
    </QueryClientProvider>
  );
}

async function openYaml() {
  fireEvent.click(await screen.findByRole('tab', { name: 'YAML' }));
  return screen.findByTestId('recipe-yaml-editor');
}

beforeEach(() => {
  window.localStorage.clear();
  vi.clearAllMocks();
  post.mockReset();
  put.mockReset();
  const required = {
    isAxiosError: true,
    response: { status: 400, data: { code: 'elevation_required' } },
  };
  post.mockRejectedValueOnce(required).mockResolvedValue(connector);
  put.mockRejectedValueOnce(required).mockResolvedValue(connector);
});

describe('recipe action save retry', () => {
  it.each([false, true])(
    'asks for scoped elevation and repeats the exact %s save',
    async (edit) => {
      const onCreated = vi.fn();
      renderFlow(edit, onCreated);
      fireEvent.click(
        screen.getByRole('button', { name: edit ? /^save$/i : /test & add connector/i })
      );
      const complete = await screen.findByRole('button', { name: 'Complete step-up' });
      const api = edit ? put : post;
      expect(api).toHaveBeenCalledTimes(1);
      const before = api.mock.calls[0];
      // Edits made while confirming must not change the request being approved.
      fireEvent.change(screen.getByLabelText(/^display name/i), { target: { value: 'Another name' } });
      fireEvent.click(complete);
      await waitFor(() => expect(api).toHaveBeenCalledTimes(2));
      expect(elevated).toHaveBeenCalledWith('connector.recipeActions', edit ? 'c1' : undefined);
      const after = api.mock.calls[1];
      expect(edit ? after.slice(0, 2) : after.slice(0, 1)).toEqual(before);
      expect(after[after.length - 1]).toEqual({
        headers: { 'X-Elevation-Token': 'elevated-token' },
      });
      if (edit) await screen.findByText('Saved connector');
      else await waitFor(() => expect(onCreated).toHaveBeenCalledWith(connector));
    }
  );

  it.each([false, true])(
    'retains name, recipe and typed secret when step-up is cancelled (%s)',
    async (edit) => {
      const onCreated = vi.fn();
      renderFlow(edit, onCreated);
      fireEvent.change(screen.getByLabelText(/^display name/i), { target: { value: 'Unsaved name' } });
      fireEvent.change(await openYaml(), {
        target: { value: recipe + '\n# keep edits' },
      });
      fireEvent.change(screen.getByLabelText('API token'), { target: { value: 'typed-secret' } });
      const save = screen.getByRole('button', { name: edit ? /^save$/i : /test & add connector/i });
      fireEvent.click(save);
      fireEvent.click(await screen.findByRole('button', { name: 'Cancel step-up' }));
      expect(save).not.toBeDisabled();
      expect(screen.getByLabelText(/^display name/i)).toHaveValue('Unsaved name');
      expect(await screen.findByLabelText('Recipe (YAML)')).toHaveValue(recipe + '\n# keep edits');
      expect(screen.getByLabelText('API token')).toHaveValue('typed-secret');
      expect(edit ? put : post).toHaveBeenCalledTimes(1);
      expect(onCreated).not.toHaveBeenCalled();
    }
  );

  it.each([false, true])(
    'retains edits and stops retrying after an elevated save fails (%s)',
    async (edit) => {
      const api = edit ? put : post;
      api.mockReset();
      const required = {
        isAxiosError: true,
        response: { status: 400, data: { code: 'elevation_required' } },
      };
      api.mockRejectedValueOnce(required).mockRejectedValueOnce(required);
      const onCreated = vi.fn();
      renderFlow(edit, onCreated);
      fireEvent.change(screen.getByLabelText('API token'), { target: { value: 'typed-secret' } });
      const save = screen.getByRole('button', { name: edit ? /^save$/i : /test & add connector/i });
      fireEvent.click(save);
      fireEvent.click(await screen.findByRole('button', { name: 'Complete step-up' }));
      await waitFor(() => expect(api).toHaveBeenCalledTimes(2));
      await waitFor(() => expect(save).not.toBeDisabled());
      expect(screen.queryByRole('button', { name: 'Complete step-up' })).not.toBeInTheDocument();
      expect(await openYaml()).toHaveValue(recipe);
      expect(screen.getByLabelText('API token')).toHaveValue('typed-secret');
      expect(onCreated).not.toHaveBeenCalled();
    }
  );

  describe('an action edited in the Form tab', () => {
    const editedPath = '/items/{external_id}/rescan';
    const editedRecipe = recipe.replace('path: /rescan', `path: "${editedPath}"`);
    const bodyOf = (call: unknown[], edit: boolean) =>
      (edit ? call[1] : call[0]) as { config: { recipe: string } };

    it.each([false, true])('sends the Form edit as YAML and keeps it in both tabs when step-up is cancelled (%s)', async (edit) => {
      renderFlow(edit);
      fireEvent.change(await screen.findByLabelText('Action path'), { target: { value: editedPath } });
      const save = screen.getByRole('button', { name: edit ? /^save$/i : /test & add connector/i });
      fireEvent.click(save);
      fireEvent.click(await screen.findByRole('button', { name: 'Cancel step-up' }));

      const api = edit ? put : post;
      expect(api).toHaveBeenCalledTimes(1);
      expect(bodyOf(api.mock.calls[0], edit).config.recipe).toBe(editedRecipe);
      expect(save).not.toBeDisabled();
      expect(screen.getByLabelText('Action path')).toHaveValue(editedPath);
      expect(await openYaml()).toHaveValue(editedRecipe);
      fireEvent.click(screen.getByRole('tab', { name: 'Form' }));
      expect(await screen.findByLabelText('Action path')).toHaveValue(editedPath);
    });

    it.each([false, true])('keeps the Form edit in both tabs when the elevated save fails (%s)', async (edit) => {
      const api = edit ? put : post;
      api.mockReset();
      const required = { isAxiosError: true, response: { status: 400, data: { code: 'elevation_required' } } };
      api.mockRejectedValueOnce(required).mockRejectedValueOnce({
        isAxiosError: true,
        response: { status: 400, data: { details: [{ field: 'config.recipe.actions.rescan.path', msg: 'must be a relative path' }] } },
      });
      renderFlow(edit);
      fireEvent.change(await screen.findByLabelText('Action path'), { target: { value: editedPath } });
      const save = screen.getByRole('button', { name: edit ? /^save$/i : /test & add connector/i });
      fireEvent.click(save);
      fireEvent.click(await screen.findByRole('button', { name: 'Complete step-up' }));
      await waitFor(() => expect(api).toHaveBeenCalledTimes(2));
      await waitFor(() => expect(save).not.toBeDisabled());

      expect(bodyOf(api.mock.calls[1], edit).config.recipe).toBe(editedRecipe);
      const field = await screen.findByLabelText('Action path');
      expect(field).toHaveValue(editedPath);
      expect(field).toHaveAttribute('aria-invalid', 'true');
      expect(await openYaml()).toHaveValue(editedRecipe);
    });
  });
});
