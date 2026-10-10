import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import '../../i18n';
import { docTree, docs } from '../../data/fixtures';
import { DocsPage } from './DocsPage';

let admin = true;
vi.mock('../../hooks/useRole', async (orig) => ({
  ...(await orig<typeof import('../../hooks/useRole')>()),
  useIsInstanceAdmin: () => admin,
}));
vi.mock('../../components/docs/ImportDocsDialog', () => ({
  ImportDocsDialog: ({ open }: { open: boolean }) => (open ? <p>Import dialog open</p> : null),
}));
vi.mock('../../api/generated/docs/docs', async (orig) => ({
  ...(await orig<typeof import('../../api/generated/docs/docs')>()),
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

describe('DocsPage import entry point', () => {
  it('offers an Import action to instance admins that opens the dialog', () => {
    admin = true;
    renderDocs();
    expect(screen.queryByText('Import dialog open')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Import' }));
    expect(screen.getByText('Import dialog open')).toBeInTheDocument();
  });

  it('hides the Import action from everyone else', () => {
    admin = false;
    renderDocs();
    expect(screen.queryByRole('button', { name: 'Import' })).toBeNull();
  });
});
