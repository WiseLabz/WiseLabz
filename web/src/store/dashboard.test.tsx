import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import { useDashboard, useWidgetPolling, useRangeDays, DEFAULT_LAYOUT, type WidgetId } from './dashboard';

const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

beforeEach(() => {
  localStorage.clear();
  useDashboard.setState({ layout: DEFAULT_LAYOUT, hydrated: false });
});

describe('dashboard store', () => {
  it('hydrates layout from GET /api/dashboard/layout', async () => {
    server.use(
      http.get('/api/dashboard/layout', () =>
        HttpResponse.json({
          widgets: [{ id: 'docs', type: 'docs_health', x: 0, y: 0, w: 6, h: 1, enabled: false }],
        })
      )
    );

    await useDashboard.getState().hydrate();

    const layout = useDashboard.getState().layout;
    const docs = layout.find((w) => w.id === 'docs');
    expect(docs?.enabled).toBe(false);
    expect(useDashboard.getState().hydrated).toBe(true);
  });

  it('falls back to the cached/default layout when the API call fails', async () => {
    server.use(http.get('/api/dashboard/layout', () => HttpResponse.error()));

    await useDashboard.getState().hydrate();

    expect(useDashboard.getState().layout).toEqual(DEFAULT_LAYOUT);
    expect(useDashboard.getState().hydrated).toBe(true);
  });

  it('toggle persists the updated layout via PUT /api/dashboard/layout', async () => {
    let sentWidgets: unknown[] = [];
    server.use(
      http.put('/api/dashboard/layout', async ({ request }) => {
        const body = (await request.json()) as { widgets: unknown[] };
        sentWidgets = body.widgets;
        return HttpResponse.json(body);
      })
    );

    useDashboard.getState().toggle('alerts');

    expect(useDashboard.getState().layout.find((w) => w.id === 'alerts')?.enabled).toBe(false);
    await vi.waitFor(() => expect(sentWidgets.length).toBeGreaterThan(0));
    const alertsWire = sentWidgets.find((w) => (w as { id: string }).id === 'alerts') as {
      enabled: boolean;
    };
    expect(alertsWire.enabled).toBe(false);
  });

  it('setOrder reorders the layout locally', () => {
    const ids = useDashboard
      .getState()
      .layout.map((w) => w.id)
      .reverse() as WidgetId[];
    server.use(http.put('/api/dashboard/layout', () => HttpResponse.json({ widgets: [] })));

    useDashboard.getState().setOrder(ids);

    expect(useDashboard.getState().layout.map((w) => w.id)).toEqual(ids);
  });

  it('reset restores the administrator default via POST /api/dashboard/layout/reset', async () => {
    server.use(
      http.post('/api/dashboard/layout/reset', () =>
        HttpResponse.json({
          widgets: [{ id: 'roster', type: 'service_status', x: 0, y: 0, w: 4, h: 1, enabled: true }],
        })
      )
    );
    useDashboard.setState({
      layout: [{ id: 'roster', enabled: false, span: 4 }, ...DEFAULT_LAYOUT.slice(1)],
    });

    await useDashboard.getState().reset();

    expect(useDashboard.getState().layout.find((w) => w.id === 'roster')?.enabled).toBe(true);
  });
});

describe('useWidgetPolling', () => {
  it('derives pollingEnabled/refetchInterval from the widget layout and toggles it', () => {
    server.use(http.put('/api/dashboard/layout', () => HttpResponse.json({ widgets: [] })));
    const { result } = renderHook(() => useWidgetPolling('alerts'));

    expect(result.current.pollingEnabled).toBe(false);
    expect(result.current.refetchInterval).toBe(false);

    act(() => result.current.togglePolling());

    expect(result.current.pollingEnabled).toBe(true);
    expect(result.current.refetchInterval).toBe(30_000);
  });
});

describe('useRangeDays', () => {
  it('resolves the ?range= URL param to a day count, defaulting to 7d', () => {
    const { result } = renderHook(() => useRangeDays(), {
      wrapper: ({ children }) => <MemoryRouter initialEntries={['/?range=30d']}>{children}</MemoryRouter>,
    });
    expect(result.current).toBe(30);

    const { result: defaultResult } = renderHook(() => useRangeDays(), {
      wrapper: ({ children }) => <MemoryRouter>{children}</MemoryRouter>,
    });
    expect(defaultResult.current).toBe(7);
  });
});
