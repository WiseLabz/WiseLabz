/**
 * Documentation viewer + the doc-format DiffViewer. Left: hierarchical tree.
 * Center: the rendered doc, or its revision history with a version-to-version
 * diff. The diff is the payoff — it shows exactly what a sync (or an editor)
 * changed between revisions, the same view the changes feed links into.
 */
import { useMemo, useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion } from 'motion/react';
import { useGetDocsTree, useGetDocsDocId } from '../../api/generated/docs/docs';
import { Panel } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { RoleGate } from '../../components/ui/RoleGate';
import { Skeleton, SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { Markdown } from '../../components/docs/Markdown';
import { DocTree } from '../../components/docs/DocTree';
import { DocHistory } from './DocHistory';
import { cn } from '../../lib/cn';
import { relativeTime, fullDate } from '../../lib/time';
import { FileTextIcon, HistoryIcon, SparklesIcon, EditIcon, SearchIcon } from '../../components/icons';

type Tab = 'read' | 'history';

export function DocsPage({ initialTab = 'read' }: { initialTab?: Tab } = {}) {
  const { t } = useTranslation();
  const { docId } = useParams<{ docId: string }>();
  const tree = useGetDocsTree();

  // Default to the lab root when no doc is selected in the URL.
  const activeId = docId ?? tree.data?.docId;

  return (
    <div className="mx-auto flex max-w-330 gap-6 px-6 py-6">
      {/* Tree */}
      <aside className="hidden w-60 shrink-0 lg:block">
        <Panel className="sticky top-6 p-2">
          <Link
            to="/docs/all"
            className="mb-1 flex items-center gap-2 rounded-md px-2.5 py-2 text-xs text-ink-muted transition-colors hover:bg-surface-raised hover:text-ink"
          >
            <SearchIcon size={14} />
            {t('docs.searchAllLink')}
          </Link>
          {tree.isLoading ? (
            <SkeletonRows rows={6} />
          ) : tree.isError || !tree.data ? (
            <ErrorState description={t('docs.treeLoadError')} onRetry={() => tree.refetch()} />
          ) : (
            <DocTree tree={tree.data} />
          )}
        </Panel>
      </aside>

      {/* Content */}
      <section className="min-w-0 flex-1">
        {activeId ? (
          // key by docId so per-doc UI state (active tab) resets on navigation
          <DocReader key={activeId} docId={activeId} initialTab={initialTab} />
        ) : (
          <Panel className="min-h-[60vh]">
            <EmptyState
              icon={<FileTextIcon size={20} />}
              title={t('docs.selectTitle')}
              description={t('docs.selectDesc')}
            />
          </Panel>
        )}
      </section>
    </div>
  );
}

function DocReader({ docId, initialTab = 'read' }: { docId: string; initialTab?: Tab }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch } = useGetDocsDocId(docId);
  const [tab, setTab] = useState<Tab>(initialTab);

  const triggerLabel = useMemo(() => {
    if (!data) return null;
    return data.kind === 'lab' ? t('docs.labOverview') : t('docs.serviceDoc');
  }, [data, t]);

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
        <ErrorState description={t('docs.docLoadError')} onRetry={() => refetch()} />
      </Panel>
    );

  return (
    <Panel>
      {/* Doc header */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line-soft px-6 py-4">
        <div className="min-w-0">
          <div className="mb-1 flex items-center gap-2">
            <span className="rounded bg-signal-tint px-1.5 py-0.5 text-2xs font-semibold uppercase tracking-wide text-signal-bright">
              {triggerLabel}
            </span>
            <span className="nums font-mono text-2xs text-ink-faint">
              {t('docs.versionMeta', {
                version: data.currentVersion,
                time: t('common.ago', { time: relativeTime(data.updatedAt) }),
              })}
            </span>
          </div>
          <h1 className="truncate text-lg font-semibold tracking-tight text-ink">{data.title}</h1>
        </div>

        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-lg border border-line-soft bg-canvas-sunken p-0.5">
            <TabButton
              active={tab === 'read'}
              onClick={() => setTab('read')}
              icon={<FileTextIcon size={14} />}
            >
              {t('docs.read')}
            </TabButton>
            <TabButton
              active={tab === 'history'}
              onClick={() => setTab('history')}
              icon={<HistoryIcon size={14} />}
            >
              {t('docs.history')}
            </TabButton>
          </div>
          <RoleGate>
            <Button variant="secondary" size="sm" onClick={() => navigate(`/docs/${docId}/edit`)}>
              <EditIcon size={14} />
              {t('docs.editAction')}
            </Button>
          </RoleGate>
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
                <div className="mb-4 flex items-center gap-2 rounded-md bg-signal-tint px-3 py-2 text-xs text-signal">
                  <SparklesIcon size={14} />
                  {t('docs.labBanner')}
                </div>
              )}
              <Markdown source={data.content} />
              <p className="mt-8 border-t border-line-soft pt-3 text-2xs text-ink-faint">
                {t('docs.reconciledFooter', { date: fullDate(data.updatedAt) })}
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
        active ? 'text-ink' : 'text-ink-muted hover:text-ink'
      )}
    >
      {active && (
        <motion.span
          layoutId="doc-tab"
          className="absolute inset-0 -z-10 rounded-md bg-surface-raised"
          transition={{ type: 'spring', stiffness: 500, damping: 36 }}
        />
      )}
      {icon}
      {children}
    </button>
  );
}
