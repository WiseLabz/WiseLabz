/**
 * The notification routing matrix: event types (rows) × channels (columns). Each
 * cell is an on/off toggle deciding whether that event reaches that channel; each
 * row carries a minimum-severity threshold applied to all its channels. Channel
 * columns are disabled when the channel itself is off. Controlled — the parent
 * owns the NotificationConfig and persists it.
 */
import { useTranslation } from 'react-i18next';
import type { NotificationConfig, NotificationChannelType, Severity } from '../../api/model';
import { Severity as SeverityEnum } from '../../api/model';
import { Toggle, Select } from './parts';
import { cn } from '../../lib/cn';

const CHANNEL_LABELS: Record<NotificationChannelType, string> = {
  in_app: 'In-app',
  smtp: 'Email',
  webhook: 'Webhook',
};

function eventLabel(eventType: string): string {
  return eventType.replace(/[._]/g, ' ');
}

export function EventRoutingTable({
  config,
  onChange,
  disabled,
}: {
  config: NotificationConfig;
  onChange: (next: NotificationConfig) => void;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const channels = config.channels;
  const eventTypes = Array.from(new Set(config.routing.map((r) => r.eventType)));

  const findRoute = (eventType: string, channel: NotificationChannelType) =>
    config.routing.find((r) => r.eventType === eventType && r.channel === channel);

  const setCell = (eventType: string, channel: NotificationChannelType, enabled: boolean) => {
    onChange({
      ...config,
      routing: config.routing.map((r) =>
        r.eventType === eventType && r.channel === channel ? { ...r, enabled } : r
      ),
    });
  };

  const setRowSeverity = (eventType: string, minSeverity: Severity) => {
    onChange({
      ...config,
      routing: config.routing.map((r) => (r.eventType === eventType ? { ...r, minSeverity } : r)),
    });
  };

  const rowSeverity = (eventType: string): Severity =>
    findRoute(eventType, channels[0]?.type)?.minSeverity ?? SeverityEnum.info;

  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b border-line-soft">
            <th className="py-2 pr-4 text-left font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.notifications.event')}
            </th>
            {channels.map((c) => (
              <th key={c.type} className="px-3 py-2 text-center">
                <span
                  className={cn(
                    'font-mono text-2xs',
                    c.enabled ? 'text-ink-muted' : 'text-ink-faint line-through'
                  )}
                >
                  {t(`settings.notifications.channel.${c.type}`, {
                    defaultValue: CHANNEL_LABELS[c.type],
                  })}
                </span>
              </th>
            ))}
            <th className="px-3 py-2 text-right font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
              {t('settings.notifications.minSeverity')}
            </th>
          </tr>
        </thead>
        <tbody>
          {eventTypes.map((eventType) => (
            <tr key={eventType} className="border-b border-line-soft last:border-0">
              <td className="py-2.5 pr-4">
                <span className="font-mono text-xs text-ink">{eventLabel(eventType)}</span>
              </td>
              {channels.map((c) => {
                const route = findRoute(eventType, c.type);
                const cellDisabled = disabled || !c.enabled || !route;
                return (
                  <td key={c.type} className="px-3 py-2.5">
                    <div className="flex justify-center">
                      <Toggle
                        size="sm"
                        checked={Boolean(route?.enabled) && c.enabled}
                        disabled={cellDisabled}
                        onChange={(enabled) => setCell(eventType, c.type, enabled)}
                        label={`${eventLabel(eventType)} → ${CHANNEL_LABELS[c.type]}`}
                      />
                    </div>
                  </td>
                );
              })}
              <td className="px-3 py-2.5 text-right">
                <Select
                  aria-label={t('settings.notifications.minSeverityFor', {
                    event: eventLabel(eventType),
                  })}
                  value={rowSeverity(eventType)}
                  disabled={disabled}
                  onChange={(e) => setRowSeverity(eventType, e.target.value as Severity)}
                  className="w-28 py-1.5 text-xs"
                >
                  <option value={SeverityEnum.info}>{t('status.severity.info')}</option>
                  <option value={SeverityEnum.warning}>{t('status.severity.warning')}</option>
                  <option value={SeverityEnum.critical}>{t('status.severity.critical')}</option>
                </Select>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
