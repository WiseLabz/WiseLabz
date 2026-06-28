/**
 * Documentation viewer + the doc-format DiffViewer. Left: hierarchical tree.
 * Center: the rendered doc, or its revision history with a version-to-version
 * diff. The diff is the payoff — it shows exactly what a sync (or an editor)
 * changed between revisions, the same view the changes feed links into.
 */
import { useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { AnimatePresence, motion } from 'motion/react';
import { useGetDocsTree, useGetDocsDocId } from '../../api/generated/docs/docs';
import { Panel } from '../../components/ui/Panel';
import { Skeleton, SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { Markdown } from '../../components/docs/Markdown';
import { DocTree } from '../../components/docs/DocTree';
import { DocHistory } from './DocHistory';
import { cn } from '../../lib/cn';
import { relativeTime, fullDate } from '../../lib/time';
import { FileTextIcon, HistoryIcon, SparklesIcon } from '../../components/icons';

type Tab = 'read' | 'history';

export function DocsPage() {
  const { docId } = useParams<{ docId: string }>();
  const tree = useGetDocsTree();

  // Default to the lab root when no doc is selected in the URL.
  const activeId = docId ?? tree.data?.docId;

  return (
    <div className="mx-auto flex max-w-[1320px] gap-6 px-6 py-6">
      {/* Tree */}
      <aside className="hidden w-60 shrink-0 lg:block">
        <Panel className="sticky top-6 p-2">
          {tree.isLoading ? (
            <SkeletonRows rows={6} />
          ) : tree.isError || !tree.data ? (
            <ErrorState description="Couldn't load the doc tree." onRetry={() => tree.refetch()} />
          ) : (
            <DocTree tree={tree.data} />
          )}
        </Panel>
      </aside>

      {/* Content */}
      <section className="min-w-0 flex-1">
        {activeId ? (
          // key by docId so per-doc UI state (active tab) resets on navigation
          <DocReader key={activeId} docId={activeId} />
        ) : (
          <Panel className="min-h-[60vh]">
            <EmptyState
              icon={<FileTextIcon size={20} />}
              title="Select a document"
              description="Pick a service from the tree to read its live documentation."
            />
          </Panel>
        )}
      </section>
    </div>
  );
}

function DocReader({ docId }: { docId: string }) {
  const { data, isLoading, isError, refetch } = useGetDocsDocId(docId);
  const [tab, setTab] = useState<Tab>('read');

  const triggerLabel = useMemo(() => {
    if (!data) return null;
    return data.kind === 'lab' ? 'Lab overview' : 'Service doc';
  }, [data]);

  if (isLoading)
    return (
      <Panel className="p-8">
        <Skeleton className="mb-4 h-7 w-1/3" />
        <SkeletonRows rows={8} />
      </Panel>
    );
  if (isError || !data)
    return (
      <Panel className="min-h-[50vh]">
        <ErrorState description="This document couldn't be loaded." onRetry={() => refetch()} />
      </Panel>
    );

  return (
    <Panel>
      {/* Doc header */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-line-soft)] px-6 py-4">
        <div className="min-w-0">
          <div className="mb-1 flex items-center gap-2">
            <span className="rounded bg-[var(--color-signal-tint)] px-1.5 py-0.5 text-2xs font-semibold uppercase tracking-wide text-[var(--color-signal-bright)]">
              {triggerLabel}
            </span>
            <span className="nums font-mono text-2xs text-[var(--color-ink-faint)]">
              v{data.currentVersion} · updated {relativeTime(data.updatedAt)} ago
            </span>
          </div>
          <h1 className="truncate text-lg font-semibold tracking-tight text-[var(--color-ink)]">
            {data.title}
          </h1>
        </div>

        <div className="flex items-center gap-1 rounded-lg border border-[var(--color-line-soft)] bg-[var(--color-canvas-sunken)] p-0.5">
          <TabButton active={tab === 'read'} onClick={() => setTab('read')} icon={<FileTextIcon size={14} />}>
            Read
          </TabButton>
          <TabButton active={tab === 'history'} onClick={() => setTab('history')} icon={<HistoryIcon size={14} />}>
            History
          </TabButton>
        </div>
      </div>

      <div className="px-6 py-5">
        <AnimatePresence mode="wait">
          {tab === 'read' ? (
            <motion.div
              key="read"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            >
              {data.kind === 'lab' && (
                <div className="mb-4 flex items-center gap-2 rounded-md bg-[var(--color-signal-tint)] px-3 py-2 text-xs text-[var(--color-signal)]">
                  <SparklesIcon size={14} />
                  Kept current by WiseLabz — open History to see what the last sync changed.
                </div>
              )}
              <Markdown source={data.content} />
              <p className="mt-8 border-t border-[var(--color-line-soft)] pt-3 text-2xs text-[var(--color-ink-faint)]">
                Last reconciled {fullDate(data.updatedAt)} · generated by WiseLabz
              </p>
            </motion.div>
          ) : (
            <motion.div
              key="history"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            >
              <DocHistory docId={docId} currentVersion={data.currentVersion} />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </Panel>
  );
}

function TabButton({
  active,
  onClick,
  icon,
  children,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'relative flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors',
        active ? 'text-[var(--color-ink)]' : 'text-[var(--color-ink-muted)] hover:text-[var(--color-ink)]',
      )}
    >
      {active && (
        <motion.span
          layoutId="doc-tab"
          className="absolute inset-0 -z-10 rounded-md bg-[var(--color-surface-raised)]"
          transition={{ type: 'spring', stiffness: 500, damping: 36 }}
        />
      )}
      {icon}
      {children}
    </button>
  );
}
