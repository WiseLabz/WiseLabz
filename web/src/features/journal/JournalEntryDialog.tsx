import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { postJournal, putJournalId } from '../../api/generated/journal/journal';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import { useGetDocsTree } from '../../api/generated/docs/docs';
import type { TimelineItem } from '../../api/model';
import { useIsInstanceAdmin, useOperatorConnectorIds } from '../../hooks/useRole';
import { flattenDocTree } from '../../lib/docTree';
import { toast } from '../../lib/toast';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Markdown } from '../../components/docs/Markdown';
import { EntityPicker } from '../../components/manager/EntityPicker';

const fieldClass = 'w-full rounded-md border border-line-soft bg-canvas px-3 py-2 text-sm text-ink';

function localDatetime(timestamp: string) {
  const date = new Date(timestamp);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 19);
}

export function JournalEntryDialog({
  entry,
  onClose,
}: {
  entry?: TimelineItem;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const admin = useIsInstanceAdmin();
  const operatorIds = useOperatorConnectorIds();
  const { data: connectors = [] } = useGetConnectors();
  const { data: tree } = useGetDocsTree();
  const scopes = connectors.filter((c) => operatorIds.has(c.id) || c.id === entry?.connectorId);
  const [chosenScope, setScope] = useState<string | null>(entry?.connectorId ?? null);
  const scope = chosenScope ?? (admin ? '' : (scopes[0]?.id ?? ''));
  const [body, setBody] = useState(entry?.body ?? '');
  const [occurredAt, setOccurredAt] = useState(
    localDatetime(entry?.timestamp ?? new Date().toISOString())
  );
  const [docId, setDocId] = useState(entry?.docId ?? '');
  const [entityRef, setEntityRef] = useState(entry?.entityRef ?? '');
  const [entityKind, setEntityKind] = useState(entry?.entityKind ?? '');
  const [entityName, setEntityName] = useState(entry?.entityName ?? '');
  const queryClient = useQueryClient();
  const docs = flattenDocTree(tree).filter(
    (d) =>
      !d.branch &&
      d.docId !== 'root' &&
      d.docId !== 'lab' &&
      (d.serviceId ?? '') === scope &&
      (scope || admin || d.origin === 'human')
  );
  const save = useMutation({
    mutationFn: () => {
      const input = {
        body,
        occurredAt:
          entry && occurredAt === localDatetime(entry.timestamp)
            ? entry.timestamp
            : new Date(occurredAt).toISOString(),
        connectorId: scope,
        docId,
        entityRef,
        entityKind,
        entityName,
      };
      return entry ? putJournalId(entry.id, input) : postJournal(input);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['journal-timeline'] });
      onClose();
    },
    onError: () => toast.error(t('journal.saveError')),
  });
  const canSave = entry
    ? scope === entry.connectorId || (scope ? operatorIds.has(scope) : admin)
    : scope
      ? operatorIds.has(scope)
      : admin;
  return (
    <Dialog open onClose={onClose} title={t(entry ? 'journal.editEntry' : 'journal.newEntry')}>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        <label>
          {t('journal.body')}
          <textarea
            className={fieldClass}
            rows={5}
            required
            maxLength={50000}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            autoFocus
          />
        </label>
        {body && (
          <section
            aria-label={t('journal.preview')}
            className="max-h-48 overflow-auto rounded-md bg-canvas-sunken px-3"
          >
            <Markdown source={body} />
          </section>
        )}
        <label>
          {t('journal.occurredAt')}
          <input
            className={fieldClass}
            type="datetime-local"
            step="1"
            required
            value={occurredAt}
            onChange={(e) => setOccurredAt(e.target.value)}
          />
        </label>
        <label>
          {t('journal.scope')}
          <select
            className={fieldClass}
            value={scope}
            onChange={(e) => {
              setScope(e.target.value);
              setDocId('');
              setEntityRef('');
              setEntityKind('');
              setEntityName('');
            }}
          >
            {(admin || entry?.connectorId === '') && <option value="">{t('journal.lab')}</option>}
            {scopes.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        {scope && (
          <EntityPicker
            connectorId={scope}
            value={entityRef}
            onChange={setEntityRef}
            onEntityChange={(entity) => {
              setEntityKind(entity?.kind ?? '');
              setEntityName(entity?.name ?? '');
            }}
          />
        )}
        <div className="grid grid-cols-2 gap-3">
          <label>
            {t('journal.entityKind')}
            <input
              className={fieldClass}
              value={entityKind}
              maxLength={100}
              onChange={(e) => setEntityKind(e.target.value)}
            />
          </label>
          <label>
            {t('journal.entityName')}
            <input
              className={fieldClass}
              value={entityName}
              maxLength={500}
              onChange={(e) => setEntityName(e.target.value)}
            />
          </label>
        </div>
        <label>
          {t('journal.entityRef')}
          <input
            className={fieldClass}
            value={entityRef}
            maxLength={2048}
            onChange={(e) => setEntityRef(e.target.value)}
          />
        </label>
        <label>
          {t('journal.doc')}
          <select className={fieldClass} value={docId} onChange={(e) => setDocId(e.target.value)}>
            <option value="">{t('journal.noDoc')}</option>
            {docs.map((d) => (
              <option key={d.docId} value={d.docId}>
                {d.title}
              </option>
            ))}
          </select>
        </label>
        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            disabled={!body.trim() || !occurredAt || !canSave || save.isPending}
          >
            {t('common.save')}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
