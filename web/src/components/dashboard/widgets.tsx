/**
 * The five dashboard widgets. Each fetches through the generated React Query
 * hooks (curated MSW data behind them), folds in live WS state, and renders its
 * own loading / empty / error. They differ in shape on purpose — no identical
 * card grid: the roster is a dense dot+word list, changes/alerts are feeds,
 * sync is a live timeline, docs is a coverage meter.
 */
import { useNavigate } from 'react-router-dom';
import { AnimatePresence, motion } from 'motion/react';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import { useGetDashboardOverview } from '../../api/generated/dashboard/dashboard';
import { useGetAlerts } from '../../api/generated/alerts/alerts';
import { useGetDocsTree } from '../../api/generated/docs/docs';
import { useLive } from '../../store/live';
import { relativeTime } from '../../lib/time';
import { StatusPill, SeverityTag } from '../ui/StatusDot';
import { statusMeta, toneColor } from '../ui/status';
import { SkeletonRows, EmptyState, ErrorState } from '../ui/states';
import { ArrowRightIcon, FileTextIcon, CheckIcon } from '../icons';
import { categoryIcon } from '../categoryIcon';
import type { Connector, ServiceStatus } from '../../api/model';

const STATUS_RANK: Record<ServiceStatus, number> = {
  offline: 0,
  degraded: 1,
  unknown: 2,
  online: 3,
};

/* ── Service roster (hero) ─────────────────────────────────────────────── */

export function ServiceRosterWidget() {
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch } = useGetConnectors();
  const overrides = useLive((s) => s.statusOverrides);

  if (isLoading) return <SkeletonRows rows={6} />;
  if (isError || !data)
    return <ErrorState description="Couldn't load services." onRetry={() => refetch()} />;
  if (data.length === 0)
    return (
      <EmptyState
        title="No services connected"
        description="Add your first connector to start generating documentation."
      />
    );

  const merged: Connector[] = data
    .map((c) => ({ ...c, status: overrides[c.id] ?? c.status }))
    .sort((a, b) => STATUS_RANK[a.status] - STATUS_RANK[b.status] || a.name.localeCompare(b.name));

  const counts = merged.reduce(
    (acc, c) => ((acc[c.status] = (acc[c.status] ?? 0) + 1), acc),
    {} as Record<ServiceStatus, number>,
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* count strip */}
      <div className="flex items-center gap-4 border-b border-line-soft px-4 py-2.5">
        {(['online', 'degraded', 'offline'] as ServiceStatus[]).map((s) => (
          <div key={s} className="flex items-center gap-1.5">
            <span
              className="h-1.75 w-1.75"
              style={{ backgroundColor: toneColor[statusMeta[s].tone].fg }}
            />
            <span className="nums font-mono text-sm font-semibold text-ink">
              {counts[s] ?? 0}
            </span>
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
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-canvas-sunken text-ink-faint group-hover:text-signal-bright">
                <Icon size={16} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium text-ink">
                    {c.name}
                  </span>
                  {live && (
                    <span className="rounded bg-signal-tint px-1 text-[10px] font-semibold text-signal-bright">
                      updated
                    </span>
                  )}
                </span>
                <span className="block truncate font-mono text-2xs text-[var(--color-ink-faint)]">
                  {c.type} · {c.url?.replace(/^https?:\/\//, '')}
                </span>
              </span>
              <span className="hidden shrink-0 text-right sm:block">
                <StatusPill status={c.status} />
                <span className="nums block font-mono text-2xs text-[var(--color-ink-faint)]">
                  {relativeTime(c.lastSyncAt)} ago
                </span>
              </span>
              <ArrowRightIcon
                size={15}
                className="shrink-0 text-[var(--color-line-strong)] transition-colors group-hover:text-[var(--color-ink-muted)]"
              />
            </button>
          );
        })}
      </div>
    </div>
  );
}

/* ── Alert summary ─────────────────────────────────────────────────────── */

