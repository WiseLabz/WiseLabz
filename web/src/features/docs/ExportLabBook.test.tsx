import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ExportLabBook } from './ExportLabBook';

const { get, downloadBlob, error, renderLabBook } = vi.hoisted(() => ({
  get: vi.fn(),
  downloadBlob: vi.fn(),
  error: vi.fn(),
  renderLabBook: vi.fn(),
}));
vi.mock('../../api/axios-instance', () => ({ AXIOS_INSTANCE: { get } }));
vi.mock('../../lib/download', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/download')>()),
  downloadBlob,
}));
vi.mock('../../lib/toast', () => ({ toast: { error } }));
vi.mock('./labBook', () => ({ renderLabBook }));

const original = '<!DOCTYPE html><html><body>server book</body></html>';
const rendered = '<!DOCTYPE html><html><body><figure><svg></svg></figure></body></html>';
function response() {
  return {
    data: { text: async () => original },
    headers: { 'content-disposition': 'attachment; filename="dated.html"' },
  };
}
function start(format = 'html') {
  fireEvent.change(screen.getByRole('combobox', { name: 'Lab Book format' }), {
    target: { value: format },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Export Lab Book' }));
}
function blobText(blob: Blob): Promise<string> {
  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.readAsText(blob);
  });
}
beforeEach(() => {
  get.mockResolvedValue(response());
  renderLabBook.mockResolvedValue(rendered);
});
afterEach(() => {
  cleanup();
  document.querySelectorAll('iframe').forEach((frame) => frame.remove());
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.resetAllMocks();
});

