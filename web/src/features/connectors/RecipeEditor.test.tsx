import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useState } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { RecipeEditor } from './RecipeEditor';

// The recipe editor and its YAML editor load lazily; CI with coverage is slower than the 1s default.
configure({ asyncUtilTimeout: 5000 });

if (!Range.prototype.getClientRects) {
  Object.defineProperty(Range.prototype, 'getClientRects', { value: () => [] as unknown as DOMRectList });
}

vi.mock('./RecipeBuilder', () => ({
  RecipeBuilder: ({ value, onChange, disabled }: {
    value: string;
    onChange: (value: string) => void;
    disabled?: boolean;
  }) => (
    <div>
      <input id="recipe-field-category" aria-label="Category" />
      <textarea
        aria-label="Form recipe"
        id="connector-field-recipe"
        value={value}
        readOnly={disabled}
        onChange={(event) => onChange(event.currentTarget.value)}
      />
    </div>
  ),
}));

function ControlledEditor({ initialValue, disabled = false }: { initialValue: string; disabled?: boolean }) {
  const [value, setValue] = useState(initialValue);
  return <RecipeEditor value={value} onChange={setValue} disabled={disabled} />;
}

function renderEditor(value = '', props: { disabled?: boolean } = {}) {
  return render(<ControlledEditor initialValue={value} {...props} />);
}

beforeEach(() => {
  window.localStorage.clear();
});

describe('RecipeEditor tabs', () => {
  it.each([
    ['', 'empty'],
    ['category: storage\n', 'parseable'],
  ])('opens the Form tab by default for %s text', (value) => {
    renderEditor(value);
    expect(screen.getByRole('tab', { name: /form/i })).toHaveAttribute('aria-selected', 'true');
  });

  it('opens the YAML tab by default when the text has a syntax error', () => {
    renderEditor('category: [\n');
    expect(screen.getByRole('tab', { name: /yaml/i })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: /form/i })).toBeDisabled();
    expect(screen.getByRole('status')).toHaveTextContent(/line \d+/i);
  });

  it('switches views with keyboard controls and remembers the selection', async () => {
    const view = renderEditor('category: storage\n');
    const formTab = screen.getByRole('tab', { name: /form/i });
    formTab.focus();
    fireEvent.keyDown(formTab, { key: 'ArrowRight' });
    const yamlTab = screen.getByRole('tab', { name: /yaml/i });
    expect(yamlTab).toHaveAttribute('aria-selected', 'true');
    expect(yamlTab).toHaveFocus();

    view.unmount();
    renderEditor('category: storage\n');
    expect(screen.getByRole('tab', { name: /yaml/i })).toHaveAttribute('aria-selected', 'true');
    await screen.findByRole('textbox', { name: /recipe/i });
  });

  it('keeps working when browser storage is unavailable', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage unavailable');
    });
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage unavailable');
    });
    try {
      renderEditor('category: storage\n');
      fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));
      expect(screen.getByRole('tab', { name: /yaml/i })).toHaveAttribute('aria-selected', 'true');
    } finally {
      getItem.mockRestore();
      setItem.mockRestore();
    }
  });

  it('shares the exact text edited in the Form tab with YAML', async () => {
    renderEditor('category: storage\n');
    const form = await screen.findByRole('textbox', { name: 'Form recipe' });
    const updated = 'category: network\n# keep this comment\n';
    fireEvent.change(form, { target: { value: updated } });
    fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));

    const yaml = await screen.findByRole('textbox', { name: /recipe/i });
    await waitFor(() => {
      const value = yaml instanceof HTMLTextAreaElement ? yaml.value : yaml.textContent;
      expect(value).toContain('category: network');
      expect(value).toContain('# keep this comment');
    });
  });
});

describe('RecipeEditor Form availability', () => {
  it.each([
    ['non-mapping root', '- value\n'],
    ['multiple documents', '---\ncategory: storage\n---\ncategory: network\n'],
    ['an anchor', 'defaults: &shared\n  name: value\n'],
    ['an alias', 'defaults: &shared value\ncopy: *shared\n'],
    ['a merge key', 'defaults: &shared\n  name: value\nmerged:\n  <<: *shared\n'],
  ])('keeps YAML usable when the form cannot represent %s', async (_description, value) => {
    renderEditor(value);
    expect(screen.getByRole('tab', { name: /yaml/i })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: /form/i })).toBeDisabled();
    expect(screen.getByRole('status')).toHaveTextContent(/line \d+/i);
    await screen.findByRole('textbox', { name: /recipe/i });
  });
});

