import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  deleteDocsDocIdAttachmentsAid,
  getGetDocsDocIdQueryKey,
  getGetDocsDocIdAttachmentsQueryKey,
} from '../../api/generated/docs/docs';
import type { DocAttachment } from '../../api/model';
import { attachmentMarkdown } from './attachmentUpload';
import { Button } from '../ui/Button';
import { toast } from '../../lib/toast';

export function AttachmentsPanel({
  docId,
  attachments,
  content,
  editable,
  onInsert,
}: {
  docId: string;
  attachments: DocAttachment[];
  content: string;
  editable: boolean;
  onInsert: (text: string) => void;
}) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const remove = useMutation({
    mutationFn: (id: string) => deleteDocsDocIdAttachmentsAid(docId, id),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: getGetDocsDocIdAttachmentsQueryKey(docId) });
      void client.invalidateQueries({ queryKey: getGetDocsDocIdQueryKey(docId) });
    },
    onError: () => toast.error(t('docs.attachments.deleteError')),
  });
  return (
    <section
      aria-label={t('docs.attachments.title')}
      className="mt-4 rounded border border-line-soft p-4"
    >
      <h2 className="mb-2 font-semibold">{t('docs.attachments.title')}</h2>
      <p className="mb-3 text-xs text-ink-muted">{t('docs.attachments.hint')}</p>
      {!attachments.length && <p className="text-xs">{t('docs.attachments.empty')}</p>}
      <ul className="space-y-2">
        {attachments.map((a) => (
          <li key={a.id} className="flex flex-wrap items-center gap-3 text-sm">
            <a href={a.url} target="_blank" rel="noopener noreferrer">
              {a.filename}
            </a>
            <span className="text-xs text-ink-muted">
              {t('docs.attachments.sizeKb', { size: Math.ceil(a.size / 1024) })}
            </span>
            {!content.includes(`attachment:${a.id}`) && (
              <span className="text-xs">{t('docs.attachments.unused')}</span>
            )}
            <Button
              size="sm"
              variant="secondary"
              disabled={!editable}
              onClick={() => onInsert(attachmentMarkdown(a))}
            >
              {t('docs.attachments.insert')}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={!editable || remove.isPending}
              onClick={() => {
                if (window.confirm(t('docs.attachments.deleteConfirm', { name: a.filename })))
                  remove.mutate(a.id);
              }}
            >
              {t('docs.attachments.delete')}
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
