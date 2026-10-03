import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ExportLabBook } from './ExportLabBook';

const { get, downloadBlob, error } = vi.hoisted(() => ({
  get: vi.fn(),
  downloadBlob: vi.fn(),
  error: vi.fn(),
}));
vi.mock('../../api/axios-instance', () => ({ AXIOS_INSTANCE: { get } }));
vi.mock('../../lib/download', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/download')>()),
  downloadBlob,
}));
vi.mock('../../lib/toast', () => ({ toast: { error } }));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('Lab Book download', () => {
  it.each(['html', 'md.zip'])('downloads %s using the server filename', async (format) => {
    const data = new Blob(['offline']);
    get.mockResolvedValue({
      data,
      headers: { 'content-disposition': `attachment; filename="dated.${format}"` },
    });
    render(<ExportLabBook />);
    fireEvent.change(screen.getByRole('combobox', { name: 'Lab Book format' }), {
      target: { value: format },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Export Lab Book' }));
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledWith(data, `dated.${format}`));
    expect(get).toHaveBeenCalledWith('/docs/export', { params: { format }, responseType: 'blob' });
  });
  it('shows errors and permits retry', async () => {
    get.mockRejectedValue(new Error('failed'));
    render(<ExportLabBook />);
    fireEvent.click(screen.getByRole('button', { name: 'Export Lab Book' }));
    await waitFor(() => expect(error).toHaveBeenCalledWith('Could not export Lab Book.'));
    expect(screen.getByRole('button', { name: 'Export Lab Book' })).toBeEnabled();
  });
});
