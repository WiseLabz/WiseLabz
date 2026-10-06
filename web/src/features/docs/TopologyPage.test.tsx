import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes, useLocation, useNavigationType, useParams } from 'react-router-dom';
import dagre from '@dagrejs/dagre';
import type { ComponentType, ReactNode } from 'react';
import type { TopologyGraph } from '../../api/model';
import '../../i18n';
import { TopologyPage } from './TopologyPage';

const { graphHook, pathHook, connectorsHook, docsHook, postTopology, role, fitView } = vi.hoisted(() => ({
  fitView: vi.fn(),
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
      children,
    }: {
      children?: ReactNode;
      nodes: Array<{ id: string; data: unknown }>;
      edges: Array<{ id: string; data?: { highlighted?: boolean }; label?: string }>;
      nodeTypes: Record<string, ComponentType<never>>;
    }) => {
      const NodeView = nodeTypes.topology;
      return React.createElement('div', { 'data-testid': 'flow-canvas' }, [
        children,
        ...nodes.map((node) =>
          React.createElement(NodeView, { key: node.id, id: node.id, data: node.data } as never)
        ),
        ...edges.map((edge) =>
          React.createElement(
            'span',
            {
              key: edge.id,
              'data-testid': 'flow-edge',
              'data-edge-id': edge.id,
              'data-highlighted': String(edge.data?.highlighted),
            },
            edge.label
          )
        ),
      ]);
    },
    useReactFlow: () => ({ fitView }),
    Background: () => null,
    Handle: () => null,
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
          docId: 'connector-a',
          serviceId: 'connector-a',
          kind: 'service',
          branch: true,
          title: 'Router Connector',
          children: [
            {
              docId: 'connector-doc',
              serviceId: 'connector-a',
              kind: 'service',
              title: 'Router Connector service doc',
            },
          ],
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

function LocationProbe() {
  const location = useLocation();
  const type = useNavigationType();
  return (
    <>
      <output data-testid="search">{location.search}</output>
      <output data-testid="nav-type">{type}</output>
    </>
  );
}

function DocumentRouteProbe() {
  const { docId } = useParams();
  return (
    <>
      <p>Document page</p>
      <output data-testid="document-route">{docId}</output>
    </>
  );
}

function renderTopology(url = '/topology') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[url]}>
        <LocationProbe />
        <Routes>
          <Route path="/topology" element={<TopologyPage />} />
          <Route path="/entities/:id" element={<p>Entity page</p>} />
          <Route path="/docs/:docId" element={<DocumentRouteProbe />} />
          <Route path="/services/:id" element={<p>Connector page</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

function highlightedEdgeIds() {
  return screen
    .getAllByTestId('flow-edge')
    .filter((edge) => edge.getAttribute('data-highlighted') === 'true')
    .map((edge) => edge.getAttribute('data-edge-id'));
}

const hostToVM = {
  found: true,
  hops: 1,
  truncated: false,
  path: [
    { connectorId: 'connector-a', kind: 'host', name: 'Host Alpha', nodeId: 'identity-a' },
    {
      connectorId: 'connector-a',
      kind: 'vm',
      name: 'VM Beta',
      nodeId: 'identity-b',
      fromNodeId: 'identity-a',
      edgeKind: 'runs_on',
    },
  ],
};

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
    expect(graphHook).toHaveBeenCalledWith({ includeUnlinked: true }, expect.anything());
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
    expect(graphHook).toHaveBeenCalledWith(
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
            fromNodeId: 'identity-a',
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
    await waitFor(() => expect(highlightedEdgeIds()).toEqual(['edge-1']));
    expect(screen.getByTestId('search')).toHaveTextContent('?from=Host+Alpha&to=VM+Beta');
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
    expect(screen.getByRole('alert')).toHaveTextContent('Enter a starting point');
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
    expect(screen.getByTestId('document-route')).toHaveTextContent('connector-doc');
  });

  it('opens connector details when its docs-tree branch has no service document', async () => {
    docsHook.mockReturnValue({
      data: {
        docId: 'root',
        title: 'Lab Documentation',
        kind: 'lab',
        children: [
          {
            docId: 'connector-a',
            serviceId: 'connector-a',
            kind: 'service',
            branch: true,
            title: 'Router Connector',
            children: [],
          },
        ],
      },
    });
    renderTopology();
    screen.getByText('Browse nodes (3)').closest('details')!.open = true;
    fireEvent.click(
      within(screen.getByTestId('flow-canvas')).getByRole('link', { name: 'Router Connector' })
    );

    expect(await screen.findByText('Connector page')).toBeInTheDocument();
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
    expect(screen.getByRole('alert')).toHaveTextContent('Enter a starting point');
    expect(pathHook).toHaveBeenLastCalledWith(
      { from: '_', to: undefined },
      expect.objectContaining({ query: expect.objectContaining({ enabled: false }) })
    );
  });

  it('writes filters to the URL by replacing history and restores them from the URL', () => {
    renderTopology();
    fireEvent.change(screen.getByRole('combobox', { name: 'Connector' }), {
      target: { value: 'connector-a' },
    });
    fireEvent.change(screen.getByRole('combobox', { name: 'Entity kind' }), {
      target: { value: 'host' },
    });
    fireEvent.click(screen.getByRole('checkbox', { name: 'Show unlinked identities' }));
    expect(screen.getByTestId('search')).toHaveTextContent(
      '?connector=connector-a&kind=host&includeUnlinked=true'
    );
    expect(screen.getByTestId('nav-type')).toHaveTextContent('REPLACE');
  });

  it('restores filters and trace from an initial URL', () => {
    pathHook.mockReturnValue({ data: hostToVM, isError: false, isLoading: false });
    renderTopology('/topology?connector=connector-a&kind=host&includeUnlinked=true&from=Host+Alpha&to=VM+Beta');
    expect(graphHook).toHaveBeenCalledWith(
      { connector: 'connector-a', kind: 'host', includeUnlinked: true },
      expect.anything()
    );
    expect(screen.getByRole('combobox', { name: 'Connector' })).toHaveValue('connector-a');
    expect(screen.getByRole('combobox', { name: 'Entity kind' })).toHaveValue('host');
    expect(screen.getByRole('checkbox', { name: 'Show unlinked identities' })).toBeChecked();
    expect(screen.getByRole('textbox', { name: 'From' })).toHaveValue('Host Alpha');
    expect(screen.getByRole('textbox', { name: 'To' })).toHaveValue('VM Beta');
    expect(pathHook).toHaveBeenLastCalledWith(
      { from: 'Host Alpha', to: 'VM Beta' },
      expect.objectContaining({ query: expect.objectContaining({ enabled: true }) })
    );
    expect(highlightedEdgeIds()).toEqual(['edge-1']);
  });

  it('keeps every kind selectable after a kind filter narrows the response', () => {
    graphHook.mockImplementation((params: { kind?: string }) => ({
      data: params.kind
        ? { ...graph, nodes: graph.nodes.filter((n) => n.kind === params.kind), edges: [] }
        : graph,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    }));
    renderTopology();
    const select = screen.getByRole('combobox', { name: 'Entity kind' });
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'All kinds',
      'host',
      'service',
      'vm',
    ]);
    fireEvent.change(select, { target: { value: 'host' } });
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'All kinds',
      'host',
      'service',
      'vm',
    ]);
    expect(select).toHaveValue('host');
  });

  it('submits the trace with Enter and allows a from-only trace', async () => {
    pathHook.mockReturnValue({ data: hostToVM, isError: false, isLoading: false });
    renderTopology();
    fireEvent.change(screen.getByRole('textbox', { name: 'From' }), {
      target: { value: 'Host Alpha' },
    });
    fireEvent.submit(screen.getByRole('form', { name: 'Trace a path' }));
    expect(screen.getByTestId('search')).toHaveTextContent('?from=Host+Alpha');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(pathHook).toHaveBeenLastCalledWith(
      { from: 'Host Alpha', to: undefined },
      expect.objectContaining({ query: expect.objectContaining({ enabled: true }) })
    );
    await waitFor(() => expect(highlightedEdgeIds()).toEqual(['edge-1']));
    expect(screen.getByRole('button', { name: 'Trace path' })).toHaveAttribute('type', 'submit');
  });

  it('says the path walk was limited when the endpoint truncates', () => {
    pathHook.mockReturnValue({
      data: { ...hostToVM, truncated: true },
      isError: false,
      isLoading: false,
    });
    renderTopology('/topology?from=Host+Alpha');
    expect(screen.getByText(/path was cut short/i)).toBeInTheDocument();
  });

  it('reports a failed Mermaid export', async () => {
    role.admin = true;
    postTopology.mockRejectedValue(new Error('forbidden'));
    renderTopology();
    fireEvent.click(screen.getByRole('button', { name: 'Open Mermaid document' }));
    expect(await screen.findByRole('alert')).toHaveTextContent("Couldn't export the Mermaid");
    expect(screen.queryByText('Document page')).not.toBeInTheDocument();
  });

  it('shows a filter-specific empty state that clears the filters', () => {
    graphHook.mockImplementation((params: { kind?: string }) => ({
      data: params.kind ? emptyGraph : graph,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    }));
    renderTopology('/topology?kind=rack&from=Host+Alpha');
    expect(screen.getByText('No topology matches these filters')).toBeInTheDocument();
    expect(screen.queryByText('No topology yet')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));
    expect(screen.getByTestId('search')).toHaveTextContent('?from=Host+Alpha');
    expect(screen.getByRole('link', { name: 'Host Alpha', hidden: true })).toBeInTheDocument();
  });

  it('merges parallel edges between the same pair into one labelled edge', () => {
    graphHook.mockReturnValue({
      data: {
        ...graph,
        edges: [
          ...graph.edges,
          { id: 'edge-1b', source: 'identity-a', target: 'identity-b', kind: 'resolves_to' },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    renderTopology();
    const edges = screen.getAllByTestId('flow-edge');
    expect(edges).toHaveLength(2);
    expect(edges.map((e) => e.textContent)).toContain('runs_on, resolves_to');
  });

  it('does not re-run the dagre layout for a trace or a filter keystroke', async () => {
    const layout = vi.spyOn(dagre, 'layout');
    pathHook.mockReturnValue({ data: hostToVM, isError: false, isLoading: false });
    renderTopology();
    const initial = layout.mock.calls.length;
    expect(initial).toBeGreaterThan(0);
    fireEvent.change(screen.getByRole('textbox', { name: 'From' }), {
      target: { value: 'Host Alpha' },
    });
    fireEvent.change(screen.getByRole('textbox', { name: 'To' }), { target: { value: 'VM Beta' } });
    fireEvent.click(screen.getByRole('button', { name: 'Trace path' }));
    await waitFor(() => expect(highlightedEdgeIds()).toEqual(['edge-1']));
    expect(layout.mock.calls.length).toBe(initial);
  });

  describe('trace highlight exactness', () => {
    const node = (id: string, name: string) => ({ id, type: 'identity' as const, name, kind: 'vm' });
    const branching: TopologyGraph = {
      truncated: false,
      nodes: [node('a', 'A'), node('b', 'B'), node('c', 'C'), node('d', 'D')],
      edges: [
        { id: 'ab', source: 'a', target: 'b', kind: 'runs_on' },
        { id: 'ac', source: 'a', target: 'c', kind: 'runs_on' },
        { id: 'bd', source: 'b', target: 'd', kind: 'runs_on' },
        // Decoys: edges between steps that are merely adjacent in BFS order.
        { id: 'bc', source: 'b', target: 'c', kind: 'runs_on' },
        { id: 'cd', source: 'c', target: 'd', kind: 'runs_on' },
      ],
    };
    const step = (name: string, extra: Record<string, unknown> = {}) => ({
      connectorId: 'c1',
      kind: 'vm',
      name,
      nodeId: name.toLowerCase(),
      ...extra,
    });

    it('highlights exactly the edges a from-only walk followed', async () => {
      graphHook.mockReturnValue({ data: branching, isLoading: false, isError: false });
      pathHook.mockReturnValue({
        data: {
          found: true,
          truncated: false,
          path: [
            step('A'),
            step('B', { fromNodeId: 'a', edgeKind: 'runs_on' }),
            step('C', { fromNodeId: 'a', edgeKind: 'runs_on' }),
            step('D', { fromNodeId: 'b', edgeKind: 'runs_on' }),
          ],
        },
        isError: false,
        isLoading: false,
      });
      renderTopology('/topology?from=A');
      await waitFor(() => expect(highlightedEdgeIds().sort()).toEqual(['ab', 'ac', 'bd']));
    });

    it('decides by edge direction when both directions exist', async () => {
      graphHook.mockReturnValue({
        data: {
          ...branching,
          nodes: branching.nodes.slice(0, 2),
          edges: [
            { id: 'ab', source: 'a', target: 'b', kind: 'runs_on' },
            { id: 'ba', source: 'b', target: 'a', kind: 'runs_on' },
          ],
        },
        isLoading: false,
        isError: false,
      });
      const path = (reversed: boolean) => ({
        data: {
          found: true,
          hops: 1,
          truncated: false,
          path: [
            step('A'),
            step('B', {
              fromNodeId: 'a',
              edgeKind: 'runs_on',
              ...(reversed ? { edgeReversed: true } : {}),
            }),
          ],
        },
        isError: false,
        isLoading: false,
      });
      pathHook.mockReturnValue(path(false));
      const view = renderTopology('/topology?from=A&to=B');
      await waitFor(() => expect(highlightedEdgeIds()).toEqual(['ab']));
      view.unmount();
      pathHook.mockReturnValue(path(true));
      renderTopology('/topology?from=A&to=B');
      await waitFor(() => expect(highlightedEdgeIds()).toEqual(['ba']));
    });

    it('highlights a merged edge only when a part on it is the traversed kind', async () => {
      graphHook.mockReturnValue({
        data: {
          ...branching,
          nodes: branching.nodes.slice(0, 2),
          edges: [
            { id: 'ab1', source: 'a', target: 'b', kind: 'runs_on' },
            { id: 'ab2', source: 'a', target: 'b', kind: 'resolves_to' },
          ],
        },
        isLoading: false,
        isError: false,
      });
      pathHook.mockReturnValue({
        data: {
          found: true,
          hops: 1,
          truncated: false,
          path: [step('A'), step('B', { fromNodeId: 'a', edgeKind: 'proxies_to' })],
        },
        isError: false,
        isLoading: false,
      });
      renderTopology('/topology?from=A&to=B');
      expect(highlightedEdgeIds()).toEqual([]);
    });
  });

  it('collapses duplicate edge parts into one label line', () => {
    graphHook.mockReturnValue({
      data: {
        ...graph,
        edges: [
          { id: 'x1', source: 'identity-a', target: 'identity-b', kind: 'runs_on', sourceLabel: 'p1' },
          { id: 'x2', source: 'identity-a', target: 'identity-b', kind: 'runs_on', sourceLabel: 'p2' },
        ],
      },
      isLoading: false,
      isError: false,
    });
    renderTopology();
    expect(screen.getByTestId('flow-edge')).toHaveTextContent(/^runs_on$/);
  });

  describe('viewport refit', () => {
    const withNodes = (ids: string[]): TopologyGraph => ({
      truncated: false,
      nodes: graph.nodes.filter((n) => ids.includes(n.id)),
      edges: graph.edges,
    });

    it('refits when the node set changes but not on trace or styling changes', async () => {
      graphHook.mockImplementation((params: { includeUnlinked?: boolean; connector?: string }) => ({
        // `connector` swaps in a fresh response with the same nodes.
        data: params.includeUnlinked
          ? withNodes(['identity-a', 'identity-b', 'connector-a'])
          : withNodes(['identity-a', 'identity-b']),
        isLoading: false,
        isError: false,
        refetch: vi.fn(),
      }));
      pathHook.mockReturnValue({ data: hostToVM, isError: false, isLoading: false });
      renderTopology();
      await waitFor(() => expect(screen.getByTestId('flow-canvas')).toBeInTheDocument());
      fitView.mockClear();

      fireEvent.change(screen.getByRole('combobox', { name: 'Connector' }), {
        target: { value: 'connector-a' },
      });
      fireEvent.change(screen.getByRole('textbox', { name: 'From' }), {
        target: { value: 'Host Alpha' },
      });
      fireEvent.submit(screen.getByRole('form', { name: 'Trace a path' }));
      await waitFor(() => expect(highlightedEdgeIds()).toEqual(['edge-1']));
      expect(fitView).not.toHaveBeenCalled();

      fireEvent.click(screen.getByRole('checkbox', { name: 'Show unlinked identities' }));
      await waitFor(() => expect(fitView).toHaveBeenCalledTimes(1));
    });
  });

  it('names the legend as a group and shows progress while a trace is in flight', () => {
    pathHook.mockReturnValue({ data: undefined, isError: false, isLoading: true, isFetching: true });
    renderTopology('/topology?from=Host+Alpha');
    expect(screen.getByRole('group', { name: 'Edge kind legend' })).toBeInTheDocument();
    expect(screen.getByText('Tracing path…')).toHaveAttribute('role', 'status');
  });

  describe('kind options', () => {
    const unfiltered = (params: { kind?: string; connector?: string }) => ({
      data: params.kind
        ? { ...graph, nodes: graph.nodes.filter((n) => n.kind === params.kind), edges: [] }
        : params.connector
          ? { ...graph, nodes: graph.nodes.filter((n) => n.kind === 'vm'), edges: [] }
          : graph,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    const options = () =>
      within(screen.getByRole('combobox', { name: 'Entity kind' }))
        .getAllByRole('option')
        .map((o) => o.textContent);

    it('lists every kind when the page loads with a kind in the URL', () => {
      graphHook.mockImplementation(unfiltered);
      renderTopology('/topology?kind=host');
      expect(options()).toEqual(['All kinds', 'host', 'service', 'vm']);
    });

    it('follows the connector filter while a kind is set', () => {
      graphHook.mockImplementation(unfiltered);
      renderTopology('/topology?kind=vm');
      fireEvent.change(screen.getByRole('combobox', { name: 'Connector' }), {
        target: { value: 'connector-a' },
      });
      expect(options()).toEqual(['All kinds', 'vm']);
    });
  });
});
