/**
 * Settings → System (operator-only). Read-only instance health: backend +
 * integration health, the live WebSocket connection state, the sync schedule, and
 * the running version. No mutations here — it's an instrument readout.
 */
import { useTranslation } from 'react-i18next';
import { useGetSystemInfo, useGetHealth } from '../../api/generated/system/system';
import type { HealthComponentsItemStatus } from '../../api/model';
import { useLive, type WsState } from '../../store/live';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { SkeletonRows, ErrorState } from '../../components/ui/states';
import { ToneTag } from '../../components/ui/ToneTag';
import { type Tone } from '../../components/ui/status';
import { SubHeader, Section } from './parts';
import { GaugeIcon, ServerIcon, ClockIcon, NetworkIcon } from '../../components/icons';

const healthTone: Record<HealthComponentsItemStatus, Tone> = {
  ok: 'ok',
  degraded: 'warn',
  down: 'err',
};

const wsTone: Record<WsState, Tone> = {
  open: 'ok',
  connecting: 'warn',
  closed: 'err',
};

export function SystemPage() {
  const { t } = useTranslation();
  const ws = useLive((s) => s.ws);
  const info = useGetSystemInfo();
  const health = useGetHealth();

  if (info.isLoading || health.isLoading) return <Loading />;

  return (
    <div>
      <SubHeader
        title={t('settings.system.title', { defaultValue: 'System' })}
        description={t('settings.system.subtitle', {
          defaultValue: 'Instance health, connectivity, and version.',
        })}
      />

      <div className="mb-4 grid gap-4 sm:grid-cols-3">
        <Stat
          icon={<NetworkIcon size={15} />}
          label={t('settings.system.wsStatus', { defaultValue: 'WebSocket' })}
          value={
            <ToneTag
              tone={wsTone[ws]}
              label={t(`settings.system.ws.${ws}`, {
                defaultValue:
                  ws === 'open' ? 'connected' : ws === 'connecting' ? 'connecting' : 'disconnected',
              })}
            />
          }
        />
        <Stat
          icon={<ClockIcon size={15} />}
          label={t('settings.system.syncSchedule', { defaultValue: 'Sync schedule' })}
          value={
            <span className="font-mono text-sm text-ink">{info.data?.syncSchedule ?? '—'}</span>
          }
        />
        <Stat
          icon={<GaugeIcon size={15} />}
          label={t('settings.system.version', { defaultValue: 'Version' })}
          value={<span className="font-mono text-sm text-ink">v{info.data?.version ?? '—'}</span>}
        />
      </div>

      <Section
        title={t('settings.system.healthTitle', { defaultValue: 'Backend health' })}
        description={
          health.data
            ? t('settings.system.overall', {
                defaultValue: 'Overall: {{status}}',
                status: health.data.status,
              })
            : undefined
        }
      >
        {health.isError || !health.data ? (
          <ErrorState
            description={t('settings.system.healthError', {
              defaultValue: 'Could not load health.',
            })}
            onRetry={() => health.refetch()}
          />
        ) : (
          <ul className="divide-y divide-line-soft">
            {health.data.components.map((c) => (
              <li
                key={c.name}
                className="flex items-center justify-between gap-4 py-2.5 first:pt-0 last:pb-0"
              >
                <div className="min-w-0">
                  <p className="font-mono text-xs text-ink">{c.name}</p>
                  {c.detail && <p className="font-mono text-2xs text-ink-faint">{c.detail}</p>}
                </div>
                <ToneTag tone={healthTone[c.status]} label={c.status} />
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Panel>
        <PanelHeader
          title={t('settings.system.integrationsTitle', { defaultValue: 'Integrations' })}
          icon={<ServerIcon size={14} />}
          count={info.data?.integrations.length}
        />
        {info.isError || !info.data ? (
          <ErrorState
            description={t('settings.system.infoError', {
              defaultValue: 'Could not load instance info.',
            })}
            onRetry={() => info.refetch()}
          />
        ) : (
          <ul className="divide-y divide-line-soft">
            {info.data.integrations.map((i, idx) => (
              <li
                key={i.name ?? idx}
                className="flex items-center justify-between gap-4 px-4 py-2.5"
              >
                <div className="min-w-0">
                  <p className="text-sm text-ink">{i.name}</p>
                  {i.detail && <p className="font-mono text-2xs text-ink-faint">{i.detail}</p>}
                </div>
                {i.status && <ToneTag tone={healthTone[i.status]} label={i.status} />}
              </li>
            ))}
          </ul>
        )}
      </Panel>
    </div>
  );
}

function Stat({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: React.ReactNode;
}) {
  return (
    <Panel className="p-4">
      <p className="mb-2 flex items-center gap-1.5 font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
        <span className="text-signal">{icon}</span>
        {label}
      </p>
      {value}
    </Panel>
  );
}

function Loading() {
  return (
    <div>
      <SubHeader title="System" />
      <SkeletonRows rows={6} />
    </div>
  );
}
