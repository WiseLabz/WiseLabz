/**
 * Dashboard — the flagship surface. A 6-column widget grid that varies cell
 * shape by content (no identical card grid). View mode: staggered entrance +
 * a live sync sweep across the canvas during a fleet sync. Edit mode: a
 * springy drag-to-reorder list + enable toggles, persisted per browser.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AnimatePresence, motion, Reorder, useDragControls } from 'motion/react';
import { ErrorBoundary } from 'react-error-boundary';
import { useDashboard, type WidgetId } from '../../store/dashboard';
import { useUi } from '../../store/ui';
import { useLive } from '../../store/live';
import { useGetDashboardOverview } from '../../api/generated/dashboard/dashboard';
import { relativeTime } from '../../lib/time';
import { Button } from '../../components/ui/Button';
import { ErrorState } from '../../components/ui/states';
import { WidgetFrame } from '../../components/dashboard/WidgetFrame';
import {
  ServiceRosterWidget,
  AlertSummaryWidget,
  RecentChangesWidget,
  SyncActivityWidget,
  DocsHealthWidget,
} from '../../components/dashboard/widgets';
import {
  LayersIcon,
  BellIcon,
  DiffIcon,
  SyncIcon,
  FileTextIcon,
  GripIcon,
  CheckIcon,
} from '../../components/icons';

interface WidgetMeta {
  title: string;
  Icon: React.ComponentType<{ size?: number; className?: string }>;
  Component: React.ComponentType;
  minH: number;
}

const REGISTRY: Record<WidgetId, WidgetMeta> = {
  roster: { title: 'Service roster', Icon: LayersIcon, Component: ServiceRosterWidget, minH: 420 },
  alerts: { title: 'Alerts', Icon: BellIcon, Component: AlertSummaryWidget, minH: 420 },
  changes: { title: 'Recent changes', Icon: DiffIcon, Component: RecentChangesWidget, minH: 300 },
  sync: { title: 'Sync activity', Icon: SyncIcon, Component: SyncActivityWidget, minH: 300 },
  docs: { title: 'Documentation', Icon: FileTextIcon, Component: DocsHealthWidget, minH: 200 },
};

const SPAN_CLASS: Record<number, string> = {
  2: 'lg:col-span-2',
  3: 'lg:col-span-3',
  4: 'lg:col-span-4',
  6: 'lg:col-span-6',
};

export function DashboardPage() {
  const { t } = useTranslation();
  const widgetTitle = (id: WidgetId) => t(`dashboard.widget.${id}`);
  const layout = useDashboard((s) => s.layout);
  const editing = useUi((s) => s.editingDashboard);
  const setEditing = useUi((s) => s.setEditingDashboard);
  const lastSync = useLive((s) => s.activity.find((a) => a.kind === 'sync')?.at);
  const job = useLive((s) => s.jobs.global);
  const syncing = !!job && job.phase !== 'done' && job.phase !== 'error';

  const visible = layout.filter((w) => w.enabled);

  return (
    <div className="relative mx-auto max-w-330 px-6 py-7">
      <SyncSweep active={syncing} />

      {/* Instrument readout band — the command surface. Big mono figures on a soft
          raised panel; a faint blueprint grid keeps the identity without the hard edge. */}
      <header className="relative mb-6 overflow-hidden rounded-xl border border-line-soft bg-surface shadow-(--shadow-raised)">
        <div className="absolute inset-0 bg-blueprint opacity-[0.16]" aria-hidden="true" />
        <div className="relative flex flex-wrap items-end justify-between gap-6 px-5 py-4">
          <div className="flex items-end gap-x-7 gap-y-3">
            <div className="pr-1">
              <p className="font-mono text-2xs uppercase tracking-[0.2em] text-ink-faint">
                {t('dashboard.kicker')}
              </p>
              <p className="mt-1 max-w-[22ch] text-sm leading-snug text-ink-muted">
                {syncing ? t('dashboard.reconciling') : t('dashboard.reconciled')}
              </p>
            </div>
            <StatusReadout />
          </div>

          <div className="flex items-center gap-4">
            <span className="hidden font-mono text-2xs text-ink-faint sm:inline">
              {t('dashboard.lastSync', { time: lastSync ? `${relativeTime(lastSync)} ago` : '—' })}
            </span>
            <Button
              variant={editing ? 'primary' : 'secondary'}
              size="sm"
              onClick={() => setEditing(!editing)}
            >
              {editing ? <CheckIcon size={15} /> : <GripIcon size={15} />}
              {editing ? t('common.done') : t('dashboard.editLayout')}
            </Button>
          </div>
        </div>
      </header>

      <AnimatePresence mode="wait">
        {editing ? (
          <EditMode key="edit" />
        ) : (
          <motion.div
            key="view"
            className="grid grid-cols-1 gap-px overflow-hidden rounded-xl border border-line-soft bg-line-soft shadow-(--shadow-panel) lg:grid-cols-6"
            initial="hidden"
            animate="show"
            variants={{ show: { transition: { staggerChildren: 0.05 } } }}
          >
            {visible.map((w) => {
              const meta = REGISTRY[w.id];
              return (
                <motion.div
                  key={w.id}
                  layout
                  variants={{
                    hidden: { opacity: 0, y: 16 },
                    show: { opacity: 1, y: 0 },
                  }}
                  transition={{ type: 'spring', stiffness: 320, damping: 30 }}
                  className={`${SPAN_CLASS[w.span]} bg-canvas`}
                  style={{ minHeight: meta.minH }}
                >
                  <ErrorBoundary
                    fallbackRender={({ resetErrorBoundary }) => (
                      <WidgetFrame title={widgetTitle(w.id)} icon={<meta.Icon size={15} />}>
                        <ErrorState
                          description={t('dashboard.widgetFailed', { title: widgetTitle(w.id) })}
                          onRetry={resetErrorBoundary}
                        />
                      </WidgetFrame>
                    )}
                  >
                    <WidgetFrame title={widgetTitle(w.id)} icon={<meta.Icon size={15} />}>
                      <meta.Component />
                    </WidgetFrame>
                  </ErrorBoundary>
                </motion.div>
              );
            })}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

/* ── Edit mode: reorder + toggle ───────────────────────────────────────── */

function EditMode() {
  const { t } = useTranslation();
  const layout = useDashboard((s) => s.layout);
  const setOrder = useDashboard((s) => s.setOrder);
  const toggle = useDashboard((s) => s.toggle);
  const [ids, setIds] = useState<WidgetId[]>(layout.map((w) => w.id));

  const commit = (next: WidgetId[]) => {
    setIds(next);
    setOrder(next);
  };

  return (
    <motion.div
      key="edit"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="mx-auto max-w-2xl"
    >
      <p className="mb-3 text-sm text-ink-muted">{t('dashboard.editHint')}</p>
      <Reorder.Group axis="y" values={ids} onReorder={commit} className="flex flex-col gap-2">
        {ids.map((id) => (
          <EditRow
            key={id}
            id={id}
            enabled={layout.find((w) => w.id === id)?.enabled ?? true}
            onToggle={() => toggle(id)}
          />
        ))}
      </Reorder.Group>
    </motion.div>
  );
}

function EditRow({
  id,
  enabled,
  onToggle,
}: {
  id: WidgetId;
  enabled: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslation();
  const controls = useDragControls();
  const meta = REGISTRY[id];
  return (
    <Reorder.Item
      value={id}
      dragListener={false}
      dragControls={controls}
      className="flex items-center gap-3 rounded-lg border border-line-soft bg-surface px-3 py-2.5 shadow-(--shadow-panel)"
      whileDrag={{ scale: 1.02, boxShadow: 'var(--shadow-pop)' }}
    >
      <button
        onPointerDown={(e) => controls.start(e)}
        aria-label={t('dashboard.dragReorder')}
        className="cursor-grab text-ink-faint active:cursor-grabbing"
      >
        <GripIcon size={16} />
      </button>
      <meta.Icon size={16} className="text-ink-muted" />
      <span className="flex-1 text-sm font-medium text-ink">{t(`dashboard.widget.${id}`)}</span>
      <button
        role="switch"
        aria-checked={enabled}
        onClick={onToggle}
        className="relative h-5 w-9 rounded-full transition-colors"
        style={{ backgroundColor: enabled ? 'var(--color-signal)' : 'var(--color-line-strong)' }}
      >
        <motion.span
          layout
          transition={{ type: 'spring', stiffness: 600, damping: 34 }}
          className="absolute top-0.5 h-4 w-4 rounded-full bg-ink"
          style={{ left: enabled ? '18px' : '2px' }}
        />
      </button>
    </Reorder.Item>
  );
}

/* ── Sync sweep: the signature motion moment ───────────────────────────── */

function SyncSweep({ active }: { active: boolean }) {
  return (
    <AnimatePresence>
      {active && (
        <motion.div
          className="pointer-events-none absolute inset-0 z-1 overflow-hidden"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
        >
          {/* a single bright scan line, blueprint-style, sweeping the canvas */}
          <motion.div
            className="absolute inset-x-0 h-24"
            style={{
              background:
                'linear-gradient(to bottom, transparent, var(--color-signal-tint), transparent)',
            }}
            initial={{ y: '-30%' }}
            animate={{ y: '160%' }}
            transition={{ repeat: Infinity, duration: 1.6, ease: 'linear' }}
          >
            <span className="absolute bottom-0 left-0 right-0 h-px bg-signal" />
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

/* ── Status readout: large mono instrument figures ─────────────────────── */

function StatusReadout() {
  const { t } = useTranslation();
  const { data } = useGetDashboardOverview();
  const c = data?.statusCounts;
  const total = c ? (c.online ?? 0) + (c.degraded ?? 0) + (c.offline ?? 0) + (c.unknown ?? 0) : 0;

  const cells: { label: string; value: number; tone: string }[] = [
    { label: t('dashboard.readout.services'), value: total, tone: 'var(--color-ink)' },
    { label: t('dashboard.readout.online'), value: c?.online ?? 0, tone: 'var(--color-ok)' },
    { label: t('dashboard.readout.degraded'), value: c?.degraded ?? 0, tone: 'var(--color-warn)' },
    { label: t('dashboard.readout.offline'), value: c?.offline ?? 0, tone: 'var(--color-err)' },
    {
      label: t('dashboard.readout.alerts'),
      value: data?.pendingAlerts ?? 0,
      tone: 'var(--color-signal-bright)',
    },
  ];

  return (
    <div className="flex items-end gap-x-6 gap-y-2">
      {cells.map((cell) => (
        <div key={cell.label} className="flex flex-col">
          <span
            className="nums font-mono text-3xl font-semibold leading-none tabular-nums"
            style={{ color: cell.tone }}
          >
            {String(cell.value).padStart(2, '0')}
          </span>
          <span className="mt-1.5 font-mono text-2xs uppercase tracking-[0.18em] text-ink-faint">
            {cell.label}
          </span>
        </div>
      ))}
    </div>
  );
}
