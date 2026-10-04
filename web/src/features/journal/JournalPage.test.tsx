import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { JournalPage } from './JournalPage';
import type { TimelineItem } from '../../api/model';

const { get, create, update, remove, role, error } = vi.hoisted(() => ({
  get: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  role: { admin: false, operator: true, userId: 'author' },
  error: vi.fn(),
}));
vi.mock('../../api/generated/journal/journal', () => ({
  getTimeline: get,
  postJournal: create,
  putJournalId: update,
  deleteJournalId: remove,
}));
vi.mock('../../hooks/useRole', () => ({
  useIsInstanceAdmin: () => role.admin,
  useCanMutate: () => role.admin || role.operator,
  useOperatorConnectorIds: () => new Set(role.operator ? ['c'] : []),
}));
vi.mock('../../api/generated/me/me', () => ({ useGetMe: () => ({ data: { id: role.userId } }) }));
vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({ data: [{ id: 'c', name: 'Router' }] }),
}));
vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsTree: () => ({
    data: {
      docId: 'root',
      branch: true,
      children: [{ docId: 'd', title: 'Router notes', serviceId: 'c' }],
    },
  }),
}));
vi.mock('../../components/docs/Mermaid', () => ({ Mermaid: () => null }));
vi.mock('../../components/manager/EntityPicker', () => ({
  EntityPicker: ({
    onChange,
    onEntityChange,
  }: {
    onChange: (ref: string) => void;
    onEntityChange: (entity: { kind: string; name: string }) => void;
  }) => (
    <button
      type="button"
      onClick={() => {
        onChange('vm/100');
        onEntityChange({ kind: 'vm', name: 'router' });
      }}
    >
      Pick router
    </button>
  ),
}));
vi.mock('../../lib/toast', () => ({ toast: { error } }));

const note: TimelineItem = {
  id: 'j',
  kind: 'journal',
  timestamp: '2020-01-01T00:00:12.123456789Z',
  title: '',
  body: 'Replaced **router**',
  connectorId: 'c',
  docId: '',
  createdBy: 'author',
  entityKind: 'vm',
  entityName: 'router',
  entityRef: 'vm/100',
  status: '',
};
function Location() {
  return <output data-testid="location">{useLocation().search}</output>;
}
function mount(path = '/journal') {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <MemoryRouter initialEntries={[path]}>
        <JournalPage />
        <Location />
      </MemoryRouter>
    </QueryClientProvider>
  );
}
beforeEach(() => {
  vi.clearAllMocks();
  role.admin = false;
  role.operator = true;
  role.userId = 'author';
  get.mockResolvedValue({ items: [note], total: 1, page: 1, pageSize: 30 });
  create.mockResolvedValue({ id: 'new' });
  update.mockResolvedValue({ id: 'j' });
  remove.mockResolvedValue(undefined);
});

