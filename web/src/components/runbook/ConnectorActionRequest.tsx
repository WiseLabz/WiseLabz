/**
 * Read-only view of a connector_action step's preview: the action's label,
 * description, downtime and the HTTP request it will send. Shared by the run
 * preview (StartRunDialog) and the run detail (RunDetail).
 */
import { useTranslation } from 'react-i18next';
import type { RestartPreview } from '../../api/model';
import { ActionRequestBlock } from '../manager/LifecycleOp';
import type { ActionRequestMessages } from '../manager/LifecycleOp';

export function ConnectorActionRequest({ preview }: { preview: RestartPreview }) {
  const { t } = useTranslation();
  const messages: ActionRequestMessages = {
    request: t('runbooks.runs.actionRequest'),
    method: t('runbooks.runs.actionMethod'),
    url: t('runbooks.runs.actionUrl'),
    headers: t('runbooks.runs.actionHeaders'),
    body: t('runbooks.runs.actionBody'),
  };
  const downtime = preview.estimatedDowntimeSeconds;

  return (
    <div className="space-y-2">
      {(preview.label || preview.userDefined) && (
        <div className="flex flex-wrap items-center gap-2">
          {preview.label && <p className="text-xs font-medium text-ink">{preview.label}</p>}
          {preview.userDefined && (
            <span className="rounded-sm border border-line-soft px-1.5 py-0.5 text-2xs text-ink-muted">
              {t('runbooks.runs.actionUserDefined')}
            </span>
          )}
        </div>
      )}
      {preview.description && <p className="text-xs text-ink-muted">{preview.description}</p>}
      <dl>
        <div>
          <dt className="text-2xs text-ink-faint">{t('runbooks.runs.downtime')}</dt>
          <dd className="mt-0.5 text-xs text-ink">
            {downtime > 0
              ? t('runbooks.runs.downtimeSeconds', { count: downtime })
              : t('runbooks.runs.actionNoDowntime')}
          </dd>
        </div>
      </dl>
      {preview.request && <ActionRequestBlock request={preview.request} messages={messages} />}
    </div>
  );
}