describe('RecipeEditor YAML editor', () => {
  it('shows an editable textarea while the lazy YAML editor loads', () => {
    const onChange = vi.fn();
    render(<RecipeEditor value="category: storage\n" onChange={onChange} label="Recipe" />);
    fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    const fallback = screen.getByRole('textbox', { name: /recipe/i });
    expect(fallback).toBeInstanceOf(HTMLTextAreaElement);
    expect(fallback).not.toHaveAttribute('readonly');
    const updated = 'category: network\n';
    fireEvent.change(fallback, { target: { value: updated } });
    expect(onChange).toHaveBeenCalledWith(updated);
  });

  it('keeps the YAML control labelled and read-only when editing is disabled', async () => {
    renderEditor('category: storage\n', { disabled: true });
    fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    const editor = await screen.findByRole('textbox', { name: /recipe/i });
    expect(editor).toHaveAttribute('aria-label', 'Recipe (YAML)');
    if (editor instanceof HTMLTextAreaElement) expect(editor).toHaveAttribute('readonly');
    else expect(editor).toHaveAttribute('aria-readonly', 'true');
    expect(screen.getByText(/read.only/i)).toBeInTheDocument();
  });

  it('shows syntax and current server errors as line diagnostics', async () => {
    const malformed = 'category: [a,, b]\n';
    const syntaxView = render(<RecipeEditor value={malformed} onChange={vi.fn()} label="Recipe" />);
    await waitFor(() => expect(document.querySelector('.cm-lintRange-error')).toBeInTheDocument());
    syntaxView.unmount();

    const recipe = 'category: storage\nendpoints:\n  - name: items\n    entity:\n      external_id: id\n';
    render(
      <RecipeEditor
        value={recipe}
        onChange={vi.fn()}
        label="Recipe"
        errors={[{ field: 'config.recipe.endpoints[0].entity.external_id', msg: 'Unknown field' }]}
        errorRecipe={recipe}
      />,
    );
    fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    await waitFor(() => expect(document.querySelector('.cm-lintRange-error')).toBeInTheDocument());
  });

  it('marks previous server errors as stale instead of marking the changed text', async () => {
    const recipe = 'category: storage\n';
    render(
      <RecipeEditor
        value={recipe}
        onChange={vi.fn()}
        label="Recipe"
        errors={[{ field: 'config.recipe.category', msg: 'Invalid category' }]}
        errorRecipe="category: network\n"
      />,
    );
    fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    expect(screen.getByRole('status')).toHaveTextContent(/earlier recipe/i);
    await screen.findByRole('textbox', { name: /recipe/i });
    expect(document.querySelector('.cm-lintRange-error')).not.toBeInTheDocument();
  });

  it('routes focus requests to a located Form field and keeps the current tab', async () => {
    renderEditor('category: storage\n');
    await screen.findByRole('textbox', { name: 'Category' });
    const event = new CustomEvent('connector-recipe-focus', {
      detail: { field: 'config.recipe.category' },
      cancelable: true,
    });

    expect(window.dispatchEvent(event)).toBe(false);
    expect(screen.getByRole('tab', { name: /form/i })).toHaveAttribute('aria-selected', 'true');
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Category' })).toHaveFocus());
  });

  it('routes focus requests to the located YAML line and leaves YAML selected', async () => {
    const recipe = 'category: storage\nendpoints:\n  - name: items\n    entity:\n      external_id: id\n';
    renderEditor(recipe);
    fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));
    await screen.findByRole('textbox', { name: /recipe/i });
    const event = new CustomEvent('connector-recipe-focus', {
      detail: { field: 'config.recipe.endpoints[0].entity.external_id' },
      cancelable: true,
    });

    expect(window.dispatchEvent(event)).toBe(false);
    await waitFor(() => expect(screen.getByRole('textbox', { name: /recipe/i })).toHaveFocus());
    expect(screen.getByRole('tab', { name: /yaml/i })).toHaveAttribute('aria-selected', 'true');
  });

  it('keeps the textarea functional if the lazy YAML module fails to load', async () => {
    vi.doMock('./RecipeYamlEditor', () => {
      throw new Error('YAML chunk failed to load');
    });
    try {
      renderEditor('category: storage\n');
      fireEvent.click(screen.getByRole('tab', { name: /yaml/i }));
      const fallback = await screen.findByRole('textbox', { name: /recipe/i });
      await new Promise((resolve) => window.setTimeout(resolve, 0));
      const updated = 'category: network\n';
      fireEvent.change(fallback, { target: { value: updated } });
      expect(fallback).toHaveValue(updated);
    } finally {
      vi.doUnmock('./RecipeYamlEditor');
    }
  });
});
