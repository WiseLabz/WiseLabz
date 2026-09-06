/** Alerts — drift the diff engine flagged for a human. Resolve, dismiss, snooze. */
import { useState } from 'react';
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetAlerts,
  getGetAlertsQueryKey,
  postAlertsAlertIdResolve,
  postAlertsAlertIdDismiss,
  postAlertsAlertIdSnooze,
} from '../../api/generated/alerts/alerts';
import { SeverityTag } from '../../components/ui/StatusDot';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Pagination } from '../../components/ui/Pagination';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { SavedViewsMenu } from '../../components/views/SavedViewsMenu';
import { relativeTime } from '../../lib/time';
import { CheckIcon, XIcon, ClockIcon } from '../../components/icons';
import type { Severity } from '../../api/model';

const FILTERS: { key: string; value: Severity | 'all' }[] = [
  { key: 'alerts.filterAll', value: 'all' },
  { key: 'alerts.filterCritical', value: 'critical' },
  { key: 'alerts.filterWarning', value: 'warning' },
  { key: 'alerts.filterInfo', value: 'info' },
];

export function AlertsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [severity, setSeverity] = useState<Severity | 'all'>('all');
  const pageSize = 20;
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetAlerts({ page, pageSize });
  const pending = (data?.items ?? []).filter(
    (a) => a.status === 'pending' && (severity === 'all' || a.severity === severity)
  );
  const pageCount = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;

  const resolveAlert = useMutation({
    mutationFn: (alertId: string) => postAlertsAlertIdResolve(alertId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAlertsQueryKey() }),
  });
  const dismissAlert = useMutation({
    mutationFn: (alertId: string) => postAlertsAlertIdDismiss(alertId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAlertsQueryKey() }),
  });
  const snoozeAlert = useMutation({
    mutationFn: (alertId: string) =>
      postAlertsAlertIdSnooze(alertId, { until: new Date(Date.now() + 60 * 60 * 1000).toISOString() }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAlertsQueryKey() }),
  });

  return (
    <div className="mx-auto max-w-205 px-6 py-6">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-ink">{t('alerts.title')}</h1>
          <p className="text-sm text-ink-muted">{t('alerts.subtitle')}</p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-lg border border-line-soft bg-canvas-sunken p-0.5">
            {FILTERS.map((f) => (
              <button
                key={f.value}
                onClick={() => setSeverity(f.value)}
                className="relative rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors"
                style={{
                  color: severity === f.value ? 'var(--color-ink)' : 'var(--color-ink-muted)',
                }}
              >
                {severity === f.value && (
                  <motion.span
                    layoutId="alert-filter"
                    className="absolute inset-0 -z-10 rounded-md bg-surface-raised"
                    transition={{ type: 'spring', stiffness: 500, damping: 36 }}
                  />
                )}
                {t(f.key)}
              </button>
            ))}
          </div>
          <SavedViewsMenu
            surface="alerts"
            filters={{ severity }}
            onApply={(f) => setSeverity(f.severity ?? 'all')}
          />
        </div>
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
                      <span className="text-accent-secondary-bright">{a.serviceName}</span> ·{' '}
                      {t('common.ago', { time: relativeTime(a.createdAt) })}
                    </p>
                  </div>
                </div>
                <div className="mt-3 flex items-center justify-end gap-2 border-t border-line-soft pt-3">
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={snoozeAlert.isPending}
                    onClick={() => snoozeAlert.mutate(a.id)}
                  >
                    <ClockIcon size={14} /> {t('common.snooze')}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={dismissAlert.isPending}
                    onClick={() => dismissAlert.mutate(a.id)}
                  >
                    <XIcon size={14} /> {t('common.dismiss')}
                  </Button>
                  <Button
                    variant="primary"
                    size="sm"
                    disabled={resolveAlert.isPending}
                    onClick={() => resolveAlert.mutate(a.id)}
                  >
                    <CheckIcon size={14} /> {t('common.resolve')}
                  </Button>
                </div>
              </Panel>
            </motion.div>
          ))}
          {pageCount > 1 && (
            <Pagination page={page} pageCount={pageCount} onPage={setPage} className="justify-center pt-1" />
          )}
        </div>
      )}
    </div>
  );
}
