import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { TopologyGraph } from '../../api/model';
import i18n from '../../i18n';
import { LANGUAGES } from '../../i18n/languages';
import { TopologyPage } from './TopologyPage';

// Unlike TopologyPage.test.tsx this suite renders the real React Flow, so the
// node Link click path and the edge renderer run against the actual library.
const { graphHook, pathHook } = vi.hoisted(() => ({ graphHook: vi.fn(), pathHook: vi.fn() }));

vi.mock('../../api/generated/topology/topology', () => ({
  useGetTopologyGraph: graphHook,
  useGetTopologyPath: pathHook,
}));
vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({ data: [] }),
}));
vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsTree: () => ({ data: undefined }),
  postDocsTopology: vi.fn(),
}));
vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => false }));

const graph: TopologyGraph = {
  truncated: false,
  nodes: [
    { id: 'identity-a', type: 'identity', name: 'Host Alpha', kind: 'host' },
    { id: 'identity-b', type: 'identity', name: 'VM Beta', kind: 'vm' },
  ],
  edges: [
    { id: 'edge-1', source: 'identity-a', target: 'identity-b', kind: 'runs_on', detail: 'vmid 7' },
  ],
};

// React Flow measures nodes through ResizeObserver; report every observed
// element once so nodes get dimensions and edges are drawn.
class TestResizeObserver {
  constructor(private callback: ResizeObserverCallback) {}
  observe(target: Element) {
    queueMicrotask(() =>
      this.callback(
        [{ target, contentRect: { width: 190, height: 64 } } as ResizeObserverEntry],
        this as unknown as ResizeObserver
      )
    );
  }
  unobserve() {}
  disconnect() {}
}
class TestDOMMatrix {
  m22 = 1;
  constructor(transform?: string) {
    const scale = /scale\(([\d.]+)\)/.exec(transform ?? '');
    if (scale) this.m22 = Number(scale[1]);
  }
}

const protoDescriptors = {
  offsetHeight: Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight'),
  offsetWidth: Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetWidth'),
  getBBox: Object.getOwnPropertyDescriptor(SVGElement.prototype, 'getBBox'),
};

function restore(target: object, key: keyof typeof protoDescriptors) {
  const original = protoDescriptors[key];
  if (original) Object.defineProperty(target, key, original);
  else delete (target as Record<string, unknown>)[key];
}

beforeEach(() => {
  graphHook.mockReturnValue({ data: graph, isLoading: false, isError: false, refetch: vi.fn() });
  pathHook.mockReturnValue({ data: undefined, isError: false, isLoading: false });
  vi.stubGlobal('ResizeObserver', TestResizeObserver);
  vi.stubGlobal('DOMMatrixReadOnly', TestDOMMatrix);
  Object.defineProperties(HTMLElement.prototype, {
    offsetHeight: { configurable: true, get: () => 600 },
    offsetWidth: { configurable: true, get: () => 900 },
  });
  Object.defineProperty(SVGElement.prototype, 'getBBox', {
    configurable: true,
    value: () => ({ x: 0, y: 0, width: 0, height: 0 }),
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  restore(HTMLElement.prototype, 'offsetHeight');
  restore(HTMLElement.prototype, 'offsetWidth');
  restore(SVGElement.prototype, 'getBBox');
});

describe('TopologyPage with the real React Flow', () => {
  it('renders nodes and an edge label, and a node link navigates to the entity', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/topology']}>
          <Routes>
            <Route path="/topology" element={<TopologyPage />} />
            <Route path="/entities/:id" element={<p>Entity page</p>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
    const region = screen.getByRole('region', { name: 'Topology graph' });
    expect(region.querySelector('.react-flow')).not.toBeNull();
    expect(await screen.findByText('runs_on · vmid 7')).toBeInTheDocument();
    const link = region.querySelector('a[aria-label="Host Alpha"]');
    expect(link).not.toBeNull();
    fireEvent.click(link!);
    expect(await screen.findByText('Entity page')).toBeInTheDocument();
  });

  it('keeps nodes clickable: React Flow must not mark them pointer-events: none', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/topology']}>
          <Routes>
            <Route path="/topology" element={<TopologyPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
    const region = screen.getByRole('region', { name: 'Topology graph' });
    await screen.findByText('runs_on · vmid 7');
    const nodes = region.querySelectorAll<HTMLElement>('.react-flow__node');
    expect(nodes.length).toBe(2);
    // jsdom has no layout, but React Flow sets this inline, so a real mouse
    // would be swallowed by the pane exactly when this is 'none'.
    nodes.forEach((node) => expect(getComputedStyle(node).pointerEvents).not.toBe('none'));
  });

  it('labels edges for assistive tech with names, not raw IDs', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/topology']}>
          <Routes>
            <Route path="/topology" element={<TopologyPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
    const region = screen.getByRole('region', { name: 'Topology graph' });
    await screen.findByText('runs_on · vmid 7');
    expect(
      region.querySelector('.react-flow__edge[aria-label="Host Alpha runs_on · vmid 7 VM Beta"]')
    ).not.toBeNull();
  });

  it('gives each node a single tab stop, and Enter/click on it navigates', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/topology']}>
          <Routes>
            <Route path="/topology" element={<TopologyPage />} />
            <Route path="/entities/:id" element={<p>Entity page</p>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
    const region = screen.getByRole('region', { name: 'Topology graph' });
    await screen.findByText('runs_on · vmid 7');
    // The wrapper must not be focusable: only the inner link is a tab stop.
    region
      .querySelectorAll<HTMLElement>('.react-flow__node')
      .forEach((node) => expect(node.getAttribute('tabindex')).toBeNull());
    expect(region.querySelectorAll('.react-flow__node a[href]').length).toBe(2);
    const link = region.querySelector<HTMLAnchorElement>('a[aria-label="Host Alpha"]')!;
    expect(link.getAttribute('href')).toBe('/entities/identity-a');
    // Keyboard activation of a link is a click; the link is what navigates.
    fireEvent.click(link);
    expect(await screen.findByText('Entity page')).toBeInTheDocument();
  });

  it('builds the edge label from the translated template', async () => {
    i18n.addResourceBundle('pt-BR', 'translation', await LANGUAGES['pt-BR']!.load!());
    await i18n.changeLanguage('pt-BR');
    try {
      render(
        <QueryClientProvider client={new QueryClient()}>
          <MemoryRouter initialEntries={['/topology']}>
            <Routes>
              <Route path="/topology" element={<TopologyPage />} />
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>
      );
      const region = screen.getByRole('region', { name: 'Grafo da topologia' });
      await screen.findByText('runs_on · vmid 7');
      expect(
        region.querySelector('.react-flow__edge[aria-label="Host Alpha → VM Beta: runs_on · vmid 7"]')
      ).not.toBeNull();
    } finally {
      await i18n.changeLanguage('en');
    }
  });
});
