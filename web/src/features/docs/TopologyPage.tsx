import type { CSSProperties, FormEvent } from 'react';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import dagre from '@dagrejs/dagre';
import {
  Background,
  BaseEdge,
  Controls,
  EdgeLabelRenderer,
  Handle,
  Position,
  ReactFlow,
  getBezierPath,
} from '@xyflow/react';
import type { Edge, EdgeProps, Node, NodeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import type { DocNode, TopologyEdge, TopologyNode } from '../../api/model';
import { useGetTopologyGraph, useGetTopologyPath } from '../../api/generated/topology/topology';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import { postDocsTopology, useGetDocsTree } from '../../api/generated/docs/docs';
import { useIsInstanceAdmin } from '../../hooks/useRole';
import { Button } from '../../components/ui/Button';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { EmptyState, ErrorState, SkeletonRows } from '../../components/ui/states';

const nodeWidth = 190;
const nodeHeight = 64;
const emptyNodes: TopologyNode[] = [];
const emptyEdges: TopologyEdge[] = [];
const edgeColors: Record<string, string> = {
  resolves_to: 'var(--color-accent-primary)',
  proxies_to: 'var(--color-warn)',
  runs_on: 'var(--color-accent-secondary)',
  dependency: 'var(--color-ink-muted)',
  contains: 'var(--color-ink-faint)',
  same_as: 'var(--color-ok)',
};

type NodeData = { item: TopologyNode; href?: string };
type FlowNode = Node<NodeData>;
type FlowEdge = Edge<{ label: string; parts: string[]; highlighted: boolean }>;
type Positions = Map<string, { x: number; y: number }>;

function isConnectorNode(item: TopologyNode) {
  return (
    item.type === 'node' &&
    item.kind === 'service' &&
    Boolean(item.connectorId) &&
    item.ref === item.connectorId
  );
}

function GraphNode({ data }: NodeProps<FlowNode>) {
  const identity = data.item.type === 'identity';
  const connector = isConnectorNode(data.item);
  const className = `flex h-full w-full flex-col justify-center rounded-md border px-3 text-left shadow-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent-primary ${
    identity
      ? 'border-accent-primary/50 bg-surface-raised text-ink'
      : connector
        ? 'border-accent-secondary/50 bg-surface text-ink-muted'
        : 'border-line-strong bg-surface text-ink-muted'
  }`;
  const content = (
    <>
      <span className="truncate text-sm font-medium">{data.item.name}</span>
      <span className="truncate font-mono text-[10px] text-ink-faint">
        {identity
          ? data.item.kind
          : connector
            ? data.item.connectorId
            : `${data.item.connectorId ?? ''} · ${data.item.kind ?? ''}`}
      </span>
    </>
  );
  // React Flow only draws an edge between nodes that expose handles.
  const handles = (
    <>
      <Handle type="target" position={Position.Left} isConnectable={false} className="!opacity-0" />
      <Handle type="source" position={Position.Right} isConnectable={false} className="!opacity-0" />
    </>
  );
  if (!data.href) {
    return (
      <div className={className} aria-label={data.item.name}>
        {handles}
        {content}
      </div>
    );
  }
  return (
    <Link
      to={data.href}
      className={className}
      aria-label={data.item.name}
      title={identity ? data.item.kind : data.item.connectorId}
    >
      {handles}
      {content}
    </Link>
  );
}

function GraphEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  style,
  markerEnd,
}: EdgeProps<FlowEdge>) {
  const [path, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  });
  return (
    <>
      <BaseEdge id={id} path={path} style={style} markerEnd={markerEnd} />
      {data?.label && (
        <EdgeLabelRenderer>
          <span
            className="pointer-events-none absolute flex flex-col items-center rounded bg-surface px-1.5 py-0.5 font-mono text-[10px] text-ink-muted"
            style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
          >
            {(data.parts ?? [data.label]).map((part) => (
              <span key={part}>{part}</span>
            ))}
          </span>
        </EdgeLabelRenderer>
      )}
    </>
  );
}

