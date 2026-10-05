import { fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError } from 'axios';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguagePreference } from '../../i18n';
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

const finding = (id: string, severity: string, status: string) => ({
  id, connectorId: 'c1', checkType: 'compliance', severity, title: `finding ${id}`, description: '', status,
  detectedCount: 1, firstDetectedAt: '2026-09-01T00:00:00Z', lastSeenAt: '2026-09-01T00:00:00Z',
});

const base = {
  id: 'entity-1', kind: 'vm', name: 'router', gone: false,
  members: [{ connectorId: 'c1', connectorName: 'Proxmox lab', docId: 'doc-1', kind: 'vm', ref: '42', name: 'router' }],
  relatedByIp: [], neighbors: [], history: [], findings: [], onReportingConnectors: [], runbooks: [],
};

const statusError = (status: number) =>
  new AxiosError('failed', 'ERR_BAD_REQUEST', undefined, undefined, { status } as never);

describe('EntityDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void setLanguagePreference('en');
    entityHook.mockReturnValue({ data: { ...base, gone: true }, isLoading: false, isError: false, refetch: vi.fn() });
  });

  afterEach(async () => {
    await setLanguagePreference('en');
  });

  it('shows the no-longer-observed banner and the visible member', () => {
    mount();
    expect(screen.getByRole('status')).toHaveTextContent('This entity is no longer observed.');
    expect(screen.getByRole('heading', { level: 1, name: 'router' })).toBeInTheDocument();
    expect(entityHook).toHaveBeenCalledWith('entity-1');
  });

  it('shows the loading skeleton', () => {
    entityHook.mockReturnValue({ data: undefined, isLoading: true, isError: false, refetch: vi.fn() });
    mount();
    expect(screen.getByLabelText('Loading')).toBeInTheDocument();
  });

  it('shows not-found without a retry button on 404', () => {
    const refetch = vi.fn();
    entityHook.mockReturnValue({ data: undefined, isLoading: false, isError: true, error: statusError(404), refetch });
    mount();
    expect(screen.getByRole('alert')).toHaveTextContent('Entity not found');
    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();
  });

  it('shows the load error with a working retry on other failures', () => {
    const refetch = vi.fn();
    entityHook.mockReturnValue({ data: undefined, isLoading: false, isError: true, error: statusError(500), refetch });
    mount();
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load this entity');
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetch).toHaveBeenCalledOnce();
  });

  it('names connectors and links each member to its connector and doc', () => {
    mount();
    expect(screen.getByRole('link', { name: 'Proxmox lab' })).toHaveAttribute('href', '/services/c1');
    expect(screen.getByRole('link', { name: 'Connector doc' })).toHaveAttribute('href', '/docs/doc-1');
    expect(screen.queryByText('c1')).not.toBeInTheDocument();
  });

  it('shows when a departed member was last observed', () => {
    entityHook.mockReturnValue({
      data: { ...base, members: [{ ...base.members[0], goneAt: '2026-09-01T00:00:00Z' }] },
      isLoading: false, isError: false, refetch: vi.fn(),
    });
    mount();
    expect(screen.getByText(/No longer observed since/)).toBeInTheDocument();
  });

  it('links neighbours and IP links to other visible entities only', () => {
    const self = { connectorId: 'c1', kind: 'vm', name: 'router', ref: '42' };
    entityHook.mockReturnValue({
      data: {
        ...base,
        neighbors: [{ kind: 'dependency', from: self, to: { connectorId: 'c1', kind: 'vm', name: 'switch', ref: '7', entityId: 'entity-2' } }],
        relatedByIp: [{ reason: 'IP address', from: self, to: { connectorId: 'c1', kind: 'vm', name: 'orphan', ref: '8' } }],
      },
      isLoading: false, isError: false, refetch: vi.fn(),
    });
    mount();
    expect(screen.getByRole('link', { name: 'switch' })).toHaveAttribute('href', '/entities/entity-2');
    expect(screen.queryByRole('link', { name: 'orphan' })).not.toBeInTheDocument();
    expect(screen.getByText('orphan', { exact: false })).toBeInTheDocument();
  });

  it.each([
    ['en', 'critical · open'],
    ['pt-BR', 'crítico · aberto'],
  ])('translates finding severity and status (%s)', async (lng, label) => {
    await setLanguagePreference(lng as 'en' | 'pt-BR');
    entityHook.mockReturnValue({
      data: { ...base, findings: [finding('f1', 'critical', 'open')] },
      isLoading: false, isError: false, refetch: vi.fn(),
    });
    mount();
    expect(screen.getByText(label)).toBeInTheDocument();
  });
});
