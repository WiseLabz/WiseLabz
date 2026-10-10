import { useCallback, useEffect, useRef, useState } from 'react';
import { isAxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import { startRunbookRun, useStartRunbookRun } from '../../api/generated/runbooks/runbooks';
import type {
  Runbook,
  RunbookRunConflict,
  RunbookRunStep,
  RunbookRunStepKind,
} from '../../api/model';
import { Button } from '../ui/Button';
import { Dialog } from '../ui/Dialog';
import { ElevationConfirm } from '../manager/ElevationConfirm';
import { runErrorMessage } from './runErrors';
import { ConnectorActionRequest } from './ConnectorActionRequest';
import { formatRunbookValue } from './runbookStepValues';

export function StartRunDialog({
  runbook,
  open,
  onClose,
  onStarted,
}: {
  runbook: Runbook;
  open: boolean;
  onClose: () => void;
  onStarted: (runId: string) => void;
}) {
  const { t } = useTranslation();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [startError, setStartError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<ActiveRunConflict | null>(null);
  const elevationToken = useRef<string | null>(null);

  const previewRequest = useStartRunbookRun();
  const { mutate: mutatePreview, reset: resetPreview } = previewRequest;
  const startRequest = useStartRunbookRun({
    mutation: {
      mutationFn: ({ runbookId, params }) => {
        const token = elevationToken.current;
        return startRunbookRun(
          runbookId,
          params,
          params?.dryRun || !token ? undefined : { headers: { 'X-Elevation-Token': token } }
        );
      },
    },
  });
  const preview =
    previewRequest.data && 'canStart' in previewRequest.data ? previewRequest.data : null;
  const previewError = previewRequest.isError;

  const loadPreview = useCallback(() => {
    mutatePreview({ runbookId: runbook.id, params: { dryRun: true } });
  }, [runbook.id, mutatePreview]);

  useEffect(() => {
    if (open) loadPreview();
    else resetPreview();
  }, [open, loadPreview, resetPreview]);

  const close = () => {
    setConfirmOpen(false);
    setStartError(null);
    setConflict(null);
    elevationToken.current = null;
    onClose();
  };

  const stepsCanStart = Boolean(
    preview?.canStart &&
    preview.steps.length > 0 &&
    preview.steps.every((step) => !step.redacted && step.canExecute)
  );
  const canStart = Boolean(
    stepsCanStart && !(preview?.requiresApproval && !preview.approverAvailable)
  );
  const approvalRequired = Boolean(preview?.requiresApproval);

  const start = async (token: string | null) => {
    elevationToken.current = token;
    setStartError(null);
    setConflict(null);
    try {
      const result = await startRequest.mutateAsync({
        runbookId: runbook.id,
        params: { dryRun: false },
      });
      if ('canStart' in result) {
        setConfirmOpen(false);
        setStartError(t('runbooks.runs.startError'));
        return;
      }
      setConfirmOpen(false);
      onStarted(result.id);
      onClose();
    } catch (error) {
      const activeRun = getRunbookRunConflict(error);
      setConfirmOpen(false);
      if (activeRun) setConflict(activeRun);
      else setStartError(runErrorMessage(error, t, 'runbooks.runs.startError'));
    } finally {
      elevationToken.current = null;
    }
  };

  return (
    <>
      {!confirmOpen && (
        <Dialog
          open={open}
          onClose={close}
          title={t('runbooks.runs.previewTitle', { title: runbook.title })}
          size="lg"
        >
          <div className="space-y-4">
            <p className="text-sm text-ink-muted">
              {t(
                approvalRequired
                  ? 'runbooks.runs.approvalPreviewNotice'
                  : 'runbooks.runs.previewNotice'
              )}
            </p>

            {previewError ? (
              <div className="space-y-3" role="alert">
                <p className="text-sm text-err">{t('runbooks.runs.previewError')}</p>
                <Button size="sm" onClick={loadPreview} disabled={previewRequest.isPending}>
                  {t('runbooks.runs.retryPreview')}
                </Button>
              </div>
            ) : !preview ? (
              <p role="status" className="text-sm text-ink-muted">
                {t('runbooks.runs.loading')}
              </p>
            ) : (
              <>
                {!stepsCanStart && (
                  <p className="rounded-md border border-warn/40 bg-warn-tint px-3 py-2 text-sm text-ink">
                    {preview.steps.length === 0
                      ? t('runbooks.runs.emptyRunbook')
                      : t('runbooks.runs.blockedSummary')}
                  </p>
                )}
                {approvalRequired && !preview.approverAvailable && (
                  <p role="note" className="rounded-md border border-warn/40 px-3 py-2 text-sm text-ink">
                    {t('runbooks.runs.noApproverAvailable')}
                  </p>
                )}

                <ol className="max-h-[55vh] space-y-3 overflow-y-auto pr-1">
                  {[...preview.steps]
                    .sort((a, b) => a.position - b.position)
                    .map((step) => (
                      <PreviewStep key={step.id} step={step} />
                    ))}
                </ol>

                {startError && (
                  <p role="alert" className="text-sm text-err">
                    {startError}
                  </p>
                )}
                {conflict && (
                  <div role="alert" className="space-y-2 rounded-md border border-warn/40 p-3">
                    <p className="text-sm text-ink">
                      {conflict.message || t('runbooks.runs.conflictFallback')}
                    </p>
                    <a
                      href={`/runbook-runs/${encodeURIComponent(conflict.runId)}`}
                      onClick={(event) => {
                        if (
                          event.button !== 0 ||
                          event.metaKey ||
                          event.ctrlKey ||
                          event.shiftKey ||
                          event.altKey
                        )
                          return;
                        event.preventDefault();
                        onStarted(conflict.runId);
                        close();
                      }}
                      className="inline-flex text-sm text-signal-bright underline underline-offset-2"
                    >
                      {t('runbooks.runs.openExistingRun')}
                    </a>
                  </div>
                )}
              </>
            )}

            <div className="flex justify-end gap-2 border-t border-line-soft pt-3">
              <Button variant="ghost" onClick={close}>
                {t('common.cancel')}
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  setStartError(null);
                  setConflict(null);
                  setConfirmOpen(true);
                }}
                disabled={!canStart || previewRequest.isPending || startRequest.isPending}
              >
                {t(approvalRequired ? 'runbooks.runs.requestApproval' : 'runbooks.runs.start')}
              </Button>
            </div>
          </div>
        </Dialog>
      )}

      {confirmOpen && (
        <ElevationConfirm
          open
          resourceName={runbook.title}
          action="runbook.run"
          target={runbook.id}
          title={t('runbooks.runs.startConfirmTitle', { title: runbook.title })}
          description={t(
            approvalRequired
              ? 'runbooks.runs.approvalPreviewNotice'
              : 'runbooks.runs.startConfirmDescription'
          )}
          confirmLabel={t(
            approvalRequired ? 'runbooks.runs.requestApproval' : 'runbooks.runs.confirmStart'
          )}
          isPending={startRequest.isPending}
          onClose={() => setConfirmOpen(false)}
          onConfirm={start}
        />
      )}
    </>
  );
}

