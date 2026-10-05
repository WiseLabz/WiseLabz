import { useCallback, useMemo, useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import dagre from '@dagrejs/dagre';
import {
  Background,
  BaseEdge,
  Controls,
  EdgeLabelRenderer,
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
type FlowEdge = Edge<{ label: string; highlighted: boolean }>;

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
  if (!data.href) {
    return (
      <div className={className} aria-label={data.item.name}>
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
            className="pointer-events-none absolute rounded bg-surface px-1.5 py-0.5 font-mono text-[10px] text-ink-muted"
            style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}
          >
            {data.label}
          </span>
        </EdgeLabelRenderer>
      )}
    </>
  );
}

const nodeTypes = { topology: GraphNode };
const edgeTypes = { topology: GraphEdge };

function layoutGraph(
  items: TopologyNode[],
  relations: TopologyEdge[],
  getHref: (item: TopologyNode) => string | undefined,
  traceKeys: Set<string>
) {
  const graph = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}));
  graph.setGraph({ rankdir: 'LR', nodesep: 36, ranksep: 90, marginx: 30, marginy: 30 });
  items.forEach((node) => graph.setNode(node.id, { width: nodeWidth, height: nodeHeight }));
  relations.forEach((edge) => graph.setEdge(edge.source, edge.target));
  dagre.layout(graph);
  const nodes: FlowNode[] = items.map((item) => {
    const pos = graph.node(item.id);
    return {
      id: item.id,
      type: 'topology',
      position: { x: pos.x - nodeWidth / 2, y: pos.y - nodeHeight / 2 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: { item, href: getHref(item) },
    };
  });
  const edges: FlowEdge[] = relations.map((edge) => {
    const key = `${edge.source}\0${edge.target}`;
    const reverseKey = `${edge.target}\0${edge.source}`;
    const highlighted = traceKeys.has(key) || traceKeys.has(reverseKey);
    return {
      id: edge.id,
      source: edge.source,
      target: edge.target,
      type: 'topology',
      label: `${edge.kind}${edge.detail ? ` · ${edge.detail}` : ''}`,
      data: {
        label: `${edge.kind}${edge.detail ? ` · ${edge.detail}` : ''}`,
        highlighted,
      },
      style: {
        stroke: highlighted
          ? 'var(--color-accent-primary-bright)'
          : edgeColors[edge.kind] || edgeColors.dependency,
        strokeWidth: highlighted ? 3 : 1.5,
        strokeDasharray: edge.kind === 'same_as' ? '5 4' : undefined,
      },
    };
  });
  return { nodes, edges };
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
    { query: { enabled: Boolean(fromParam && toParam), retry: false } }
  );
  const connectorsQuery = useGetConnectors();
  const docsTree = useGetDocsTree();
  const generate = useMutation({
    mutationFn: () => postDocsTopology(),
    onSuccess: (result) => navigate(`/docs/${encodeURIComponent(result.docId)}`),
  });
  const items = (graphQuery.data?.nodes ?? emptyNodes).filter(
    (item) => item.type === 'identity' || isConnectorNode(item)
  );
  const visibleNodeIDs = new Set(items.map((item) => item.id));
  const relations = (graphQuery.data?.edges ?? emptyEdges).filter(
    (edge) => visibleNodeIDs.has(edge.source) && visibleNodeIDs.has(edge.target)
  );
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
  const flow = useMemo(
    () => layoutGraph(items, relations, getNodeHref, highlights),
    [items, relations, getNodeHref, highlights]
  );
  const connectorList = (connectorsQuery.data ?? []) as Array<{ id: string; name: string }>;
  const kinds = Array.from(
    new Set(items.map((item) => item.kind).filter((value): value is string => Boolean(value)))
  ).sort();
  const updateFilter = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearchParams(next);
  };
  const runTrace = () => {
    const start = fromRef.current?.value.trim() ?? '';
    const end = toRef.current?.value.trim() ?? '';
    if (!start || !end || start.length > 256 || end.length > 256) {
      setTraceValidation(true);
      return;
    }
    setTraceValidation(false);
    const next = new URLSearchParams(searchParams);
    next.set('from', start);
    next.set('to', end);
    setSearchParams(next);
  };
  const clearTrace = () => {
    const next = new URLSearchParams(searchParams);
    next.delete('from');
    next.delete('to');
    setSearchParams(next);
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
            <input
              aria-label={t('docs.topology.kindFilter')}
              name="topology-kind"
              autoComplete="off"
              list="topology-kinds"
              value={kind}
              placeholder={t('docs.topology.allKinds')}
              onChange={(e) => updateFilter('kind', e.target.value)}
              className="h-8 rounded-sm border border-line-strong bg-surface px-2 text-sm text-ink"
            />
            <datalist id="topology-kinds">
              {kinds.map((item) => (
                <option key={item} value={item} />
              ))}
            </datalist>
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
        <div className="grid gap-3 border-b border-line-soft p-4 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end">
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
          <Button size="sm" onClick={runTrace}>
            {t('docs.topology.trace')}
          </Button>
          {(fromParam || toParam) && (
            <Button size="sm" variant="ghost" onClick={clearTrace}>
              {t('docs.topology.clearTrace')}
            </Button>
          )}
        </div>
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
            {t('docs.topology.noPath')}
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
          <EmptyState
            title={t('docs.topology.emptyTitle')}
            description={t('docs.topology.emptyDesc')}
          />
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
                nodes={flow.nodes}
                edges={flow.edges}
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
                <Controls />
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
