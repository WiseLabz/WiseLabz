import { useEffect, useState } from 'react';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { isAxiosError } from 'axios';
import {
  getTimeline,
  deleteJournalId,
  usePostTimelineNarrate,
} from '../../api/generated/journal/journal';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import { useGetMe } from '../../api/generated/me/me';
import type { TimelineItem, TimelineNarration, TimelineNarrationSource } from '../../api/model';
import { useCanMutate, useIsInstanceAdmin } from '../../hooks/useRole';
import { Markdown } from '../../components/docs/Markdown';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { EmptyState, ErrorState, SkeletonRows } from '../../components/ui/states';
import { toast } from '../../lib/toast';
import { auditActionLabel } from '../../lib/auditActionLabel';
import { JournalEntryDialog } from './JournalEntryDialog';

const kinds = ['change', 'sync', 'alert', 'doc', 'journal', 'audit'] as const;
const fieldClass = 'rounded-md border border-line-soft bg-canvas px-3 py-2 text-sm text-ink';

function sourceLink(row: TimelineItem, admin: boolean) {
  switch (row.kind) {
    case 'change':
      return `/changes/${row.id}`;
    case 'alert':
      return `/alerts/${row.id}`;
    case 'doc':
      return `/docs/${row.docId}`;
    case 'sync':
      return `/services/${row.connectorId}`;
    case 'audit':
      if (admin) return '/settings/audit';
      return row.docId
        ? `/docs/${row.docId}`
        : row.connectorId
          ? `/services/${row.connectorId}`
          : '';
    case 'journal':
      return row.docId
        ? `/docs/${row.docId}`
        : row.connectorId
          ? `/services/${row.connectorId}`
          : '';
  }
}

// Narration sources are trimmed rows; give them the row shape sourceLink reads.
function sourceRow(source: TimelineNarrationSource): TimelineItem {
  return {
    ...source,
    body: '',
    createdBy: '',
    entityKind: '',
    entityName: '',
    entityRef: '',
    status: '',
  };
}

// Splits plain narration text around [n] citations that match a source; the
// text itself is never interpreted as Markdown or HTML.
function NarrationText({ text, sources }: { text: string; sources: TimelineNarrationSource[] }) {
  const known = new Set(sources.map((source) => source.n));
  return (
    <p className="whitespace-pre-wrap text-sm text-ink">
      {text.split(/(\[\d+\])/).map((part, i) => {
        const n = Number(/^\[(\d+)\]$/.exec(part)?.[1]);
        return known.has(n) ? (
          <a
            key={i}
            className="text-accent-primary hover:underline"
            href={`#narration-source-${n}`}
          >
            {part}
          </a>
        ) : (
          part
        );
      })}
    </p>
  );
}

