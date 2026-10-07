import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import { ExpiringCertificatesWidget } from './widgets';
import { widgetsFromWire } from '../../store/dashboard';

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  server.resetHandlers();
  cleanup();
});
afterAll(() => server.close());

function renderWidget() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <ExpiringCertificatesWidget title="Expiring certificates" icon={null} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ExpiringCertificatesWidget', () => {
  it('shows the expiry bands, unreachable marker, and entity links', async () => {
    server.use(http.get('/api/certificates', () => HttpResponse.json([
      { name: 'expired.lab:443', connectorId: 'c1', connectorName: 'TLS Probe', externalId: 'expired.lab:443', entityId: 'entity-1', notAfter: '2026-10-01T00:00:00Z', daysLeft: -1, unreachable: true },
      { name: 'soon.lab:443', connectorId: 'c1', connectorName: 'TLS Probe', externalId: 'soon.lab:443', notAfter: '2026-10-14T00:00:00Z', daysLeft: 7, unreachable: false },
      { name: 'month.lab:443', connectorId: 'c2', connectorName: 'NPM', externalId: 'month.lab:443', notAfter: '2026-11-06T00:00:00Z', daysLeft: 30, unreachable: false },
      { name: 'later.lab:443', connectorId: 'c2', connectorName: 'NPM', externalId: 'later.lab:443', notAfter: '2026-11-07T00:00:00Z', daysLeft: 31, unreachable: false },
    ])));
    renderWidget();

    expect(await screen.findByText('expired.lab:443')).toBeInTheDocument();
    expect(screen.getByText('Expired')).toBeInTheDocument();
    expect(screen.getByText('7 days or fewer')).toBeInTheDocument();
    expect(screen.getByText('30 days or fewer')).toBeInTheDocument();
    expect(screen.getByText('Later')).toBeInTheDocument();
    expect(screen.getByText('Unreachable')).toBeInTheDocument();
    expect(screen.getByText('Expired less than a day ago')).toBeInTheDocument();
    expect(screen.getByText('7 days left')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /expired\.lab:443/ })).toHaveAttribute('href', '/entities/entity-1');
    expect(screen.getByRole('link', { name: /soon\.lab:443/ })).toHaveAttribute('href', '/services/c1');
  });

  it('uses singular and plural wording for days left and days since expiry', async () => {
    server.use(http.get('/api/certificates', () => HttpResponse.json([
      { name: 'old.lab:443', connectorId: 'c1', connectorName: 'TLS Probe', externalId: 'old.lab:443', notAfter: '2026-09-01T00:00:00Z', daysLeft: -4, unreachable: false },
      { name: 'yesterday.lab:443', connectorId: 'c1', connectorName: 'TLS Probe', externalId: 'yesterday.lab:443', notAfter: '2026-10-05T00:00:00Z', daysLeft: -2, unreachable: false },
      { name: 'tomorrow.lab:443', connectorId: 'c1', connectorName: 'TLS Probe', externalId: 'tomorrow.lab:443', notAfter: '2026-10-08T00:00:00Z', daysLeft: 1, unreachable: false },
    ])));
    renderWidget();

    expect(await screen.findByText('Expired 3 days ago')).toBeInTheDocument();
    expect(screen.getByText('Expired 1 day ago')).toBeInTheDocument();
    expect(screen.getByText('1 day left')).toBeInTheDocument();
  });

  it('explains how to add a source when no certificates are available', async () => {
    server.use(http.get('/api/certificates', () => HttpResponse.json([])));
    renderWidget();

    expect(await screen.findByText('No certificates to show')).toBeInTheDocument();
    expect(screen.getByText(/add a tls probe or nginx proxy manager connector/i)).toBeInTheDocument();
  });
});

describe('ExpiringCertificatesWidget default', () => {
  it('is appended disabled to a saved layout that predates it', () => {
    const layout = widgetsFromWire([
      { id: 'roster', type: 'service_status', x: 0, y: 0, w: 4, h: 1, enabled: true },
    ]);
    expect(layout.find((widget) => widget.id === 'certificates')?.enabled).toBe(false);
    expect(layout.find((widget) => widget.id === 'roster')?.enabled).toBe(true);
  });
});
