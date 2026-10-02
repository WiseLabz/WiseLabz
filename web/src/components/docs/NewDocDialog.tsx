import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import {
  postDocs,
  patchDocsDocId,
  useGetDocsTree,
  getGetDocsQueryKey,
  getGetDocsTreeQueryKey,
} from '../../api/generated/docs/docs';
import type { Doc } from '../../api/model';
import { useIsInstanceAdmin, useOperatorConnectorIds } from '../../hooks/useRole';
import { flattenDocTree } from '../../lib/docTree';
import { Dialog } from '../ui/Dialog';
import { Button } from '../ui/Button';
import { toast } from '../../lib/toast';

const fieldClass = 'w-full rounded-md border border-line-soft bg-canvas px-3 py-2 text-sm text-ink';

export function NewDocDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return open ? <NewDocForm onClose={onClose} /> : null;
}

function NewDocForm({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const admin = useIsInstanceAdmin();
  const operatorIds = useOperatorConnectorIds();
  const { data: connectors = [] } = useGetConnectors();
  const { data: tree } = useGetDocsTree();
  const scopes = connectors.filter((c) => operatorIds.has(c.id));
  const [chosenScope, setScope] = useState<string | null>(null);
  const scope = chosenScope ?? (admin ? '' : (scopes[0]?.id ?? ''));
  const [title, setTitle] = useState('');
  const [parentId, setParentId] = useState('');
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const parents = flattenDocTree(tree).filter(
    (n) =>
      !n.branch &&
      n.docId !== 'root' &&
      n.docId !== 'lab' &&
      (n.serviceId ?? '') === scope &&
      (scope || n.origin === 'human')
  );
  const create = useMutation({
    mutationFn: () => postDocs({ title: title.trim(), serviceId: scope, parentId }),
    onSuccess: (doc) => {
      void queryClient.invalidateQueries({ queryKey: getGetDocsQueryKey() });
      void queryClient.invalidateQueries({ queryKey: getGetDocsTreeQueryKey() });
      onClose();
      navigate(`/docs/${doc.docId}/edit`);
    },
    onError: () =>
      toast.error(
        t('docs.human.createError', {
          defaultValue: 'Could not create doc. Check its scope and parent.',
        })
      ),
  });
  return (
    <Dialog open onClose={onClose} title={t('docs.human.new', { defaultValue: 'New doc' })}>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <label>
          {t('docs.human.title', { defaultValue: 'Title' })}
          <input
            className={fieldClass}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            required
            maxLength={500}
            autoFocus
          />
        </label>
        <label>
          {t('docs.human.scope', { defaultValue: 'Scope' })}
          <select
            className={fieldClass}
            value={scope}
            onChange={(e) => {
              setScope(e.target.value);
              setParentId('');
            }}
          >
            {admin && <option value="">Lab</option>}
            {scopes.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          {t('docs.human.parent', { defaultValue: 'Parent' })}
          <select
            className={fieldClass}
            value={parentId}
            onChange={(e) => setParentId(e.target.value)}
          >
            <option value="">{t('docs.human.root', { defaultValue: 'Scope root' })}</option>
            {parents.map((n) => (
              <option key={n.docId} value={n.docId}>
                {n.title}
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
            disabled={!title.trim() || create.isPending || (!admin && !operatorIds.has(scope))}
          >
            {t('docs.human.create', { defaultValue: 'Create doc' })}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export function DocMetadataDialog({ doc, onClose }: { doc: Doc; onClose: () => void }) {
  const { t } = useTranslation();
  const [title, setTitle] = useState(doc.title);
  const [parentId, setParentId] = useState(doc.parentId ?? '');
  const { data: tree } = useGetDocsTree();
  const queryClient = useQueryClient();
  const nodes = flattenDocTree(tree);
  const subtree = nodes.find((n) => n.docId === doc.docId);
  const excluded = new Set(flattenDocTree(subtree).map((n) => n.docId));
  const parents = nodes.filter(
    (n) =>
      !n.branch &&
      n.docId !== 'root' &&
      n.docId !== 'lab' &&
      !excluded.has(n.docId) &&
      (n.serviceId ?? '') === (doc.serviceId ?? '') &&
      (doc.serviceId || doc.origin !== 'human' || n.origin === 'human')
  );
  const update = useMutation({
    mutationFn: () => patchDocsDocId(doc.docId, { title: title.trim(), parentId }),
    onSuccess: () => {
      void queryClient.invalidateQueries();
      onClose();
    },
    onError: () =>
      toast.error(
        t('docs.human.moveError', {
          defaultValue:
            'Could not move doc. Parents must share its scope and depth cannot exceed five.',
        })
      ),
  });
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('docs.human.organize', { defaultValue: 'Rename or move doc' })}
    >
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          update.mutate();
        }}
      >
        <label>
          {t('docs.human.title', { defaultValue: 'Title' })}
          <input
            className={fieldClass}
            required
            maxLength={500}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
        </label>
        <label>
          {t('docs.human.parent', { defaultValue: 'Parent' })}
          <select
            className={fieldClass}
            value={parentId}
            onChange={(e) => setParentId(e.target.value)}
          >
            <option value="">{t('docs.human.root', { defaultValue: 'Scope root' })}</option>
            {parents.map((n) => (
              <option key={n.docId} value={n.docId}>
                {n.title}
              </option>
            ))}
          </select>
        </label>
        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" disabled={!title.trim() || update.isPending}>
            {t('common.save', { defaultValue: 'Save' })}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
