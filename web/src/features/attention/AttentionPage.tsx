import { useState } from 'react';
import { motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetAttention,
  getGetAttentionQueryKey,
} from '../../api/generated/attention/attention';
import {
  postAlertsAlertIdResolve,
  postAlertsAlertIdDismiss,
  postAlertsAlertIdSnooze,
} from '../../api/generated/alerts/alerts';
import { postFindingsFindingIdResolve } from '../../api/generated/findings/findings';
import { SeverityTag } from '../../components/ui/StatusDot';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Pagination } from '../../components/ui/Pagination';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { RunbookPanel } from '../../components/runbook/RunbookPanel';
import { relativeTime } from '../../lib/time';
import { CheckIcon, XIcon, ClockIcon } from '../../components/icons';

export function AttentionPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const pageSize = 20;
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetAttention({ page, pageSize });
  const pageCount = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;

  const resolveAlert = useMutation({
    mutationFn: (alertId: string) => postAlertsAlertIdResolve(alertId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAttentionQueryKey() }),
  });

  const dismissAlert = useMutation({
    mutationFn: (alertId: string) => postAlertsAlertIdDismiss(alertId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAttentionQueryKey() }),
  });

  const snoozeAlert = useMutation({
    mutationFn: (alertId: string) =>
      postAlertsAlertIdSnooze(alertId, { until: new Date(Date.now() + 60 * 60 * 1000).toISOString() }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAttentionQueryKey() }),
  });

  const resolveFinding = useMutation({
    mutationFn: (findingId: string) => postFindingsFindingIdResolve(findingId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetAttentionQueryKey() }),
  });

  return (
    <div className="mx-auto max-w-205 px-6 py-6">
      <header className="mb-5">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-ink">{t('attention.title')}</h1>
          <p className="text-sm text-ink-muted">{t('attention.subtitle')}</p>
        </div>
      </header>

      {isLoading ? (
        <Panel>
          <SkeletonRows rows={4} />
        </Panel>
      ) : isError || !data ? (
        <Panel className="min-h-[40vh]">
          <ErrorState description={t('attention.loadError')} onRetry={() => refetch()} />
        </Panel>
      ) : data.data.length === 0 ? (
        <Panel className="min-h-[40vh]">
          <EmptyState
            icon={<CheckIcon size={20} />}
            title={t('attention.emptyTitle')}
            description={t('attention.emptyDesc')}
          />
        </Panel>
      ) : (
        <div className="flex flex-col gap-3">
          {data.data.map((item: typeof data.data[number], idx: number) => (
            <motion.div
              key={item.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.04, duration: 0.25 }}
            >
              <Panel className="p-4">
                <div className="flex items-start gap-3">
                  <SeverityTag severity={item.severity} className="mt-0.5" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="inline-block rounded px-2 py-1 text-2xs font-medium" style={{
                        backgroundColor: item.kind === 'alert' ? 'var(--color-surface-raised)' : 'var(--color-surface)',
                        color: 'var(--color-ink-muted)',
                      }}>
                        {item.kind === 'alert' ? 'Alert' : 'Finding'}
                      </span>
                      <p className="text-sm font-medium text-ink">{item.title}</p>
                    </div>
                    <p className="mt-1.5 font-mono text-2xs text-ink-faint">
                      <span className="text-accent-secondary-bright">{item.connectorId}</span> ·{' '}
                      {t('common.ago', { time: relativeTime(item.detectedAt) })}
                    </p>
                  </div>
                </div>

                {item.runbookId && <RunbookPanel runbookId={item.runbookId} />}

                <div className="mt-3 flex items-center justify-end gap-2 border-t border-line-soft pt-3">
                  {item.kind === 'alert' ? (
                    <>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={snoozeAlert.isPending}
                        onClick={() => snoozeAlert.mutate(item.id)}
                      >
                        <ClockIcon size={14} /> {t('common.snooze')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={dismissAlert.isPending}
                        onClick={() => dismissAlert.mutate(item.id)}
                      >
                        <XIcon size={14} /> {t('common.dismiss')}
                      </Button>
                      <Button
                        variant="primary"
                        size="sm"
                        disabled={resolveAlert.isPending}
                        onClick={() => resolveAlert.mutate(item.id)}
                      >
                        <CheckIcon size={14} /> {t('common.resolve')}
                      </Button>
                    </>
                  ) : (
                    <>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={resolveFinding.isPending}
                        onClick={() => resolveFinding.mutate(item.id)}
                      >
                        <CheckIcon size={14} /> {t('common.resolve')}
                      </Button>
                    </>
                  )}
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
