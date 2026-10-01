import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError, type AxiosResponse } from 'axios';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import '../../i18n';
import { DocProposalsPage } from './DocProposalsPage';

const approveMock = vi.fn();
const rejectMock = vi.fn();
const toastMock = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));

const proposal = {
  id: 'p1',
  docId: 'doc-1',
  docTitle: 'Router runbook',
  baseVersion: 3,
  content: '# New body',
  summary: 'Fix the reset steps',
  authorId: 'u1',
  status: 'pending',
  createdAt: new Date().toISOString(),
};

vi.mock('../../api/generated/docs/docs', () => ({
  getGetDocsEditProposalsQueryKey: () => ['/api/docs/edit-proposals'],
  useGetDocsEditProposals: () => ({
    data: { items: [proposal], total: 1, page: 1, pageSize: 20 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useGetDocsEditProposalsProposalId: () => ({ data: proposal, isLoading: false, isError: false }),
  postDocsEditProposalsProposalIdApprove: (id: string) => approveMock(id),
  postDocsEditProposalsProposalIdReject: (id: string) => rejectMock(id),
}));

vi.mock('../../lib/toast', () => ({ toast: toastMock }));

function conflict(data: unknown) {
  return new AxiosError('conflict', 'ERR_BAD_REQUEST', undefined, undefined, {
    status: 409,
    data,
  } as AxiosResponse);
}

function renderPage() {
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <DocProposalsPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

beforeEach(() => {
  approveMock.mockReset();
  rejectMock.mockReset();
  toastMock.success.mockReset();
  toastMock.error.mockReset();
});
afterEach(cleanup);

describe('DocProposalsPage', () => {
  it('lists the proposal with its summary and loads content only when expanded', () => {
    renderPage();
    expect(screen.getByText('Router runbook')).toBeInTheDocument();
    expect(screen.getByText('Fix the reset steps')).toBeInTheDocument();
    expect(screen.queryByText('# New body')).not.toBeInTheDocument();
    const details = screen.getByText('View proposed content').closest('details') as HTMLDetailsElement;
    details.open = true;
    fireEvent(details, new Event('toggle'));
    expect(screen.getByText('# New body')).toBeInTheDocument();
  });

  it('approves a proposal', async () => {
    approveMock.mockResolvedValue({ ...proposal, status: 'approved' });
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }));
    await waitFor(() => expect(approveMock).toHaveBeenCalledWith('p1'));
    await waitFor(() =>
      expect(toastMock.success).toHaveBeenCalledWith('Proposal approved and applied as a new version.')
    );
  });

  it('rejects a proposal', async () => {
    rejectMock.mockResolvedValue({ ...proposal, status: 'rejected' });
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Reject' }));
    await waitFor(() => expect(rejectMock).toHaveBeenCalledWith('p1'));
    await waitFor(() => expect(toastMock.success).toHaveBeenCalledWith('Proposal rejected.'));
  });

  it('explains a stale-version conflict on approve', async () => {
    approveMock.mockRejectedValue(conflict({ docId: 'doc-1', currentVersion: 4 }));
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }));
    await waitFor(() =>
      expect(toastMock.error).toHaveBeenCalledWith(
        'The doc changed since this proposal was written. It was not applied.'
      )
    );
  });

  it('explains an already-reviewed conflict on approve', async () => {
    approveMock.mockRejectedValue(conflict({ error: { code: 'already_reviewed' } }));
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }));
    await waitFor(() =>
      expect(toastMock.error).toHaveBeenCalledWith('This proposal was already reviewed.')
    );
  });
});
