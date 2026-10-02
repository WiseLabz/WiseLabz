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
  const client = useQueryClient();
  const remove = useMutation({
    mutationFn: (id: string) => deleteDocsDocIdAttachmentsAid(docId, id),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: getGetDocsDocIdAttachmentsQueryKey(docId) });
      void client.invalidateQueries({ queryKey: getGetDocsDocIdQueryKey(docId) });
    },
    onError: () => toast.error('Could not delete attachment'),
  });
  return (
    <section aria-label="Attachments" className="mt-4 rounded border border-line-soft p-4">
      <h2 className="mb-2 font-semibold">Attachments</h2>
      <p className="mb-3 text-xs text-ink-muted">
        Drop or paste images, PDFs or text in the editor to upload.
      </p>
      {!attachments.length && <p className="text-xs">No attachments yet.</p>}
      <ul className="space-y-2">
        {attachments.map((a) => (
          <li key={a.id} className="flex flex-wrap items-center gap-3 text-sm">
            <a href={a.url} target="_blank" rel="noopener noreferrer">
              {a.filename}
            </a>
            <span className="text-xs text-ink-muted">{Math.ceil(a.size / 1024)} KB</span>
            {!content.includes(`attachment:${a.id}`) && <span className="text-xs">Unused</span>}
            <Button
              size="sm"
              variant="secondary"
              disabled={!editable}
              onClick={() => onInsert(attachmentMarkdown(a))}
            >
              Insert
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={!editable || remove.isPending}
              onClick={() => {
                if (window.confirm(`Delete ${a.filename}? Links to it will stop working.`))
                  remove.mutate(a.id);
              }}
            >
              Delete
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
