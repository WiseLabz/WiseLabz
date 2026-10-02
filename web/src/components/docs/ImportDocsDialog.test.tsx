import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ImportDocsDialog } from './ImportDocsDialog';
import { postDocsImport, postDocsImportImportIdCommit } from '../../api/generated/docs/docs';
import type { DocImportPreview } from '../../api/model';

const preview: DocImportPreview = {
  id: 'imp-1',
  expiresAt: '2026-10-02T13:00:00Z',
  docCount: 3,
  attachmentCount: 1,
  mappings: [{ source: 'Lab/index.md', link: '[[Proxmox]]', target: '/docs/d3' }],
  tree: [
    {
      docId: 'd1',
      title: 'Lab',
      path: 'Lab',
      serviceId: '',
      folder: true,
      attachmentCount: 0,
      children: [
        {
          docId: 'd2',
          title: 'Network (imported)',
          path: 'Lab/Network.md',
          serviceId: '',
          folder: false,
          attachmentCount: 1,
          children: [],
        },
      ],
    },
    {
      docId: 'd3',
      title: 'Proxmox',
      path: 'Lab/Proxmox.md',
      serviceId: 'c1',
      folder: false,
      attachmentCount: 0,
      children: [],
    },
  ],
  warnings: [{ path: 'Lab/index.md', message: 'unresolved link [[Nowhere]]; left as text' }],
  skipped: [{ path: 'Lab/.obsidian', message: 'hidden file or folder' }],
  collisions: [{ path: 'Lab/Network.md', title: 'Network', newTitle: 'Network (imported)' }],
};

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({ data: [{ id: 'c1', name: 'pve' }] }),
}));
vi.mock('../../api/generated/docs/docs', () => ({
  postDocsImport: vi.fn(),
  postDocsImportImportIdCommit: vi.fn(),
}));
vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

function show(onClose = vi.fn()) {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <Routes>
          <Route path="/" element={<ImportDocsDialog open onClose={onClose} />} />
          <Route path="/docs/:docId" element={<p>Imported doc</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
  return onClose;
}

function chooseZip() {
  const file = new File(['PK'], 'vault.zip', { type: 'application/zip' });
  fireEvent.change(screen.getByLabelText('Vault zip'), { target: { files: [file] } });
  fireEvent.click(screen.getByRole('button', { name: 'Preview import' }));
  return file;
}

describe('ImportDocsDialog', () => {
  it('uploads, previews the tree with warnings, commits and opens the first doc', async () => {
    vi.mocked(postDocsImport).mockResolvedValue(preview);
    vi.mocked(postDocsImportImportIdCommit).mockResolvedValue([
      { docId: 'd1', title: 'Lab', parentId: '', serviceId: '' },
    ]);
    const onClose = show();
    expect(screen.getByRole('button', { name: 'Preview import' })).toBeDisabled();
    const file = chooseZip();
    expect(
      await screen.findByText('Docs: 3 · Attachments: 1 · Links rewritten: 1')
    ).toBeInTheDocument();
    expect(postDocsImport).toHaveBeenCalledWith({ file });
    expect(screen.getByText('Network (imported)')).toBeInTheDocument();
    expect(screen.getByText('pve')).toBeInTheDocument();
    expect(screen.getByText('Network → Network (imported)')).toBeInTheDocument();
    expect(screen.getByText('Warnings (1)')).toBeInTheDocument();
    expect(screen.getByText(/unresolved link \[\[Nowhere\]\]/)).toBeInTheDocument();
    expect(screen.getByText('Skipped files (1)')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Import 3 docs' }));
    await waitFor(() => expect(postDocsImportImportIdCommit).toHaveBeenCalledWith('imp-1'));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(await screen.findByText('Imported doc')).toBeInTheDocument();
  });

  it('goes back to file selection and stays open when the upload fails', async () => {
    vi.mocked(postDocsImport).mockRejectedValueOnce(new Error('bad zip'));
    const onClose = show();
    chooseZip();
    const { toast } = await import('../../lib/toast');
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(onClose).not.toHaveBeenCalled();
    vi.mocked(postDocsImport).mockResolvedValue({ ...preview, docCount: 0, tree: [] });
    fireEvent.click(screen.getByRole('button', { name: 'Preview import' }));
    expect(
      await screen.findByText('No Markdown notes were found in this archive.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Import 0 docs' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Choose another file' }));
    expect(screen.getByLabelText('Vault zip')).toBeInTheDocument();
  });
});
