import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import { CertificateExpiryPackOffer } from './CertificateExpiryPackOffer';

const role = vi.hoisted(() => ({ isAdmin: true }));
vi.mock('../../api/generated/me/me', () => ({
  useGetMe: () => ({ data: { role: role.isAdmin ? 'admin' : 'user' }, isLoading: false }),
}));

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
  role.isAdmin = true;
  localStorage.clear();
});
afterAll(() => server.close());

function renderOffer(onFinished = vi.fn()) {
  return {
    onFinished,
    ...render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <CertificateExpiryPackOffer requested onFinished={onFinished} />
      </QueryClientProvider>,
    ),
  };
}

describe('CertificateExpiryPackOffer', () => {
  it('offers the uninstalled pack and installs it in one action', async () => {
    let installed = false;
    server.use(
      http.get('/api/compliance/packs', () => HttpResponse.json({ items: [
        { id: 'certificate-expiry', name: 'Certificate expiry', description: '', installed },
      ] })),
      http.post('/api/compliance/packs/certificate-expiry/install', () => {
        installed = true;
        return HttpResponse.json({ installed: 6, skipped: 0 });
      }),
    );
    const { onFinished } = renderOffer();

    fireEvent.click(await screen.findByRole('button', { name: 'Install certificate expiry rules' }));
    await waitFor(() => expect(onFinished).toHaveBeenCalledOnce());
    expect(installed).toBe(true);
  });

  it('does not offer a pack that is already installed', async () => {
    const onFinished = vi.fn();
    server.use(
      http.get('/api/compliance/packs', () => HttpResponse.json({ items: [
        { id: 'certificate-expiry', name: 'Certificate expiry', description: '', installed: true },
      ] })),
    );
    renderOffer(onFinished);

    await waitFor(() => expect(onFinished).toHaveBeenCalledOnce());
    expect(screen.queryByRole('button', { name: 'Install certificate expiry rules' })).not.toBeInTheDocument();
  });

  it('does not request the admin-gated pack list or show an offer without install permission', async () => {
    role.isAdmin = false;
    let requested = false;
    server.use(http.get('/api/compliance/packs', () => {
      requested = true;
      return HttpResponse.json({ items: [] });
    }));
    const { onFinished } = renderOffer();

    await waitFor(() => expect(onFinished).toHaveBeenCalledOnce());
    expect(requested).toBe(false);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('keeps the offer usable when localStorage throws', async () => {
    server.use(http.get('/api/compliance/packs', () => HttpResponse.json({ items: [
      { id: 'certificate-expiry', name: 'Certificate expiry', description: '', installed: false },
    ] })));
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('storage blocked'); });
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('storage blocked'); });
    try {
      const { onFinished } = renderOffer();
      expect(await screen.findByRole('button', { name: 'Not now' })).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'Not now' }));
      expect(onFinished).toHaveBeenCalledOnce();
    } finally {
      getItem.mockRestore();
      setItem.mockRestore();
    }
  });

  it('remembers a decline for the browser and skips later offers', async () => {
    let listRequests = 0;
    server.use(http.get('/api/compliance/packs', () => {
      listRequests += 1;
      return HttpResponse.json({ items: [
        { id: 'certificate-expiry', name: 'Certificate expiry', description: '', installed: false },
      ] });
    }));
    const first = renderOffer();
    fireEvent.click(await screen.findByRole('button', { name: 'Not now' }));
    await waitFor(() => expect(first.onFinished).toHaveBeenCalledOnce());
    first.unmount();

    const second = renderOffer();
    await waitFor(() => expect(second.onFinished).toHaveBeenCalledOnce());
    expect(listRequests).toBe(1);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
