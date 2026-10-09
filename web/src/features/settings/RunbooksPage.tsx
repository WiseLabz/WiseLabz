/**
 * Runbooks list (operator-only). Create, edit, and delete the actionable
 * runbooks that attach to a change type or alert severity — surfaced inline
 * by RunbookPanel elsewhere. The backend enforces one runbook per target, so
 * create/update conflicts are surfaced as a dedicated inline error rather
 * than a generic toast.
 */
import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import {
  useGetRunbooks,
  getGetRunbooksQueryKey,
  postRunbooks,
  putRunbooksRunbookId,
  deleteRunbooksRunbookId,
} from '../../api/generated/runbooks/runbooks';
import {
  useGetConnectors,
  useGetConnectorsSchema,
  useGetConnectorsConnectorIdConfigFields,
  useGetConnectorsConnectorIdSnapshots,
  useGetConnectorsConnectorIdSnapshotsSnapshotId,
} from '../../api/generated/connectors/connectors';
import { useGetComplianceSchema } from '../../api/generated/compliance/compliance';
import type {
  ComplianceSchemaAttributes,
  Runbook,
  RunbookStepInput,
  RunbookStepKind,
  RunbookStepInputOperator,
  RunbookStepVerb,
  RunbookTargetType,
} from '../../api/model';
import { ComplianceAttributeSpecType } from '../../api/model/complianceAttributeSpecType';
import { RunbookStepInputOperator as RunbookStepOperator } from '../../api/model/runbookStepInputOperator';
import { Severity } from '../../api/model/severity';
import { Button, IconButton } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Dialog } from '../../components/ui/Dialog';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { EntityPicker } from '../../components/manager/EntityPicker';
import { toast } from '../../lib/toast';
import { SubHeader, Field, TextInput, Select } from './parts';
import { FileTextIcon, PlusIcon, EditIcon, XIcon, ChevronDownIcon } from '../../components/icons';
import { RunHistory } from '../../components/runbook/RunHistory';
import { RunDetail } from '../../components/runbook/RunDetail';

const TARGET_TYPES: RunbookTargetType[] = ['change_type', 'alert_severity', 'finding_check_type'];
const MAX_STEPS = 20;
const DEFAULT_TIMEOUT_SECONDS = 300;
const MIN_TIMEOUT_SECONDS = 10;
const MAX_TIMEOUT_SECONDS = 1800;
/** Lifecycle verbs are never named actions; they stay `lifecycle` steps. */
const LIFECYCLE_VERBS = ['restart', 'start', 'stop'];

/** Maps a server step field name to the i18n key of its editor label; fields
 * without an entry fall back to the raw field name. */
const STEP_FIELD_LABEL_KEYS: Record<string, string> = {
  kind: 'settings.runbooks.steps.kindLabel',
  title: 'settings.runbooks.steps.titleLabel',
  connectorId: 'settings.runbooks.steps.connectorLabel',
  verb: 'settings.runbooks.steps.verbLabel',
  action: 'settings.runbooks.steps.actionLabel',
  timeoutSeconds: 'settings.runbooks.steps.timeoutLabel',
  entityRef: 'entityPicker.label',
  fieldKey: 'settings.runbooks.steps.fieldLabel',
  targetValue: 'settings.runbooks.steps.valueLabel',
  attribute: 'settings.runbooks.steps.attributeLabel',
  operator: 'settings.runbooks.steps.operatorLabel',
  expectedValue: 'settings.runbooks.steps.expectedValueLabel',
};

interface StepError {
  field: string;
  msg: string;
}

/** A step being edited: same shape as RunbookStepInput, but with a stable
 * client-side key so React can track rows across add/remove/reorder before
 * any of them have a server id. */
interface StepDraft {
  key: string;
  id?: string;
  kind?: RunbookStepKind;
  title: string;
  connectorId: string;
  /** Empty for a loaded step that is not a lifecycle step. */
  verb: RunbookStepVerb | '';
  entityRef: string;
  /** Named action of a connector_action step; empty for every other kind. */
  action: string;
  /** UI-only kind learned from the selected entity. */
  entityKind?: string;
  timeoutSeconds?: number | '';
  fieldKey: string;
  targetValue: unknown;
  attribute: string;
  operator: RunbookStepInputOperator | '';
  expectedValue: unknown;
  redacted?: boolean;
}

let stepKeySeq = 0;
function newStepDraft(): StepDraft {
  return {
    key: `new-${++stepKeySeq}`,
    kind: 'lifecycle',
    title: '',
    connectorId: '',
    verb: 'restart',
    entityRef: '',
    action: '',
    entityKind: '',
    timeoutSeconds: '',
    fieldKey: '',
    targetValue: '',
    attribute: '',
    operator: RunbookStepOperator.eq,
    expectedValue: '',
    redacted: false,
  };
}

interface Draft {
  title: string;
  body: string;
  targetType: RunbookTargetType;
  targetValue: string;
  docId: string;
  snapshotId: string;
  steps: StepDraft[];
}

const emptyDraft: Draft = {
  title: '',
  body: '',
  targetType: 'change_type',
  targetValue: '',
  docId: '',
  snapshotId: '',
  steps: [],
};

