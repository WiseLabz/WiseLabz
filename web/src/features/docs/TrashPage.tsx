import { Link } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useGetDocsTrash, postDocsDocIdRestore } from '../../api/generated/docs/docs';
import { Panel } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { ErrorState, SkeletonRows } from '../../components/ui/states';
import { fullDate } from '../../lib/time';
import { toast } from '../../lib/toast';

export function TrashPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const trash = useGetDocsTrash();
  const restore = useMutation({
    mutationFn: (id: string) => postDocsDocIdRestore(id),
    onSuccess: () => {
      void queryClient.invalidateQueries();
    },
    onError: () =>
      toast.error(t('docs.human.restoreError', { defaultValue: 'Could not restore doc' })),
  });
  return (
    <div className="mx-auto max-w-205 px-6 py-6">
      <h1 className="mb-2 text-xl font-semibold">
        {t('docs.human.trash', { defaultValue: 'Trash' })}
      </h1>
      <p className="mb-4 text-sm text-ink-muted">
        {t('docs.human.trashDescription', {
          defaultValue:
            'Deleted docs are kept for 30 days by default. Restoring a parent restores its deletion batch.',
        })}
      </p>
      <Link to="/docs">{t('docs.human.back', { defaultValue: 'Back to docs' })}</Link>
      <Panel className="mt-4">
        {trash.isLoading ? (
          <SkeletonRows rows={5} />
        ) : trash.isError ? (
          <ErrorState onRetry={() => trash.refetch()} />
        ) : !trash.data?.length ? (
          <p className="p-4">{t('docs.human.emptyTrash', { defaultValue: 'Trash is empty' })}</p>
        ) : (
          <ul>
            {trash.data.map((doc) => (
              <li
                key={doc.docId}
                className="flex items-center justify-between gap-3 border-b border-line-soft p-4"
              >
                <div>
                  <p>{doc.title}</p>
                  <p className="text-xs text-ink-faint">{fullDate(doc.deletedAt ?? '')}</p>
                </div>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={restore.isPending}
                  onClick={() => restore.mutate(doc.docId)}
                >
                  {t('docs.human.restore', { defaultValue: 'Restore' })}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </Panel>
    </div>
  );
}
