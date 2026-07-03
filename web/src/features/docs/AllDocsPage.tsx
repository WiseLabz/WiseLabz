/** All Docs — flat, searchable list of every document in the lab, for when
 *  the tree in DocsPage isn't the fastest way to find one. */
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { useGetDocs } from '../../api/generated/docs/docs';
import { Panel } from '../../components/ui/Panel';
import { Pagination } from '../../components/ui/Pagination';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { relativeTime } from '../../lib/time';
import { SearchIcon, FileTextIcon } from '../../components/icons';

export function AllDocsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const pageSize = 20;

  const [q, setQ] = useState('');
  const [search, setSearch] = useState('');
  // ponytail: single-use debounce, promote to a hook if a second search box shows up
  useEffect(() => {
    const id = setTimeout(() => setSearch(q), 300);
    return () => clearTimeout(id);
  }, [q]);

  const { data, isLoading, isError, refetch } = useGetDocs({ page, pageSize, search });
  const pageCount = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;

  return (
    <div className="mx-auto max-w-205 px-6 py-6">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-ink">{t('docs.allDocs.title')}</h1>
          <p className="text-sm text-ink-muted">{t('docs.allDocs.subtitle')}</p>
        </div>
        <div className="flex h-9 items-center gap-2 rounded-md border border-line-soft bg-surface px-2.5">
          <SearchIcon size={15} className="text-ink-faint" />
          <input
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setPage(1);
            }}
            placeholder={t('docs.allDocs.searchPlaceholder')}
            className="w-56 bg-transparent text-sm text-ink outline-none placeholder:text-ink-faint"
          />
        </div>
      </header>

      {isLoading ? (
        <Panel>
          <SkeletonRows rows={6} />
        </Panel>
      ) : isError || !data ? (
        <Panel className="min-h-[40vh]">
          <ErrorState description={t('docs.allDocs.loadError')} onRetry={() => refetch()} />
        </Panel>
      ) : data.items.length === 0 ? (
        <Panel className="min-h-[40vh]">
          <EmptyState
            icon={<FileTextIcon size={20} />}
            title={t('docs.allDocs.noMatchTitle')}
            description={t('docs.allDocs.noMatchDesc')}
          />
        </Panel>
      ) : (
        <div className="flex flex-col gap-3">
          <Panel>
            <ul>
              {data.items.map((doc) => (
                <li key={doc.docId} className="border-b border-line-soft last:border-0">
                  <button
                    onClick={() => navigate(`/docs/${doc.docId}`)}
                    className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-raised"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-ink">{doc.title}</p>
                      <p className="font-mono text-2xs text-ink-faint">{doc.kind}</p>
                    </div>
                    <span className="shrink-0 text-2xs text-ink-faint">
                      {t('common.ago', { time: relativeTime(doc.updatedAt) })}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </Panel>
          {pageCount > 1 && (
            <Pagination page={page} pageCount={pageCount} onPage={setPage} className="justify-center pt-1" />
          )}
        </div>
      )}
    </div>
  );
}
