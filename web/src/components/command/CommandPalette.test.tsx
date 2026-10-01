import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
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

vi.mock('../../hooks/useRole', () => ({ useCanMutate: () => true }));
vi.mock('../../lib/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe('CommandPalette', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    HTMLElement.prototype.scrollIntoView = vi.fn();
    useUi.setState({ paletteOpen: true });
  });
  afterEach(() => useUi.setState({ paletteOpen: false }));

  it('ignores navigation and Enter when a search has no matches', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <CommandPalette />
        </MemoryRouter>
      </QueryClientProvider>
    );

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'no matching command' } });
    const dialog = screen.getByRole('dialog');

    expect(() => {
      fireEvent.keyDown(dialog, { key: 'ArrowDown' });
      fireEvent.keyDown(dialog, { key: 'ArrowUp' });
      fireEvent.keyDown(dialog, { key: 'Enter' });
    }).not.toThrow();
    expect(screen.getByText('No matches for “no matching command”')).toBeInTheDocument();
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
});
