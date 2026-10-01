import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import type { Extension } from '@codemirror/state';
import '../../i18n';
import { curatedHandlers } from '../../mocks/curated';
import { docs } from '../../data/fixtures';
import { getGetDocsDocIdQueryKey } from '../../api/generated/docs/docs';
import { useLive } from '../../store/live';
import { toast } from '../../lib/toast';
import { DocEditorPage } from './DocEditorPage';

vi.mock('../../hooks/useRole', () => ({
  useConnectorRole: () => 'operator',
  useIsInstanceAdmin: () => true,
}));
vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@uiw/react-codemirror', () => ({
  default: ({
    value,
    onChange,
    editable,
    extensions,
  }: {
    value: string;
    onChange: (next: string) => void;
    editable: boolean;
    extensions: Extension[];
  }) => {
    const attribute = extensions.find(
      (extension) => (extension as { value?: Record<string, string> }).value?.['aria-label']
    );
    const label = (attribute as { value?: Record<string, string> } | undefined)?.value?.[
      'aria-label'
    ];
    return (
      <textarea
        aria-label={label}
        readOnly={!editable}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    );
  },
}));

const server = setupServer(
  http.post('*/docs/:docId/lock/release', () => {
    releaseRequest();
    return HttpResponse.json({});
  }),
  ...curatedHandlers
);
const lockRequest = vi.fn();
const releaseRequest = vi.fn();
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
beforeEach(() => {
  vi.clearAllMocks();
  useLive.setState({ docLocks: {} });
  server.use(
    http.post('*/docs/:docId/lock', () => {
      lockRequest();
      return HttpResponse.json({ userId: 'self', expiresAt: '2099-01-01T00:00:00Z' });
    }),
    http.post('*/docs/:docId/ai-suggest', () =>
      HttpResponse.json({ requestId: 'req-1' }, { status: 202 })
    )
  );
});
afterEach(() => {
  vi.useRealTimers();
  server.resetHandlers();
});
afterAll(() => server.close());

async function renderEditor() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/docs/doc-pve1/edit']}>
        <Routes>
          <Route path="/docs/:docId/edit" element={<DocEditorPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
  const editor = await screen.findByRole('textbox', { name: 'Markdown' });
  return { client, editor, ...view };
}
async function startEditing() {
  fireEvent.click(screen.getByRole('button', { name: 'Start editing' }));
  await waitFor(() =>
    expect(screen.getByRole('textbox', { name: 'Markdown' })).not.toHaveAttribute('readonly')
  );
}
async function requestSuggestion() {
  await startEditing();
  fireEvent.click(screen.getByRole('button', { name: 'Suggest update' }));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Thinking…' })).toBeDisabled());
}

const deliver = (
  client: QueryClient,
  requestId: string,
  docId: string,
  status: string,
  fullContent?: string
) =>
  act(() =>
    client.setQueryData(['doc-ai-suggestion', docId, requestId], {
      docId,
      requestId,
      status,
      fullContent,
    })
  );

