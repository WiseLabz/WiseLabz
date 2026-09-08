import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { useUi } from '../../store/ui';
import { CommandPalette } from './CommandPalette';

vi.mock('../../api/generated/connectors/connectors', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/connectors/connectors')>()),
  useGetConnectors: () => ({ data: [] }),
}));

vi.mock('../../api/generated/docs/docs', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/docs/docs')>()),
  useGetDocsTree: () => ({ data: { children: [] } }),
}));

describe('CommandPalette', () => {
  beforeEach(() => {
    HTMLElement.prototype.scrollIntoView = vi.fn();
    useUi.setState({ paletteOpen: true });
  });
  afterEach(() => useUi.setState({ paletteOpen: false }));

  it('ignores navigation and Enter when a search has no matches', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
        </MemoryRouter>
      </QueryClientProvider>
    );

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'no matching command' } });
    const dialog = screen.getByRole('dialog');

    expect(() => {
      fireEvent.keyDown(dialog, { key: 'ArrowDown' });
      fireEvent.keyDown(dialog, { key: 'ArrowUp' });
      fireEvent.keyDown(dialog, { key: 'Enter' });
    }).not.toThrow();
    expect(screen.getByText('No matches for “no matching command”')).toBeInTheDocument();
    expect(useUi.getState().paletteOpen).toBe(true);
  });
});
