import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { NewDocDialog } from './NewDocDialog';
import { postDocs } from '../../api/generated/docs/docs';

let admin = false;
vi.mock('../../hooks/useRole', () => ({
  useIsInstanceAdmin: () => admin,
  useOperatorConnectorIds: () => new Set(['c1']),
}));
vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({
    data: [
      { id: 'c1', name: 'Operated' },
      { id: 'c2', name: 'Viewed' },
    ],
  }),
}));
vi.mock('../../api/generated/docs/docs', () => ({
  getGetDocsQueryKey: () => ['/docs'],
  getGetDocsTreeQueryKey: () => ['/docs/tree'],
  postDocs: vi.fn().mockResolvedValue({ docId: 'new-note' }),
  useGetDocsTree: () => ({
    data: {
      docId: 'root',
      branch: true,
      children: [
        {
          docId: 'lab',
          branch: true,
          children: [{ docId: 'lab-note', title: 'Handbook', origin: 'human', serviceId: '' }],
        },
        {
          docId: 'c1',
          branch: true,
          serviceId: 'c1',
          children: [
            { docId: 'service-note', title: 'Run notes', serviceId: 'c1', origin: 'human' },
          ],
        },
      ],
    },
  }),
}));

afterEach(() => {
  cleanup();
  admin = false;
  vi.clearAllMocks();
});
function show() {
  const client = new QueryClient();
  client.setQueryData(['/docs/tree'], { docId: 'root' });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <Routes>
          <Route path="/" element={<NewDocDialog open onClose={vi.fn()} />} />
          <Route path="/docs/:docId/edit" element={<p>New editor</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
  return client;
}

describe('NewDocDialog', () => {
  it('offers only operated scopes and their parents, then creates and opens the editor', async () => {
    const client = show();
    expect(screen.queryByRole('option', { name: 'Viewed' })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Lab' })).not.toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Handbook' })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Incident notes' } });
    fireEvent.change(screen.getByLabelText('Parent'), { target: { value: 'service-note' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create doc' }));
    await waitFor(() =>
      expect(postDocs).toHaveBeenCalledWith({
        title: 'Incident notes',
        serviceId: 'c1',
        parentId: 'service-note',
      })
    );
    expect(await screen.findByText('New editor')).toBeInTheDocument();
    expect(client.getQueryState(['/docs/tree'])?.isInvalidated).toBe(true);
  });

  it('offers human lab parents to admins and resets parent when scope changes', async () => {
    admin = true;
    show();
    expect(screen.getByRole('option', { name: 'Lab' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Notes' } });
    fireEvent.change(screen.getByLabelText('Parent'), { target: { value: 'lab-note' } });
    fireEvent.change(screen.getByLabelText('Scope'), { target: { value: 'c1' } });
    expect(screen.getByLabelText('Parent')).toHaveValue('');
    fireEvent.click(screen.getByRole('button', { name: 'Create doc' }));
    await waitFor(() =>
      expect(postDocs).toHaveBeenCalledWith({ title: 'Notes', serviceId: 'c1', parentId: '' })
    );
  });
});
