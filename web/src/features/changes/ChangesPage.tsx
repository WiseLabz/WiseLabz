/** Global change feed — every detected infra change, newest first. */
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { motion } from 'motion/react';
import { useGetChanges } from '../../api/generated/changes/changes';
import { SeverityTag } from '../../components/ui/StatusDot';
import { Panel } from '../../components/ui/Panel';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { relativeTime } from '../../lib/time';
import { ArrowRightIcon } from '../../components/icons';
import type { Severity } from '../../api/model';

const FILTERS: { key: string; value: Severity | 'all' }[] = [
  { key: 'changes.filterAll', value: 'all' },
  { key: 'changes.filterCritical', value: 'critical' },
  { key: 'changes.filterWarning', value: 'warning' },
  { key: 'changes.filterInfo', value: 'info' },
];

export function ChangesPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch } = useGetChanges(undefined);
  const [filter, setFilter] = useState<Severity | 'all'>('all');

  const items = (data?.items ?? []).filter((c) => filter === 'all' || c.severity === filter);

  return (
    <div className="mx-auto max-w-[900px] px-6 py-6">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-[var(--color-ink)]">{t('changes.title')}</h1>
          <p className="text-sm text-[var(--color-ink-muted)]">
            {data ? t('changes.countDetected', { count: data.total }) : t('changes.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-[var(--color-line-soft)] bg-[var(--color-canvas-sunken)] p-0.5">
          {FILTERS.map((f) => (
            <button
              key={f.value}
              onClick={() => setFilter(f.value)}
              className="relative rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors"
              style={{ color: filter === f.value ? 'var(--color-ink)' : 'var(--color-ink-muted)' }}
            >
              {filter === f.value && (
                <motion.span
                  layoutId="chg-filter"
                  className="absolute inset-0 -z-10 rounded-md bg-[var(--color-surface-raised)]"
                  transition={{ type: 'spring', stiffness: 500, damping: 36 }}
                />
              )}
              {t(f.key)}
            </button>
          ))}
        </div>
      </header>

      <Panel>
        {isLoading ? (
          <SkeletonRows rows={6} />
        ) : isError || !data ? (
          <ErrorState description={t('changes.loadError')} onRetry={() => refetch()} />
        ) : items.length === 0 ? (
          <EmptyState title={t('changes.emptyTitle')} description={t('changes.emptyDesc')} />
        ) : (
          items.map((c, idx) => (
            <motion.button
              key={c.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.03, duration: 0.25 }}
              onClick={() => navigate(`/changes/${c.id}`)}
              className="group flex w-full items-center gap-4 border-b border-[var(--color-line-soft)] px-4 py-3 text-left transition-colors last:border-0 hover:bg-[var(--color-surface-raised)]"
            >
              <SeverityTag severity={c.severity} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm text-[var(--color-ink)]">{c.summary}</p>
                <p className="flex items-center gap-1.5 font-mono text-2xs text-[var(--color-ink-faint)]">
                  <span className="text-[var(--color-signal-bright)]">{c.serviceName}</span>
                  <span>·</span>
                  <span>{c.changeType}</span>
                  <span>·</span>
                  <span>{t('common.ago', { time: relativeTime(c.detectedAt) })}</span>
                </p>
              </div>
              {c.willTriggerAi && (
                <span className="hidden rounded bg-[var(--color-signal-tint)] px-1.5 py-0.5 text-2xs font-semibold text-[var(--color-signal)] sm:block">
                  {t('changes.aiUpdate')}
                </span>
              )}
              <ArrowRightIcon
                size={15}
                className="shrink-0 text-[var(--color-line-strong)] transition-colors group-hover:text-[var(--color-ink-muted)]"
              />
            </motion.button>
          ))
        )}
      </Panel>
    </div>
  );
}