describe('Journal', () => {
  it('renders Markdown, entity context, source links and cursor pages', async () => {
    get.mockImplementation((params: { cursor: string }) =>
      Promise.resolve(
        params.cursor
          ? { items: [{ ...note, id: 'older', body: 'Older note' }], total: 2 }
          : { items: [note], total: 2, nextCursor: 'next' }
      )
    );
    mount();
    expect(await screen.findByText('Replaced', { exact: false })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View source' })).toHaveAttribute(
      'href',
      '/services/c'
    );
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
    expect(await screen.findByText('Older note')).toBeInTheDocument();
    expect(get).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: 'next' }),
      undefined,
      expect.any(AbortSignal)
    );
  });

  it('keeps filters in the URL and sends them before paging', async () => {
    mount('/journal?connectorId=c&kinds=journal&allSyncRuns=true');
    await screen.findByText('Replaced', { exact: false });
    expect(get).toHaveBeenCalledWith(
      expect.objectContaining({
        connectorId: 'c',
        kinds: 'journal',
        allSyncRuns: true,
        cursor: '',
      }),
      undefined,
      expect.any(AbortSignal)
    );
    fireEvent.change(screen.getByLabelText('Source'), { target: { value: 'change' } });
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('kinds=change'));
    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith(
        expect.objectContaining({ kinds: 'change', cursor: '' }),
        undefined,
        expect.any(AbortSignal)
      )
    );
  });

  it('creates a backdated note with entity and document context and preview', async () => {
    mount();
    fireEvent.click(screen.getByRole('button', { name: 'New entry' }));
    fireEvent.change(screen.getByLabelText('Note (Markdown)'), {
      target: { value: '**Maintenance**' },
    });
    expect(screen.getByLabelText('Preview')).toHaveTextContent('Maintenance');
    fireEvent.change(screen.getByLabelText('Occurred at'), {
      target: { value: '2020-01-01T12:00' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Pick router' }));
    fireEvent.change(screen.getByLabelText('Document (optional)'), { target: { value: 'd' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({
        body: '**Maintenance**',
        occurredAt: new Date('2020-01-01T12:00').toISOString(),
        connectorId: 'c',
        docId: 'd',
        entityRef: 'vm/100',
        entityKind: 'vm',
        entityName: 'router',
      })
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('allows author edits and confirms deletion, hiding controls for other viewers', async () => {
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Edit' }));
    fireEvent.change(screen.getByLabelText('Note (Markdown)'), {
      target: { value: 'Edited context' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(
        'j',
        expect.objectContaining({ body: 'Edited context', occurredAt: note.timestamp })
      )
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(remove).not.toHaveBeenCalled();
    const confirmation = await screen.findByRole('dialog');
    fireEvent.click(within(confirmation).getByRole('button', { name: 'Delete' }));
    await waitFor(() => expect(remove).toHaveBeenCalledWith('j'));
  });

  it('shows no write controls for a non-author viewer', async () => {
    role.operator = false;
    role.userId = 'viewer';
    mount();
    await screen.findByText('Replaced', { exact: false });
    expect(screen.queryByRole('button', { name: 'New entry' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Lab action' })).not.toBeInTheDocument();
  });

  it('keeps the dialog open and reports mutation failure', async () => {
    create.mockRejectedValue(new Error('denied'));
    mount();
    fireEvent.click(screen.getByRole('button', { name: 'New entry' }));
    fireEvent.change(screen.getByLabelText('Note (Markdown)'), { target: { value: 'Note' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(error).toHaveBeenCalledWith('Could not save the entry'));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('writes date range and all-sync-runs filters to the URL and the request', async () => {
    mount();
    await screen.findByText('Replaced', { exact: false });
    fireEvent.change(screen.getByLabelText('From (UTC)'), { target: { value: '2020-01-02' } });
    fireEvent.change(screen.getByLabelText('Through (UTC)'), { target: { value: '2020-01-03' } });
    fireEvent.click(screen.getByLabelText('Show all sync runs'));
    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith(
        expect.objectContaining({
          after: '2020-01-02T00:00:00Z',
          before: '2020-01-03T23:59:59.999999999Z',
          allSyncRuns: true,
        }),
        undefined,
        expect.any(AbortSignal)
      )
    );
    expect(screen.getByTestId('location')).toHaveTextContent('allSyncRuns=true');
    fireEvent.click(screen.getByLabelText('Show all sync runs'));
    await waitFor(() =>
      expect(screen.getByTestId('location')).not.toHaveTextContent('allSyncRuns')
    );
  });

  it('offers lab actions, lab-wide notes and edits on others notes to instance admins', async () => {
    role.admin = true;
    role.userId = 'someone-else';
    mount();
    await screen.findByText('Replaced', { exact: false });
    expect(screen.getByRole('option', { name: 'Lab action' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'New entry' }));
    const scope = within(screen.getByRole('dialog')).getByLabelText('Scope');
    expect(scope).toHaveValue('');
    expect(within(scope).getByRole('option', { name: 'Lab-wide' })).toBeInTheDocument();
  });

  it('limits a non-admin operator to connector-scoped notes', async () => {
    mount();
    await screen.findByText('Replaced', { exact: false });
    expect(screen.queryByRole('option', { name: 'Lab action' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'New entry' }));
    const scope = within(screen.getByRole('dialog')).getByLabelText('Scope');
    expect(scope).toHaveValue('c');
    expect(within(scope).queryByRole('option', { name: 'Lab-wide' })).not.toBeInTheDocument();
  });
});
