import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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

function renderPanel() {
  return render(
    <>
      <label htmlFor="connector-field-recipe">Recipe</label>
      <textarea id="connector-field-recipe" />
      <TestRecipePanel {...input} />
    </>,
  );
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