describe('DocEditorPage', () => {
  it('keeps a stale draft and surfaces the current revision instead of overwriting it', async () => {
    const doc = docs['doc-pve1'];
    const original = { ...doc };
    try {
      const { editor } = await renderEditor();
      await startEditing();
      fireEvent.change(editor, { target: { value: '# Local draft' } });
      doc.content = '# Remote revision';
      doc.currentVersion = original.currentVersion + 1;
      fireEvent.click(screen.getByRole('button', { name: 'Save' }));
      await screen.findByText(
        `A newer version (v${doc.currentVersion}) was generated while you were editing.`
      );
      expect(editor).toHaveValue('# Local draft');
      expect(doc.content).toBe('# Remote revision');
    } finally {
      Object.assign(doc, original);
    }
  });

  it('keeps server updates clean and loads a new editing baseline only on request', async () => {
    const { client, editor } = await renderEditor();
    const original = { ...docs['doc-pve1'] };
    act(() =>
      client.setQueryData(getGetDocsDocIdQueryKey('doc-pve1'), {
        ...original,
        content: '# Remote revision',
        currentVersion: original.currentVersion + 1,
      })
    );
    await screen.findByRole('button', { name: 'Load latest' });
    expect(editor).toHaveValue(original.content);
    expect(screen.queryByText('Unsaved')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    expect(lockRequest).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Load latest' }));
    expect(editor).toHaveValue('# Remote revision');
    await startEditing();
    fireEvent.change(editor, { target: { value: '# Local edit' } });
    expect(screen.getByText('Unsaved')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Discard' }));
    expect(editor).toHaveValue('# Remote revision');
    expect(screen.queryByText('Unsaved')).not.toBeInTheDocument();
  });

  it('blocks editing until a lock succeeds and shows a named conflict holder', async () => {
    server.use(
      http.post('*/docs/:docId/lock', () =>
        HttpResponse.json(
          {
            userId: 'private-user-id',
            userName: 'Ada',
            expiresAt: '2099-01-01T00:00:00Z',
          },
          { status: 409 }
        )
      )
    );
    const { editor } = await renderEditor();
    expect(editor).toHaveAttribute('readonly');
    fireEvent.click(screen.getByRole('button', { name: 'Start editing' }));
    await screen.findByText('Ada is currently editing this doc.');
    expect(editor).toHaveAttribute('readonly');
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Suggest update' })).toBeDisabled();
    expect(screen.queryByText(/private-user-id/)).not.toBeInTheDocument();
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining("Couldn't acquire"));
  });

  it('stops editing when renewal fails and preserves the draft for retry', async () => {
    const { editor } = await renderEditor();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await startEditing();
    fireEvent.change(editor, { target: { value: '# Local draft' } });
    server.use(http.post('*/docs/:docId/lock', () => new HttpResponse(null, { status: 500 })));
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    await waitFor(() => expect(editor).toHaveAttribute('readonly'));
    expect(editor).toHaveValue('# Local draft');
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    server.use(http.post('*/docs/:docId/lock', () => HttpResponse.json({})));
    await startEditing();
    expect(editor).toHaveValue('# Local draft');
  });

  it('does not release its lock when the draft changes, and releases on unmount', async () => {
    const { editor, unmount } = await renderEditor();
    await startEditing();
    fireEvent.change(editor, { target: { value: '# Local draft' } });
    expect(releaseRequest).not.toHaveBeenCalled();
    unmount();
    await waitFor(() => expect(releaseRequest).toHaveBeenCalledOnce());
  });

  it('waits for the matching completed result before allowing acceptance', async () => {
    const { client, editor } = await renderEditor();
    await requestSuggestion();
    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument();
    deliver(client, 'other-request', 'doc-pve1', 'complete', '# Wrong request');
    deliver(client, 'req-1', 'other-doc', 'complete', '# Wrong doc');
    deliver(client, 'req-1', 'doc-pve1', 'streaming', '# Partial');
    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument();
    deliver(client, 'req-1', 'doc-pve1', 'complete', '# Server proposal');
    fireEvent.click(await screen.findByRole('button', { name: 'Accept' }));
    expect(editor).toHaveValue('# Server proposal');
    expect(screen.getByText('AI draft')).toBeInTheDocument();
  });

  it('retains a result arriving before the POST response', async () => {
    const { client } = await renderEditor();
    deliver(client, 'req-1', 'doc-pve1', 'complete', '# Early proposal');
    await startEditing();
    fireEvent.click(screen.getByRole('button', { name: 'Suggest update' }));
    expect(await screen.findByRole('button', { name: 'Accept' })).toBeEnabled();
  });

  it('surfaces a server AI error without producing an editable suggestion', async () => {
    const { client, editor } = await renderEditor();
    const original = (editor as HTMLTextAreaElement).value;
    await requestSuggestion();
    deliver(client, 'req-1', 'doc-pve1', 'error');
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('AI suggestion failed.'));
    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument();
    expect(editor).toHaveValue(original);
    expect(screen.getByRole('button', { name: 'Suggest update' })).toBeEnabled();
  });

  it('times out a missing WS result and allows another request', async () => {
    await renderEditor();
    await startEditing();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    fireEvent.click(screen.getByRole('button', { name: 'Suggest update' }));
    await act(() => vi.advanceTimersByTimeAsync(100));
    await act(() => vi.advanceTimersByTimeAsync(150_000));
    expect(toast.error).toHaveBeenCalledWith('AI suggestion timed out. Try again.');
    expect(screen.getByRole('button', { name: 'Suggest update' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: 'Accept' })).not.toBeInTheDocument();
  });
});
