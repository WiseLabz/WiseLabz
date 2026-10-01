import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import '../../i18n';
import { toast } from '../../lib/toast';
import { ShareDialog } from './ShareDialog';

// jsdom doesn't implement <dialog>'s imperative API (used by ui/Dialog.tsx).
HTMLDialogElement.prototype.showModal = vi.fn(function (this: HTMLDialogElement) {
  this.open = true;
});
HTMLDialogElement.prototype.close = vi.fn(function (this: HTMLDialogElement) {
  this.open = false;
});

const { createMock } = vi.hoisted(() => ({ createMock: vi.fn() }));
vi.mock('../../api/generated/docs/docs', () => ({
  postDocsShareLinks: createMock,
  getGetDocsShareLinksQueryKey: () => ['/docs/share-links'],
}));

const writeText = vi.fn().mockResolvedValue(undefined);
Object.assign(navigator, { clipboard: { writeText } });

function Wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
}

function renderDialog(open = true) {
  return render(
    <ShareDialog open={open} onClose={vi.fn()} node={{ docId: 'c1', title: 'Connector One' }} />,
    { wrapper: Wrapper }
  );
}

describe('ShareDialog (#240 PR2)', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.restoreAllMocks();
    Object.assign(navigator, { clipboard: { writeText } });
  });

  it('renders the subtitle naming the shared node', () => {
    renderDialog();
    expect(screen.getByText(/Connector One/)).toBeInTheDocument();
  });

  it('defaults to the 7-day TTL preset selected', () => {
    renderDialog();
    const weekButton = screen.getByRole('button', { name: '7 days' });
    expect(weekButton.className).toMatch(/border-accent-primary/);
  });

  it('creating a link calls the mutation with docTreeRoot and a future expiresAt', async () => {
    createMock.mockResolvedValue({ id: 's1', token: 'wlz_share_xyz' });
    renderDialog();
    fireEvent.click(screen.getByRole('button', { name: /create link/i }));
    await waitFor(() => expect(createMock).toHaveBeenCalledTimes(1));
    const [body] = createMock.mock.calls[0];
    expect(body.docTreeRoot).toBe('c1');
    expect(new Date(body.expiresAt).getTime()).toBeGreaterThan(Date.now());
  });

  it('shows the one-time token URL after creation and copies it', async () => {
    createMock.mockResolvedValue({
      id: 's1',
      token: 'wlz_share_xyz',
      docTreeRoot: 'c1',
      createdBy: 'u1',
      createdAt: '',
      expiresAt: '',
      revokedAt: '',
      lastAccessedAt: '',
    });
    renderDialog();
    fireEvent.click(screen.getByRole('button', { name: /create link/i }));

    await waitFor(() => expect(screen.getByText(/won't be shown again/i)).toBeInTheDocument());
    const input = screen.getByDisplayValue(/\/share\/wlz_share_xyz$/) as HTMLInputElement;
    expect(input).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /copy/i }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(input.value));
    expect(await screen.findByText(/copied/i)).toBeInTheDocument();
  });

  it.each([true, false])('uses the HTTP fallback and reports copy success=%s', async (success) => {
    Object.assign(navigator, { clipboard: undefined });
    const execCommand = vi.fn().mockReturnValue(success);
    Object.defineProperty(document, 'execCommand', { value: execCommand, configurable: true });
    const error = vi.spyOn(toast, 'error');
    createMock.mockResolvedValue({ token: 'wlz_share_xyz' });
    renderDialog();
    fireEvent.click(screen.getByRole('button', { name: /create link/i }));
    const input = await screen.findByDisplayValue(/\/share\/wlz_share_xyz$/);
    fireEvent.click(screen.getByRole('button', { name: /^copy$/i }));
    await waitFor(() => expect(execCommand).toHaveBeenCalledWith('copy'));
    if (success) {
      expect(await screen.findByText(/copied/i)).toBeInTheDocument();
      expect(error).not.toHaveBeenCalled();
    } else {
      await waitFor(() => expect(error).toHaveBeenCalled());
      expect(screen.queryByText(/^copied$/i)).not.toBeInTheDocument();
      expect(input).toBeVisible();
    }
  });

  it('renders nothing when no node is selected', () => {
    const { container } = render(<ShareDialog open={true} onClose={vi.fn()} node={null} />);
    // Dialog chrome (title bar) still renders; the body content area is empty.
    expect(container.querySelector('input')).not.toBeInTheDocument();
  });
});
