import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { ComponentType, ReactNode } from 'react';
import type { TopologyGraph } from '../../api/model';
import '../../i18n';
import { TopologyPage } from './TopologyPage';

const { graphHook, pathHook, connectorsHook, docsHook, postTopology, role } = vi.hoisted(() => ({
  graphHook: vi.fn(),
  pathHook: vi.fn(),
  connectorsHook: vi.fn(),
  docsHook: vi.fn(),
  postTopology: vi.fn(),
  role: { admin: false },
}));

vi.mock('../../api/generated/topology/topology', () => ({
  useGetTopologyGraph: graphHook,
  useGetTopologyPath: pathHook,
}));
vi.mock('../../api/generated/connectors/connectors', () => ({ useGetConnectors: connectorsHook }));
vi.mock('../../api/generated/docs/docs', () => ({
  useGetDocsTree: docsHook,
  postDocsTopology: postTopology,
}));
vi.mock('../../hooks/useRole', () => ({ useIsInstanceAdmin: () => role.admin }));
vi.mock('@xyflow/react', async () => {
  const React = await import('react');
  return {
    ReactFlow: ({
      nodes,
      edges,
      nodeTypes,
    }: {
      nodes: Array<{ id: string; data: unknown }>;
      edges: Array<{ id: string; data?: { highlighted?: boolean }; label?: string }>;
      nodeTypes: Record<string, ComponentType<never>>;
    }) => {
      const NodeView = nodeTypes.topology;
      return React.createElement('div', { 'data-testid': 'flow-canvas' }, [
        ...nodes.map((node) =>
          React.createElement(NodeView, { key: node.id, id: node.id, data: node.data } as never)
        ),
        ...edges.map((edge) =>
          React.createElement(
            'span',
            {
              key: edge.id,
              'data-testid': 'flow-edge',
              'data-highlighted': String(edge.data?.highlighted),
            },
            edge.label
          )
        ),
      ]);
    },
    Background: () => null,
    Controls: () => null,
    BaseEdge: () => null,
    EdgeLabelRenderer: ({ children }: { children: ReactNode }) => children,
    Position: { Left: 'left', Right: 'right' },
    getBezierPath: () => ['', 0, 0],
  };
});

const graph: TopologyGraph = {
  truncated: false,
  nodes: [
    { id: 'identity-a', type: 'identity', name: 'Host Alpha', kind: 'host' },
    { id: 'identity-b', type: 'identity', name: 'VM Beta', kind: 'vm' },
    {
      id: 'connector-a',
      type: 'node',
      name: 'Router Connector',
      kind: 'service',
      connectorId: 'connector-a',
      ref: 'connector-a',
    },
    {
      id: 'unresolved-a',
      type: 'node',
      name: 'Unresolved endpoint',
      kind: 'service',
      connectorId: 'connector-a',
      ref: 'unresolved-ref',
    },
  ],
  edges: [
    { id: 'edge-1', source: 'identity-a', target: 'identity-b', kind: 'runs_on' },
    {
      id: 'edge-2',
      source: 'connector-a',
      target: 'identity-a',
      kind: 'proxies_to',
      detail: 'port 443',
    },
    { id: 'edge-3', source: 'connector-a', target: 'unresolved-a', kind: 'dependency' },
  ],
};
const emptyGraph: TopologyGraph = { nodes: [], edges: [], truncated: false };

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

const rect = {
  x: 0,
  y: 0,
  width: 900,
  height: 600,
  top: 0,
  right: 900,
  bottom: 600,
  left: 0,
  toJSON: () => ({}),
};

beforeEach(() => {
  vi.clearAllMocks();
  role.admin = false;
  graphHook.mockReturnValue({ data: graph, isLoading: false, isError: false, refetch: vi.fn() });
  pathHook.mockReturnValue({ data: undefined, isError: false, isLoading: false });
  connectorsHook.mockReturnValue({ data: [{ id: 'connector-a', name: 'Router Connector' }] });
  docsHook.mockReturnValue({
    data: {
      docId: 'root',
      children: [
        {
          docId: 'connector-doc',
          serviceId: 'connector-a',
          kind: 'service',
          title: 'Router Connector',
        },
      ],
    },
  });
  postTopology.mockResolvedValue({ docId: 'generated-doc' });
  vi.stubGlobal('ResizeObserver', TestResizeObserver);
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    () => rect as DOMRect
  );
});

afterEach(() => vi.restoreAllMocks());