// The app palette is always dark; these map React Flow's chrome onto the
// live theme tokens so custom palettes are followed too.
const reactFlowTheme = {
  '--xy-background-color': 'transparent',
  '--xy-controls-button-background-color': 'var(--color-surface-raised)',
  '--xy-controls-button-background-color-hover': 'var(--color-surface-overlay)',
  '--xy-controls-button-color': 'var(--color-ink)',
  '--xy-controls-button-color-hover': 'var(--color-ink)',
  '--xy-controls-button-border-color': 'var(--color-line-strong)',
  '--xy-edge-label-background-color': 'var(--color-surface)',
  '--xy-edge-label-color': 'var(--color-ink-muted)',
} as CSSProperties;

const nodeTypes = { topology: GraphNode };
const edgeTypes = { topology: GraphEdge };

// Layout depends only on the graph shape, so a trace, a doc-tree refresh or a
// theme change never re-runs dagre.
function layoutPositions(items: TopologyNode[], relations: TopologyEdge[]): Positions {
  const graph = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
  graph.setGraph({ rankdir: 'LR', nodesep: 36, ranksep: 150, marginx: 30, marginy: 30 });
  items.forEach((node) => graph.setNode(node.id, { width: nodeWidth, height: nodeHeight }));
  relations.forEach((edge) => graph.setEdge(edge.source, edge.target));
  dagre.layout(graph);
  const positions: Positions = new Map();
  items.forEach((item) => {
    const pos = graph.node(item.id);
    positions.set(item.id, { x: pos.x - nodeWidth / 2, y: pos.y - nodeHeight / 2 });
  });
  return positions;
}

function flowNodes(
  items: TopologyNode[],
  positions: Positions,
  getHref: (item: TopologyNode) => string | undefined
): FlowNode[] {
  return items.map((item) => ({
    id: item.id,
    type: 'topology',
    position: positions.get(item.id) ?? { x: 0, y: 0 },
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    data: { item, href: getHref(item) },
  }));
}

// Several edge kinds between the same ordered pair would draw on top of one
// another, so they collapse into one edge whose label lists every kind.
function flowEdges(relations: TopologyEdge[], traceKeys: Set<string>): FlowEdge[] {
  const pairs = new Map<string, TopologyEdge[]>();
  for (const edge of relations) {
    const key = `${edge.source}\0${edge.target}`;
    pairs.set(key, [...(pairs.get(key) ?? []), edge]);
  }
  return Array.from(pairs.entries()).map(([key, group]) => {
    const first = group[0];
    const highlighted = traceKeys.has(key) || traceKeys.has(`${first.target}\0${first.source}`);
    const parts = group.map((edge) => `${edge.kind}${edge.detail ? ` · ${edge.detail}` : ''}`);
    const label = parts.join(', ');
    return {
      id: first.id,
      source: first.source,
      target: first.target,
      type: 'topology',
      label,
      data: { label, parts, highlighted },
      style: {
        stroke: highlighted
          ? 'var(--color-ink)'
          : edgeColors[first.kind] || edgeColors.dependency,
        strokeWidth: highlighted ? 3.5 : 1.5,
        strokeDasharray: first.kind === 'same_as' ? '5 4' : undefined,
      },
    };
  });
}

function findConnectorDoc(node: DocNode | undefined, connectorId: string): string | undefined {
  if (!node) return undefined;
  if (node.serviceId === connectorId) return node.docId;
  for (const child of node.children ?? []) {
    const found = findConnectorDoc(child, connectorId);
    if (found) return found;
  }
  return undefined;
}

function traceKeys(
  path: Array<{ connectorId: string; kind: string; name: string; nodeId?: string }>,
  nodes: TopologyNode[]
) {
  const ids = path.map((step) => {
    if (step.nodeId) return step.nodeId;
    const candidates = nodes.filter(
      (node) => node.name === step.name && (!step.kind || node.kind === step.kind)
    );
    return (
      candidates.find((node) => node.type === 'node' && node.connectorId === step.connectorId)
        ?.id ?? candidates[0]?.id
    );
  });
  const result = new Set<string>();
  for (let i = 1; i < ids.length; i++) {
    if (ids[i - 1] && ids[i]) result.add(`${ids[i - 1]}\0${ids[i]}`);
  }
  return result;
}

