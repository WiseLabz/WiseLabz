import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AxiosError, type AxiosResponse } from 'axios';
import { afterEach, expect, it, vi } from 'vitest';
import '../../i18n';
import { TrashPage } from './TrashPage';
import { postDocsDocIdRestore } from '../../api/generated/docs/docs';

const toastMock = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('../../lib/toast', () => ({ toast: toastMock }));
vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsTrash: () => ({
    data: [{ docId: 'deleted', title: 'Deleted note', deletedAt: '2026-10-01T12:00:00Z' }],
  }),
  postDocsDocIdRestore: vi.fn().mockResolvedValue({ docId: 'deleted' }),
}));
afterEach(cleanup);
function renderPage(client = new QueryClient()) {
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <TrashPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}
it('restores the selected deletion batch and invalidates views', async () => {
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  renderPage(client);
  expect(screen.getByText('Deleted note')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Restore' }));
  await waitFor(() => expect(postDocsDocIdRestore).toHaveBeenCalledWith('deleted'));
  await waitFor(() => expect(invalidate).toHaveBeenCalled());
});
it('explains a refused restore when a newer generated doc exists', async () => {
  vi.mocked(postDocsDocIdRestore).mockRejectedValueOnce(
    new AxiosError('conflict', 'ERR_BAD_REQUEST', undefined, undefined, {
      status: 409,
      data: { error: 'generated_doc_exists' },
    } as AxiosResponse)
  );
  renderPage();
  fireEvent.click(screen.getByRole('button', { name: 'Restore' }));
  await waitFor(() =>
    expect(toastMock.error).toHaveBeenCalledWith(
      'A newer generated doc with this title exists. Delete it first to restore this one.'
    )
  );
});
