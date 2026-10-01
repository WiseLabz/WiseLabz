/**
 * Availability + MTTR strip (24h / 7d / 30d) and a status/latency sparkline
 * for one connector, fed by GET /connectors/{id}/uptime and
 * /uptime/history. A window with zero checks renders "No data", never 0%.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  useGetConnectorsConnectorIdUptime,
  useGetConnectorsConnectorIdUptimeHistory,
} from '../../api/generated/connectors/connectors';
import type { UptimeHistoryBucket, UptimeWindowStats } from '../../api/model';
import { Panel } from '../../components/ui/Panel';
import { SkeletonRows, ErrorState } from '../../components/ui/states';
import { toneColor, statusMeta } from '../../components/ui/status';
import { durationLabel } from '../../lib/time';

const WINDOWS = ['24h', '7d', '30d'] as const;
type Window = (typeof WINDOWS)[number];

export function UptimePanel({ id }: Readonly<{ id: string }>) {
  const { t } = useTranslation();
  const [window, setWindow] = useState<Window>('24h');
  const uptime = useGetConnectorsConnectorIdUptime(id);
  const history = useGetConnectorsConnectorIdUptimeHistory(id, { window });

  return (
    <Panel className="p-5">
      <h2 className="mb-3 text-sm font-semibold text-ink">{t('services.detail.uptime.title')}</h2>
      {uptime.isLoading ? (
        <SkeletonRows rows={2} />
      ) : uptime.isError || !uptime.data ? (
        <ErrorState
          description={t('services.detail.uptime.loadError')}
          onRetry={() => uptime.refetch()}
        />
      ) : (
        <div className="grid grid-cols-3 gap-3">
          {WINDOWS.map((w) => (
            <WindowStat key={w} label={w} stats={uptime.data.windows[w]} />
          ))}
        </div>
      )}

      <div className="mt-4 flex items-center justify-between">
        <p className="text-2xs text-ink-faint">
          {t('services.detail.uptime.historyLabel', { window })}
        </p>
        <div className="flex gap-0.5" role="group" aria-label={t('services.detail.uptime.title')}>
          {WINDOWS.map((w) => (
            <button
              key={w}
              onClick={() => setWindow(w)}
              aria-pressed={window === w}
              className="rounded px-1.5 py-1 text-2xs font-medium transition-colors"
              style={{
                color: window === w ? 'var(--color-ink)' : 'var(--color-ink-faint)',
                backgroundColor: window === w ? 'var(--color-surface-raised)' : 'transparent',
              }}
            >
              {w}
            </button>
          ))}
        </div>
      </div>
      <div className="mt-2">
        {history.isLoading ? (
          <SkeletonRows rows={1} />
        ) : history.isError ? (
          <p className="text-2xs text-err">{t('services.detail.uptime.loadError')}</p>
        ) : (history.data?.buckets.length ?? 0) === 0 ? (
          <p className="text-2xs text-ink-faint">{t('services.detail.uptime.noHistory')}</p>
        ) : (
          <Sparkline buckets={history.data!.buckets} />
        )}
      </div>
    </Panel>
  );
}

function WindowStat({ label, stats }: Readonly<{ label: string; stats?: UptimeWindowStats }>) {
  const { t } = useTranslation();
  const hasData = !!stats && stats.checkCount > 0;
  return (
    <div>
      <p className="font-mono text-2xs text-ink-faint">{label}</p>
      {hasData ? (
        <>
          <p className="nums text-lg font-semibold text-ink">{stats.availabilityPct.toFixed(2)}%</p>
          <p className="font-mono text-2xs text-ink-faint">
            {t('services.detail.uptime.mttr')}{' '}
            {stats.outageCount > 0
              ? durationLabel(Math.round(stats.mttrSeconds * 1000))
              : t('services.detail.uptime.mttrNone')}
            {stats.outageCount > 0 && ` · ${t('services.detail.uptime.outages', { count: stats.outageCount })}`}
          </p>
        </>
      ) : (
        <p className="text-sm text-ink-faint">{t('services.detail.uptime.noData')}</p>
      )}
    </div>
  );
}

const W = 280;
const H = 48;
const STRIP_H = 6;

/** Latency polyline over a per-bucket status strip (colour + title, not colour alone). */
function Sparkline({ buckets }: Readonly<{ buckets: UptimeHistoryBucket[] }>) {
  const { t } = useTranslation();
  const lats = buckets.map((b) => b.avgLatencyMs ?? 0);
  const max = Math.max(1, ...lats);
  const step = buckets.length > 1 ? W / (buckets.length - 1) : 0;
  const points = buckets
    .map((b, i) => `${(i * step).toFixed(1)},${(H - ((b.avgLatencyMs ?? 0) / max) * (H - 4) - 2).toFixed(1)}`)
    .join(' ');
  const cell = W / buckets.length;

  return (
    <div>
      <svg
        viewBox={`0 0 ${W} ${H + STRIP_H + 2}`}
        className="h-16 w-full"
        role="img"
        aria-label={t('services.detail.uptime.latencyMax', { ms: max })}
        preserveAspectRatio="none"
      >
        <polyline points={points} fill="none" stroke="var(--color-accent-primary)" strokeWidth="1.5" />
        {buckets.map((b, i) => (
          <rect
            key={b.start}
            x={i * cell}
            y={H + 2}
            width={Math.max(cell - 0.5, 0.5)}
            height={STRIP_H}
            fill={toneColor[statusMeta[b.status].tone].fg}
          >
            <title>{`${b.start} · ${b.status}`}</title>
          </rect>
        ))}
      </svg>
      <p className="font-mono text-2xs text-ink-faint">
        {t('services.detail.uptime.latencyMax', { ms: max })}
      </p>
    </div>
  );
}
