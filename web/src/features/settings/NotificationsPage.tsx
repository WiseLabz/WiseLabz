/**
 * Settings → Notifications (operator-only). Channel enablement + a per-channel
 * test, plus the full event×channel routing matrix (EventRoutingTable). Local
 * draft state; "Save routing" persists the whole config.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetNotificationsConfig,
  putNotificationsConfig,
  postNotificationsConfigTest,
  getGetNotificationsConfigQueryKey,
} from '../../api/generated/settings/settings';
import type { NotificationConfig, NotificationChannelType } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState } from '../../components/ui/states';
import { toast } from '../../lib/toast';
import { SubHeader, Section, ToggleRow } from './parts';
import { EventRoutingTable } from './EventRoutingTable';

const CHANNEL_LABELS: Record<NotificationChannelType, string> = {
  in_app: 'In-app',
  smtp: 'Email (SMTP)',
  webhook: 'Webhook',
};

export function NotificationsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetNotificationsConfig();

  const [draft, setDraft] = useState<NotificationConfig | null>(null);
  // Adjust state during render (React-blessed alternative to a syncing effect):
  // re-seed whenever the query yields a fresh reference, e.g. after an invalidate.
  const [seeded, setSeeded] = useState<NotificationConfig | null>(null);
  if (data && data !== seeded) {
    setSeeded(data);
    setDraft({ channels: data.channels.map((c) => ({ ...c })), routing: data.routing.map((r) => ({ ...r })) });
  }

  const save = useMutation({
    mutationFn: (body: NotificationConfig) => putNotificationsConfig(body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetNotificationsConfigQueryKey() });
      toast.success(t('settings.notifications.saved', { defaultValue: 'Notification routing saved.' }));
    },
    onError: () => toast.error(t('settings.notifications.saveError', { defaultValue: 'Could not save routing.' })),
  });

  const test = useMutation({
    mutationFn: (channel: NotificationChannelType) => postNotificationsConfigTest({ channel }),
    onSuccess: (result) => {
      if (result.ok) toast.success(result.message ?? t('settings.notifications.testOk', { defaultValue: 'Test sent.' }));
      else toast.error(result.message ?? t('settings.notifications.testFail', { defaultValue: 'Test failed.' }));
    },
    onError: () => toast.error(t('settings.notifications.testFail', { defaultValue: 'Test failed.' })),
  });

  if (isLoading) return <Loading />;
  if (isError || !data || !draft)
    return (
      <div>
        <SubHeader title={t('settings.notifications.title', { defaultValue: 'Notifications' })} />
        <ErrorState
          description={t('settings.notifications.loadError', { defaultValue: 'Could not load notification settings.' })}
          onRetry={() => refetch()}
        />
      </div>
    );

  const setChannelEnabled = (type: NotificationChannelType, enabled: boolean) =>
    setDraft((d) =>
      d ? { ...d, channels: d.channels.map((c) => (c.type === type ? { ...c, enabled } : c)) } : d,
    );

  const dirty = JSON.stringify(draft) !== JSON.stringify(data);

  return (
    <div>
      <SubHeader
        title={t('settings.notifications.title', { defaultValue: 'Notifications' })}
        description={t('settings.notifications.subtitle', { defaultValue: 'Where events go and at what severity.' })}
      />

      <Section title={t('settings.notifications.channelsTitle', { defaultValue: 'Channels' })}>
        <div className="space-y-3">
          {draft.channels.map((c) => (
            <ToggleRow
              key={c.type}
              title={t(`settings.notifications.channel.${c.type}`, { defaultValue: CHANNEL_LABELS[c.type] })}
              description={
                c.type === 'in_app'
                  ? t('settings.notifications.inAppDesc', { defaultValue: 'Live toasts and the activity feed.' })
                  : c.type === 'smtp'
                    ? t('settings.notifications.smtpDesc', { defaultValue: 'Email via the configured SMTP server.' })
                    : t('settings.notifications.webhookDesc', { defaultValue: 'POST events to an external URL.' })
              }
              checked={c.enabled}
              onChange={(enabled) => setChannelEnabled(c.type, enabled)}
            />
          ))}
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          {draft.channels.map((c) => (
            <Button
              key={c.type}
              variant="ghost"
              size="sm"
              disabled={!c.enabled || test.isPending}
              onClick={() => test.mutate(c.type)}
            >
              {t('settings.notifications.testChannel', {
                defaultValue: 'Test {{channel}}',
                channel: t(`settings.notifications.channel.${c.type}`, { defaultValue: CHANNEL_LABELS[c.type] }),
              })}
            </Button>
          ))}
        </div>
      </Section>

      <Section
        title={t('settings.notifications.routingTitle', { defaultValue: 'Event routing' })}
        description={t('settings.notifications.routingDesc', {
          defaultValue: 'Per event, choose which enabled channels fire and the minimum severity that triggers them.',
        })}
        action={
          <Button
            variant="primary"
            size="sm"
            disabled={!dirty || save.isPending}
            onClick={() => save.mutate(draft)}
          >
            {t('settings.notifications.saveRouting', { defaultValue: 'Save routing' })}
          </Button>
        }
      >
        <EventRoutingTable config={draft} onChange={setDraft} disabled={save.isPending} />
      </Section>
    </div>
  );
}

function Loading() {
  return (
    <div>
      <SubHeader title="Notifications" />
      <SkeletonRows rows={6} />
    </div>
  );
}
