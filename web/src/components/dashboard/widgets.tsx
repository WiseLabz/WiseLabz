/**
 * The five dashboard widgets. Each fetches through the generated React Query
 * hooks (curated MSW data behind them), folds in live WS state, and renders its
 * own loading / empty / error inside its own WidgetFrame chrome — no identical
 * card grid: the roster is a dense dot+word list, changes/alerts are feeds,
 * sync is a live timeline, docs is a coverage meter.
 */
import type { ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion } from 'motion/react';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import { useGetDashboardOverview } from '../../api/generated/dashboard/dashboard';
import { useGetAlerts } from '../../api/generated/alerts/alerts';
import { useGetDocsTree } from '../../api/generated/docs/docs';
import { useGetAttention } from '../../api/generated/attention/attention';
import { useDashboard, useRangeDays, useWidgetPolling, type WidgetId } from '../../store/dashboard';
import { useLive } from '../../store/live';
import { relativeTime } from '../../lib/time';
import { StatusPill, SeverityTag } from '../ui/StatusDot';
import { statusMeta, toneColor } from '../ui/status';
import { SkeletonRows, EmptyState, ErrorState } from '../ui/states';
import { ArrowRightIcon, FileTextIcon, CheckIcon } from '../icons';
import { categoryIcon } from '../categoryIcon';
import { WidgetFrame } from './WidgetFrame';
import type { Connector, ServiceStatus, Severity } from '../../api/model';

interface WidgetProps {
  title: string;
  icon: ReactNode;
}

const SEVERITY_FILTERS: { key: string; value: Severity | 'all' }[] = [
  { key: 'alerts.filterAll', value: 'all' },
  { key: 'alerts.filterCritical', value: 'critical' },
  { key: 'alerts.filterWarning', value: 'warning' },
  { key: 'alerts.filterInfo', value: 'info' },
];

/** Compact severity-filter tabs, inline in a widget's body. */
function SeverityTabs({
  widgetId,
  severityFilter,
}: {
  widgetId: WidgetId;
  severityFilter: Severity | 'all';
}) {
  const { t } = useTranslation();
  const setWidgetOptions = useDashboard((s) => s.setWidgetOptions);

  return (
    <div className="flex items-center gap-0.5 border-b border-line-soft px-2 py-1.5">
      {SEVERITY_FILTERS.map((f) => (
        <button
          key={f.value}
          onClick={() => setWidgetOptions(widgetId, { severityFilter: f.value })}
          className="rounded px-1.5 py-1 text-2xs font-medium transition-colors"
          style={{
            color: severityFilter === f.value ? 'var(--color-ink)' : 'var(--color-ink-faint)',
            backgroundColor:
              severityFilter === f.value ? 'var(--color-surface-raised)' : 'transparent',
          }}
        >
          {t(f.key)}
        </button>
      ))}
    </div>
  );
}

function filterBySeverity<T extends { severity: Severity }>(
  items: T[],
  severityFilter: Severity | 'all'
): T[] {
  return severityFilter === 'all' ? items : items.filter((i) => i.severity === severityFilter);
}

const STATUS_RANK: Record<ServiceStatus, number> = {
  offline: 0,
  degraded: 1,
  unknown: 2,
  online: 3,
};

/* ── Service roster (hero) ─────────────────────────────────────────────── */

