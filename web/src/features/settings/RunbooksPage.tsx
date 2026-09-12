/**
 * Runbooks list (operator-only). Create, edit, and delete the actionable
 * runbooks that attach to a change type or alert severity — surfaced inline
 * by RunbookPanel elsewhere. The backend enforces one runbook per target, so
 * create/update conflicts are surfaced as a dedicated inline error rather
 * than a generic toast.
 */
import { useState } from 'react';
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
import type { Runbook, RunbookTargetType } from '../../api/model';
import { Severity } from '../../api/model/severity';
import { Button, IconButton } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Dialog } from '../../components/ui/Dialog';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { toast } from '../../lib/toast';
import { SubHeader, Field, TextInput, Select } from './parts';
import { FileTextIcon, PlusIcon, EditIcon, XIcon } from '../../components/icons';

const TARGET_TYPES: RunbookTargetType[] = ['change_type', 'alert_severity'];

interface Draft {
  title: string;
  body: string;
  targetType: RunbookTargetType;
  targetValue: string;
  docId: string;
  snapshotId: string;
}

const emptyDraft: Draft = {
  title: '',
  body: '',
  targetType: 'change_type',
  targetValue: '',
  docId: '',
  snapshotId: '',
};

export function RunbooksPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetRunbooks({ pageSize: 100 });

  const [editing, setEditing] = useState<Runbook | 'new' | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [formError, setFormError] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<Runbook | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: getGetRunbooksQueryKey() });

  const openCreate = () => {
    setDraft(emptyDraft);
    setFormError(null);
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
    });
    setFormError(null);
    setEditing(rb);
  };

  const closeDialog = () => {
    setEditing(null);
    setFormError(null);
  };

  const toPayload = () => ({
    title: draft.title.trim(),
    body: draft.body,
    targetType: draft.targetType,
    targetValue: draft.targetValue.trim(),
    docId: draft.docId.trim() || null,
    snapshotId: draft.snapshotId.trim() || null,
  });

  const conflictMessage = t('settings.runbooks.conflict');

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
                  placeholder={t('settings.runbooks.changeTypePlaceholder')}
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
            <Field label={t('settings.runbooks.docIdLabel')} htmlFor="runbook-doc-id" hint={t('settings.runbooks.docIdHint')}>
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
        confirmLabel={remove.isPending ? t('settings.runbooks.deleting') : t('settings.runbooks.delete')}
        cancelLabel={t('common.cancel')}
        confirmDisabled={remove.isPending}
      />
    </div>
  );
}
