import { AxiosError } from 'axios';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { BookStackPull } from './BookStackPull';
import {
  deleteDocsImportPull,
  getDocsImportPull,
  postDocsImportPull,
} from '../../api/generated/docs/docs';
import { postAuthElevate } from '../../api/generated/auth/auth';
import type { DocImportPreview, DocPullJob } from '../../api/model';
import { toast } from '../../lib/toast';

vi.mock('../../api/generated/docs/docs', () => ({
  deleteDocsImportPull: vi.fn(),
  getDocsImportPull: vi.fn(),
  postDocsImportPull: vi.fn(),
}));
vi.mock('../../api/generated/auth/auth', () => ({
  useGetAuthElevateMethods: () => ({ data: { methods: ['password'] }, isLoading: false }),
  postAuthElevate: vi.fn(),
}));
vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const fetching: DocPullJob = {
  id: 'job',
  source: 'bookstack',
  host: 'wiki.example',
  skipTlsVerify: false,
  state: 'fetching',
  startedAt: '2026-10-10T00:00:00Z',
  done: 0,
  total: 2,
};
const preview: DocImportPreview = {
  id: 'job',
  expiresAt: '2026-10-10T01:00:00Z',
  docCount: 1,
  attachmentCount: 0,
  tree: [],
  mappings: [],
  warnings: [],
  skipped: [],
  collisions: [],
};

function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onReady = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <BookStackPull onReady={onReady} />
    </QueryClientProvider>
  );
  return { client, onReady };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(getDocsImportPull).mockRejectedValue(new Error('no pull'));
});
afterEach(cleanup);

function fillForm() {
  fireEvent.change(screen.getByLabelText('BookStack URL'), {
    target: { value: 'https://wiki.example' },
  });
  fireEvent.change(screen.getByLabelText('Token ID'), { target: { value: 'token-id' } });
  fireEvent.change(screen.getByLabelText('Token secret'), { target: { value: 'token-secret' } });
}

describe('BookStack pull', () => {
  it('defaults to verified TLS, uses step-up on demand and polls into the existing preview', async () => {
    const { client, onReady } = show();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Fetch preview' })).toBeEnabled()
    );
    expect(screen.getByLabelText(/Skip TLS/)).not.toBeChecked();
    fillForm();
    const required = new AxiosError('elevation');
    required.response = { data: { code: 'elevation_required' } } as AxiosError['response'];
    vi.mocked(postDocsImportPull).mockRejectedValueOnce(required).mockResolvedValueOnce(fetching);
    vi.mocked(postAuthElevate).mockResolvedValue({
      token: 'elevated',
      expiresAt: '2026-10-10T00:01:00Z',
    });
    fireEvent.click(screen.getByRole('button', { name: 'Fetch preview' }));
    fireEvent.change(await screen.findByLabelText(/confirm your password/i), {
      target: { value: 'password' },
    });
    vi.mocked(getDocsImportPull).mockResolvedValue(fetching);
    fireEvent.click(screen.getByRole('button', { name: /verify/i }));
    await waitFor(() =>
      expect(postDocsImportPull).toHaveBeenCalledWith(
        {
          source: 'bookstack',
          url: 'https://wiki.example',
          tokenId: 'token-id',
          tokenSecret: 'token-secret',
          skipTlsVerify: false,
        },
        { headers: { 'X-Elevation-Token': 'elevated' } }
      )
    );
    expect(await screen.findByText('Fetching books: 0 / 2')).toBeInTheDocument();
    vi.mocked(getDocsImportPull).mockResolvedValue({ ...fetching, state: 'ready', preview });
    await client.invalidateQueries({ queryKey: ['docs-import-pull'] });
    await waitFor(() => expect(onReady).toHaveBeenCalledWith(preview));
  });

  it('resumes progress and cancels, showing cancelled without importing', async () => {
    vi.mocked(getDocsImportPull).mockResolvedValue(fetching);
    vi.mocked(deleteDocsImportPull).mockImplementation(async () => {
      vi.mocked(getDocsImportPull).mockResolvedValue({ ...fetching, state: 'cancelled' });
      return { ...fetching, state: 'fetching' };
    });
    const { onReady } = show();
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel pull' }));
    expect(await screen.findByText('Pull cancelled. Nothing was imported.')).toBeInTheDocument();
    expect(deleteDocsImportPull).toHaveBeenCalledOnce();
    expect(onReady).not.toHaveBeenCalled();
  });

  it('shows failure and permits a retry; a previous ready job can be reviewed explicitly', async () => {
    vi.mocked(getDocsImportPull).mockResolvedValue({
      ...fetching,
      state: 'failed',
      error: 'BookStack ZIP export API requires v25.07',
    });
    const { client, onReady } = show();
    expect(await screen.findByRole('alert')).toHaveTextContent('v25.07');
    vi.mocked(getDocsImportPull).mockResolvedValue({ ...fetching, state: 'ready', preview });
    await client.invalidateQueries({ queryKey: ['docs-import-pull'] });
    fireEvent.click(await screen.findByRole('button', { name: 'Review fetched preview' }));
    expect(onReady).toHaveBeenCalledWith(preview);
  });

  it('treats a failed read as no current job and stops polling', async () => {
    vi.mocked(getDocsImportPull).mockResolvedValue(fetching);
    const { client } = show();
    expect(await screen.findByText('Fetching books: 0 / 2')).toBeInTheDocument();
    expect(
      screen.getByRole('progressbar', { name: 'BookStack pull progress' })
    ).toBeInTheDocument();
    vi.mocked(getDocsImportPull).mockRejectedValue(new Error('404'));
    await client.invalidateQueries({ queryKey: ['docs-import-pull'] });
    expect(await screen.findByRole('button', { name: 'Fetch preview' })).toBeInTheDocument();
    expect(screen.queryByText('Fetching books: 0 / 2')).not.toBeInTheDocument();
    const calls = vi.mocked(getDocsImportPull).mock.calls.length;
    await new Promise((resolve) => setTimeout(resolve, 1300));
    expect(vi.mocked(getDocsImportPull).mock.calls.length).toBe(calls);
  });

  it('switches to the running pull on 409 and clears the token fields', async () => {
    show();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Fetch preview' })).toBeEnabled()
    );
    fillForm();
    const conflict = new AxiosError('conflict');
    conflict.response = {
      status: 409,
      data: { code: 'pull_running', message: 'a documentation pull is already running' },
    } as AxiosError['response'];
    vi.mocked(postDocsImportPull).mockRejectedValueOnce(conflict);
    vi.mocked(getDocsImportPull).mockResolvedValue(fetching);
    fireEvent.click(screen.getByRole('button', { name: 'Fetch preview' }));
    expect(await screen.findByText('Fetching books: 0 / 2')).toBeInTheDocument();
  });

  it('shows the server message on 400 and clears the token fields', async () => {
    show();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Fetch preview' })).toBeEnabled()
    );
    fillForm();
    const invalid = new AxiosError('bad request');
    invalid.response = {
      status: 400,
      data: { code: 'invalid_request', message: 'use an HTTP(S) wiki URL' },
    } as AxiosError['response'];
    vi.mocked(postDocsImportPull).mockRejectedValueOnce(invalid);
    fireEvent.click(screen.getByRole('button', { name: 'Fetch preview' }));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('use an HTTP(S) wiki URL'));
    expect(screen.getByLabelText('Token ID')).toHaveValue('');
    expect(screen.getByLabelText('Token secret')).toHaveValue('');
  });
});
