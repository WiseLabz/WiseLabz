import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { AxiosError } from 'axios';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { JournalPage } from './JournalPage';
import type { TimelineItem, TimelineNarration } from '../../api/model';

const { get, narrate, create, update, remove, role, error } = vi.hoisted(() => ({
  get: vi.fn(),
  narrate: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  role: { admin: false, operator: true, userId: 'author' },
  error: vi.fn(),
}));
vi.mock('../../api/generated/journal/journal', async () => {
  const { useMutation } = await import('@tanstack/react-query');
  return {
    getTimeline: get,
    postJournal: create,
    putJournalId: update,
    deleteJournalId: remove,
    usePostTimelineNarrate: () =>
      useMutation({ mutationFn: (vars: { params?: object }) => narrate(vars.params) }),
  };
});
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
const narration: TimelineNarration = {
  narration: 'Router replaced [1] after the alert [2]. **Not bold** <b>raw</b> [9]',
  sources: [
    {
      n: 1,
      kind: 'journal',
      id: 'j',
      docId: '',
      connectorId: 'c',
      timestamp: '2020-01-01T00:00:12.123456789Z',
      title: 'Replaced router',
    },
    {
      n: 2,
      kind: 'alert',
      id: 'a',
      docId: '',
      connectorId: 'c',
      timestamp: '2020-01-02T00:00:00.000000000Z',
      title: 'Disk full',
    },
  ],
  after: '2020-01-01T00:00:00.000000000Z',
  before: '',
  eventCount: 2,
  totalEvents: 2,
  truncated: false,
  provider: 'mock',
  fallbackUsed: false,
};
function failure(status: number) {
  return new AxiosError('failed', 'ERR_BAD_RESPONSE', undefined, undefined, {
    status,
    data: {},
    statusText: '',
    headers: {},
    config: {} as never,
  });
}
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
    expect(screen.getByRole('option', { name: 'Lab action' })).toBeInTheDocument();
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
    expect(screen.getByRole('option', { name: 'Lab action' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'New entry' }));
    const scope = within(screen.getByRole('dialog')).getByLabelText('Scope');
    expect(scope).toHaveValue('c');
    expect(within(scope).queryByRole('option', { name: 'Lab-wide' })).not.toBeInTheDocument();
  });

  it.each([
    { admin: false, docId: 'd', connectorId: 'c', href: '/docs/d' },
    { admin: false, docId: '', connectorId: 'c', href: '/services/c' },
    { admin: false, docId: '', connectorId: '', href: '' },
    { admin: true, docId: 'd', connectorId: 'c', href: '/settings/audit' },
    { admin: true, docId: '', connectorId: '', href: '/settings/audit' },
  ])(
    'links audit sources for $admin admin with doc $docId and connector $connectorId',
    async ({ admin, docId, connectorId, href }) => {
      role.admin = admin;
      get.mockResolvedValue({
        items: [{ ...note, kind: 'audit', title: 'runbook.update', body: '', docId, connectorId }],
        total: 1,
      });
      mount('/journal?kinds=audit');
      await screen.findByText('Lab action', { selector: 'span' });
      if (href) {
        expect(screen.getByRole('link', { name: 'View source' })).toHaveAttribute('href', href);
      } else {
        expect(screen.queryByRole('link', { name: 'View source' })).not.toBeInTheDocument();
      }
    }
  );

  describe('window narration', () => {
    const summarize = () =>
      fireEvent.click(screen.getByRole('button', { name: 'Summarize this window' }));

    it('sends the current filters and renders plain text with citation and source links', async () => {
      narrate.mockResolvedValue(narration);
      mount('/journal?connectorId=c&kinds=journal&after=2020-01-01T00:00:00Z');
      await screen.findByText('Replaced', { exact: false });
      summarize();
      expect(await screen.findByText('AI summary of this window')).toBeInTheDocument();
      expect(narrate).toHaveBeenCalledWith(
        expect.objectContaining({
          connectorId: 'c',
          kinds: 'journal',
          after: '2020-01-01T00:00:00Z',
        })
      );
      const summary = screen.getByRole('region', { name: 'AI summary of this window' });
      expect(summary).toHaveTextContent('From 1/1/2020 (UTC)');
      expect(summary).toHaveTextContent('Scope: Router');
      expect(summary).toHaveTextContent('Source: Note');
      expect(summary).toHaveTextContent('2 events summarized');
      // Narration is plain text: no Markdown emphasis, no raw HTML element.
      expect(summary).toHaveTextContent('**Not bold** <b>raw</b> [9]');
      expect(summary.querySelector('b, strong')).toBeNull();
      const citations = within(summary).getAllByRole('link', { name: /^\[\d\]$/ });
      expect(citations.map((a) => a.getAttribute('href'))).toEqual([
        '#narration-source-1',
        '#narration-source-2',
      ]);
      expect(within(summary).getByRole('link', { name: 'Replaced router' })).toHaveAttribute(
        'href',
        '/services/c'
      );
      expect(within(summary).getByRole('link', { name: 'Disk full' })).toHaveAttribute(
        'href',
        '/alerts/a'
      );
      expect(screen.queryByText(/left out/)).not.toBeInTheDocument();
    });

    it.each([
      { admin: false, docId: 'd', connectorId: 'c', href: '/docs/d' },
      { admin: false, docId: '', connectorId: 'c', href: '/services/c' },
      { admin: false, docId: '', connectorId: '', href: null },
      { admin: true, docId: 'd', connectorId: 'c', href: '/settings/audit' },
    ])(
      'links an audit source for admin=$admin with doc "$docId" and connector "$connectorId"',
      async ({ admin, docId, connectorId, href }) => {
        role.admin = admin;
        narrate.mockResolvedValue({
          ...narration,
          narration: 'Runbook edited [1].',
          sources: [
            {
              n: 1,
              kind: 'audit',
              id: 'x',
              docId,
              connectorId,
              timestamp: '2020-01-01T00:00:00.000000000Z',
              title: 'runbook.update',
            },
          ],
          eventCount: 1,
          totalEvents: 1,
        });
        mount();
        await screen.findByText('Replaced', { exact: false });
        summarize();
        const summary = await screen.findByRole('region', { name: 'AI summary of this window' });
        expect(within(summary).getByText(/runbook\.update/)).toBeInTheDocument();
        if (href) {
          expect(within(summary).getByRole('link', { name: 'runbook.update' })).toHaveAttribute(
            'href',
            href
          );
        } else {
          expect(within(summary).queryByRole('link', { name: 'runbook.update' })).toBeNull();
        }
        expect(summary.querySelector('a[href="/settings/audit"]') !== null).toBe(admin);
      }
    );

    it.each([
      [409, 'AI is not enabled. An administrator can turn it on in Settings.'],
      [502, 'Could not summarize this window. The journal below is unaffected.'],
    ])('keeps the journal list usable when narration fails with %i', async (status, message) => {
      narrate.mockRejectedValue(failure(status));
      mount();
      await screen.findByText('Replaced', { exact: false });
      const calls = get.mock.calls.length;
      summarize();
      expect(await screen.findByRole('alert')).toHaveTextContent(message);
      expect(screen.getByText('Replaced', { exact: false })).toBeInTheDocument();
      expect(get).toHaveBeenCalledTimes(calls);
      expect(screen.getByRole('button', { name: 'Summarize this window' })).toBeEnabled();
    });

    it('says so when the window has no visible events', async () => {
      narrate.mockResolvedValue({
        ...narration,
        narration: '',
        sources: [],
        eventCount: 0,
        totalEvents: 0,
      });
      mount();
      await screen.findByText('Replaced', { exact: false });
      summarize();
      expect(
        await screen.findByText('There are no visible events in this window to summarize.')
      ).toBeInTheDocument();
      expect(screen.queryByText('AI summary of this window')).not.toBeInTheDocument();
    });

    it('notes when older events were left out', async () => {
      narrate.mockResolvedValue({ ...narration, eventCount: 2, totalEvents: 250, truncated: true });
      mount();
      await screen.findByText('Replaced', { exact: false });
      summarize();
      expect(
        await screen.findByText(
          'The window holds 250 events; only the 2 most recent fit the summary, so older events are left out.'
        )
      ).toBeInTheDocument();
    });

    const filterChanges: [string, () => void][] = [
      [
        'source',
        () => fireEvent.change(screen.getByLabelText('Source'), { target: { value: 'change' } }),
      ],
      ['scope', () => fireEvent.change(screen.getByLabelText('Scope'), { target: { value: 'c' } })],
      ['all sync runs', () => fireEvent.click(screen.getByLabelText('Show all sync runs'))],
    ];

    it.each(filterChanges)('clears the narration when the %s filter changes', async (_, change) => {
      narrate.mockResolvedValue(narration);
      mount();
      await screen.findByText('Replaced', { exact: false });
      summarize();
      expect(await screen.findByText('AI summary of this window')).toBeInTheDocument();
      change();
      await waitFor(() =>
        expect(screen.queryByText('AI summary of this window')).not.toBeInTheDocument()
      );
    });

    it('drops a narration that resolves after the filters changed', async () => {
      let resolve!: (value: TimelineNarration) => void;
      narrate.mockReturnValue(new Promise<TimelineNarration>((r) => (resolve = r)));
      mount();
      await screen.findByText('Replaced', { exact: false });
      summarize();
      expect(await screen.findByRole('button', { name: 'Summarizing…' })).toBeDisabled();
      fireEvent.change(screen.getByLabelText('Scope'), { target: { value: 'c' } });
      resolve(narration);
      await waitFor(() =>
        expect(screen.getByRole('button', { name: 'Summarize this window' })).toBeEnabled()
      );
      await act(async () => {});
      expect(screen.queryByText('AI summary of this window')).not.toBeInTheDocument();
    });
  });
});
