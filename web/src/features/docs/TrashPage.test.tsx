import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it, vi } from 'vitest';
import '../../i18n';
import { TrashPage } from './TrashPage';
import { postDocsDocIdRestore } from '../../api/generated/docs/docs';

vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsTrash: () => ({
    data: [{ docId: 'deleted', title: 'Deleted note', deletedAt: '2026-10-01T12:00:00Z' }],
  }),
  postDocsDocIdRestore: vi.fn().mockResolvedValue({ docId: 'deleted' }),
}));
afterEach(cleanup);
it('restores the selected deletion batch and invalidates views', async () => {
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <TrashPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
  expect(screen.getByText('Deleted note')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Restore' }));
  await waitFor(() => expect(postDocsDocIdRestore).toHaveBeenCalledWith('deleted'));
  await waitFor(() => expect(invalidate).toHaveBeenCalled());
});
