/** Single change — metadata, the DiffViewer, and the accept/reject resolution loop. */
import { useState } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetChangesChangeId,
  postChangesChangeIdAck,
  postChangesChangeIdDismiss,
  getGetChangesChangeIdQueryKey,
} from '../../api/generated/changes/changes';
import { getGetChangesQueryKey } from '../../api/generated/changes/changes';
import { DiffViewer } from '../../components/diff/DiffViewer';
import { SeverityTag } from '../../components/ui/StatusDot';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Skeleton, SkeletonRows, ErrorState } from '../../components/ui/states';
import { fullDate } from '../../lib/time';
import { ArrowRightIcon, CheckIcon, XIcon, SparklesIcon, FileTextIcon } from '../../components/icons';

export function ChangeDetailPage() {
  const { t } = useTranslation();
  const { changeId } = useParams<{ changeId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetChangesChangeId(changeId ?? '');

  // The accept/reject loop. On success the panel plays the resolve motion (PRODUCT
  // motion moment #4) and routes back to the feed; the caches are invalidated so
  // the resolved change drops out of the list.
  const [resolved, setResolved] = useState<null | 'acknowledged' | 'dismissed'>(null);
  const resolve = useMutation({
    mutationFn: (action: 'ack' | 'dismiss') =>
      action === 'ack'
        ? postChangesChangeIdAck(changeId ?? '')
        : postChangesChangeIdDismiss(changeId ?? ''),
    onSuccess: (_res, action) => {
      setResolved(action === 'ack' ? 'acknowledged' : 'dismissed');
      queryClient.invalidateQueries({ queryKey: getGetChangesQueryKey() });
      if (changeId) {
        queryClient.invalidateQueries({ queryKey: getGetChangesChangeIdQueryKey(changeId) });
      }
    },
  });
  const pending = resolve.isPending;

  return (
    <div className="mx-auto max-w-[840px] px-6 py-6">
      <button
        onClick={() => navigate('/changes')}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-[var(--color-ink-muted)] transition-colors hover:text-[var(--color-ink)]"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('changes.back')}
      </button>

      {isLoading ? (
        <Panel className="p-6">
          <Skeleton className="mb-3 h-6 w-2/3" />
          <SkeletonRows rows={5} />
        </Panel>
      ) : isError || !data ? (
        <Panel className="min-h-[40vh]">
          <ErrorState description={t('changes.detailLoadError')} onRetry={() => refetch()} />
        </Panel>
      ) : (
        <AnimatePresence onExitComplete={() => navigate('/changes')}>
          {!resolved && (
            <motion.div
              key="panel"
              exit={{ opacity: 0, y: -8, scale: 0.985 }}
              transition={{ duration: 0.22, ease: [0.22, 1, 0.36, 1] }}
            >
              <Panel>
          <div className="border-b border-[var(--color-line-soft)] px-6 py-5">
            <div className="mb-2 flex flex-wrap items-center gap-2">
              <SeverityTag severity={data.severity} />
              <span className="font-mono text-2xs text-[var(--color-ink-faint)]">{data.changeType}</span>
              {data.willTriggerAi && (
                <span className="inline-flex items-center gap-1 rounded bg-[var(--color-signal-tint)] px-1.5 py-0.5 text-2xs font-semibold text-[var(--color-signal)]">
                  <SparklesIcon size={11} /> {t('changes.aiUpdateQueued')}
                </span>
              )}
            </div>
            <h1 className="text-lg font-semibold tracking-tight text-balance text-[var(--color-ink)]">
              {data.summary}
            </h1>
            <p className="mt-1 font-mono text-2xs text-[var(--color-ink-faint)]">
              <span className="text-[var(--color-signal-bright)]">{data.serviceName}</span> ·{' '}
              {t('changes.detected', { date: fullDate(data.detectedAt) })}
            </p>
          </div>

          <div className="px-6 py-5">
            <DiffViewer diff={data.diff} />

            {data.affectedDocIds && data.affectedDocIds.length > 0 && (
              <div className="mt-4 flex flex-wrap items-center gap-2">
                <span className="text-2xs uppercase tracking-wider text-[var(--color-ink-faint)]">
                  {t('changes.affectedDocs')}
                </span>
                {data.affectedDocIds.map((id) => (
                  <button
                    key={id}
                    onClick={() => navigate(`/docs/${id}`)}
                    className="inline-flex items-center gap-1.5 rounded-md border border-[var(--color-line-soft)] bg-[var(--color-canvas-sunken)] px-2 py-1 text-xs text-[var(--color-ink-muted)] transition-colors hover:border-[var(--color-signal-soft)] hover:text-[var(--color-ink)]"
                  >
                    <FileTextIcon size={13} />
                    {id}
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="flex items-center justify-end gap-2 border-t border-[var(--color-line-soft)] px-6 py-4">
            <Button
              variant="ghost"
              size="md"
              disabled={pending}
              onClick={() => resolve.mutate('dismiss')}
            >
              <XIcon size={15} /> {t('common.dismiss')}
            </Button>
            <Button
              variant="primary"
              size="md"
              disabled={pending}
              onClick={() => resolve.mutate('ack')}
            >
              <CheckIcon size={15} /> {t('common.acknowledge')}
            </Button>
          </div>
              </Panel>
            </motion.div>
          )}
        </AnimatePresence>
      )}
    </div>
  );
}