function PreviewStep({ step }: { step: RunbookRunStep }) {
  const { t } = useTranslation();
  const restricted = step.redacted || !step.kind || !step.title;
  const kind = step.kind ?? 'lifecycle';
  const target = getStepTarget(step, kind, t);
  const blockedReason = getBlockedReason(step, t);

  return (
    <li className="rounded-md border border-line-soft bg-canvas-sunken p-3">
      <div className="flex items-start gap-3">
        <span className="font-mono text-xs text-ink-faint">{step.position + 1}.</span>
        {restricted ? (
          <div className="space-y-2">
            <p className="text-sm text-ink-muted">{t('runbooks.runs.restrictedStep')}</p>
            <p className="text-xs text-ink-muted">{t('runbooks.runs.blocked.no_viewer_grant')}</p>
          </div>
        ) : (
          <div className="min-w-0 flex-1 space-y-2">
            <div>
              <h3 className="text-sm font-medium text-ink">{step.title}</h3>
              <p className="mt-0.5 text-xs text-ink-faint">{getKindLabel(kind, t)}</p>
            </div>

            <dl className="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
              <div>
                <dt className="text-2xs text-ink-faint">{t('runbooks.runs.target')}</dt>
                <dd className="mt-0.5 text-ink">{target}</dd>
              </div>
              {step.verb && (
                <div>
                  <dt className="text-2xs text-ink-faint">{t('runbooks.runs.verbLabel')}</dt>
                  <dd className="mt-0.5 text-ink">{t(`runbooks.runs.verb.${step.verb}`)}</dd>
                </div>
              )}
              {step.entityRef && (
                <div>
                  <dt className="text-2xs text-ink-faint">{t('runbooks.runs.entity')}</dt>
                  <dd className="mt-0.5 font-mono text-ink">{step.entityRef}</dd>
                </div>
              )}
              {kind === 'connector_action' && step.action && (
                <div>
                  <dt className="text-2xs text-ink-faint">{t('runbooks.runs.actionName')}</dt>
                  <dd className="mt-0.5 font-mono text-ink">{step.action}</dd>
                </div>
              )}
              {(kind === 'sync_and_wait' || kind === 'wait_until_healthy') && (
                <div>
                  <dt className="text-2xs text-ink-faint">{t('runbooks.runs.timeoutLabel')}</dt>
                  <dd className="mt-0.5 font-mono text-ink">
                    {t('runbooks.runs.timeoutSeconds', { count: step.timeoutSeconds ?? 0 })}
                  </dd>
                </div>
              )}
              {kind === 'config_push' && (
                <>
                  {step.fieldKey && (
                    <div>
                      <dt className="text-2xs text-ink-faint">{t('runbooks.runs.fieldLabel')}</dt>
                      <dd className="mt-0.5 font-mono text-ink">{step.fieldKey}</dd>
                    </div>
                  )}
                  <div>
                    <dt className="text-2xs text-ink-faint">
                      {t('runbooks.runs.configPushValueLabel')}
                    </dt>
                    <dd className="mt-0.5 text-ink">
                      {step.currentValueKnown
                        ? t('runbooks.runs.currentToTarget', {
                            current: formatRunbookValue(step.currentValue),
                            target: formatRunbookValue(step.targetValue, true),
                          })
                        : t('runbooks.runs.unknownCurrentToTarget', {
                            target: formatRunbookValue(step.targetValue, true),
                          })}
                    </dd>
                  </div>
                </>
              )}
              {kind === 'wait_for_entity' && (
                <>
                  <div>
                    <dt className="text-2xs text-ink-faint">
                      {t('runbooks.runs.waitConditionLabel')}
                    </dt>
                    <dd className="mt-0.5 font-mono text-ink">
                      {t('runbooks.runs.waitCondition', {
                        attribute: step.attribute ?? '',
                        operator: step.operator ?? '',
                        expected: formatRunbookValue(step.expectedValue, true),
                      })}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-2xs text-ink-faint">{t('runbooks.runs.timeoutLabel')}</dt>
                    <dd className="mt-0.5 font-mono text-ink">
                      {t('runbooks.runs.timeoutSeconds', { count: step.timeoutSeconds ?? 0 })}
                    </dd>
                  </div>
                </>
              )}
            </dl>

            {kind === 'lifecycle' && step.preview && <LifecycleImpact preview={step.preview} />}

            {kind === 'connector_action' && step.preview && (
              <ConnectorActionRequest preview={step.preview} />
            )}

            {blockedReason && (
              <p className="text-xs text-warn" role="note">
                {blockedReason}
              </p>
            )}
          </div>
        )}
      </div>
    </li>
  );
}

