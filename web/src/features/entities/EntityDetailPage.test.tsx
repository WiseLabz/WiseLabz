import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { EntityDetailPage } from './EntityDetailPage';

const { entityHook } = vi.hoisted(() => ({ entityHook: vi.fn() }));
vi.mock('../../api/generated/search/search', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/search/search')>()),
  useGetEntitiesId: (id: string) => entityHook(id),
}));

function mount() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={['/entities/entity-1']}>
        <Routes><Route path="/entities/:id" element={<EntityDetailPage />} /></Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('EntityDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    entityHook.mockReturnValue({
      data: {
        id: 'entity-1', kind: 'vm', name: 'router', gone: true,
        members: [{ connectorId: 'c1', kind: 'vm', ref: '42', name: 'router' }],
        relatedByIp: [], neighbors: [], history: [], findings: [], onReportingConnectors: [], runbooks: [],
      },
      isLoading: false, isError: false, refetch: vi.fn(),
    });
  });

  it('shows the no-longer-observed banner and the visible member', () => {
    mount();
    expect(screen.getByRole('status')).toHaveTextContent('This entity is no longer observed.');
    expect(screen.getByRole('heading', { level: 1, name: 'router' })).toBeInTheDocument();
    expect(entityHook).toHaveBeenCalledWith('entity-1');
  });
});
