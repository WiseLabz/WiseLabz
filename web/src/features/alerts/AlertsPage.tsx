/** Alerts — drift the diff engine flagged for a human. Resolve, dismiss, snooze. */
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { useGetAlerts } from '../../api/generated/alerts/alerts';
import { SeverityTag } from '../../components/ui/StatusDot';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { relativeTime } from '../../lib/time';
import { CheckIcon, XIcon, ClockIcon } from '../../components/icons';

export function AlertsPage() {
  const { t } = useTranslation();
  const { data, isLoading, isError, refetch } = useGetAlerts(undefined);
  const pending = (data?.items ?? []).filter((a) => a.status === 'pending');

  return (
    <div className="mx-auto max-w-205 px-6 py-6">
      <header className="mb-5">
        <h1 className="text-xl font-semibold tracking-tight text-ink">{t('alerts.title')}</h1>
        <p className="text-sm text-ink-muted">{t('alerts.subtitle')}</p>
      </header>

      {isLoading ? (
        <Panel>
          <SkeletonRows rows={4} />
        </Panel>
      ) : isError || !data ? (
        <Panel className="min-h-[40vh]">
          <ErrorState description={t('alerts.loadError')} onRetry={() => refetch()} />
        </Panel>
      ) : pending.length === 0 ? (
        <Panel className="min-h-[40vh]">
          <EmptyState
            icon={<CheckIcon size={20} />}
            title={t('alerts.allClearTitle')}
            description={t('alerts.allClearDesc')}
          />
        </Panel>
      ) : (
        <div className="flex flex-col gap-3">
          {pending.map((a, idx) => (
            <motion.div
              key={a.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.04, duration: 0.25 }}
            >
              <Panel className="p-4">
                <div className="flex items-start gap-3">
                  <SeverityTag severity={a.severity} className="mt-0.5" />
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-ink">{a.title}</p>
                    {a.description && (
                      <p className="mt-1 text-sm leading-relaxed text-ink-muted">{a.description}</p>
                    )}
                    <p className="mt-1.5 font-mono text-2xs text-ink-faint">
                      <span className="text-signal-bright">{a.serviceName}</span> ·{' '}
                      {t('common.ago', { time: relativeTime(a.createdAt) })}
                    </p>
                  </div>
                </div>
                <div className="mt-3 flex items-center justify-end gap-2 border-t border-line-soft pt-3">
                  <Button variant="ghost" size="sm">
                    <ClockIcon size={14} /> {t('common.snooze')}
                  </Button>
                  <Button variant="ghost" size="sm">
                    <XIcon size={14} /> {t('common.dismiss')}
                  </Button>
                  <Button variant="primary" size="sm">
                    <CheckIcon size={14} /> {t('common.resolve')}
                  </Button>
                </div>
              </Panel>
            </motion.div>
          ))}
        </div>
      )}
    </div>
  );
}