function LifecycleImpact({ preview }: { preview: NonNullable<RunbookRunStep['preview']> }) {
  const { t } = useTranslation();
  const affectedEntities = preview.affectedEntities ?? [];
  return (
    <section className="space-y-2 rounded-md border border-line-soft bg-surface p-3">
      <h4 className="text-xs font-medium text-ink">{t('runbooks.runs.lifecycleImpact')}</h4>
      <dl className="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
        <div>
          <dt className="text-2xs text-ink-faint">{t('runbooks.runs.previewTarget')}</dt>
          <dd className="mt-0.5 text-ink">{preview.targetService}</dd>
        </div>
        <div>
          <dt className="text-2xs text-ink-faint">{t('runbooks.runs.downtime')}</dt>
          <dd className="mt-0.5 text-ink">
            {preview.estimatedDowntimeSeconds > 0
              ? t('runbooks.runs.downtimeSeconds', { count: preview.estimatedDowntimeSeconds })
              : t('runbooks.runs.downtimeIndefinite')}
          </dd>
        </div>
      </dl>
      <div>
        <h5 className="text-2xs text-ink-faint">{t('runbooks.runs.affectedEntities')}</h5>
        {affectedEntities.length > 0 ? (
          <ul className="mt-1 space-y-1 text-xs text-ink">
            {affectedEntities.map((entity) => (
              <li key={entity}>{entity}</li>
            ))}
          </ul>
        ) : (
          <p className="mt-1 text-xs text-ink-muted">{t('runbooks.runs.noAffectedEntities')}</p>
        )}
      </div>
      <div>
        <h5 className="text-2xs text-ink-faint">{t('runbooks.runs.dependencies')}</h5>
        {preview.dependentServices.length > 0 ? (
          <ul className="mt-1 divide-y divide-line-soft text-xs">
            {preview.dependentServices.map((service) => (
              <li key={`${service.kind}-${service.name}`} className="py-1 text-ink">
                {service.name}{' '}
                <span className="font-mono text-2xs text-ink-faint">{service.kind}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="mt-1 text-xs text-ink-muted">{t('runbooks.runs.noDependencies')}</p>
        )}
      </div>
    </section>
  );
}

type Translate = ReturnType<typeof useTranslation>['t'];

function getStepTarget(step: RunbookRunStep, kind: RunbookRunStepKind, t: Translate) {
  if (kind === 'manual') return t('runbooks.runs.noTarget');
  if (kind === 'lifecycle')
    return step.connectorName || step.preview?.targetService || t('runbooks.runs.unknownTarget');
  return step.connectorName || t('runbooks.runs.unknownTarget');
}

function getKindLabel(kind: RunbookRunStepKind, t: Translate) {
  switch (kind) {
    case 'lifecycle':
      return t('runbooks.runs.kind.lifecycle');
    case 'sync_and_wait':
      return t('runbooks.runs.kind.sync_and_wait');
    case 'wait_until_healthy':
      return t('runbooks.runs.kind.wait_until_healthy');
    case 'manual':
      return t('runbooks.runs.kind.manual');
    case 'config_push':
      return t('runbooks.runs.kind.config_push');
    case 'wait_for_entity':
      return t('runbooks.runs.kind.wait_for_entity');
    case 'connector_action':
      return t('runbooks.runs.kind.connector_action');
  }
}

function getBlockedReason(step: RunbookRunStep, t: Translate) {
  if (step.redacted || step.canExecute) return null;
  switch (step.executeBlockedReason) {
    case 'no_viewer_grant':
      return t('runbooks.runs.blocked.no_viewer_grant');
    case 'no_operator_grant':
      return t('runbooks.runs.blocked.no_operator_grant', { connector: step.connectorName ?? '' });
    case 'preview_unavailable':
      return t('runbooks.runs.blocked.preview_unavailable');
    case 'unsupported_field':
      return t('runbooks.runs.blocked.unsupported_field');
    default:
      return t('runbooks.runs.blocked.unknown');
  }
}

// An active-run conflict always names the run; no_eligible_approver does not.
type ActiveRunConflict = RunbookRunConflict & { runId: string };

function getRunbookRunConflict(error: unknown): ActiveRunConflict | null {
  if (!isAxiosError(error) || error.response?.status !== 409) return null;
  const body = error.response.data as Partial<RunbookRunConflict> | undefined;
  if (!body || typeof body.runId !== 'string' || !body.runId) return null;
  return {
    runId: body.runId,
    code: typeof body.code === 'string' ? body.code : 'run_conflict',
    message: typeof body.message === 'string' ? body.message : '',
  };
}