export function RunbooksPage() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const runId = searchParams.get('runId');
  const historyId = searchParams.get('runbookId');
  const selectRun = (id: string | null) =>
    setSearchParams((params) => {
      const next = new URLSearchParams(params);
      if (id) next.set('runId', id);
      else next.delete('runId');
      return next;
    });
  const selectHistory = (id: string | null) =>
    setSearchParams((params) => {
      const next = new URLSearchParams(params);
      if (id) next.set('runbookId', id);
      else next.delete('runbookId');
      return next;
    });
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetRunbooks({ pageSize: 100 });

  const [editing, setEditing] = useState<Runbook | 'new' | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [formError, setFormError] = useState<string | null>(null);
  const [stepErrors, setStepErrors] = useState<Record<number, StepError[]>>({});
  const [toDelete, setToDelete] = useState<Runbook | null>(null);

  const connectors = useGetConnectors();
  const schemas = useGetConnectorsSchema();
  const complianceSchema = useGetComplianceSchema({ query: { enabled: editing !== null } });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: getGetRunbooksQueryKey() });

  const openCreate = () => {
    setDraft(emptyDraft);
    setFormError(null);
    setStepErrors({});
    setEditing('new');
  };

  const openEdit = (rb: Runbook) => {
    setDraft({
      title: rb.title,
      body: rb.body,
      targetType: rb.targetType,
      targetValue: rb.targetValue,
      docId: rb.docId ?? '',
      snapshotId: rb.snapshotId ?? '',
      steps: rb.steps.map((s) => ({
        key: s.id,
        id: s.id,
        kind: s.kind,
        title: s.title,
        connectorId: s.connectorId,
        verb: s.verb as RunbookStepVerb | '',
        entityRef: s.entityRef,
        action: s.action ?? '',
        entityKind: '',
        timeoutSeconds: s.timeoutSeconds > 0 ? s.timeoutSeconds : '',
        fieldKey: s.fieldKey ?? '',
        targetValue: parseStepValue(s.targetValue),
        attribute: s.attribute ?? '',
        operator: s.operator ?? '',
        expectedValue: parseStepValue(s.expectedValue),
        redacted: !s.kind,
      })),
    });
    setFormError(null);
    setStepErrors({});
    setEditing(rb);
  };

  const closeDialog = () => {
    setEditing(null);
    setFormError(null);
    setStepErrors({});
  };

  // A runbook with a redacted step cannot be edited step-wise: the server
  // keeps every stored step when `steps` is absent, so we omit it entirely and
  // lock the list instead of echoing placeholders back.
  const stepsLocked = draft.steps.some((s) => s.redacted);

  const toStepInput = (s: StepDraft): RunbookStepInput => {
    const kind = s.kind || 'lifecycle';
    const base: RunbookStepInput = {
      ...(s.id ? { id: s.id } : {}),
      kind,
      title: s.title.trim(),
    };

    if (kind === 'lifecycle') {
      return {
        ...base,
        connectorId: s.connectorId,
        verb: (s.verb || undefined) as RunbookStepVerb | undefined,
        ...(s.entityRef ? { entityRef: s.entityRef } : {}),
      };
    }

    if (kind === 'sync_and_wait' || kind === 'wait_until_healthy') {
      const timeout =
        s.timeoutSeconds !== '' && s.timeoutSeconds !== undefined
          ? Number(s.timeoutSeconds)
          : undefined;
      return {
        ...base,
        connectorId: s.connectorId,
        ...(timeout !== undefined && !isNaN(timeout) ? { timeoutSeconds: timeout } : {}),
      };
    }

    if (kind === 'config_push') {
      return {
        ...base,
        connectorId: s.connectorId,
        fieldKey: s.fieldKey,
        targetValue: s.targetValue,
        ...(s.entityRef ? { entityRef: s.entityRef } : {}),
      };
    }

    if (kind === 'wait_for_entity') {
      const timeout =
        s.timeoutSeconds !== '' && s.timeoutSeconds !== undefined
          ? Number(s.timeoutSeconds)
          : undefined;
      return {
        ...base,
        connectorId: s.connectorId,
        entityRef: s.entityRef,
        attribute: s.attribute,
        operator: s.operator || undefined,
        expectedValue: s.expectedValue,
        ...(timeout !== undefined && !isNaN(timeout) ? { timeoutSeconds: timeout } : {}),
      };
    }

    if (kind === 'connector_action') {
      return {
        ...base,
        connectorId: s.connectorId,
        action: s.action,
        ...(s.entityRef ? { entityRef: s.entityRef } : {}),
      };
    }

    // kind === 'manual'
    return base;
  };

  const toPayload = () => ({
    title: draft.title.trim(),
    body: draft.body,
    targetType: draft.targetType,
    targetValue: draft.targetValue.trim(),
    docId: draft.docId.trim() || null,
    snapshotId: draft.snapshotId.trim() || null,
    ...(stepsLocked ? {} : { steps: draft.steps.map(toStepInput) }),
  });

  const conflictMessage = t('settings.runbooks.conflict');

  const applyFieldErrors = (error: unknown): boolean => {
    if (!isAxiosError(error) || error.response?.status !== 400) return false;
    const details = (
      error.response?.data as { details?: { field: string; msg: string }[] } | undefined
    )?.details;
    if (!details || details.length === 0) return false;
    const nextStepErrors: Record<number, StepError[]> = {};
    const otherMessages: string[] = [];
    for (const d of details) {
      const m = /^steps\[(\d+)\](?:\.(\w+))?/.exec(d.field);
      if (m) {
        const idx = Number(m[1]);
        (nextStepErrors[idx] ??= []).push({ field: m[2] ?? '', msg: d.msg });
      } else {
        otherMessages.push(d.msg);
      }
    }
    setStepErrors(nextStepErrors);
    if (otherMessages.length > 0) setFormError(otherMessages.join(' '));
    else if (Object.keys(nextStepErrors).length > 0)
      setFormError(t('settings.runbooks.steps.invalid'));
    return true;
  };

  const create = useMutation({
    mutationFn: () => postRunbooks(toPayload()),
    onSuccess: () => {
      invalidate();
      closeDialog();
      toast.success(t('settings.runbooks.toastCreated'));
    },
    onError: (error) => {
      if (isAxiosError(error) && error.response?.status === 409) {
        setFormError(conflictMessage);
        return;
      }
      if (applyFieldErrors(error)) return;
      toast.error(t('settings.runbooks.toastCreateError'));
    },
  });

  const update = useMutation({
    mutationFn: (id: string) => putRunbooksRunbookId(id, toPayload()),
    onSuccess: () => {
      invalidate();
      closeDialog();
      toast.success(t('settings.runbooks.toastSaved'));
    },
    onError: (error) => {
      if (isAxiosError(error) && error.response?.status === 409) {
        setFormError(conflictMessage);
        return;
      }
      if (applyFieldErrors(error)) return;
      toast.error(t('settings.runbooks.toastSaveError'));
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteRunbooksRunbookId(id),
    onSuccess: () => {
      invalidate();
      setToDelete(null);
      toast.success(t('settings.runbooks.toastDeleted'));
    },
    onError: () => toast.error(t('settings.runbooks.toastDeleteError')),
  });

  const saving = create.isPending || update.isPending;
  const canSave = draft.title.trim() !== '' && draft.targetValue.trim() !== '';

  // Step errors are keyed by array index, so any structural change drops them.
  const addStep = () => {
    setStepErrors({});
    setDraft((d) =>
      d.steps.length >= MAX_STEPS ? d : { ...d, steps: [...d.steps, newStepDraft()] }
    );
  };

  const removeStep = (key: string) => {
    setStepErrors({});
    setDraft((d) => ({ ...d, steps: d.steps.filter((s) => s.key !== key) }));
  };

  const moveStep = (key: string, dir: -1 | 1) => {
    setStepErrors({});
    setDraft((d) => {
      const idx = d.steps.findIndex((s) => s.key === key);
      const next = idx + dir;
      if (idx < 0 || next < 0 || next >= d.steps.length) return d;
      const steps = [...d.steps];
      [steps[idx], steps[next]] = [steps[next], steps[idx]];
      return { ...d, steps };
    });
  };

  const updateStep = (key: string, patch: Partial<StepDraft>) =>
    setDraft((d) => ({
      ...d,
      steps: d.steps.map((s) => (s.key === key ? { ...s, ...patch } : s)),
    }));

  // Named actions a connector_action step can run: the connector's own actions
  // (never a lifecycle verb) whose entity scope matches the chosen target.
  const actionOptions = (connectorId: string, entityRef: string) =>
    (connectors.data?.find((c) => c.id === connectorId)?.actions ?? []).filter(
      (action) => !LIFECYCLE_VERBS.includes(action.name) && action.entityScope === !!entityRef
    );

  // Keeps the chosen action only while it still matches the target's entity scope.
  const keepAction = (connectorId: string, entityRef: string, action: string) =>
    actionOptions(connectorId, entityRef).some((option) => option.name === action) ? action : '';

  const lifecycleVerbs = (connectorId: string, entityRef: string): RunbookStepVerb[] => {
    const selected = connectors.data?.find((c) => c.id === connectorId);
    if (selected?.type === 'custom') {
      return LIFECYCLE_VERBS.filter((verb) =>
        selected.actions.some(
          (action) => action.name === verb && action.entityScope === !!entityRef
        )
      ) as RunbookStepVerb[];
    }
    return (
      schemas.data?.find((s) => s.type === selected?.type)?.lifecycleVerbs ??
      (['restart', 'start', 'stop'] as RunbookStepVerb[])
    );
  };

  return (
    <div>
      <SubHeader
        title={t('settings.runbooks.title')}
        description={t('settings.runbooks.subtitle')}
      />

      <div className="mb-4 flex justify-end">
        <Button variant="primary" size="sm" onClick={openCreate}>
          <PlusIcon size={14} />
          {t('settings.runbooks.new')}
        </Button>
      </div>

      <Panel>
        {isLoading ? (
          <SkeletonRows rows={4} />
        ) : isError || !data ? (
          <ErrorState description={t('settings.runbooks.loadError')} onRetry={() => refetch()} />
        ) : data.items.length === 0 ? (
          <EmptyState
            icon={<FileTextIcon size={20} />}
            title={t('settings.runbooks.emptyTitle')}
            description={t('settings.runbooks.emptyDesc')}
            action={
              <Button variant="secondary" size="sm" onClick={openCreate}>
                <PlusIcon size={14} />
                {t('settings.runbooks.new')}
              </Button>
            }
          />
        ) : (
          <ul className="divide-y divide-line-soft">
            {data.items.map((rb) => (
              <li key={rb.id} className="flex items-center gap-4 px-4 py-3">
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-canvas-sunken text-ink-faint">
                  <FileTextIcon size={15} />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-ink">{rb.title}</p>
                  <p className="font-mono text-2xs text-ink-faint">
                    {t(`settings.runbooks.targetType.${rb.targetType}`, {
                      defaultValue: rb.targetType,
                    })}{' '}
                    · {rb.targetValue}
                  </p>
                </div>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => selectHistory(rb.id)}
                  aria-label={t('runbooks.runs.historyFor', { title: rb.title })}
                >
                  {t('runbooks.runs.history')}
                </Button>
                <IconButton label={t('settings.runbooks.editLabel')} onClick={() => openEdit(rb)}>
                  <EditIcon size={15} />
                </IconButton>
                <IconButton
                  label={t('settings.runbooks.deleteLabel')}
                  onClick={() => setToDelete(rb)}
                  className="hover:text-err"
                >
                  <XIcon size={15} />
                </IconButton>
              </li>
            ))}
          </ul>
        )}
      </Panel>

      {historyId && !runId && (
        <Dialog
          open
          onClose={() => selectHistory(null)}
          title={t('runbooks.runs.history')}
          size="lg"
        >
          <RunHistory runbookId={historyId} onSelectRun={selectRun} />
          <div className="mt-4 flex justify-end">
            <Button variant="ghost" size="sm" onClick={() => selectHistory(null)}>
              {t('common.close')}
            </Button>
          </div>
        </Dialog>
      )}
      {runId && (
        <Dialog
          open
          onClose={() => selectRun(null)}
          title={t('runbooks.runs.detailTitle')}
          size="lg"
        >
          <RunDetail key={runId} runId={runId} onClose={() => selectRun(null)} showTitle={false} />
        </Dialog>
      )}

      <Dialog
        open={editing !== null}
        onClose={closeDialog}
        title={
          editing === 'new' ? t('settings.runbooks.createTitle') : t('settings.runbooks.editTitle')
        }
        size="md"
      >
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (!canSave) return;
            if (editing === 'new') create.mutate();
            else if (editing) update.mutate(editing.id);
          }}
        >
          <Field label={t('settings.runbooks.titleLabel')} htmlFor="runbook-title">
            <TextInput
              id="runbook-title"
              autoFocus
              value={draft.title}
              onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
            />
          </Field>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t('settings.runbooks.targetTypeLabel')} htmlFor="runbook-target-type">
              <Select
                id="runbook-target-type"
                value={draft.targetType}
                onChange={(e) =>
                  setDraft((d) => ({
                    ...d,
                    targetType: e.target.value as RunbookTargetType,
                    targetValue: '',
                  }))
                }
              >
                {TARGET_TYPES.map((tt) => (
                  <option key={tt} value={tt}>
                    {t(`settings.runbooks.targetType.${tt}`, { defaultValue: tt })}
                  </option>
                ))}
              </Select>
            </Field>

            <Field label={t('settings.runbooks.targetValueLabel')} htmlFor="runbook-target-value">
              {draft.targetType === 'alert_severity' ? (
                <Select
                  id="runbook-target-value"
                  value={draft.targetValue}
                  onChange={(e) => setDraft((d) => ({ ...d, targetValue: e.target.value }))}
                >
                  <option value="" disabled>
                    {t('settings.runbooks.targetValuePlaceholder')}
                  </option>
                  {Object.values(Severity).map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
                </Select>
              ) : (
                <TextInput
                  id="runbook-target-value"
                  value={draft.targetValue}
                  placeholder={
                    draft.targetType === 'finding_check_type'
                      ? t('settings.runbooks.checkTypePlaceholder')
                      : t('settings.runbooks.changeTypePlaceholder')
                  }
                  onChange={(e) => setDraft((d) => ({ ...d, targetValue: e.target.value }))}
                />
              )}
            </Field>
          </div>

          <Field label={t('settings.runbooks.bodyLabel')} htmlFor="runbook-body">
            <textarea
              id="runbook-body"
              rows={6}
              value={draft.body}
              onChange={(e) => setDraft((d) => ({ ...d, body: e.target.value }))}
              className="w-full rounded-md border border-line-strong bg-canvas-sunken px-3 py-2 text-sm text-ink outline-none placeholder:text-ink-faint focus-visible:border-accent-primary-soft"
            />
          </Field>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field
              label={t('settings.runbooks.docIdLabel')}
              htmlFor="runbook-doc-id"
              hint={t('settings.runbooks.docIdHint')}
            >
              <TextInput
                id="runbook-doc-id"
                value={draft.docId}
                onChange={(e) => setDraft((d) => ({ ...d, docId: e.target.value }))}
              />
            </Field>
            <Field
              label={t('settings.runbooks.snapshotIdLabel')}
              htmlFor="runbook-snapshot-id"
              hint={t('settings.runbooks.snapshotIdHint')}
            >
              <TextInput
                id="runbook-snapshot-id"
                value={draft.snapshotId}
                onChange={(e) => setDraft((d) => ({ ...d, snapshotId: e.target.value }))}
              />
            </Field>
          </div>

          <div>
            <div className="mb-1.5 flex items-center justify-between">
              <span className="font-mono text-2xs text-ink-faint">
                {t('settings.runbooks.steps.heading')}
              </span>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={addStep}
                disabled={stepsLocked || draft.steps.length >= MAX_STEPS}
              >
                <PlusIcon size={13} />
                {t('settings.runbooks.steps.add')}
              </Button>
            </div>
            <p className="mb-2 text-2xs leading-relaxed text-ink-faint">
              {t('settings.runbooks.steps.hint')}
            </p>
            {stepsLocked && (
              <p className="mb-2 text-2xs leading-relaxed text-ink-muted">
                {t('settings.runbooks.steps.lockedHint')}
              </p>
            )}

            {draft.steps.length === 0 ? (
              <p className="text-xs text-ink-muted">{t('settings.runbooks.steps.empty')}</p>
            ) : (
              <fieldset disabled={stepsLocked} className="m-0 min-w-0 border-0 p-0">
                <ul className="space-y-3">
                  {draft.steps.map((step, index) => {
                    const isRedacted = !!step.redacted;
                    const kind = step.kind || 'lifecycle';
                    return (
                      <li
                        key={step.key}
                        className="space-y-2 rounded-md border border-line-soft bg-canvas-sunken p-3"
                      >
                        <div className="flex items-center gap-2">
                          <TextInput
                            aria-label={t('settings.runbooks.steps.titleLabel')}
                            value={step.title}
                            placeholder={t('settings.runbooks.steps.titlePlaceholder')}
                            disabled={stepsLocked || isRedacted}
                            readOnly={stepsLocked || isRedacted}
                            onChange={(e) => updateStep(step.key, { title: e.target.value })}
                            className="flex-1"
                          />
                          {isRedacted ? (
                            <span className="font-mono text-2xs italic text-ink-faint">
                              {t('settings.runbooks.steps.restricted')}
                            </span>
                          ) : (
                            <>
                              <IconButton
                                label={t('settings.runbooks.steps.moveUp')}
                                onClick={() => moveStep(step.key, -1)}
                                disabled={stepsLocked || index === 0}
                              >
                                <ChevronDownIcon size={14} className="rotate-180" />
                              </IconButton>
                              <IconButton
                                label={t('settings.runbooks.steps.moveDown')}
                                onClick={() => moveStep(step.key, 1)}
                                disabled={stepsLocked || index === draft.steps.length - 1}
                              >
                                <ChevronDownIcon size={14} />
                              </IconButton>
                              <IconButton
                                label={t('settings.runbooks.steps.remove')}
                                onClick={() => removeStep(step.key)}
                                disabled={stepsLocked}
                                className="hover:text-err"
                              >
                                <XIcon size={14} />
                              </IconButton>
                            </>
                          )}
                        </div>

                        {isRedacted ? (
                          <p className="text-2xs italic text-ink-faint">
                            {t('settings.runbooks.steps.redactedHint')}
                          </p>
                        ) : (
                          <div className="grid gap-2 sm:grid-cols-3">
                            <label className="block">
                              <span className="mb-1 block text-2xs text-ink-faint">
                                {t('settings.runbooks.steps.kindLabel')}
                              </span>
                              <Select
                                aria-label={t('settings.runbooks.steps.kindLabel')}
                                disabled={stepsLocked}
                                value={kind}
                                onChange={(e) => {
                                  const nextKind = e.target.value as RunbookStepKind;
                                  if (nextKind === 'manual') {
                                    updateStep(step.key, {
                                      kind: nextKind,
                                      connectorId: '',
                                      verb: '',
                                      entityRef: '',
                                      action: '',
                                      entityKind: '',
                                      timeoutSeconds: '',
                                      fieldKey: '',
                                      targetValue: '',
                                      attribute: '',
                                      operator: RunbookStepOperator.eq,
                                      expectedValue: '',
                                    });
                                  } else if (
                                    nextKind === 'sync_and_wait' ||
                                    nextKind === 'wait_until_healthy'
                                  ) {
                                    updateStep(step.key, {
                                      kind: nextKind,
                                      verb: '',
                                      entityRef: '',
                                      action: '',
                                      entityKind: '',
                                      timeoutSeconds: step.timeoutSeconds || '',
                                      fieldKey: '',
                                      targetValue: '',
                                      attribute: '',
                                      operator: RunbookStepOperator.eq,
                                      expectedValue: '',
                                    });
                                  } else if (nextKind === 'config_push') {
                                    updateStep(step.key, {
                                      kind: nextKind,
                                      verb: '',
                                      entityRef: '',
                                      action: '',
                                      entityKind: '',
                                      timeoutSeconds: '',
                                      fieldKey: '',
                                      targetValue: '',
                                      attribute: '',
                                      operator: RunbookStepOperator.eq,
                                      expectedValue: '',
                                    });
                                  } else if (nextKind === 'connector_action') {
                                    updateStep(step.key, {
                                      kind: nextKind,
                                      verb: '',
                                      entityRef: '',
                                      action: '',
                                      entityKind: '',
                                      timeoutSeconds: '',
                                      fieldKey: '',
                                      targetValue: '',
                                      attribute: '',
                                      operator: RunbookStepOperator.eq,
                                      expectedValue: '',
                                    });
                                  } else if (nextKind === 'wait_for_entity') {
                                    updateStep(step.key, {
                                      kind: nextKind,
                                      verb: '',
                                      action: '',
                                      entityKind: '',
                                      fieldKey: '',
                                      targetValue: '',
                                      attribute: '',
                                      operator: RunbookStepOperator.eq,
                                      expectedValue: '',
                                      timeoutSeconds: '',
                                    });
                                  } else {
                                    // lifecycle
                                    const verbs = lifecycleVerbs(step.connectorId, step.entityRef);
                                    updateStep(step.key, {
                                      kind: nextKind,
                                      verb: verbs[0] ?? '',
                                      action: '',
                                      entityKind: '',
                                      timeoutSeconds: '',
                                    });
                                  }
                                }}
                              >
                                <option value="lifecycle">
                                  {t('settings.runbooks.steps.kinds.lifecycle')}
                                </option>
                                <option value="sync_and_wait">
                                  {t('settings.runbooks.steps.kinds.sync_and_wait')}
                                </option>
                                <option value="wait_until_healthy">
                                  {t('settings.runbooks.steps.kinds.wait_until_healthy')}
                                </option>
                                <option value="config_push">
                                  {t('settings.runbooks.steps.kinds.config_push')}
                                </option>
                                <option value="wait_for_entity">
                                  {t('settings.runbooks.steps.kinds.wait_for_entity')}
                                </option>
                                <option value="connector_action">
                                  {t('settings.runbooks.steps.kinds.connector_action')}
                                </option>
                                <option value="manual">
                                  {t('settings.runbooks.steps.kinds.manual')}
                                </option>
                              </Select>
                            </label>

                            {kind !== 'manual' && (
                              <label className="block">
                                <span className="mb-1 block text-2xs text-ink-faint">
                                  {t('settings.runbooks.steps.connectorLabel')}
                                </span>
                                <Select
                                  aria-label={t('settings.runbooks.steps.connectorLabel')}
                                  disabled={stepsLocked}
                                  value={step.connectorId}
                                  onChange={(e) => {
                                    const connectorId = e.target.value;
                                    if (kind === 'lifecycle') {
                                      const verbs = lifecycleVerbs(connectorId, '');
                                      updateStep(step.key, {
                                        connectorId,
                                        entityRef: '',
                                        entityKind: '',
                                        verb: verbs[0] ?? '',
                                      });
                                    } else {
                                      updateStep(step.key, {
                                        connectorId,
                                        entityRef: '',
                                        action: '',
                                        entityKind: '',
                                        fieldKey: '',
                                        targetValue: '',
                                        attribute: '',
                                        operator: RunbookStepOperator.eq,
                                        expectedValue: '',
                                      });
                                    }
                                  }}
                                >
                                  <option value="" disabled>
                                    {t('settings.runbooks.steps.connectorPlaceholder')}
                                  </option>
                                  {(connectors.data ?? []).map((c) => (
                                    <option key={c.id} value={c.id}>
                                      {c.name}
                                    </option>
                                  ))}
                                </Select>
                              </label>
                            )}

                            {kind === 'lifecycle' && (
                              <label className="block">
                                <span className="mb-1 block text-2xs text-ink-faint">
                                  {t('settings.runbooks.steps.verbLabel')}
                                </span>
                                <Select
                                  aria-label={t('settings.runbooks.steps.verbLabel')}
                                  disabled={stepsLocked}
                                  value={step.verb}
                                  onChange={(e) =>
                                    updateStep(step.key, {
                                      verb: e.target.value as RunbookStepVerb,
                                    })
                                  }
                                >
                                  {(() => {
                                    const verbs = lifecycleVerbs(step.connectorId, step.entityRef);
                                    return verbs.map((v) => (
                                      <option key={v} value={v}>
                                        {v}
                                      </option>
                                    ));
                                  })()}
                                </Select>
                              </label>
                            )}

                            {(kind === 'sync_and_wait' || kind === 'wait_until_healthy') && (
                              <label className="block">
                                <span className="mb-1 block text-2xs text-ink-faint">
                                  {t('settings.runbooks.steps.timeoutLabel')}
                                </span>
                                <TextInput
                                  aria-label={t('settings.runbooks.steps.timeoutLabel')}
                                  type="number"
                                  min={MIN_TIMEOUT_SECONDS}
                                  max={MAX_TIMEOUT_SECONDS}
                                  placeholder={String(DEFAULT_TIMEOUT_SECONDS)}
                                  disabled={stepsLocked}
                                  value={step.timeoutSeconds ?? ''}
                                  onChange={(e) =>
                                    updateStep(step.key, {
                                      timeoutSeconds:
                                        e.target.value === '' ? '' : Number(e.target.value),
                                    })
                                  }
                                />
                              </label>
                            )}

                            {kind === 'lifecycle' && step.connectorId && (
                              <EntityPicker
                                connectorId={step.connectorId}
                                value={step.entityRef}
                                disabled={stepsLocked}
                                onChange={(entityRef) => {
                                  const verbs = lifecycleVerbs(step.connectorId, entityRef);
                                  updateStep(step.key, {
                                    entityRef,
                                    verb: verbs.includes(step.verb as RunbookStepVerb)
                                      ? step.verb
                                      : (verbs[0] ?? ''),
                                  });
                                }}
                              />
                            )}

                            {kind === 'connector_action' && step.connectorId && (
                              <EntityPicker
                                connectorId={step.connectorId}
                                value={step.entityRef}
                                disabled={stepsLocked}
                                onChange={(entityRef) =>
                                  updateStep(step.key, {
                                    entityRef,
                                    action: keepAction(step.connectorId, entityRef, step.action),
                                  })
                                }
                              />
                            )}

                            {kind === 'connector_action' && step.connectorId && (
                              <label className="block">
                                <span className="mb-1 block text-2xs text-ink-faint">
                                  {t('settings.runbooks.steps.actionLabel')}
                                </span>
                                <Select
                                  aria-label={t('settings.runbooks.steps.actionLabel')}
                                  disabled={stepsLocked}
                                  value={step.action}
                                  onChange={(e) => updateStep(step.key, { action: e.target.value })}
                                >
                                  <option value="" disabled>
                                    {actionOptions(step.connectorId, step.entityRef).length === 0
                                      ? t('settings.runbooks.steps.noActions')
                                      : t('settings.runbooks.steps.actionPlaceholder')}
                                  </option>
                                  {actionOptions(step.connectorId, step.entityRef).map((action) => (
                                    <option key={action.name} value={action.name}>
                                      {action.label || action.name}
                                    </option>
                                  ))}
                                </Select>
                              </label>
                            )}

                            {(kind === 'config_push' || kind === 'wait_for_entity') && (
                              <RunbookStepKindFields
                                step={step}
                                complianceAttributes={
                                  complianceSchema.data?.attributes?.[
                                    connectors.data?.find((c) => c.id === step.connectorId)?.type ??
                                      ''
                                  ]
                                }
                                disabled={stepsLocked}
                                onChange={(patch) => updateStep(step.key, patch)}
                              />
                            )}

                            {kind === 'manual' && (
                              <div className="flex items-center sm:col-span-2">
                                <p className="text-2xs text-ink-faint">
                                  {t('settings.runbooks.steps.manualHint')}
                                </p>
                              </div>
                            )}
                          </div>
                        )}

                        {(stepErrors[index] ?? []).map((err, i) => {
                          const labelKey = STEP_FIELD_LABEL_KEYS[err.field];
                          const label = labelKey ? t(labelKey) : err.field;
                          return (
                            <p key={`${err.field}-${i}`} role="alert" className="text-2xs text-err">
                              {label && <span className="font-medium">{label}:</span>} {err.msg}
                            </p>
                          );
                        })}
                      </li>
                    );
                  })}
                </ul>
              </fieldset>
            )}
          </div>

          {formError && (
            <p role="alert" className="text-xs text-err">
              {formError}
            </p>
          )}

          <div className="flex items-center justify-end gap-2 pt-1">
            <Button type="button" variant="ghost" size="sm" onClick={closeDialog}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="primary" size="sm" disabled={!canSave || saving}>
              {saving ? t('settings.runbooks.saving') : t('common.save')}
            </Button>
          </div>
        </form>
      </Dialog>

      <ConfirmDialog
        open={!!toDelete}
        onClose={() => setToDelete(null)}
        onConfirm={() => {
          if (toDelete) remove.mutate(toDelete.id);
        }}
        tone="danger"
        title={t('settings.runbooks.deleteTitle')}
        description={t('settings.runbooks.deleteDesc', { title: toDelete?.title ?? '' })}
        confirmLabel={
          remove.isPending ? t('settings.runbooks.deleting') : t('settings.runbooks.delete')
        }
        cancelLabel={t('common.cancel')}
        confirmDisabled={remove.isPending}
      />
    </div>
  );
}

