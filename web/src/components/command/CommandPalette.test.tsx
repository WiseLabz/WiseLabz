import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { postSync, postConnectorsConnectorIdSync } from '../../api/generated/connectors/connectors';
import { toast } from '../../lib/toast';
import { useUi } from '../../store/ui';
import { CommandPalette } from './CommandPalette';

vi.mock('../../api/generated/connectors/connectors', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/connectors/connectors')>()),
  useGetConnectors: () => ({
    data: [
      {
        id: 'svc-1',
        name: 'Service One',
        category: 'virtualization',
        type: 'proxmox',
        enabled: true,
      },
    ],
  }),
  postSync: vi.fn().mockResolvedValue({}),
  postConnectorsConnectorIdSync: vi.fn().mockResolvedValue({}),
}));

vi.mock('../../api/generated/docs/docs', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/docs/docs')>()),
  useGetDocsTree: () => ({ data: { children: [] } }),
}));

const { searchHook } = vi.hoisted(() => ({ searchHook: vi.fn() }));
vi.mock('../../api/generated/search/search', () => ({ useGetSearch: searchHook }));
function Location() {
  return (
    <output data-testid="location">
      {useLocation().pathname}
      {useLocation().search}
    </output>
  );
}

vi.mock('../../hooks/useRole', () => ({ useCanMutate: () => true }));
vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe('CommandPalette', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchHook.mockReturnValue({ data: { docs: [], runbooks: [], entities: [] } });
    HTMLElement.prototype.scrollIntoView = vi.fn();
    useUi.setState({ paletteOpen: true });
  });
  afterEach(() => useUi.setState({ paletteOpen: false }));

  it('shows no matches for a server query and keeps See all results ungrouped', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
        </MemoryRouter>
      </QueryClientProvider>
    );

    fireEvent.change(screen.getByRole('combobox'), { target: { value: '☃☃' } });
    const dialog = screen.getByRole('dialog');

    await waitFor(() => expect(searchHook.mock.lastCall?.[0]).toHaveProperty('q', '☃☃'));
    expect(screen.getByText('No matches for “☃☃”')).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Entities' })).not.toBeInTheDocument();
    const all = screen.getByRole('option', { name: 'See all results' });
    expect(all.closest('[role="group"]')).toBeNull();
    fireEvent.keyDown(dialog, { key: 'ArrowDown' });
    fireEvent.keyDown(dialog, { key: 'ArrowUp' });
    expect(screen.getByRole('combobox')).toHaveAttribute('aria-activedescendant', all.id);
    expect(useUi.getState().paletteOpen).toBe(true);
  });

  it('exposes combobox/listbox semantics and closes on Escape from anywhere', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
        </MemoryRouter>
      </QueryClientProvider>
    );

    const input = screen.getByRole('combobox');
    expect(screen.getByRole('dialog')).toHaveAttribute('aria-modal', 'true');
    expect(input).toHaveAccessibleName();
    expect(input).toHaveAttribute('aria-controls', screen.getByRole('listbox').id);
    const active = input.getAttribute('aria-activedescendant');
    expect(active).toBeTruthy();
    expect(document.getElementById(active!)).toHaveAttribute('role', 'option');

    fireEvent.keyDown(document.body, { key: 'Escape' });
    expect(useUi.getState().paletteOpen).toBe(false);
  });
  it.each([
    ['Sync all', null],
    ['Sync Service One', 'svc-1'],
  ])('sends a real REST sync request for %s without a mock WebSocket', async (label, id) => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
        </MemoryRouter>
      </QueryClientProvider>
    );
    fireEvent.click(screen.getByRole('option', { name: new RegExp(`^${label}`) }));
    await waitFor(() =>
      expect(id ? postConnectorsConnectorIdSync : postSync).toHaveBeenCalledOnce()
    );
    if (id) expect(postConnectorsConnectorIdSync).toHaveBeenCalledWith(id);
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Sync started'));
  });

  it('surfaces a failed REST sync request', async () => {
    vi.mocked(postSync).mockRejectedValueOnce(new Error('offline'));
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
        </MemoryRouter>
      </QueryClientProvider>
    );
    fireEvent.click(screen.getByRole('option', { name: /^Sync all/ }));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(toast.success).not.toHaveBeenCalled();
  });
  it('debounces server groups, keeps full-text hits, and opens their destinations', async () => {
    searchHook.mockReturnValue({
      data: {
        docs: [{ id: 'd', title: 'Recovery notes', snippet: 'IP 10.0.0.1' }],
        runbooks: [{ id: 'r', title: 'Restart safely', snippet: 'Procedure' }],
        entities: [
          {
            connectorId: 'c',
            connectorName: 'Gateway',
            docId: 'd',
            kind: 'device',
            name: 'edge',
            ip: '10.0.0.1',
          },
        ],
      },
    });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
          <Location />
        </MemoryRouter>
      </QueryClientProvider>
    );
    fireEvent.change(screen.getByRole('combobox'), { target: { value: '10.0.0.1' } });
    expect(searchHook.mock.lastCall?.[1]).toEqual({ query: { enabled: false } });
    await waitFor(() =>
      expect(screen.getByRole('option', { name: /Recovery notes/ })).toBeInTheDocument()
    );
    expect(screen.getByRole('group', { name: 'Runbooks' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Entities' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /Restart safely/ })).toBeInTheDocument();
    expect(searchHook).toHaveBeenLastCalledWith(
      { q: '10.0.0.1', limit: 5 },
      { query: { enabled: true } }
    );
    fireEvent.click(screen.getByRole('option', { name: /edge/ }));
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/docs/d'));
  });
  it('offers See all results even without top hits', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
          <Location />
        </MemoryRouter>
      </QueryClientProvider>
    );
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'host & ip' } });
    expect(screen.queryByRole('group', { name: 'Entities' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('option', { name: 'See all results' }));
    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent('/search?q=host+%26+ip')
    );
  });
});
