/**
 * Small panel showing the runbook attached to a change type or alert severity,
 * if one exists. The backend enforces a unique target per runbook, so at most
 * one can match — silent (renders nothing) while loading or when none exists,
 * since most change types/severities won't have one.
 */
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { useGetRunbooks, useGetRunbooksRunbookId } from '../../api/generated/runbooks/runbooks';
import type { Severity } from '../../api/model/severity';
import { FileTextIcon } from '../icons';

type RunbookPanelProps =
  | { changeType: string; alertSeverity?: never; runbookId?: never }
  | { alertSeverity: Severity; changeType?: never; runbookId?: never }
  | { runbookId: string; changeType?: never; alertSeverity?: never };

export function RunbookPanel(props: RunbookPanelProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const byTarget = useGetRunbooks(
    'changeType' in props && props.changeType
      ? { changeType: props.changeType }
      : 'alertSeverity' in props && props.alertSeverity
        ? { alertSeverity: props.alertSeverity }
        : undefined,
    { query: { enabled: !('runbookId' in props && props.runbookId) } }
  );
  const byId = useGetRunbooksRunbookId('runbookId' in props ? (props.runbookId ?? '') : '', {
    query: { enabled: Boolean('runbookId' in props && props.runbookId) },
  });

  const isLoading = 'runbookId' in props && props.runbookId ? byId.isLoading : byTarget.isLoading;
  const runbook = 'runbookId' in props && props.runbookId ? byId.data : byTarget.data?.items[0];

  if (isLoading || !runbook) return null;

  return (
    <section className="mt-4 rounded-lg border border-line-soft bg-canvas-sunken p-3" aria-labelledby="runbook-heading">
      <h2 id="runbook-heading" className="text-xs font-medium text-ink">
        {t('runbooks.heading')}
      </h2>
      <p className="mt-1.5 text-sm font-medium text-ink">{runbook.title}</p>
      <p className="mt-1 whitespace-pre-wrap text-xs leading-relaxed text-ink-muted">{runbook.body}</p>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        {runbook.docId && (
          <button
            onClick={() => navigate(`/docs/${runbook.docId}`)}
            className="inline-flex items-center gap-1.5 rounded-md border border-line-soft bg-canvas-sunken px-2 py-1 text-xs text-ink-muted transition-colors hover:border-accent-secondary-soft hover:text-accent-secondary-bright"
          >
            <FileTextIcon size={13} />
            {runbook.docId}
          </button>
        )}
        {runbook.snapshotId && (
          <span className="text-2xs text-ink-faint">
            {t('runbooks.snapshotRef', { snapshotId: runbook.snapshotId })}
          </span>
        )}
      </div>
    </section>
  );
}
