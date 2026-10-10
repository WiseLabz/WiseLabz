import { useId, useState } from 'react';
import type { ReactNode } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import {
  cancelRunbookRun,
  confirmRunbookRunStep,
  getGetRunbookRunQueryKey,
  getListRunbookRunsQueryKey,
  approveRunbookRun,
  resumeRunbookRun,
  rejectRunbookRun,
  useGetRunbookRun,
} from '../../api/generated/runbooks/runbooks';
import { ResumeRunbookRunBodyDecision } from '../../api/model';
import type { RunbookRunStep } from '../../api/model';
import { ElevationConfirm } from '../manager/ElevationConfirm';
import { Button } from '../ui/Button';
import { ConfirmDialog } from '../ui/ConfirmDialog';
import { Dialog } from '../ui/Dialog';
import { Skeleton } from '../ui/states';
import { toast } from '../../lib/toast';
import { ToneTag } from '../ui/ToneTag';
import type { Tone } from '../ui/status';
import { ConnectorActionRequest } from './ConnectorActionRequest';
import { isConflict, runErrorMessage } from './runErrors';
import { formatRunbookValue } from './runbookStepValues';
import { useAuth } from '../../store/auth';

type RunT = ReturnType<typeof useTranslation>['t'];

export function RunDetail({
  runId,
  onClose,
  showTitle = true,
}: {
  runId: string;
  onClose?: () => void;
  /** Hide the heading when the surrounding page or dialog already provides it. */
  showTitle?: boolean;
}) {
  const { t, i18n } = useTranslation();
  const headingId = useId();
  const queryClient = useQueryClient();
  const { data: run, isLoading, isError, refetch } = useGetRunbookRun(runId);
  const currentUserId = useAuth((state) => state.user?.id);
  const [confirmCancelOpen, setConfirmCancelOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [unknownWarningOpen, setUnknownWarningOpen] = useState(false);
  const [decisionOpen, setDecisionOpen] = useState(false);
  const [decision, setDecision] = useState<ResumeRunbookRunBodyDecision | null>(null);
  // The step and run revision the operator saw when the decision dialog opened.
  const [decisionBasis, setDecisionBasis] = useState<{ stepId: string; updatedAt: string } | null>(
    null
  );
  const [elevationOpen, setElevationOpen] = useState(false);
  const [approvalElevationOpen, setApprovalElevationOpen] = useState(false);
  const [runbookDeleted, setRunbookDeleted] = useState(false);
  const [resumeError, setResumeError] = useState<string | null>(null);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetRunbookRunQueryKey(runId) });
    if (run?.runbookId) {
      void queryClient.invalidateQueries({ queryKey: getListRunbookRunsQueryKey(run.runbookId) });
    }
  };

  const confirm = useMutation({
    mutationFn: (stepId: string) => confirmRunbookRunStep(runId, stepId),
    onSuccess: () => {
      refresh();
      toast.success(t('runbooks.runs.confirmSuccess'));
    },
    onError: (error) => {
      toast.error(runErrorMessage(error, t, 'runbooks.runs.confirmError'));
      if (isConflict(error)) refresh();
    },
  });

  const resume = useMutation({
    mutationFn: ({
      token,
      decision,
      basis,
    }: {
      token: string | null;
      decision: ResumeRunbookRunBodyDecision | null;
      basis: { stepId: string; updatedAt: string } | null;
    }) =>
      resumeRunbookRun(
        runId,
        decision ? { decision, ...basis } : undefined,
        token ? { headers: { 'X-Elevation-Token': token } } : undefined
      ),
    onSuccess: () => {
      refresh();
      setElevationOpen(false);
      setResumeError(null);
      toast.success(t('runbooks.runs.resumeSuccess'));
    },
    onError: (error) => {
      const response = isAxiosError(error)
        ? (error.response?.data as { code?: string; message?: string } | undefined)
        : undefined;
      setElevationOpen(false);
      if (response?.code === 'runbook_deleted') {
        setRunbookDeleted(true);
        refresh();
        setResumeError(t('runbooks.runs.resumeDeleted'));
        return;
      }
      if (response?.code === 'run_changed') {
        setDecisionOpen(false);
        setDecision(null);
        setDecisionBasis(null);
        refresh();
        setResumeError(t('runbooks.runs.resumeRunChanged'));
        return;
      }
      if (response?.code === 'unknown_step_decision_required') {
        setResumeError(t('runbooks.runs.resumeDecisionRequired'));
        return;
      }
      if (isConflict(error)) refresh();
      setResumeError(runErrorMessage(error, t, 'runbooks.runs.resumeError'));
    },
  });

  const cancel = useMutation({
    mutationFn: () => cancelRunbookRun(runId),
    onSuccess: () => {
      refresh();
      setConfirmCancelOpen(false);
      toast.success(
        run?.state === 'awaiting_approval'
          ? t('runbooks.runs.cancelRequestSuccess')
          : t('runbooks.runs.cancelSuccess')
      );
    },
    onError: (error) => {
      toast.error(runErrorMessage(error, t, 'runbooks.runs.cancelError'));
      if (isConflict(error)) refresh();
    },
  });

  const approve = useMutation({
    mutationFn: (token: string | null) =>
      approveRunbookRun(
        runId,
        token ? { headers: { 'X-Elevation-Token': token } } : undefined
      ),
    onSuccess: () => {
      refresh();
      setApprovalElevationOpen(false);
      toast.success(t('runbooks.runs.approveSuccess'));
    },
    onError: (error) => {
      toast.error(runErrorMessage(error, t, 'runbooks.runs.approveError'));
      if (isConflict(error)) refresh();
    },
  });

  const reject = useMutation({
    mutationFn: () => rejectRunbookRun(runId),
    onSuccess: () => {
      refresh();
      setRejectOpen(false);
      toast.success(t('runbooks.runs.rejectSuccess'));
    },
    onError: (error) => {
      toast.error(runErrorMessage(error, t, 'runbooks.runs.rejectError'));
      if (isConflict(error)) refresh();
    },
  });

  const waitingStep = run?.steps.find((step) => step.state === 'waiting');
  const allStepsExecutable = !!run && run.steps.every((step) => step.canExecute);
  const confirmShown = run?.state === 'waiting_manual' && waitingStep?.kind === 'manual';
  const resumeShown = run?.state === 'failed' && !!run.runbookId && !runbookDeleted;
  const canConfirm = confirmShown && allStepsExecutable;
  const canResume = resumeShown && allStepsExecutable;
  const awaitingApproval = run?.state === 'awaiting_approval';
  const isRequester = !!run && run.startedBy === currentUserId;
  // Cancel does not depend on the steps being executable: a step whose connector
  // was deleted is redacted for everyone, yet the run must stay cancellable. The
  // server enforces grants and a refusal is surfaced by cancel.onError.
  const canCancel =
    !!run &&
    (awaitingApproval ? isRequester : ['running', 'waiting_manual', 'failed'].includes(run.state));
  const showCancel =
    !!run &&
    (['running', 'waiting_manual', 'failed'].includes(run.state) ||
      (awaitingApproval && isRequester));
  const firstUnfinishedStep = run?.steps
    .slice()
    .sort((a, b) => a.position - b.position)
    .find((step) => step.state !== 'succeeded');

  // An unknown connector_action step needs an explicit resend or mark-done choice
  // before the resume request is sent; every other resume keeps its plain flow.
  const decisionRequired =
    firstUnfinishedStep?.state === 'unknown' && firstUnfinishedStep.kind === 'connector_action';

  const openResume = () => {
    setResumeError(null);
    setDecision(null);
    setDecisionBasis(
      decisionRequired && firstUnfinishedStep && run
        ? { stepId: firstUnfinishedStep.id, updatedAt: run.updatedAt }
        : null
    );
    if (decisionRequired) setDecisionOpen(true);
    else if (firstUnfinishedStep?.state === 'unknown') setUnknownWarningOpen(true);
    else setElevationOpen(true);
  };

  return (
    <>
      <section
        aria-labelledby={showTitle ? headingId : undefined}
        aria-label={showTitle ? undefined : t('runbooks.runs.detailTitle')}
        className="space-y-5"
      >
        <header className="flex min-w-0 items-start justify-between gap-3">
          <div className="min-w-0">
            {showTitle && (
              <h2 id={headingId} className="font-mono text-lg font-medium text-ink">
                {t('runbooks.runs.detailTitle')}
              </h2>
            )}
            {run && <p className="mt-1 truncate text-sm text-ink-muted">{run.runbookTitle}</p>}
            {run && (
              <div aria-live="polite" aria-atomic="true" className="mt-2">
                <RunStateTag state={run.state} label={t(`runbooks.runs.status.${run.state}`)} />
              </div>
            )}
          </div>
          {onClose && (
            <Button size="sm" onClick={onClose}>
              {t('runbooks.runs.close')}
            </Button>
          )}
        </header>

        {isLoading && (
          <div role="status" aria-label={t('runbooks.runs.loadingDetail')} className="space-y-3">
            <Skeleton className="h-3 w-2/3" />
            <Skeleton className="h-3 w-1/2" />
            <Skeleton className="h-20 w-full" />
          </div>
        )}

        {isError && (
          <div role="alert" className="space-y-3 py-3">
            <p className="text-sm text-err">{t('runbooks.runs.detailLoadError')}</p>
            <Button size="sm" onClick={() => void refetch()}>
              {t('runbooks.runs.retry')}
            </Button>
          </div>
        )}

        {run && (
          <>
            <dl className="grid gap-x-6 gap-y-3 rounded-md border border-line-soft bg-surface-raised/40 p-3 sm:grid-cols-2">
              <MetadataRow label={t('runbooks.runs.runId')} value={<code>{run.id}</code>} />
              {run.runbookId && (
                <MetadataRow
                  label={t('runbooks.runs.runbookId')}
                  value={<code>{run.runbookId}</code>}
                />
              )}
              {run.reason && (
                <MetadataRow
                  label={t('runbooks.runs.reason')}
                  value={
                    run.reason === 'approval_expired'
                      ? t('runbooks.runs.approvalExpired')
                      : run.reason
                  }
                />
              )}
              <MetadataRow
                label={t(awaitingApproval ? 'runbooks.runs.requestedAt' : 'runbooks.runs.startedAt')}
                value={<Timestamp value={run.startedAt} language={i18n.language} />}
              />
              {awaitingApproval && run.approvalExpiresAt && (
                <MetadataRow
                  label={t('runbooks.runs.approvalExpiresAt')}
                  value={<Timestamp value={run.approvalExpiresAt} language={i18n.language} />}
                />
              )}
              <MetadataRow
                label={t('runbooks.runs.updatedAt')}
                value={<Timestamp value={run.updatedAt} language={i18n.language} />}
              />
              {run.finishedAt && (
                <MetadataRow
                  label={t('runbooks.runs.finishedAt')}
                  value={<Timestamp value={run.finishedAt} language={i18n.language} />}
                />
              )}
              <MetadataRow
                label={t(awaitingApproval ? 'runbooks.runs.requestedBy' : 'runbooks.runs.startedBy')}
                value={<code>{run.startedBy}</code>}
              />
              {run.approvedBy && (
                <MetadataRow
                  label={t('runbooks.runs.approvedBy')}
                  value={<code>{run.approvedBy}</code>}
                />
              )}
              {run.approvedAt && (
                <MetadataRow
                  label={t('runbooks.runs.approvedAt')}
                  value={<Timestamp value={run.approvedAt} language={i18n.language} />}
                />
              )}
              {run.rejectedBy && (
                <MetadataRow
                  label={t('runbooks.runs.rejectedBy')}
                  value={<code>{run.rejectedBy}</code>}
                />
              )}
              {run.resumedBy && (
                <MetadataRow
                  label={t('runbooks.runs.resumedBy')}
                  value={<code>{run.resumedBy}</code>}
                />
              )}
              {run.cancelledBy && (
                <MetadataRow
                  label={t('runbooks.runs.cancelledBy')}
                  value={<code>{run.cancelledBy}</code>}
                />
              )}
            </dl>

            {(resumeError || ((!run.runbookId || runbookDeleted) && run.state === 'failed')) && (
              <p
                role={resumeError ? 'alert' : 'note'}
                className="rounded-md border border-line-soft px-3 py-2 text-xs text-ink-muted"
              >
                {resumeError ?? t('runbooks.runs.resumeDeleted')}
              </p>
            )}

            <section aria-label={t('runbooks.runs.steps')}>
              <h3 className="mb-2 font-mono text-xs text-ink-muted">{t('runbooks.runs.steps')}</h3>
              <ol className="space-y-2">
                {run.steps
                  .slice()
                  .sort((a, b) => a.position - b.position)
                  .map((step) => (
                    <RunStepRow
                      key={step.id}
                      step={step}
                      language={i18n.language}
                      t={t}
                    />
                  ))}
              </ol>
            </section>

            {(confirmShown || resumeShown) && !allStepsExecutable && (
              <p role="note" className="text-xs text-ink-muted">
                {t('runbooks.runs.blockedAction')}
              </p>
            )}

            <div className="flex flex-wrap justify-end gap-2 border-t border-line-soft pt-3">
              {awaitingApproval && run.canApprove && (
                <>
                  <Button
                    variant="danger"
                    size="sm"
                    disabled={approve.isPending || reject.isPending}
                    onClick={() => setRejectOpen(true)}
                  >
                    {t('runbooks.runs.reject')}
                  </Button>
                  <Button
                    variant="primary"
                    size="sm"
                    disabled={approve.isPending || reject.isPending}
                    onClick={() => setApprovalElevationOpen(true)}
                  >
                    {t('runbooks.runs.approve')}
                  </Button>
                </>
              )}
              {confirmShown && waitingStep && (
                <Button
                  variant="primary"
                  size="sm"
                  disabled={!canConfirm || confirm.isPending}
                  onClick={() => confirm.mutate(waitingStep.id)}
                >
                  {t('runbooks.runs.confirm')}
                </Button>
              )}
              {resumeShown && (
                <Button size="sm" disabled={!canResume || resume.isPending} onClick={openResume}>
                  {t('runbooks.runs.resume')}
                </Button>
              )}
              {showCancel && (
                <Button
                  variant="danger"
                  size="sm"
                  disabled={!canCancel || cancel.isPending}
                  onClick={() => setConfirmCancelOpen(true)}
                >
                  {t(awaitingApproval ? 'runbooks.runs.cancelRequest' : 'runbooks.runs.cancelAction')}
                </Button>
              )}
            </div>
          </>
        )}
      </section>

      {unknownWarningOpen && (
        <ConfirmDialog
          open
          onClose={() => setUnknownWarningOpen(false)}
          onConfirm={() => {
            setUnknownWarningOpen(false);
            setElevationOpen(true);
          }}
          title={t('runbooks.runs.resumeTitle')}
          description={t('runbooks.runs.resumeWarningUnknown')}
          confirmLabel={t('runbooks.runs.resumeContinue')}
          cancelLabel={t('runbooks.runs.close')}
        />
      )}

      {decisionOpen && (
        <ResumeDecisionDialog
          decision={decision}
          onDecisionChange={setDecision}
          onClose={() => setDecisionOpen(false)}
          onContinue={() => {
            setDecisionOpen(false);
            setElevationOpen(true);
          }}
        />
      )}

      {elevationOpen && run?.runbookId && !runbookDeleted && (
        <ElevationConfirm
          open={elevationOpen}
          resourceName={run.runbookTitle}
          action="runbook.run"
          target={run.runbookId}
          title={t('runbooks.runs.resumeTitle')}
          description={t('runbooks.runs.resumeDescription')}
          confirmLabel={t('runbooks.runs.resume')}
          onClose={() => setElevationOpen(false)}
          onConfirm={(token) => resume.mutate({ token, decision, basis: decisionBasis })}
          isPending={resume.isPending}
        />
      )}

      {approvalElevationOpen && run?.state === 'awaiting_approval' && (
        <ElevationConfirm
          open
          alwaysElevate
          resourceName={run.runbookTitle}
          action="runbook.approve"
          target={run.id}
          title={t('runbooks.runs.approveTitle')}
          description={t('runbooks.runs.approveDescription')}
          confirmLabel={t('runbooks.runs.approve')}
          onClose={() => setApprovalElevationOpen(false)}
          onConfirm={(token) => approve.mutate(token)}
          isPending={approve.isPending}
        />
      )}

      {rejectOpen && (
        <ConfirmDialog
          open
          onClose={() => setRejectOpen(false)}
          onConfirm={() => reject.mutate()}
          title={t('runbooks.runs.rejectTitle')}
          description={t('runbooks.runs.rejectDescription')}
          confirmLabel={t('runbooks.runs.rejectConfirm')}
          cancelLabel={t('runbooks.runs.close')}
          tone="danger"
          confirmDisabled={reject.isPending}
        />
      )}

      {confirmCancelOpen && (
        <ConfirmDialog
          open
          onClose={() => setConfirmCancelOpen(false)}
          onConfirm={() => cancel.mutate()}
          title={t(awaitingApproval ? 'runbooks.runs.cancelRequestTitle' : 'runbooks.runs.cancelTitle')}
          description={t(
            awaitingApproval
              ? 'runbooks.runs.cancelRequestDescription'
              : 'runbooks.runs.cancelDescription'
          )}
          confirmLabel={t(
            awaitingApproval ? 'runbooks.runs.cancelRequestConfirm' : 'runbooks.runs.cancelConfirm'
          )}
          cancelLabel={t('runbooks.runs.keepRun')}
          tone="danger"
          confirmDisabled={cancel.isPending}
        />
      )}
    </>
  );
}

