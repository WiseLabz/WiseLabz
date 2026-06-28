/**
 * Dashboard — the flagship surface. A 6-column widget grid that varies cell
 * shape by content (no identical card grid). View mode: staggered entrance +
 * a live sync sweep across the canvas during a fleet sync. Edit mode: a
 * springy drag-to-reorder list + enable toggles, persisted per browser.
 */
import { useState } from 'react';
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
  const layout = useDashboard((s) => s.layout);
  const editing = useUi((s) => s.editingDashboard);
  const setEditing = useUi((s) => s.setEditingDashboard);
  const lastSync = useLive((s) => s.activity.find((a) => a.kind === 'sync')?.at);
  const job = useLive((s) => s.jobs.global);
  const syncing = !!job && job.phase !== 'done' && job.phase !== 'error';

  const visible = layout.filter((w) => w.enabled);

  return (
    <div className="relative mx-auto max-w-[1320px] px-6 py-7">
      <SyncSweep active={syncing} />

      {/* Instrument readout band — the command surface. Big mono figures on a soft
          raised panel; a faint blueprint grid keeps the identity without the hard edge. */}
      <header className="relative mb-6 overflow-hidden rounded-xl border border-[var(--color-line-soft)] bg-[var(--color-surface)] shadow-[var(--shadow-raised)]">
        <div className="absolute inset-0 bg-blueprint opacity-[0.16]" aria-hidden="true" />
        <div className="relative flex flex-wrap items-end justify-between gap-6 px-5 py-4">
          <div className="flex items-end gap-x-7 gap-y-3">
            <div className="pr-1">
              <p className="font-mono text-2xs uppercase tracking-[0.2em] text-[var(--color-ink-faint)]">
                Wiselabz / overview
              </p>
              <p className="mt-1 max-w-[22ch] text-sm leading-snug text-[var(--color-ink-muted)]">
                {syncing
                  ? 'Reconciling fleet against documentation…'
                  : 'Live state, reconciled against the docs.'}
              </p>
            </div>
            <StatusReadout />
          </div>

          <div className="flex items-center gap-4">
            <span className="hidden font-mono text-2xs text-[var(--color-ink-faint)] sm:inline">
              last sync {lastSync ? `${relativeTime(lastSync)} ago` : '—'}
            </span>
            <Button
              variant={editing ? 'primary' : 'secondary'}
              size="sm"
              onClick={() => setEditing(!editing)}
            >
              {editing ? <CheckIcon size={15} /> : <GripIcon size={15} />}
              {editing ? 'Done' : 'Edit layout'}
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
            className="grid grid-cols-1 gap-px overflow-hidden rounded-xl border border-[var(--color-line-soft)] bg-[var(--color-line-soft)] shadow-[var(--shadow-panel)] lg:grid-cols-6"
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
                  className={`${SPAN_CLASS[w.span]} bg-[var(--color-canvas)]`}
                  style={{ minHeight: meta.minH }}
                >
                  <ErrorBoundary
                    fallbackRender={({ resetErrorBoundary }) => (
                      <WidgetFrame title={meta.title} icon={<meta.Icon size={15} />}>
                        <ErrorState description={`${meta.title} failed.`} onRetry={resetErrorBoundary} />
                      </WidgetFrame>
                    )}
                  >
                    <WidgetFrame title={meta.title} icon={<meta.Icon size={15} />}>
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
      <p className="mb-3 text-sm text-[var(--color-ink-muted)]">
        Drag to reorder. Toggle widgets on or off. Your layout is saved automatically.
      </p>
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
  const controls = useDragControls();
  const meta = REGISTRY[id];
  return (
    <Reorder.Item
      value={id}
      dragListener={false}
      dragControls={controls}
      className="flex items-center gap-3 rounded-lg border border-[var(--color-line-soft)] bg-[var(--color-surface)] px-3 py-2.5 shadow-[var(--shadow-panel)]"
      whileDrag={{ scale: 1.02, boxShadow: 'var(--shadow-pop)' }}
    >
      <button
        onPointerDown={(e) => controls.start(e)}
        aria-label="Drag to reorder"
        className="cursor-grab text-[var(--color-ink-faint)] active:cursor-grabbing"
      >
        <GripIcon size={16} />
      </button>
      <meta.Icon size={16} className="text-[var(--color-ink-muted)]" />
      <span className="flex-1 text-sm font-medium text-[var(--color-ink)]">{meta.title}</span>
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
          className="absolute top-0.5 h-4 w-4 rounded-full bg-white"
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
          className="pointer-events-none absolute inset-0 z-[1] overflow-hidden"
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
            <span className="absolute bottom-0 left-0 right-0 h-px bg-[var(--color-signal)]" />
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

/* ── Status readout: large mono instrument figures ─────────────────────── */

function StatusReadout() {
  const { data } = useGetDashboardOverview();
  const c = data?.statusCounts;
  const total = c ? (c.online ?? 0) + (c.degraded ?? 0) + (c.offline ?? 0) + (c.unknown ?? 0) : 0;

  const cells: { label: string; value: number; tone: string }[] = [
    { label: 'services', value: total, tone: 'var(--color-ink)' },
    { label: 'online', value: c?.online ?? 0, tone: 'var(--color-ok)' },
    { label: 'degraded', value: c?.degraded ?? 0, tone: 'var(--color-warn)' },
    { label: 'offline', value: c?.offline ?? 0, tone: 'var(--color-err)' },
    { label: 'alerts', value: data?.pendingAlerts ?? 0, tone: 'var(--color-signal-bright)' },
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
          <span className="mt-1.5 font-mono text-2xs uppercase tracking-[0.18em] text-[var(--color-ink-faint)]">
            {cell.label}
          </span>
        </div>
      ))}
    </div>
  );
}
