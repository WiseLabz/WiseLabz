import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { setupServer } from 'msw/node';
import '../../i18n';
import { curatedHandlers } from '../../mocks/curated';
import { docs } from '../../data/fixtures';
import { DocEditorPage } from './DocEditorPage';

vi.mock('@uiw/react-codemirror', () => ({
  default: ({ value, onChange }: { value: string; onChange: (next: string) => void }) => (
    <textarea aria-label="Markdown" value={value} onChange={(event) => onChange(event.target.value)} />
  ),
}));

const server = setupServer(...curatedHandlers);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe('DocEditorPage', () => {
  it('keeps a stale draft and surfaces the current revision instead of overwriting it', async () => {
    const doc = docs['doc-pve1'];
    const original = { ...doc };

    try {
      render(
        <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
          <MemoryRouter initialEntries={['/docs/doc-pve1/edit']}>
            <Routes>
              <Route path="/docs/:docId/edit" element={<DocEditorPage />} />
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>,
      );

      const editor = await screen.findByRole('textbox', { name: 'Markdown' });
      fireEvent.change(editor, { target: { value: '# Local draft' } });

      doc.content = '# Remote revision';
      doc.currentVersion = original.currentVersion + 1;
      const remoteVersion = doc.currentVersion;

      fireEvent.click(screen.getByRole('button', { name: 'Save' }));

      await screen.findByText(`A newer version (v${remoteVersion}) was generated while you were editing.`);
      expect(screen.getByRole('textbox', { name: 'Markdown' })).toHaveValue('# Local draft');
      expect(doc.content).toBe('# Remote revision');
    } finally {
      Object.assign(doc, original);
    }
  });
});
