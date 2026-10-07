import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { RecipePreviewInput } from '../../api/model';
import { setLanguagePreference } from '../../i18n';
import { TestRecipePanel } from './TestRecipePanel';

const { panelState } = vi.hoisted(() => ({
  panelState: {
    isAdmin: true,
    preview: {} as Record<string, unknown>,
    mutate: vi.fn(),
  },
}));

vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => panelState.isAdmin }));

vi.mock('../../api/generated/connectors/connectors', () => ({
  usePreviewConnectorRecipe: () => ({
    ...panelState.preview,
    mutate: (
      variables: unknown,
      callbacks?: { onSuccess?: () => void; onError?: () => void },
    ) => {
      panelState.mutate(variables);
      if (panelState.preview.isError) callbacks?.onError?.();
      else callbacks?.onSuccess?.();
    },
  }),
}));

const input = {
  connectorId: 'c1',
  url: 'https://media.example',
  verifyTls: true,
  config: { recipe: 'version: 1\ncategory: media', auth_token: '' },
};

function panelTree(panelInput: RecipePreviewInput) {
  return (
    <>
      <label htmlFor="connector-field-url">URL</label>
      <input id="connector-field-url" />
      <label htmlFor="connector-field-recipe">Recipe</label>
      <textarea id="connector-field-recipe" />
      <TestRecipePanel {...panelInput} />
    </>
  );
}

function renderPanel(panelInput: RecipePreviewInput = input) {
  return render(panelTree(panelInput));
}

beforeEach(() => {
  panelState.isAdmin = true;
  panelState.preview = { isPending: false, isError: false, data: undefined, error: undefined };
  panelState.mutate.mockClear();
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('TestRecipePanel', () => {
  it('is visible only to instance admins', () => {
    panelState.isAdmin = false;
    renderPanel();
    expect(screen.queryByRole('button', { name: /test recipe/i })).not.toBeInTheDocument();
  });

  it('submits the current connector input and shows successful endpoint results', () => {
    panelState.preview.data = {
      endpoints: [{
        name: 'libraries',
        items: 2,
        count: 2,
        skipped: 0,
        samples: [{ kind: 'library', name: 'Sample Library', externalId: 'library-1', attributes: { readOnly: true } }],
        dependencies: [{ kind: 'storage', name: 'tank' }],
      }],
      dependencies: [{ kind: 'storage', name: 'tank' }],
      errors: [],
    };
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(panelState.mutate.mock.calls[0][0]).toEqual({ data: input });
    expect(screen.getByText('Preview completed successfully.')).toBeInTheDocument();
    expect(screen.getByText('Mapped entities')).toBeInTheDocument();
    expect(screen.getByText(/Sample Library/)).toBeInTheDocument();
    expect(screen.getAllByText('tank').length).toBeGreaterThan(0);
  });

  it('shows endpoint errors while retaining successful endpoint results', () => {
    panelState.preview.data = {
      endpoints: [
        { name: 'servers', items: 1, count: 1, skipped: 0, samples: [], dependencies: [] },
        { name: 'libraries', items: 0, count: 0, skipped: 0, samples: [], dependencies: [], error: 'request timed out' },
      ],
      dependencies: [],
      errors: ['libraries: request timed out'],
    };
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(screen.getByText('Preview completed with endpoint errors.')).toBeInTheDocument();
    expect(screen.getByText('Endpoint error: request timed out')).toBeInTheDocument();
    expect(screen.getByText('libraries: request timed out')).toBeInTheDocument();
  });

  it('shows validation errors with the server location', () => {
    panelState.preview = {
      isPending: false,
      isError: true,
      data: undefined,
      error: {
        response: {
          data: { details: [{ field: 'config.recipe.endpoints[0].entity.external_id', msg: 'is required' }] },
        },
      },
    };
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(screen.getByText('Validation errors')).toBeInTheDocument();
    expect(screen.getByText('config.recipe.endpoints[0].entity.external_id')).toBeInTheDocument();
    expect(screen.getByText(/is required/)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: /recipe/i })).toHaveFocus();
  });

  it('shows a located url error and focuses the url field', () => {
    panelState.preview = {
      isPending: false,
      isError: true,
      data: undefined,
      error: { response: { data: { details: [{ field: 'url', msg: 'must match the saved connector URL' }] } } },
    };
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(screen.getByText('url')).toBeInTheDocument();
    expect(screen.getByText(/must match the saved connector URL/)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'URL' })).toHaveFocus();
  });

  it('shows the stored credentials hint only when a connector id is set', () => {
    const { unmount } = renderPanel();
    expect(screen.getByText(/Saved credentials are reused only for the saved URL/)).toBeInTheDocument();
    unmount();
    renderPanel({ ...input, connectorId: undefined });
    expect(screen.queryByText(/Saved credentials are reused only for the saved URL/)).not.toBeInTheDocument();
  });

  it('disables the button and shows progress while the preview is pending', () => {
    panelState.preview = { isPending: true, isError: false, data: undefined, error: undefined };
    renderPanel();
    const button = screen.getByRole('button', { name: 'Testing recipe…' });
    expect(button).toBeDisabled();
    expect(screen.getAllByText('Testing recipe…').length).toBeGreaterThan(1);
  });

  it('shows a generic message for a request error without validation details', () => {
    panelState.preview = { isPending: false, isError: true, data: undefined, error: new Error('network down') };
    renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(screen.getByRole('alert')).toHaveTextContent('Could not test this recipe. Check the URL and try again.');
  });

  it('hides a completed result once the input changes', () => {
    panelState.preview.data = { endpoints: [], dependencies: [], errors: [] };
    const { rerender } = renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /test recipe/i }));
    expect(screen.getByText('Preview completed successfully.')).toBeInTheDocument();
    rerender(panelTree({ ...input, config: { ...input.config, recipe: 'version: 1\ncategory: other' } }));
    expect(screen.queryByText('Preview completed successfully.')).not.toBeInTheDocument();
  });

  it('renders the panel in Brazilian Portuguese', async () => {
    await setLanguagePreference('pt-BR');
    try {
      renderPanel();
      expect(screen.getByRole('heading', { name: 'Testar receita' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Testar receita' })).toBeInTheDocument();
      expect(screen.getByText('Execute a receita atual no destino sem salvar alterações.')).toBeInTheDocument();
    } finally {
      await setLanguagePreference('en');
    }
  });
});
