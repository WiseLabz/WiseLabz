import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  getDocsDocIdBacklinks,
  getGetDocsDocIdBacklinksQueryKey,
} from '../../api/generated/docs/docs';
import {
  getEntitiesIdBacklinks,
  getGetEntitiesIdBacklinksQueryKey,
} from '../../api/generated/search/search';
import { Panel, PanelHeader } from '../ui/Panel';
import { EmptyState, ErrorState, SkeletonRows } from '../ui/states';
import { FileTextIcon } from '../icons';

export function ReferencedByPanel({ type, id }: { type: 'docs' | 'entities'; id: string }) {
  const { t } = useTranslation();
  const query = useQuery({
    queryKey:
      type === 'docs'
        ? getGetDocsDocIdBacklinksQueryKey(id)
        : getGetEntitiesIdBacklinksQueryKey(id),
    queryFn: () => (type === 'docs' ? getDocsDocIdBacklinks(id) : getEntitiesIdBacklinks(id)),
    enabled: !!id,
  });

  return (
    <Panel className="mt-6">
      <PanelHeader title={t('docs.referencedBy.title')} />
      <div className="p-4">
        {query.isLoading ? (
          <SkeletonRows rows={2} />
        ) : query.isError ? (
          <ErrorState
            description={t('docs.referencedBy.loadError')}
            onRetry={() => void query.refetch()}
          />
        ) : query.data?.length ? (
          <ul className="space-y-2">
            {query.data.map((doc) => (
              <li key={doc.id}>
                <Link
                  to={`/docs/${encodeURIComponent(doc.id)}`}
                  className="inline-flex items-center gap-2 text-sm text-accent-primary underline"
                >
                  <FileTextIcon size={14} /> {doc.title}
                </Link>
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState title={t('docs.referencedBy.empty')} />
        )}
      </div>
    </Panel>
  );
}
