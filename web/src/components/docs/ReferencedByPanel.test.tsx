import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ReferencedByPanel } from './ReferencedByPanel';

const { request } = vi.hoisted(() => ({ request: vi.fn() }));
vi.mock('../../api/generated/docs/docs', () => ({
  getDocsDocIdBacklinks: request,
  getGetDocsDocIdBacklinksQueryKey: (id: string) => [`/docs/${id}/backlinks`],
}));
vi.mock('../../api/generated/search/search', () => ({
  getEntitiesIdBacklinks: request,
  getGetEntitiesIdBacklinksQueryKey: (id: string) => [`/entities/${id}/backlinks`],
}));

afterEach(() => {
  cleanup();
  request.mockReset();
});

describe('ReferencedByPanel', () => {
  it('lists source docs returned by the backlinks endpoint', async () => {
    request.mockResolvedValue([{ id: 'doc-source', title: 'Maintenance notes' }]);
    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <MemoryRouter>
          <ReferencedByPanel type="entities" id="entity-target" />
        </MemoryRouter>
      </QueryClientProvider>
    );
    const link = await screen.findByRole('link', { name: 'Maintenance notes' });
    expect(link.getAttribute('href')).toBe('/docs/doc-source');
    expect(request).toHaveBeenCalledWith('entity-target');
  });
});