export function JournalPage() {
  const { t, i18n } = useTranslation();
  const [search, setSearch] = useSearchParams();
  const admin = useIsInstanceAdmin();
  const canMutate = useCanMutate();
  const me = useGetMe();
  const { data: connectors = [] } = useGetConnectors();
  const [editor, setEditor] = useState<TimelineItem | 'new' | null>(null);
  const [deleting, setDeleting] = useState<TimelineItem | null>(null);
  const queryClient = useQueryClient();
  const filters = {
    connectorId: search.get('connectorId') || undefined,
    after: search.get('after') || undefined,
    before: search.get('before') || undefined,
    kinds: search.get('kinds') || undefined,
    allSyncRuns: search.get('allSyncRuns') === 'true',
  };
  const timeline = useInfiniteQuery({
    queryKey: ['journal-timeline', filters],
    initialPageParam: '',
    queryFn: ({ pageParam, signal }) =>
      getTimeline({ ...filters, pageSize: 30, cursor: pageParam }, undefined, signal),
    getNextPageParam: (page) => page.nextCursor || undefined,
  });
  const rows = timeline.data?.pages.flatMap((page) => page.items) ?? [];
  const narrate = usePostTimelineNarrate();
  const resetNarration = narrate.reset;
  const filterKey = JSON.stringify(filters);
  // A narration describes one window; drop it as soon as the filters move on.
  useEffect(() => resetNarration(), [filterKey, resetNarration]);
  const remove = useMutation({
    mutationFn: (id: string) => deleteJournalId(id),
    onSuccess: () => {
      setDeleting(null);
      void queryClient.invalidateQueries({ queryKey: ['journal-timeline'] });
    },
    onError: () => toast.error(t('journal.deleteError')),
  });
  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(search);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearch(next);
  };
  const windowLabel = (n: TimelineNarration) =>
    n.after && n.before
      ? t('journal.narration.windowBetween', { after: day(n.after), before: day(n.before) })
      : n.after
        ? t('journal.narration.windowFrom', { after: day(n.after) })
        : n.before
          ? t('journal.narration.windowUntil', { before: day(n.before) })
          : t('journal.narration.windowAll');
  const day = (iso: string) => new Date(iso).toLocaleDateString(i18n.language, { timeZone: 'UTC' });
  const filterWords = [
    filters.connectorId &&
      t('journal.narration.filterScope', {
        name: connectors.find((c) => c.id === filters.connectorId)?.name ?? filters.connectorId,
      }),
    filters.kinds &&
      t('journal.narration.filterSource', {
        name: filters.kinds
          .split(',')
          .map((kind) => t(`journal.kinds.${kind}`, { defaultValue: kind }))
          .join(', '),
      }),
    filters.allSyncRuns && t('journal.narration.filterAllSyncRuns'),
  ].filter(Boolean);
  return (
    <div className="space-y-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-ink">{t('journal.title')}</h1>
          <p className="mt-1 text-sm text-ink-muted">{t('journal.description')}</p>
        </div>
        {canMutate && <Button onClick={() => setEditor('new')}>{t('journal.newEntry')}</Button>}
      </div>
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-xs text-ink-muted">
          {t('journal.scope')}
          <select
            className={fieldClass}
            value={filters.connectorId ?? ''}
            onChange={(e) => setFilter('connectorId', e.target.value)}
          >
            <option value="">{t('journal.allScopes')}</option>
            {connectors.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs text-ink-muted">
          {t('journal.source')}
          <select
            className={fieldClass}
            value={filters.kinds ?? ''}
            onChange={(e) => setFilter('kinds', e.target.value)}
          >
            <option value="">{t('journal.allSources')}</option>
            {kinds.map((kind) => (
              <option key={kind} value={kind}>
                {t(`journal.kinds.${kind}`)}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs text-ink-muted">
          {t('journal.after')}
          <input
            type="date"
            className={fieldClass}
            value={filters.after?.slice(0, 10) ?? ''}
            onChange={(e) =>
              setFilter('after', e.target.value ? `${e.target.value}T00:00:00Z` : '')
            }
          />
        </label>
        <label className="flex flex-col gap-1 text-xs text-ink-muted">
          {t('journal.before')}
          <input
            type="date"
            className={fieldClass}
            value={filters.before?.slice(0, 10) ?? ''}
            onChange={(e) =>
              setFilter('before', e.target.value ? `${e.target.value}T23:59:59.999999999Z` : '')
            }
          />
        </label>
        <label className="flex items-center gap-2 py-2 text-sm text-ink-muted">
          <input
            type="checkbox"
            checked={filters.allSyncRuns}
            onChange={(e) => setFilter('allSyncRuns', e.target.checked ? 'true' : '')}
          />
          {t('journal.allSyncRuns')}
        </label>
        <Button
          variant="secondary"
          disabled={narrate.isPending}
          onClick={() => narrate.mutate({ params: filters })}
        >
          {narrate.isPending ? t('journal.narration.loading') : t('journal.narration.button')}
        </Button>
      </div>
      {narrate.isError && (
        <p role="alert" className="text-sm text-err">
          {isAxiosError(narrate.error) && narrate.error.response?.status === 409
            ? t('journal.narration.disabled')
            : t('journal.narration.error')}
        </p>
      )}
      {narrate.data &&
        (narrate.data.totalEvents === 0 ? (
          <p className="text-sm text-ink-muted">{t('journal.narration.empty')}</p>
        ) : (
          <Panel>
            <section aria-label={t('journal.narration.title')} className="space-y-3 p-5">
              <h2 className="text-sm font-semibold text-ink">{t('journal.narration.title')}</h2>
              <p className="text-xs text-ink-muted">
                {[windowLabel(narrate.data), ...filterWords].join(' · ')} ·{' '}
                {t('journal.narration.eventCount', { count: narrate.data.eventCount })}
              </p>
              <NarrationText text={narrate.data.narration} sources={narrate.data.sources} />
              {narrate.data.truncated && (
                <p className="text-xs text-warn">
                  {t('journal.narration.truncated', {
                    count: narrate.data.eventCount,
                    total: narrate.data.totalEvents,
                  })}
                </p>
              )}
              {narrate.data.fallbackUsed && (
                <p className="text-xs text-ink-faint">
                  {t('journal.narration.fallbackUsed', { provider: narrate.data.provider })}
                </p>
              )}
              <h3 className="text-xs font-semibold text-ink-muted">
                {t('journal.narration.sources')}
              </h3>
              <ol className="space-y-1 text-xs text-ink-muted">
                {narrate.data.sources.map((source) => {
                  const link = sourceLink(sourceRow(source), admin);
                  const label = source.title || t('journal.narration.sourceFallback');
                  return (
                    <li key={source.n} id={`narration-source-${source.n}`}>
                      [{source.n}] {t(`journal.kinds.${source.kind}`)} ·{' '}
                      <time dateTime={source.timestamp}>
                        {new Date(source.timestamp).toLocaleString(i18n.language)}
                      </time>{' '}
                      ·{' '}
                      {link ? (
                        <Link className="text-accent-primary hover:underline" to={link}>
                          {label}
                        </Link>
                      ) : (
                        label
                      )}
                    </li>
                  );
                })}
              </ol>
            </section>
          </Panel>
        ))}
      <Panel>
        {timeline.isPending ? (
          <SkeletonRows />
        ) : timeline.isError ? (
          <ErrorState title={t('journal.loadError')} onRetry={() => void timeline.refetch()} />
        ) : !rows.length ? (
          <EmptyState title={t('journal.empty')} />
        ) : (
          <ol className="relative m-5 ml-6 border-l border-line-soft">
            {rows.map((row) => {
              const link = sourceLink(row, admin);
              const connector = connectors.find((c) => c.id === row.connectorId);
              const editable = row.kind === 'journal' && (admin || me.data?.id === row.createdBy);
              return (
                <li key={`${row.kind}:${row.id}`} className="relative py-4 pl-5">
                  <span
                    className={`absolute -left-1 top-6 h-2 w-2 rounded-full ${row.status === 'error' ? 'bg-err' : 'bg-accent-primary'}`}
                  />
                  <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-faint">
                    <span>{t(`journal.kinds.${row.kind}`)}</span>
                    <time dateTime={row.timestamp}>
                      {new Date(row.timestamp).toLocaleString(i18n.language)}
                    </time>
                    {row.connectorId && (
                      <Link className="hover:text-ink" to={`/services/${row.connectorId}`}>
                        {connector?.name ?? row.connectorId}
                      </Link>
                    )}
                  </div>
                  {row.title && (
                    <p className="mt-1 text-sm text-ink">
                      {row.kind === 'sync'
                        ? t(`journal.syncStatus.${row.status}`)
                        : row.kind === 'audit'
                          ? auditActionLabel(row.title, t)
                          : row.title}
                    </p>
                  )}
                  {row.body && <Markdown source={row.body} />}
                  {(row.entityRef || row.entityKind || row.entityName) && (
                    <p className="font-mono text-xs text-ink-muted">
                      {row.entityKind} · {row.entityName} · {row.entityRef}
                    </p>
                  )}
                  <div className="mt-2 flex items-center gap-3">
                    {link && (
                      <Link className="text-xs text-accent-primary hover:underline" to={link}>
                        {t('journal.viewSource')}
                      </Link>
                    )}
                    {editable && (
                      <>
                        <Button size="sm" variant="ghost" onClick={() => setEditor(row)}>
                          {t('common.edit')}
                        </Button>
                        <Button size="sm" variant="ghost" onClick={() => setDeleting(row)}>
                          {t('common.delete')}
                        </Button>
                      </>
                    )}
                  </div>
                </li>
              );
            })}
          </ol>
        )}
        {timeline.hasNextPage && (
          <div className="p-4 text-center">
            <Button
              variant="secondary"
              disabled={timeline.isFetchingNextPage}
              onClick={() => void timeline.fetchNextPage()}
            >
              {t('journal.loadMore')}
            </Button>
          </div>
        )}
      </Panel>
      {editor && (
        <JournalEntryDialog
          entry={editor === 'new' ? undefined : editor}
          onClose={() => setEditor(null)}
        />
      )}
      <ConfirmDialog
        open={!!deleting}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        title={t('journal.deleteTitle')}
        description={t('journal.deleteDescription')}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        confirmDisabled={remove.isPending}
      />
    </div>
  );
}
