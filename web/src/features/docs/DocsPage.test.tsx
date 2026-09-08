import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import '../../i18n';
import { docTree, docs } from '../../data/fixtures';
import { DocsPage } from './DocsPage';

vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsTree: () => ({ data: docTree, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetDocsDocId: (docID: string) => ({
    data: docs[docID],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));

afterEach(cleanup);

function renderDocs() {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter initialEntries={['/docs/doc-lab']}>
        <Routes>
          <Route path="/docs/:docId" element={<DocsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('DocsPage mobile drawer', () => {
  it('moves focus into the drawer, traps Tab navigation, and restores its trigger after Escape', async () => {
    renderDocs();
    const trigger = screen.getByRole('button', { name: 'Browse docs' });
    trigger.focus();
    fireEvent.click(trigger);

    const dialog = await screen.findByRole('dialog', { name: 'Browse docs' });
    const first = within(dialog).getByRole('link', { name: 'Search all docs' });
    const last = within(dialog).getByRole('link', { name: /pbs/ });
    await waitFor(() => expect(first).toHaveFocus());

    last.focus();
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(first).toHaveFocus();

    first.focus();
    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true });
    expect(last).toHaveFocus();

    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  it('restores the trigger after the close control is activated', async () => {
    renderDocs();
    const trigger = screen.getByRole('button', { name: 'Browse docs' });
    fireEvent.click(trigger);

    const close = await screen.findByRole('button', { name: 'Close doc list' });
    fireEvent.click(close);

    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Browse docs' })).not.toBeInTheDocument()
    );
    expect(trigger).toHaveFocus();
  });
});