function RunbookStepKindFields({
  step,
  complianceAttributes,
  disabled,
  onChange,
}: {
  step: StepDraft;
  complianceAttributes?: ComplianceSchemaAttributes[string];
  disabled: boolean;
  onChange: (patch: Partial<StepDraft>) => void;
}) {
  const { t } = useTranslation();
  const configFields = useGetConnectorsConnectorIdConfigFields(step.connectorId, {
    query: { enabled: step.kind === 'config_push' && !!step.connectorId },
  });
  // A loaded wait step has no entityKind until the user re-picks the entity, so
  // derive it from the picker's own (cached) latest-snapshot data.
  const needsEntityKind =
    step.kind === 'wait_for_entity' && !!step.connectorId && !step.entityKind && !!step.entityRef;
  const latestSnapshots = useGetConnectorsConnectorIdSnapshots(
    step.connectorId,
    { limit: 1 },
    { query: { enabled: needsEntityKind } }
  );
  const latestSnapshotId = latestSnapshots.data?.[0]?.id ?? '';
  const latestSnapshot = useGetConnectorsConnectorIdSnapshotsSnapshotId(
    step.connectorId,
    latestSnapshotId,
    { query: { enabled: needsEntityKind && !!latestSnapshotId } }
  );
  const entityKind =
    step.entityKind ||
    (needsEntityKind
      ? (latestSnapshot.data?.entities.find((entity) => entity.externalId === step.entityRef)
          ?.kind ?? '')
      : '');
  if (step.kind === 'config_push') {
    const fields = configFields.data ?? [];
    const field = fields.find((candidate) => candidate.key === step.fieldKey);
    const fieldLabel = t('settings.runbooks.steps.fieldLabel');
    const valueLabel = t('settings.runbooks.steps.valueLabel');
    const inputValue = getInputValue(step.targetValue);
    return (
      <>
        <label className="block">
          <span className="mb-1 block text-2xs text-ink-faint">{fieldLabel}</span>
          <Select
            aria-label={fieldLabel}
            disabled={disabled}
            value={step.fieldKey}
            onChange={(event) => {
              const next = fields.find((candidate) => candidate.key === event.target.value);
              onChange({
                fieldKey: event.target.value,
                targetValue: next?.type === 'toggle' ? false : '',
                entityRef: '',
              });
            }}
          >
            <option value="" disabled>
              {t('settings.runbooks.steps.fieldPlaceholder')}
            </option>
            {fields.map((candidate) => (
              <option key={candidate.key} value={candidate.key}>
                {candidate.label}
              </option>
            ))}
          </Select>
        </label>

        {field && (
          <label className="block">
            <span className="mb-1 block text-2xs text-ink-faint">{valueLabel}</span>
            {field.type === 'select' ? (
              <Select
                aria-label={valueLabel}
                disabled={disabled}
                value={inputValue}
                onChange={(event) => onChange({ targetValue: event.target.value })}
              >
                <option value="" disabled>
                  {t('settings.runbooks.steps.valuePlaceholder')}
                </option>
                {(field.options ?? []).map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </Select>
            ) : field.type === 'toggle' ? (
              <input
                aria-label={valueLabel}
                type="checkbox"
                checked={step.targetValue === true}
                disabled={disabled}
                onChange={(event) => onChange({ targetValue: event.target.checked })}
                className="h-4 w-4 rounded border-line-strong text-accent-primary focus-visible:ring-accent-primary"
              />
            ) : (
              <TextInput
                aria-label={valueLabel}
                type={field.type === 'number' ? 'number' : 'text'}
                step={field.type === 'number' ? 'any' : undefined}
                disabled={disabled}
                value={inputValue}
                onChange={(event) =>
                  onChange({
                    targetValue:
                      field.type === 'number'
                        ? event.target.value === ''
                          ? ''
                          : Number(event.target.value)
                        : event.target.value,
                  })
                }
              />
            )}
          </label>
        )}

        {step.connectorId && field?.entityScope && (
          <EntityPicker
            connectorId={step.connectorId}
            value={step.entityRef}
            label={t('settings.runbooks.steps.entityLabel')}
            disabled={disabled}
            hideWholeService
            onChange={(entityRef) => onChange({ entityRef })}
          />
        )}
      </>
    );
  }

  const attributes = Object.values(complianceAttributes ?? {}).flat();
  const attributesForEntity = entityKind
    ? Object.values(complianceAttributes?.[entityKind] ?? {}).flat()
    : attributes;
  const attributeNames = [
    ...new Set(
      attributesForEntity.map((attribute) => attribute.name)
    ),
  ].sort();
  const attributeType = uniqueAttributeType(attributesForEntity, step.attribute);
  const numericOperator =
    step.operator === RunbookStepOperator.gt || step.operator === RunbookStepOperator.lt;
  const textOperator =
    step.operator === RunbookStepOperator.regex || step.operator === RunbookStepOperator.contains;
  const numericExpectedValue =
    numericOperator || (!textOperator && attributeType === ComplianceAttributeSpecType.number);
  const booleanExpectedValue =
    !numericOperator && !textOperator && attributeType === ComplianceAttributeSpecType.boolean;
  const arrayExpectedValue =
    !numericOperator &&
    (step.operator === RunbookStepOperator.eq || step.operator === RunbookStepOperator.neq) &&
    attributeType === ComplianceAttributeSpecType.string_array;
  const listId = `runbook-attributes-${step.key}`;
  const waitTimeoutMinutes =
    typeof step.timeoutSeconds === 'number' ? step.timeoutSeconds / 60 : '';
  return (
    <>
      {step.connectorId && (
        <EntityPicker
          connectorId={step.connectorId}
          value={step.entityRef}
          label={t('settings.runbooks.steps.entityLabel')}
          disabled={disabled}
          hideWholeService
          onChange={(entityRef) => onChange({ entityRef })}
          onEntityChange={(entity) => {
            const entityKind = entity?.kind ?? '';
            const nextAttributes = entityKind
              ? Object.values(complianceAttributes?.[entityKind] ?? {}).flat()
              : attributes;
            const nextType = uniqueAttributeType(nextAttributes, step.attribute);
            onChange({
              entityKind,
              expectedValue: parseExpectedValue(
                expectedValueToInput(step.expectedValue, nextType),
                nextType,
                step.operator
              ),
            });
          }}
        />
      )}
      <label className="block">
        <span className="mb-1 block text-2xs text-ink-faint">
          {t('settings.runbooks.steps.attributeLabel')}
        </span>
        <TextInput
          aria-label={t('settings.runbooks.steps.attributeLabel')}
          list={listId}
          disabled={disabled}
          value={step.attribute}
          onChange={(event) => {
            const attribute = event.target.value;
            const nextType = uniqueAttributeType(attributesForEntity, attribute);
            onChange({
              attribute,
              expectedValue: parseExpectedValue(
                expectedValueToInput(step.expectedValue, nextType),
                nextType,
                step.operator
              ),
            });
          }}
        />
        <datalist id={listId}>
          {attributeNames.map((name) => (
            <option key={name} value={name} />
          ))}
        </datalist>
      </label>
      <label className="block">
        <span className="mb-1 block text-2xs text-ink-faint">
          {t('settings.runbooks.steps.operatorLabel')}
        </span>
        <Select
          aria-label={t('settings.runbooks.steps.operatorLabel')}
          disabled={disabled}
          value={step.operator}
          onChange={(event) => {
            const operator = event.target.value as RunbookStepInputOperator;
            onChange({
              operator,
              expectedValue: parseExpectedValue(
                expectedValueToInput(step.expectedValue, attributeType),
                attributeType,
                operator
              ),
            });
          }}
        >
          {Object.values(RunbookStepOperator).map((operator) => (
            <option key={operator} value={operator}>
              {t(`settings.runbooks.steps.operators.${operator}`)}
            </option>
          ))}
        </Select>
      </label>
      <label className="block">
        <span className="mb-1 block text-2xs text-ink-faint">
          {t('settings.runbooks.steps.expectedValueLabel')}
        </span>
        {booleanExpectedValue ? (
          <Select
            aria-label={t('settings.runbooks.steps.expectedValueLabel')}
            disabled={disabled}
            value={expectedValueToInput(step.expectedValue, attributeType)}
            onChange={(event) =>
              onChange({
                expectedValue: parseExpectedValue(event.target.value, attributeType, step.operator),
              })
            }
          >
            <option value="" disabled>
              {t('settings.runbooks.steps.valuePlaceholder')}
            </option>
            <option value="true">true</option>
            <option value="false">false</option>
          </Select>
        ) : (
          <TextInput
            aria-label={t('settings.runbooks.steps.expectedValueLabel')}
            type={numericExpectedValue ? 'number' : 'text'}
            step={numericExpectedValue ? 'any' : undefined}
            placeholder={arrayExpectedValue ? t('compliance.arrayHint') : undefined}
            disabled={disabled}
            value={expectedValueToInput(step.expectedValue, attributeType)}
            onChange={(event) =>
              onChange({
                expectedValue: parseExpectedValue(
                  event.target.value,
                  attributeType,
                  step.operator
                ),
              })
            }
          />
        )}
      </label>
      <label className="block">
        <span className="mb-1 block text-2xs text-ink-faint">
          {t('settings.runbooks.steps.timeoutMinutesLabel')}
        </span>
        <TextInput
          aria-label={t('settings.runbooks.steps.timeoutMinutesLabel')}
          type="number"
          min={1}
          max={30}
          placeholder="5"
          disabled={disabled}
          value={waitTimeoutMinutes}
          onChange={(event) =>
            onChange({
              timeoutSeconds: event.target.value === '' ? '' : Number(event.target.value) * 60,
            })
          }
        />
      </label>
    </>
  );
}

function parseStepValue(value?: string): unknown {
  if (value === undefined) return '';
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return value;
  }
}