describe('Lab Book export', () => {
  it('downloads pre-rendered HTML using the server filename', async () => {
    render(<ExportLabBook />);
    start();
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledOnce());
    expect(await blobText(downloadBlob.mock.calls[0][0])).toBe(rendered);
    expect(downloadBlob.mock.calls[0][1]).toBe('dated.html');
    expect(get).toHaveBeenCalledWith('/docs/export', {
      params: { format: 'html' },
      responseType: 'blob',
      signal: expect.any(AbortSignal),
    });
    expect(renderLabBook).toHaveBeenCalledWith(original, {
      signal: expect.any(AbortSignal),
      onProgress: expect.any(Function),
    });
  });

  it('keeps Markdown zip unchanged', async () => {
    const data = new Blob(['zip']);
    get.mockResolvedValue({ data, headers: {} });
    render(<ExportLabBook />);
    start('md.zip');
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledWith(data, 'lab-book.md.zip'));
    expect(renderLabBook).not.toHaveBeenCalled();
  });

  it('falls back to the unchanged server HTML when the transform fails', async () => {
    renderLabBook.mockRejectedValue(new Error('unavailable'));
    render(<ExportLabBook />);
    start();
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledOnce());
    expect(await blobText(downloadBlob.mock.calls[0][0])).toBe(original);
    expect(error).not.toHaveBeenCalled();
  });

  it('shows progress, cancels rendering without saving, and permits retry', async () => {
    let finish!: (html: string) => void;
    renderLabBook.mockImplementationOnce((_html, { onProgress }) => {
      onProgress(1, 3);
      return new Promise<string>((resolve) => {
        finish = resolve;
      });
    });
    render(<ExportLabBook />);
    start();
    await screen.findByText('Rendering diagrams: 1 / 3');
    const signal = get.mock.calls[0][1].signal as AbortSignal;
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(signal.aborted).toBe(true);
    expect(screen.getByRole('button', { name: 'Export Lab Book' })).toBeEnabled();
    start();
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledOnce());
    await act(async () => finish(rendered));
    expect(downloadBlob).toHaveBeenCalledOnce();
    expect(error).not.toHaveBeenCalled();
  });

  it('aborts the request and skips saving even if the request completes later', async () => {
    let finish!: (value: ReturnType<typeof response>) => void;
    get.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        })
    );
    render(<ExportLabBook />);
    start();
    const signal = get.mock.calls[0][1].signal as AbortSignal;
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await act(async () => finish(response()));
    expect(signal.aborted).toBe(true);
    expect(downloadBlob).not.toHaveBeenCalled();
    expect(renderLabBook).not.toHaveBeenCalled();
  });

  it('shows request errors and permits retry', async () => {
    get.mockRejectedValueOnce(new Error('failed'));
    render(<ExportLabBook />);
    start();
    await waitFor(() => expect(error).toHaveBeenCalledWith('Could not export Lab Book.'));
    expect(screen.getByRole('button', { name: 'Export Lab Book' })).toBeEnabled();
    start();
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledOnce());
  });

  it.each([false, true])('prints sandboxed HTML (transform failure: %s)', async (fails) => {
    if (fails) renderLabBook.mockRejectedValue(new Error('unavailable'));
    render(<ExportLabBook />);
    start('pdf');
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
    const frame = document.querySelector('iframe')!;
    expect(frame.getAttribute('sandbox')).toBe('allow-same-origin allow-modals');
    expect(frame.srcdoc).toBe(fails ? original : rendered);
    expect(frame.style.display).not.toBe('none');
    expect(frame.style.width).toBe('0px');
    expect(frame.style.height).toBe('0px');
    const print = vi.spyOn(frame.contentWindow!, 'print').mockImplementation(() => {});
    fireEvent.load(frame);
    await waitFor(() => expect(print).toHaveBeenCalledOnce());
    expect(get).toHaveBeenCalledWith('/docs/export', {
      params: { format: 'html' },
      responseType: 'blob',
      signal: expect.any(AbortSignal),
    });
    expect(get).toHaveBeenCalledOnce();
    expect(downloadBlob).not.toHaveBeenCalled();
    frame.contentWindow!.dispatchEvent(new Event('afterprint'));
    expect(frame.isConnected).toBe(false);
  });

  it('removes the print frame and skips printing when cancelled while it loads', async () => {
    render(<ExportLabBook />);
    const print = vi.fn();
    let attached = false;
    let frame: HTMLIFrameElement | undefined;
    // jsdom loads srcdoc frames on its own, so cancel as soon as the frame is attached.
    const observer = new MutationObserver(() => {
      if (attached) return;
      frame = document.querySelector('iframe') ?? undefined;
      if (!frame) return;
      attached = true;
      vi.spyOn(frame.contentWindow!, 'print').mockImplementation(print);
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    });
    observer.observe(document.body, { childList: true });
    start('pdf');
    await waitFor(() => expect(attached).toBe(true));
    observer.disconnect();
    expect(frame!.isConnected).toBe(false);
    fireEvent.load(frame!);
    expect(print).not.toHaveBeenCalled();
    expect(error).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Export Lab Book' })).toBeEnabled();
  });

  it('hides the progress status for a book without diagrams', async () => {
    let finish!: (html: string) => void;
    renderLabBook.mockImplementationOnce((_html, { onProgress }) => {
      onProgress(0, 0);
      return new Promise<string>((resolve) => {
        finish = resolve;
      });
    });
    render(<ExportLabBook />);
    start();
    await waitFor(() => expect(renderLabBook).toHaveBeenCalled());
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(screen.queryByRole('status')).toBeNull();
    expect(screen.queryByText(/0 \/ 0/)).toBeNull();
    await act(async () => finish(rendered));
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledOnce());
  });

  it('removes the print frame after the fallback timeout', async () => {
    vi.useFakeTimers();
    render(<ExportLabBook />);
    await act(async () => start('pdf'));
    const frame = document.querySelector('iframe')!;
    expect(frame).not.toBeNull();
    vi.spyOn(frame.contentWindow!, 'print').mockImplementation(() => {});
    await act(async () => fireEvent.load(frame));
    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });
    expect(frame.isConnected).toBe(false);
  });
});
