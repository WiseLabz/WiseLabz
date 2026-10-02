import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { DocHistory } from './DocHistory';

vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsDocIdVersions: () => ({
    isLoading: false,
    isError: false,
    data: [
      { rev: 3, trigger: 'future-trigger', author: 'a', createdAt: '2026-10-02T12:00:00Z' },
      { rev: 2, trigger: 'import', author: 'a', createdAt: '2026-10-02T11:00:00Z' },
      { rev: 1, trigger: 'manual', author: 'a', createdAt: '2026-10-02T10:00:00Z' },
    ],
  }),
  useGetDocsDocIdVersionsRev: () => ({ isLoading: false, isFetching: false, data: undefined }),
  postDocsDocIdVersionsRevRestore: vi.fn(),
  getGetDocsDocIdQueryKey: () => ['doc'],
  getGetDocsDocIdVersionsQueryKey: () => ['versions'],
  getGetDocsTreeQueryKey: () => ['tree'],
}));

afterEach(cleanup);

describe('DocHistory triggers', () => {
  it('labels import revisions and tolerates unknown triggers', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <DocHistory docId="d1" currentVersion={3} />
      </QueryClientProvider>,
    );
    expect(screen.getByText(/future-trigger/)).toBeTruthy();
    expect(screen.getAllByText(/Import|import/).length).toBeGreaterThan(0);
  });
});