function getInputValue(value: unknown): string {
  if (value === undefined || value === null) return '';
  return typeof value === 'string' ? value : String(value);
}

function inputValueToNumber(value: unknown): number | '' {
  if (value === '' || value === undefined || value === null) return '';
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : '';
}

function expectedValueToInput(value: unknown, type?: string): string {
  if (type === ComplianceAttributeSpecType.string_array && Array.isArray(value)) {
    return value.join(', ');
  }
  return getInputValue(value);
}

function parseExpectedValue(
  value: string,
  type: string | undefined,
  operator: RunbookStepInputOperator | ''
): unknown {
  if (operator === RunbookStepOperator.regex || operator === RunbookStepOperator.contains) {
    return value;
  }
  if (
    operator === RunbookStepOperator.gt ||
    operator === RunbookStepOperator.lt ||
    type === ComplianceAttributeSpecType.number
  ) {
    return inputValueToNumber(value);
  }
  if (type === ComplianceAttributeSpecType.boolean) {
    if (value === 'true') return true;
    if (value === 'false') return false;
    return value;
  }
  if (
    type === ComplianceAttributeSpecType.string_array &&
    (operator === RunbookStepOperator.eq || operator === RunbookStepOperator.neq)
  ) {
    return value.split(',').map((item) => item.trim()).filter(Boolean);
  }
  return value;
}

function uniqueAttributeType(
  attributes: { name: string; type: string }[],
  name: string
): string | undefined {
  const types = new Set(attributes.filter((attribute) => attribute.name === name).map((a) => a.type));
  return types.size === 1 ? [...types][0] : undefined;
}