const STATE_TONES: Record<string, Tone> = {
  succeeded: 'ok',
  failed: 'err',
  waiting_manual: 'warn',
  waiting: 'warn',
  running: 'idle',
  pending: 'idle',
  skipped: 'idle',
  unknown: 'idle',
  cancelled: 'idle',
  expired: 'idle',
  awaiting_approval: 'warn',
  rejected: 'err',
};

export function RunStateTag({ state, label }: { state: string; label: string }) {
  return <ToneTag tone={STATE_TONES[state] ?? 'idle'} label={label} />;
}

function RunStepRow({
  step,
  language,
  t,
}: {
  step: RunbookRunStep;
  language: string;
  t: RunT;
}) {
  const restricted =
    step.redacted || !step.kind || !step.title || (step.kind !== 'manual' && !step.connectorId);
  const stepNumber = step.position + 1;

  if (restricted) {
    return (
      <li className="rounded-md border border-line-soft bg-surface-raised/40 px-3 py-2.5">
        <div className="flex items-center justify-between gap-3">
          <span className="text-sm text-ink-muted">{t('runbooks.runs.restrictedStep')}</span>
          <span aria-live="polite" aria-atomic="true">
            <RunStateTag
              state={step.state ?? 'pending'}
              label={t(`runbooks.runs.stepState.${step.state ?? 'pending'}`)}
            />
          </span>
        </div>
        <dl className="mt-2 grid gap-2 text-2xs text-ink-faint sm:grid-cols-2">
          {step.startedAt && (
            <MetadataRow
              label={t('runbooks.runs.startedAt')}
              value={<Timestamp value={step.startedAt} language={language} />}
            />
          )}
          {step.finishedAt && (
            <MetadataRow
              label={t('runbooks.runs.finishedAt')}
              value={<Timestamp value={step.finishedAt} language={language} />}
            />
          )}
          {step.confirmedBy && (
            <MetadataRow
              label={t('runbooks.runs.confirmedBy')}
              value={<code>{step.confirmedBy}</code>}
            />
          )}
        </dl>
      </li>
    );
  }

  return (
    <li className="rounded-md border border-line-soft bg-surface-raised/40 px-3 py-2.5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h4 className="text-sm font-medium text-ink">
            <span className="mr-2 font-mono text-2xs text-ink-faint">
              {t('runbooks.runs.step', { position: stepNumber })}
            </span>
            {step.title}
          </h4>
          <p className="mt-1 text-2xs text-ink-muted">{t(`runbooks.runs.kind.${step.kind}`)}</p>
        </div>
        <span aria-live="polite" aria-atomic="true">
          <RunStateTag
            state={step.state ?? 'pending'}
            label={t(`runbooks.runs.stepState.${step.state ?? 'pending'}`)}
          />
        </span>
      </div>

      <dl className="mt-3 grid gap-x-6 gap-y-2 text-xs sm:grid-cols-2">
        {step.connectorId && (
          <MetadataRow
            label={t('runbooks.runs.connector')}
            value={
              <span className="flex flex-wrap items-baseline gap-x-2">
                <span>{step.connectorName ?? step.connectorId}</span>
                {step.connectorName && (
                  <code className="text-2xs text-ink-faint">{step.connectorId}</code>
                )}
              </span>
            }
          />
        )}
        {step.verb && (
          <MetadataRow
            label={t('runbooks.runs.verbLabel')}
            value={t(`runbooks.runs.verb.${step.verb}`)}
          />
        )}
        {step.entityRef && (
          <MetadataRow label={t('runbooks.runs.entity')} value={<code>{step.entityRef}</code>} />
        )}
        {step.kind === 'connector_action' && step.action && (
          <MetadataRow label={t('runbooks.runs.actionName')} value={<code>{step.action}</code>} />
        )}
        {step.kind === 'config_push' && step.fieldKey && (
          <MetadataRow label={t('runbooks.runs.fieldLabel')} value={<code>{step.fieldKey}</code>} />
        )}
        {step.kind === 'config_push' && (
          <MetadataRow
            label={t('runbooks.runs.configPushValueLabel')}
            value={
              step.currentValueKnown === undefined
                ? formatRunbookValue(step.targetValue, true)
                : step.currentValueKnown
                  ? t('runbooks.runs.currentToTarget', {
                      current: formatRunbookValue(step.currentValue),
                      target: formatRunbookValue(step.targetValue, true),
                    })
                  : t('runbooks.runs.unknownCurrentToTarget', {
                      target: formatRunbookValue(step.targetValue, true),
                    })
            }
          />
        )}
        {step.kind === 'wait_for_entity' && step.attribute && (
          <MetadataRow
            label={t('runbooks.runs.waitConditionLabel')}
            value={
              <code>
                {t('runbooks.runs.waitCondition', {
                  attribute: step.attribute,
                  operator: step.operator ?? '',
                  expected: formatRunbookValue(step.expectedValue, true),
                })}
              </code>
            }
          />
        )}
        {step.timeoutSeconds !== undefined && step.timeoutSeconds > 0 && (
          <MetadataRow
            label={t('runbooks.runs.timeout')}
            value={t('runbooks.runs.timeoutValue', { count: step.timeoutSeconds })}
          />
        )}
        {step.startedAt && (
          <MetadataRow
            label={t('runbooks.runs.startedAt')}
            value={<Timestamp value={step.startedAt} language={language} />}
          />
        )}
        {step.finishedAt && (
          <MetadataRow
            label={t('runbooks.runs.finishedAt')}
            value={<Timestamp value={step.finishedAt} language={language} />}
          />
        )}
        {step.confirmedBy && (
          <MetadataRow
            label={t('runbooks.runs.confirmedBy')}
            value={<code>{step.confirmedBy}</code>}
          />
        )}
      </dl>
      {step.kind === 'connector_action' && step.preview && (
        <div className="mt-3">
          <ConnectorActionRequest preview={step.preview} />
        </div>
      )}
      {step.kind === 'config_push' && step.executeBlockedReason === 'unsupported_field' && (
        <p role="note" className="mt-3 text-xs text-warn">
          {t('runbooks.runs.blocked.unsupported_field')}
        </p>
      )}
      {step.error && (
        <p className="mt-3 text-xs text-err">
          <span className="font-medium">
            {t('runbooks.runs.stepError')}
            :{' '}
          </span>
          {step.error}
        </p>
      )}
    </li>
  );
}