export function AlertSummaryWidget() {
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch } = useGetAlerts(undefined);
  const pendingLive = useLive((s) => s.pendingAlerts);

  if (isLoading) return <SkeletonRows rows={3} />;
  if (isError || !data)
    return <ErrorState description="Couldn't load alerts." onRetry={() => refetch()} />;

  const pending = data.items.filter((a) => a.status === 'pending');
  if (pending.length === 0)
    return (
      <EmptyState
        icon={<CheckIcon size={20} />}
        title="All clear"
        description="No pending alerts. Drift is documented or acknowledged."
      />
    );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-baseline gap-2 px-4 py-3">
        <motion.span
          key={Math.max(pending.length, pendingLive)}
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          className="nums text-2xl font-semibold text-[var(--color-err)]"
        >
          {Math.max(pending.length, pendingLive)}
        </motion.span>
        <span className="text-xs text-[var(--color-ink-muted)]">pending</span>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
        {pending.slice(0, 4).map((a) => (
          <button
            key={a.id}
            onClick={() => navigate('/alerts')}
            className="flex w-full flex-col gap-1 rounded-md px-2 py-2 text-left transition-colors hover:bg-[var(--color-surface-raised)]"
          >
            <span className="flex items-center justify-between gap-2">
              <SeverityTag severity={a.severity} />
              <span className="font-mono text-2xs text-[var(--color-ink-faint)]">
                {relativeTime(a.createdAt)}
              </span>
            </span>
            <span className="line-clamp-2 text-sm text-[var(--color-ink)]">{a.title}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

/* ── Recent changes ────────────────────────────────────────────────────── */

export function RecentChangesWidget() {
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch } = useGetDashboardOverview();

  if (isLoading) return <SkeletonRows rows={4} />;
  if (isError || !data)
    return <ErrorState description="Couldn't load changes." onRetry={() => refetch()} />;

  const changes = data.recentChanges ?? [];
  if (changes.length === 0)
    return <EmptyState icon={<CheckIcon size={20} />} title="No recent changes" description="Your lab matches its docs." />;

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      {changes.map((c) => (
        <button
          key={c.id}
          onClick={() => navigate(`/changes/${c.id}`)}
          className="group flex w-full items-start gap-3 border-b border-[var(--color-line-soft)] px-4 py-2.5 text-left transition-colors last:border-0 hover:bg-[var(--color-surface-raised)]"
        >
          <span className="pt-0.5">
            <SeverityTag severity={c.severity} />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm text-[var(--color-ink)]">{c.summary}</span>
            <span className="flex items-center gap-1.5 font-mono text-2xs text-[var(--color-ink-faint)]">
              <span className="text-[var(--color-signal-bright)]">{c.serviceName}</span>
              <span>·</span>
              <span>{c.changeType}</span>
              <span>·</span>
              <span>{relativeTime(c.detectedAt)}</span>
              {c.willTriggerAi && (
                <span className="ml-0.5 rounded bg-[var(--color-signal-tint)] px-1 text-[var(--color-signal)]">
                  AI
                </span>
              )}
            </span>
          </span>
        </button>
      ))}
    </div>
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

export function SyncActivityWidget() {
  const activity = useLive((s) => s.activity);
  const job = useLive((s) => s.jobs.global);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {job && job.phase !== 'done' && (
        <div className="border-b border-[var(--color-line-soft)] px-4 py-3">
          <div className="mb-1.5 flex items-center justify-between text-xs">
            <span className="font-medium text-[var(--color-ink)]">Fleet sync · {job.phase}</span>
            <span className="nums font-mono text-[var(--color-signal-bright)]">{job.percent}%</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-[var(--color-canvas-sunken)]">
            <motion.div
              className="h-full rounded-full bg-[var(--color-signal)]"
              animate={{ width: `${job.percent}%` }}
              transition={{ ease: [0.16, 1, 0.3, 1], duration: 0.5 }}
            />
          </div>
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-2">
        {activity.length === 0 ? (
          <EmptyState title="Quiet" description="Sync activity will stream here in real time." />
        ) : (
          <ol className="relative ml-1 border-l border-[var(--color-line-soft)]">
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
                      className="absolute -left-[5px] top-3 h-2 w-2 rounded-full"
                      style={{ backgroundColor: fg, boxShadow: `0 0 8px ${fg}` }}
                    />
                    <p className="text-sm text-[var(--color-ink)]">{e.label}</p>
                    {e.detail && (
                      <p className="font-mono text-2xs text-[var(--color-ink-faint)]">
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
  );
}

/* ── Docs health ───────────────────────────────────────────────────────── */

export function DocsHealthWidget() {
  const navigate = useNavigate();
  const { data: tree, isLoading, isError, refetch } = useGetDocsTree();
  const { data: connectors } = useGetConnectors();

  if (isLoading) return <SkeletonRows rows={3} />;
  if (isError || !tree)
    return <ErrorState description="Couldn't load docs." onRetry={() => refetch()} />;

  const docNodes = tree.children ?? [];
  const totalServices = connectors?.length ?? docNodes.length;
  const documented = docNodes.length;
  const pct = totalServices ? Math.round((documented / totalServices) * 100) : 0;

  return (
    <div className="flex min-h-0 flex-1 flex-col p-4">
      <div className="flex items-center gap-4">
        <CoverageRing pct={pct} />
        <div>
          <p className="nums text-sm text-[var(--color-ink)]">
            <span className="text-lg font-semibold">{documented}</span>
            <span className="text-[var(--color-ink-faint)]"> / {totalServices} services documented</span>
          </p>
          <p className="text-xs text-[var(--color-ink-muted)]">
            {totalServices - documented > 0
              ? `${totalServices - documented} awaiting first generation`
              : 'Every service has live documentation'}
          </p>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap gap-2">
        {docNodes.map((d) => (
          <button
            key={d.docId}
            onClick={() => navigate(`/docs/${d.docId}`)}
            className="flex items-center gap-1.5 rounded-md border border-[var(--color-line-soft)] bg-[var(--color-canvas-sunken)] px-2 py-1 text-xs text-[var(--color-ink-muted)] transition-colors hover:border-[var(--color-signal-soft)] hover:text-[var(--color-ink)]"
          >
            <FileTextIcon size={13} />
            {d.title.split(' — ')[0]}
          </button>
        ))}
      </div>

      <button
        onClick={() => navigate('/docs')}
        className="mt-3 flex items-center justify-center gap-1.5 rounded-md border border-[var(--color-line-soft)] py-2 text-xs font-medium text-[var(--color-ink-muted)] transition-colors hover:bg-[var(--color-surface-raised)] hover:text-[var(--color-ink)]"
      >
        Open documentation
        <ArrowRightIcon size={14} />
      </button>
    </div>
  );
}

function CoverageRing({ pct }: { pct: number }) {
  const r = 26;
  const c = 2 * Math.PI * r;
  return (
    <div className="relative h-16 w-16 shrink-0">
      <svg viewBox="0 0 64 64" className="h-16 w-16 -rotate-90">
        <circle cx="32" cy="32" r={r} fill="none" stroke="var(--color-canvas-sunken)" strokeWidth="6" />
        <motion.circle
          cx="32"
          cy="32"
          r={r}
          fill="none"
          stroke="var(--color-signal)"
          strokeWidth="6"
          strokeLinecap="round"
          strokeDasharray={c}
          initial={{ strokeDashoffset: c }}
          animate={{ strokeDashoffset: c - (c * pct) / 100 }}
          transition={{ duration: 0.9, ease: [0.16, 1, 0.3, 1] }}
        />
      </svg>
      <span className="nums absolute inset-0 flex items-center justify-center text-sm font-semibold text-[var(--color-ink)]">
        {pct}%
      </span>
    </div>
  );
}