export function TopologyPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const connector = searchParams.get('connector') ?? '';
  const kind = searchParams.get('kind') ?? '';
  const includeUnlinked = searchParams.get('includeUnlinked') === 'true';
  const fromParam = searchParams.get('from') ?? '';
  const toParam = searchParams.get('to') ?? '';
  const fromRef = useRef<HTMLInputElement>(null);
  const toRef = useRef<HTMLInputElement>(null);
  const [traceValidation, setTraceValidation] = useState(false);
  const isInstanceAdmin = useIsInstanceAdmin();
  const graphQuery = useGetTopologyGraph(
    {
      ...(connector ? { connector } : {}),
      ...(kind ? { kind } : {}),
      ...(includeUnlinked ? { includeUnlinked: true } : {}),
    },
    { query: { retry: false } }
  );
  const pathQuery = useGetTopologyPath(
    { from: fromParam || '_', to: toParam || undefined },
    { query: { enabled: Boolean(fromParam), retry: false } }
  );
  const connectorsQuery = useGetConnectors();
  const docsTree = useGetDocsTree();
  const generate = useMutation({
    mutationFn: () => postDocsTopology(),
    onSuccess: (result) => navigate(`/docs/${encodeURIComponent(result.docId)}`),
  });
  const items = useMemo(
    () =>
      (graphQuery.data?.nodes ?? emptyNodes).filter(
        (item) => item.type === 'identity' || isConnectorNode(item)
      ),
    [graphQuery.data]
  );
  const relations = useMemo(() => {
    const visibleNodeIDs = new Set(items.map((item) => item.id));
    return (graphQuery.data?.edges ?? emptyEdges).filter(
      (edge) => visibleNodeIDs.has(edge.source) && visibleNodeIDs.has(edge.target)
    );
  }, [graphQuery.data, items]);
  const highlights = useMemo(
    () => (pathQuery.data?.found ? traceKeys(pathQuery.data.path, items) : new Set<string>()),
    [items, pathQuery.data]
  );
  const getNodeHref = useCallback(
    (item: TopologyNode) => {
      if (item.type === 'identity') {
        return `/entities/${encodeURIComponent(item.id)}`;
      }
      if (isConnectorNode(item) && item.connectorId) {
        const docId = findConnectorDoc(docsTree.data, item.connectorId);
        return docId
          ? `/docs/${encodeURIComponent(docId)}`
          : `/services/${encodeURIComponent(item.connectorId)}`;
      }
      return undefined;
    },
    [docsTree.data]
  );
  const positions = useMemo(() => layoutPositions(items, relations), [items, relations]);
  const nodes = useMemo(
    () => flowNodes(items, positions, getNodeHref),
    [items, positions, getNodeHref]
  );
  const edges = useMemo(() => flowEdges(relations, highlights), [relations, highlights]);
  const connectorList = (connectorsQuery.data ?? []) as Array<{ id: string; name: string }>;
  // The kind list comes from the last response that had no kind filter, so
  // picking a kind does not collapse the select to that one option.
  const [knownKinds, setKnownKinds] = useState<string[]>([]);
  const itemKinds = useMemo(
    () =>
      Array.from(
        new Set(items.map((item) => item.kind).filter((value): value is string => Boolean(value)))
      ).sort(),
    [items]
  );
  if (!kind && graphQuery.data && knownKinds.join('\0') !== itemKinds.join('\0')) {
    setKnownKinds(itemKinds);
  }
  const kindOptions = kind && !knownKinds.includes(kind) ? [...knownKinds, kind].sort() : knownKinds;
  const updateFilter = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearchParams(next, { replace: true });
  };
  const clearFilters = () => {
    const next = new URLSearchParams(searchParams);
    ['connector', 'kind', 'includeUnlinked'].forEach((key) => next.delete(key));
    setSearchParams(next, { replace: true });
  };
  const hasFilters = Boolean(connector || kind || includeUnlinked);
  const runTrace = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const start = fromRef.current?.value.trim() ?? '';
    const end = toRef.current?.value.trim() ?? '';
    if (!start || start.length > 256 || end.length > 256) {
      setTraceValidation(true);
      return;
    }
    setTraceValidation(false);
    const next = new URLSearchParams(searchParams);
    next.set('from', start);
    if (end) next.set('to', end);
    else next.delete('to');
    setSearchParams(next);
  };
  const clearTrace = () => {
    const next = new URLSearchParams(searchParams);
    next.delete('from');
    next.delete('to');
    setSearchParams(next, { replace: true });
    setTraceValidation(false);
  };

  return (
    <div className="mx-auto max-w-7xl space-y-4 px-6 py-6">
      <Panel>
        <PanelHeader
          title={t('docs.topology.title')}
          action={
            isInstanceAdmin ? (
              <Button size="sm" disabled={generate.isPending} onClick={() => generate.mutate()}>
                {generate.isPending ? t('docs.topology.generating') : t('docs.topology.export')}
              </Button>
            ) : undefined
          }
        />
        {generate.isError && (
          <p role="alert" className="border-b border-line-soft px-4 py-2 text-sm text-err">
            {t('docs.topology.exportError')}
          </p>
        )}
        <div className="grid gap-3 border-b border-line-soft p-4 sm:grid-cols-3">
          <label className="flex flex-col gap-1 text-xs text-ink-muted">
            {t('docs.topology.connectorFilter')}
            <select
              aria-label={t('docs.topology.connectorFilter')}
              name="topology-connector"
              value={connector}
              onChange={(e) => updateFilter('connector', e.target.value)}
              className="h-8 rounded-sm border border-line-strong bg-surface px-2 text-sm text-ink"
            >
              <option value="">{t('docs.topology.allConnectors')}</option>
              {connectorList.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-ink-muted">
            {t('docs.topology.kindFilter')}
            <select
              aria-label={t('docs.topology.kindFilter')}
              name="topology-kind"
              value={kind}
              onChange={(e) => updateFilter('kind', e.target.value)}
              className="h-8 rounded-sm border border-line-strong bg-surface px-2 text-sm text-ink"
            >
              <option value="">{t('docs.topology.allKinds')}</option>
              {kindOptions.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
          <label className="flex items-center gap-2 self-end pb-2 text-sm text-ink">
            <input
              type="checkbox"
              checked={includeUnlinked}
              onChange={(e) => updateFilter('includeUnlinked', e.target.checked ? 'true' : '')}
            />
            {t('docs.topology.showUnlinked')}
          </label>
        </div>
        <form
          aria-label={t('docs.topology.traceForm')}
          onSubmit={runTrace}
          className="grid gap-3 border-b border-line-soft p-4 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end"
        >
          <label className="flex flex-col gap-1 text-xs text-ink-muted">
            {t('docs.topology.traceFrom')}
            <input
              key={fromParam}
              ref={fromRef}
              aria-label={t('docs.topology.traceFrom')}
              name="topology-from"
              autoComplete="off"
              placeholder={t('docs.topology.tracePlaceholder')}
              defaultValue={fromParam}
              maxLength={256}
              className="h-8 rounded-sm border border-line-strong bg-surface px-2 text-sm text-ink"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs text-ink-muted">
            {t('docs.topology.traceTo')}
            <input
              key={toParam}
              ref={toRef}
              aria-label={t('docs.topology.traceTo')}
              name="topology-to"
              autoComplete="off"
              placeholder={t('docs.topology.tracePlaceholder')}
              defaultValue={toParam}
              maxLength={256}
              className="h-8 rounded-sm border border-line-strong bg-surface px-2 text-sm text-ink"
            />
          </label>
          <Button size="sm" type="submit">
            {t('docs.topology.trace')}
          </Button>
          {(fromParam || toParam) && (
            <Button size="sm" variant="ghost" type="button" onClick={clearTrace}>
              {t('docs.topology.clearTrace')}
            </Button>
          )}
        </form>
        {traceValidation && (
          <p role="alert" className="px-4 pt-3 text-sm text-err">
            {t('docs.topology.traceValidation')}
          </p>
        )}
        {pathQuery.isError && (
          <p role="alert" className="px-4 pt-3 text-sm text-err">
            {isAxiosError(pathQuery.error) && pathQuery.error.response?.status === 400
              ? t('docs.topology.traceValidation')
              : t('docs.topology.traceError')}
          </p>
        )}
        {pathQuery.data && !pathQuery.data.found && (
          <p role="status" className="px-4 pt-3 text-sm text-ink-muted">
            {toParam ? t('docs.topology.noPath') : t('docs.topology.noPathFrom')}
          </p>
        )}
        {pathQuery.data?.found && pathQuery.data.truncated && (
          <p role="status" className="px-4 pt-3 text-sm text-warn">
            {t('docs.topology.pathTruncated')}
          </p>
        )}
        {pathQuery.data?.found && (
          <ol
            aria-label={t('docs.topology.traceHops')}
            className="space-y-1 px-8 py-3 text-sm text-ink"
          >
            {pathQuery.data.path.map((step, index) => (
              <li key={`${step.connectorId}:${step.kind}:${step.name}:${index}`}>
                {step.name}{' '}
                <span className="font-mono text-xs text-ink-faint">
                  {step.kind}
                  {step.edgeKind ? ` · ${step.edgeKind}` : ''}
                  {step.detail ? ` · ${step.detail}` : ''}
                </span>
              </li>
            ))}
          </ol>
        )}
        <div
          className="flex flex-wrap gap-x-4 gap-y-1 px-4 py-3 text-xs text-ink-muted"
          aria-label={t('docs.topology.legend')}
        >
          {Object.entries(edgeColors).map(([edgeKind, color]) => (
            <span key={edgeKind} className="inline-flex items-center gap-1.5">
              <span className="h-0.5 w-4" style={{ backgroundColor: color }} />
              {edgeKind}
            </span>
          ))}
        </div>
        {graphQuery.isLoading ? (
          <div className="p-4">
            <SkeletonRows rows={5} />
          </div>
        ) : graphQuery.isError ? (
          <ErrorState
            description={t('docs.topology.loadError')}
            onRetry={() => void graphQuery.refetch()}
          />
        ) : items.length === 0 ? (
          hasFilters ? (
            <EmptyState
              title={t('docs.topology.noMatchTitle')}
              description={t('docs.topology.noMatchDesc')}
              action={
                <Button size="sm" variant="ghost" onClick={clearFilters}>
                  {t('docs.topology.clearFilters')}
                </Button>
              }
            />
          ) : (
            <EmptyState
              title={t('docs.topology.emptyTitle')}
              description={t('docs.topology.emptyDesc')}
            />
          )
        ) : (
          <>
            {graphQuery.data?.truncated && (
              <p
                role="status"
                className="border-b border-warn/30 bg-warn-tint px-4 py-2 text-sm text-warn"
              >
                {t('docs.topology.truncated')}
              </p>
            )}
            <div
              className="h-[min(70vh,700px)] min-h-100"
              role="region"
              aria-label={t('docs.topology.graph')}
            >
              <ReactFlow
                nodes={nodes}
                edges={edges}
                colorMode="dark"
                style={reactFlowTheme}
                nodeTypes={nodeTypes}
                edgeTypes={edgeTypes}
                fitView
                fitViewOptions={{ padding: 0.2 }}
                nodesDraggable={false}
                nodesConnectable={false}
                elementsSelectable={false}
                proOptions={{ hideAttribution: true }}
              >
                <Background color="var(--color-line-soft)" />
                <Controls showInteractive={false} />
              </ReactFlow>
            </div>
            <details className="border-t border-line-soft p-4">
              <summary className="cursor-pointer text-sm text-ink">
                {t('docs.topology.nodeList', { count: items.length })}
              </summary>
              <ul className="mt-2 grid gap-1 sm:grid-cols-2 lg:grid-cols-3">
                {items.map((item) => (
                  <li key={item.id}>
                    {getNodeHref(item) ? (
                      <Link
                        to={getNodeHref(item)!}
                        className="text-sm text-accent-secondary-bright hover:underline"
                      >
                        {item.name}{' '}
                        <span className="font-mono text-xs text-ink-faint">· {item.kind}</span>
                      </Link>
                    ) : (
                      <span className="text-sm text-ink-muted">
                        {item.name}{' '}
                        <span className="font-mono text-xs text-ink-faint">· {item.kind}</span>
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            </details>
          </>
        )}
      </Panel>
    </div>
  );
}