function renderTopology(url = '/topology') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[url]}>
        <Routes>
          <Route path="/topology" element={<TopologyPage />} />
          <Route path="/entities/:id" element={<p>Entity page</p>} />
          <Route path="/docs/:docId" element={<p>Document page</p>} />
          <Route path="/services/:id" element={<p>Connector page</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('TopologyPage', () => {
  it('starts with linked graph nodes and sends includeUnlinked when toggled', () => {
    graphHook.mockImplementation((params: { includeUnlinked?: boolean }) => ({
      data: params.includeUnlinked
        ? {
            ...graph,
            nodes: [
              ...graph.nodes,
              { id: 'lonely', type: 'identity', name: 'Lonely', kind: 'host' },
            ],
          }
        : graph,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    }));
    renderTopology();
    expect(
      within(screen.getByTestId('flow-canvas')).queryByRole('link', {
        name: 'Unresolved endpoint',
      })
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByTestId('flow-canvas')).queryByRole('link', { name: 'Lonely' })
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Show unlinked identities' }));
    expect(graphHook).toHaveBeenLastCalledWith({ includeUnlinked: true }, expect.anything());
    screen.getByText('Browse nodes (4)').closest('details')!.open = true;
    expect(
      within(screen.getByTestId('flow-canvas')).getByRole('link', { name: 'Lonely' })
    ).toBeInTheDocument();
  });

  it('passes connector and kind filters to the graph request', () => {
    renderTopology();
    fireEvent.change(screen.getByRole('combobox', { name: 'Connector' }), {
      target: { value: 'connector-a' },
    });
    fireEvent.change(screen.getByRole('combobox', { name: 'Entity kind' }), {
      target: { value: 'host' },
    });
    expect(graphHook).toHaveBeenLastCalledWith(
      { connector: 'connector-a', kind: 'host' },
      expect.anything()
    );
  });

  it('shows truncation and accessible node text', () => {
    graphHook.mockReturnValue({
      data: { ...graph, truncated: true },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    renderTopology();
    expect(screen.getByText(/limited to the first 2,000 nodes/)).toBeInTheDocument();
    screen.getByText('Browse nodes (3)').closest('details')!.open = true;
    expect(
      within(screen.getByTestId('flow-canvas')).getByRole('link', { name: 'VM Beta' })
    ).toBeInTheDocument();
  });

  it('submits a trace and lists the returned hops', async () => {
    pathHook.mockReturnValue({
      data: {
        found: true,
        hops: 1,
        truncated: false,
        path: [
          {
            connectorId: 'connector-a',
            connectorName: 'Router Connector',
            kind: 'host',
            name: 'Host Alpha',
            nodeId: 'identity-a',
          },
          {
            connectorId: 'connector-a',
            connectorName: 'Router Connector',
            kind: 'vm',
            name: 'VM Beta',
            nodeId: 'identity-b',
            edgeKind: 'runs_on',
          },
        ],
      },
      isError: false,
      isLoading: false,
    });
    renderTopology();
    fireEvent.change(screen.getByRole('textbox', { name: 'From' }), {
      target: { value: 'Host Alpha' },
    });
    fireEvent.change(screen.getByRole('textbox', { name: 'To' }), { target: { value: 'VM Beta' } });
    fireEvent.click(screen.getByRole('button', { name: 'Trace path' }));
    await screen.findByText('Host Alpha', { selector: 'li' });
    expect(pathHook).toHaveBeenLastCalledWith(
      { from: 'Host Alpha', to: 'VM Beta' },
      expect.objectContaining({ query: expect.objectContaining({ enabled: true }) })
    );
    await waitFor(() =>
      expect(
        screen
          .getAllByTestId('flow-edge')
          .some((edge) => edge.getAttribute('data-highlighted') === 'true')
      ).toBe(true)
    );
  });

  it('shows no path and trace request errors without hiding the graph', () => {
    pathHook.mockReturnValue({
      data: { found: false, truncated: false, path: [] },
      isError: false,
      isLoading: false,
    });
    const { rerender } = renderTopology('/topology?from=Host+Alpha&to=missing');
    expect(screen.getByText('No path found between those endpoints.')).toBeInTheDocument();
    expect(
      within(screen.getByTestId('flow-canvas')).getByRole('link', { name: 'Host Alpha' })
    ).toBeInTheDocument();
    pathHook.mockReturnValue({
      data: undefined,
      isError: true,
      error: { isAxiosError: true, response: { status: 400 } },
      isLoading: false,
    });
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/topology?from=Host+Alpha&to=missing']}>
          <Routes>
            <Route path="/topology" element={<TopologyPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Enter both endpoints');
  });

  it('navigates identity clicks to the entity and connector clicks to its doc', async () => {
    const { unmount } = renderTopology();
    screen.getByText('Browse nodes (3)').closest('details')!.open = true;
    fireEvent.click(
      within(screen.getByTestId('flow-canvas')).getByRole('link', { name: 'Host Alpha' })
    );
    expect(await screen.findByText('Entity page')).toBeInTheDocument();
    unmount();
    renderTopology();
    screen.getByText('Browse nodes (3)').closest('details')!.open = true;
    fireEvent.click(
      within(screen.getByTestId('flow-canvas')).getByRole('link', { name: 'Router Connector' })
    );
    expect(await screen.findByText('Document page')).toBeInTheDocument();
  });

  it('hides Mermaid export for non-admins and opens the generated doc for admins', async () => {
    const { unmount } = renderTopology();
    expect(screen.queryByRole('button', { name: 'Open Mermaid document' })).not.toBeInTheDocument();
    unmount();
    role.admin = true;
    renderTopology();
    fireEvent.click(screen.getByRole('button', { name: 'Open Mermaid document' }));
    expect(await screen.findByText('Document page')).toBeInTheDocument();
    expect(postTopology).toHaveBeenCalledOnce();
  });

  it('renders loading, empty, and retryable error states', () => {
    graphHook.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: vi.fn(),
    });
    const { unmount } = renderTopology();
    expect(screen.getByRole('status', { name: 'Loading' })).toBeInTheDocument();
    unmount();
    graphHook.mockReturnValue({
      data: emptyGraph,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    const emptyView = renderTopology();
    expect(screen.getByText('No topology yet')).toBeInTheDocument();
    emptyView.unmount();
    const refetch = vi.fn();
    graphHook.mockReturnValue({ data: undefined, isLoading: false, isError: true, refetch });
    renderTopology();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(refetch).toHaveBeenCalledOnce();
  });

  it('shows endpoint validation before submitting an invalid trace', () => {
    renderTopology();
    fireEvent.click(screen.getByRole('button', { name: 'Trace path' }));
    expect(screen.getByRole('alert')).toHaveTextContent('Enter both endpoints');
    expect(pathHook).toHaveBeenLastCalledWith(
      { from: '_', to: undefined },
      expect.objectContaining({ query: expect.objectContaining({ enabled: false }) })
    );
  });
});