type DecisionCopy = { label: string; hint: string };

const RESUME_DECISION_COPY: Record<ResumeRunbookRunBodyDecision, DecisionCopy> = {
  [ResumeRunbookRunBodyDecision.resend]: {
    label: 'runbooks.runs.resumeDecision.resend',
    hint: 'runbooks.runs.resumeDecision.resendHint',
  },
  [ResumeRunbookRunBodyDecision.mark_done]: {
    label: 'runbooks.runs.resumeDecision.markDone',
    hint: 'runbooks.runs.resumeDecision.markDoneHint',
  },
};

/** Asks which way to resume past an unknown connector_action step. Nothing
 * is sent until one option is chosen; the chosen value is the resume body's
 * `decision`. */
function ResumeDecisionDialog({
  decision,
  onDecisionChange,
  onClose,
  onContinue,
}: {
  decision: ResumeRunbookRunBodyDecision | null;
  onDecisionChange: (decision: ResumeRunbookRunBodyDecision) => void;
  onClose: () => void;
  onContinue: () => void;
}) {
  const { t } = useTranslation();
  const groupName = useId();
  return (
    <Dialog open onClose={onClose} title={t('runbooks.runs.resumeTitle')} size="sm">
      <div className="space-y-4">
        <p role="note" className="text-xs text-warn">
          {t('runbooks.runs.resumeDecisionWarning')}
        </p>
        <fieldset className="space-y-2">
          <legend className="mb-2 text-xs text-ink-muted">
            {t('runbooks.runs.resumeDecisionLegend')}
          </legend>
          {Object.values(ResumeRunbookRunBodyDecision).map((option) => (
            <label
              key={option}
              className="flex cursor-pointer items-start gap-3 rounded-md border border-line-soft p-3"
            >
              <input
                type="radio"
                name={groupName}
                value={option}
                checked={decision === option}
                onChange={() => onDecisionChange(option)}
                className="mt-0.5 h-4 w-4 border-line-strong text-accent-primary focus-visible:ring-accent-primary"
              />
              <span className="min-w-0">
                <span className="block text-sm text-ink">
                  {t(RESUME_DECISION_COPY[option].label)}
                </span>
                <span className="block text-2xs text-ink-muted">
                  {t(RESUME_DECISION_COPY[option].hint)}
                </span>
              </span>
            </label>
          ))}
        </fieldset>
        <div className="flex justify-end gap-2">
          <Button size="sm" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button variant="primary" size="sm" disabled={!decision} onClick={onContinue}>
            {t('runbooks.runs.resumeContinue')}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

function MetadataRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-2xs text-ink-faint">{label}</dt>
      <dd className="break-words text-ink-muted">{value}</dd>
    </div>
  );
}

function Timestamp({ value, language }: { value: string; language: string }) {
  const date = new Date(value);
  const formatted = Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat(language, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
  return <time dateTime={value}>{formatted}</time>;
}