export function ServiceRosterWidget({ title, icon }: WidgetProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pollingEnabled, refetchInterval, togglePolling } = useWidgetPolling('roster');
  const { data, isLoading, isError, refetch } = useGetConnectors({
    query: { refetchInterval },
  });
  const overrides = useLive((s) => s.statusOverrides);

  let body: ReactNode;
  if (isLoading) {
    body = <SkeletonRows rows={6} />;
  } else if (isError || !data) {
    body = <ErrorState description={t('widgets.loadServicesError')} onRetry={() => refetch()} />;
  } else if (data.length === 0) {
    body = (
      <EmptyState
        title={t('widgets.roster.emptyTitle')}
        description={t('widgets.roster.emptyDesc')}
      />
    );
  } else {
    const merged: Connector[] = data
      .map((c) => ({ ...c, status: overrides[c.id] ?? c.status }))
      .sort((a, b) => STATUS_RANK[a.status] - STATUS_RANK[b.status] || a.name.localeCompare(b.name));

    const counts = merged.reduce(
      (acc, c) => ((acc[c.status] = (acc[c.status] ?? 0) + 1), acc),
      {} as Record<ServiceStatus, number>
    );

    body = (
      <div className="flex min-h-0 flex-1 flex-col">
        {/* count strip */}
        <div className="flex items-center gap-4 border-b border-line-soft px-4 py-2.5">
          {(['online', 'degraded', 'offline'] as ServiceStatus[]).map((s) => (
            <div key={s} className="flex items-center gap-1.5">
              <span
                className="h-1.75 w-1.75"
                style={{ backgroundColor: toneColor[statusMeta[s].tone].fg }}
              />
              <span className="nums font-mono text-sm font-semibold text-ink">{counts[s] ?? 0}</span>
              <span className="font-mono text-2xs text-ink-faint">{statusMeta[s].label}</span>
            </div>
          ))}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {merged.map((c) => {
            const Icon = categoryIcon[c.category];
            const live = overrides[c.id] && overrides[c.id] !== c.status;
            return (
              <button
                key={c.id}
                onClick={() => navigate('/services')}
                className="group flex w-full items-center gap-3 border-b border-line-soft px-4 py-2.5 text-left transition-colors last:border-0 hover:bg-surface-raised"
              >
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-canvas-sunken text-ink-faint group-hover:text-accent-primary-bright">
                  <Icon size={16} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium text-ink">{c.name}</span>
                    {live && (
                      <span className="rounded bg-accent-primary-tint px-1 text-[10px] font-semibold text-accent-primary-bright">
                        {t('widgets.roster.updated')}
                      </span>
                    )}
                  </span>
                  <span className="block truncate font-mono text-2xs text-ink-faint">
                    {c.type} · {c.url?.replace(/^https?:\/\//, '')}
                  </span>
                </span>
                <span className="hidden shrink-0 text-right sm:block">
                  <StatusPill status={c.status} />
                  <span className="nums block font-mono text-2xs text-ink-faint">
                    {t('common.ago', { time: relativeTime(c.lastSyncAt) })}
                  </span>
                </span>
                <ArrowRightIcon
                  size={15}
                  className="shrink-0 text-line-strong transition-colors group-hover:text-ink-muted"
                />
              </button>
            );
          })}
        </div>
      </div>
    );
  }

  return (
    <WidgetFrame
      title={title}
      icon={icon}
      onRefresh={() => void refetch()}
      pollingEnabled={pollingEnabled}
      onTogglePolling={togglePolling}
    >
      {body}
    </WidgetFrame>
  );
}

/* ── Alert summary ─────────────────────────────────────────────────────── */

export function AlertSummaryWidget({ title, icon }: WidgetProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const days = useRangeDays();
  const widgetDef = useDashboard((s) => s.layout.find((w) => w.id === 'alerts'));
  const severityFilter = widgetDef?.severityFilter ?? 'all';
  const { pollingEnabled, refetchInterval, togglePolling } = useWidgetPolling('alerts');
  const { data, isLoading, isError, refetch } = useGetAlerts({ days }, { query: { refetchInterval } });
  const pendingLive = useLive((s) => s.pendingAlerts);

  let body: ReactNode;
  if (isLoading) {
    body = <SkeletonRows rows={3} />;
  } else if (isError || !data) {
    body = <ErrorState description={t('widgets.loadAlertsError')} onRetry={() => refetch()} />;
  } else {
    const pending = filterBySeverity(
      data.items.filter((a) => a.status === 'pending'),
      severityFilter
    );
    body = (
      <>
        <SeverityTabs widgetId="alerts" severityFilter={severityFilter} />
        {pending.length === 0 ? (
          <EmptyState
            icon={<CheckIcon size={20} />}
            title={t('widgets.alerts.allClearTitle')}
            description={t('widgets.alerts.allClearDesc')}
          />
        ) : (
          <>
            <div className="flex items-baseline gap-2 px-4 py-3">
              <motion.span
                key={Math.max(pending.length, pendingLive)}
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                className="nums text-2xl font-semibold text-err"
              >
                {Math.max(pending.length, pendingLive)}
              </motion.span>
              <span className="text-xs text-ink-muted">{t('widgets.alerts.pending')}</span>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
              {pending.slice(0, 4).map((a) => (
                <button
                  key={a.id}
                  onClick={() => navigate('/alerts')}
                  className="flex w-full flex-col gap-1 rounded-md px-2 py-2 text-left transition-colors hover:bg-surface-raised"
                >
                  <span className="flex items-center justify-between gap-2">
                    <SeverityTag severity={a.severity} />
                    <span className="font-mono text-2xs text-ink-faint">
                      {relativeTime(a.createdAt)}
                    </span>
                  </span>
                  <span className="line-clamp-2 text-sm text-ink">{a.title}</span>
                </button>
              ))}
            </div>
          </>
        )}
      </>
    );
  }

  return (
    <WidgetFrame
      title={title}
      icon={icon}
      onRefresh={() => void refetch()}
      pollingEnabled={pollingEnabled}
      onTogglePolling={togglePolling}
    >
      <div className="flex min-h-0 flex-1 flex-col">{body}</div>
    </WidgetFrame>
  );
}

/* ── Recent changes ────────────────────────────────────────────────────── */

export function RecentChangesWidget({ title, icon }: WidgetProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const days = useRangeDays();
  const widgetDef = useDashboard((s) => s.layout.find((w) => w.id === 'changes'));
  const severityFilter = widgetDef?.severityFilter ?? 'all';
  const { pollingEnabled, refetchInterval, togglePolling } = useWidgetPolling('changes');
  const { data, isLoading, isError, refetch } = useGetDashboardOverview(
    { days },
    { query: { refetchInterval } }
  );

  let body: ReactNode;
  if (isLoading) {
    body = <SkeletonRows rows={4} />;
  } else if (isError || !data) {
    body = <ErrorState description={t('widgets.loadChangesError')} onRetry={() => refetch()} />;
  } else {
    const changes = filterBySeverity(data.recentChanges ?? [], severityFilter);
    body = (
      <>
        <SeverityTabs widgetId="changes" severityFilter={severityFilter} />
        {changes.length === 0 ? (
          <EmptyState
            icon={<CheckIcon size={20} />}
            title={t('widgets.changes.emptyTitle')}
            description={t('widgets.changes.emptyDesc')}
          />
        ) : (
          <div className="min-h-0 flex-1 overflow-y-auto">
            {changes.map((c) => (
              <button
                key={c.id}
                onClick={() => navigate(`/changes/${c.id}`)}
                className="group flex w-full items-start gap-3 border-b border-line-soft px-4 py-2.5 text-left transition-colors last:border-0 hover:bg-surface-raised"
              >
                <span className="pt-0.5">
                  <SeverityTag severity={c.severity} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm text-ink">{c.summary}</span>
                  <span className="flex items-center gap-1.5 font-mono text-2xs text-ink-faint">
                    <span className="text-accent-primary-bright">{c.serviceName}</span>
                    <span>·</span>
                    <span>{c.changeType}</span>
                    <span>·</span>
                    <span>{relativeTime(c.detectedAt)}</span>
                    {c.willTriggerAi && (
                      <span className="ml-0.5 rounded bg-accent-primary-tint px-1 text-accent-primary">
                        {t('widgets.changes.ai')}
                      </span>
                    )}
                  </span>
                </span>
              </button>
            ))}
          </div>
        )}
      </>
    );
  }

  return (
    <WidgetFrame
      title={title}
      icon={icon}
      onRefresh={() => void refetch()}
      pollingEnabled={pollingEnabled}
      onTogglePolling={togglePolling}
    >
      <div className="flex min-h-0 flex-1 flex-col">{body}</div>
    </WidgetFrame>
  );
}

/* ── Sync activity (live timeline) ─────────────────────────────────────── */

const TONE_KEY: Record<string, keyof typeof toneColor> = {
  signal: 'signal',
  ok: 'ok',
  warn: 'warn',
  err: 'err',
  idle: 'idle',
};

export function SyncActivityWidget({ title, icon }: WidgetProps) {
  const { t } = useTranslation();
  const activity = useLive((s) => s.activity);
  const job = useLive((s) => s.jobs.global);

  return (
    <WidgetFrame title={title} icon={icon}>
      <div className="flex min-h-0 flex-1 flex-col">
        {job && job.phase !== 'done' && (
          <div className="border-b border-line-soft px-4 py-3">
            <div className="mb-1.5 flex items-center justify-between text-xs">
              <span className="font-medium text-ink">
                {t('widgets.sync.fleetSync', { phase: job.phase })}
              </span>
              <span className="nums font-mono text-accent-primary-bright">{job.percent}%</span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-canvas-sunken">
              <motion.div
                className="h-full rounded-full bg-accent-primary"
                animate={{ width: `${job.percent}%` }}
                transition={{ ease: [0.16, 1, 0.3, 1], duration: 0.5 }}
              />
            </div>
          </div>
        )}

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-2">
          {activity.length === 0 ? (
            <EmptyState
              title={t('widgets.sync.quietTitle')}
              description={t('widgets.sync.quietDesc')}
            />
          ) : (
            <ol className="relative ml-1 border-l border-line-soft">
              <AnimatePresence initial={false}>
                {activity.slice(0, 12).map((e) => {
                  const fg = toneColor[TONE_KEY[e.tone]].fg;
                  return (
                    <motion.li
                      key={e.id}
                      layout
                      initial={{ opacity: 0, x: -8 }}
                      animate={{ opacity: 1, x: 0 }}
                      exit={{ opacity: 0 }}
                      transition={{ type: 'spring', stiffness: 500, damping: 36 }}
                      className="relative py-2 pl-4"
                    >
                      <span
                        className="absolute -left-1.25 top-3 h-2 w-2 rounded-full"
                        style={{ backgroundColor: fg, boxShadow: `0 0 8px ${fg}` }}
                      />
                      <p className="text-sm text-ink">{e.label}</p>
                      {e.detail && (
                        <p className="font-mono text-2xs text-ink-faint">
                          {e.detail} · {relativeTime(e.at)}
                        </p>
                      )}
                    </motion.li>
                  );
                })}
              </AnimatePresence>
            </ol>
          )}
        </div>
      </div>
    </WidgetFrame>
  );
}

/* ── Attention queue (merged alerts + findings) ────────────────────────── */

export function AttentionQueueWidget({ title, icon }: WidgetProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const days = useRangeDays();
  const { pollingEnabled, refetchInterval, togglePolling } = useWidgetPolling('attention');
  const { data, isLoading, isError, refetch } = useGetAttention(
    { page: 1, pageSize: 5, days },
    { query: { refetchInterval } }
  );

  let body: ReactNode;
  if (isLoading) {
    body = <SkeletonRows rows={3} />;
  } else if (isError || !data) {
    body = <ErrorState description={t('widgets.loadAttentionError')} onRetry={() => refetch()} />;
  } else if (data.data.length === 0) {
    body = (
      <EmptyState
        icon={<CheckIcon size={20} />}
        title={t('widgets.attention.emptyTitle')}
        description={t('widgets.attention.emptyDesc')}
      />
    );
  } else {
    body = (
      <div className="min-h-0 flex-1 overflow-y-auto px-2 py-2">
        {data.data.map((item) => (
          <button
            key={item.id}
            onClick={() => navigate('/attention')}
            className="flex w-full flex-col gap-1 rounded-md px-2 py-2 text-left transition-colors hover:bg-surface-raised"
          >
            <span className="flex items-center justify-between gap-2">
              <span className="flex items-center gap-1.5">
                <SeverityTag severity={item.severity} />
                <span className="rounded px-1 py-0.5 text-[10px] font-medium text-ink-faint">
                  {item.kind === 'alert' ? 'Alert' : 'Finding'}
                </span>
              </span>
              <span className="font-mono text-2xs text-ink-faint">
                {relativeTime(item.detectedAt)}
              </span>
            </span>
            <span className="line-clamp-2 text-sm text-ink">{item.title}</span>
          </button>
        ))}
      </div>
    );
  }

  return (
    <WidgetFrame
      title={title}
      icon={icon}
      onRefresh={() => void refetch()}
      pollingEnabled={pollingEnabled}
      onTogglePolling={togglePolling}
    >
      {body}
    </WidgetFrame>
  );
}

/* ── Docs health ───────────────────────────────────────────────────────── */

export function DocsHealthWidget({ title, icon }: WidgetProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pollingEnabled, refetchInterval, togglePolling } = useWidgetPolling('docs');
  const { data: tree, isLoading, isError, refetch } = useGetDocsTree({
    query: { refetchInterval },
  });
  const { data: connectors } = useGetConnectors();

  let body: ReactNode;
  if (isLoading) {
    body = <SkeletonRows rows={3} />;
  } else if (isError || !tree) {
    body = <ErrorState description={t('widgets.loadDocsError')} onRetry={() => refetch()} />;
  } else {
    const docNodes = tree.children ?? [];
    const totalServices = connectors?.length ?? docNodes.length;
    const documented = docNodes.length;
    const pct = totalServices ? Math.round((documented / totalServices) * 100) : 0;

    body = (
      <div className="flex min-h-0 flex-1 flex-col p-4">
        <div className="flex items-center gap-4">
          <CoverageRing pct={pct} />
          <div>
            <p className="nums text-sm text-ink">
              {t('widgets.docs.documented', { documented, total: totalServices })}
            </p>
            <p className="text-xs text-ink-muted">
              {totalServices - documented > 0
                ? t('widgets.docs.awaiting', { count: totalServices - documented })
                : t('widgets.docs.allDocumented')}
            </p>
          </div>
        </div>

        <div className="mt-4 flex flex-wrap gap-2">
          {docNodes.map((d) => (
            <button
              key={d.docId}
              onClick={() => navigate(`/docs/${d.docId}`)}
              className="flex items-center gap-1.5 rounded-md border border-line-soft bg-canvas-sunken px-2 py-1 text-xs text-ink-muted transition-colors hover:border-accent-primary-soft hover:text-ink"
            >
              <FileTextIcon size={13} />
              {d.title.split(' — ')[0]}
            </button>
          ))}
        </div>

        <button
          onClick={() => navigate('/docs')}
          className="mt-3 flex items-center justify-center gap-1.5 rounded-md border border-line-soft py-2 text-xs font-medium text-ink-muted transition-colors hover:bg-surface-raised hover:text-ink"
        >
          {t('widgets.docs.open')}
          <ArrowRightIcon size={14} />
        </button>
      </div>
    );
  }

  return (
    <WidgetFrame
      title={title}
      icon={icon}
      onRefresh={() => void refetch()}
      pollingEnabled={pollingEnabled}
      onTogglePolling={togglePolling}
    >
      {body}
    </WidgetFrame>
  );
}

function CoverageRing({ pct }: Readonly<{ pct: number }>) {
  const r = 26;
  const c = 2 * Math.PI * r;
  return (
    <div className="relative h-16 w-16 shrink-0">
      <svg viewBox="0 0 64 64" className="h-16 w-16 -rotate-90">
        <circle
          cx="32"
          cy="32"
          r={r}
          fill="none"
          stroke="var(--color-canvas-sunken)"
          strokeWidth="6"
        />
        <motion.circle
          cx="32"
          cy="32"
          r={r}
          fill="none"
          stroke="var(--color-accent-primary)"
          strokeWidth="6"
          strokeLinecap="round"
          strokeDasharray={c}
          initial={{ strokeDashoffset: c }}
          animate={{ strokeDashoffset: c - (c * pct) / 100 }}
          transition={{ duration: 0.9, ease: [0.16, 1, 0.3, 1] }}
        />
      </svg>
      <span className="nums absolute inset-0 flex items-center justify-center text-sm font-semibold text-ink">
        {pct}%
      </span>
    </div>
  );
}
