/** Doc edit proposals — pending doc changes suggested through MCP
 *  (`propose_doc_edit`), awaiting approval by a doc operator. The server only
 *  returns proposals the caller may review. */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import {
  getGetDocsEditProposalsQueryKey,
  postDocsEditProposalsProposalIdApprove,
  useGetDocsEditProposalsProposalId,
  postDocsEditProposalsProposalIdReject,
  useGetDocsEditProposals,
} from '../../api/generated/docs/docs';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Pagination } from '../../components/ui/Pagination';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { relativeTime } from '../../lib/time';
import { toast } from '../../lib/toast';
import { FileTextIcon } from '../../components/icons';

/** Proposed body, fetched on demand: the list endpoint omits `content` so a
 *  long queue of large proposals stays cheap to page through. */
function ProposalContent({ id }: { id: string }) {
  const { t } = useTranslation();
  const { data, isLoading, isError } = useGetDocsEditProposalsProposalId(id);
  if (isLoading) return <p className="mt-2 text-xs text-ink-muted">{t('docs.proposals.loadingContent')}</p>;
  if (isError || !data) return <p className="mt-2 text-xs text-err">{t('docs.proposals.contentError')}</p>;
  return (
    <pre className="mt-2 max-h-80 overflow-auto whitespace-pre-wrap rounded-md border border-line-soft bg-surface p-3 font-mono text-xs text-ink">
      {data.content}
    </pre>
  );
}

export function DocProposalsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [open, setOpen] = useState<Record<string, boolean>>({});
  const pageSize = 20;

  const { data, isLoading, isError, refetch } = useGetDocsEditProposals({ page, pageSize });
  const pageCount = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;
  const refresh = () => queryClient.invalidateQueries({ queryKey: getGetDocsEditProposalsQueryKey() });

  const approve = useMutation({
    mutationFn: (id: string) => postDocsEditProposalsProposalIdApprove(id),
    onSuccess: () => {
      toast.success(t('docs.proposals.approved'));
      refresh();
    },
    onError: (error) => {
      if (isAxiosError(error) && error.response?.status === 409) {
        // A Doc body means the doc moved on since the proposal's base version
        // (the proposal stays pending); an error body means it was already reviewed.
        const stale = typeof error.response.data?.docId === 'string';
        toast.error(t(stale ? 'docs.proposals.conflict' : 'docs.proposals.alreadyReviewed'));
        refresh();
        return;
      }
      toast.error(t('docs.proposals.approveError'));
    },
  });

  const reject = useMutation({
    mutationFn: (id: string) => postDocsEditProposalsProposalIdReject(id),
    onSuccess: () => {
      toast.success(t('docs.proposals.rejected'));
      refresh();
    },
    onError: (error) => {
      if (isAxiosError(error) && error.response?.status === 409) {
        toast.error(t('docs.proposals.alreadyReviewed'));
        refresh();
        return;
      }
      toast.error(t('docs.proposals.rejectError'));
    },
  });

  const busy = approve.isPending || reject.isPending;

  return (
    <div className="mx-auto max-w-205 px-6 py-6">
      <header className="mb-5">
        <h1 className="text-xl font-semibold tracking-tight text-ink">{t('docs.proposals.title')}</h1>
        <p className="text-sm text-ink-muted">{t('docs.proposals.subtitle')}</p>
      </header>

      {isLoading ? (
        <Panel>
          <SkeletonRows rows={4} />
        </Panel>
      ) : isError || !data ? (
        <Panel className="min-h-[40vh]">
          <ErrorState description={t('docs.proposals.loadError')} onRetry={() => refetch()} />
        </Panel>
      ) : data.items.length === 0 ? (
        <Panel className="min-h-[40vh]">
          <EmptyState
            icon={<FileTextIcon size={20} />}
            title={t('docs.proposals.emptyTitle')}
            description={t('docs.proposals.emptyDesc')}
          />
        </Panel>
      ) : (
        <div className="flex flex-col gap-3">
          {data.items.map((p) => (
            <Panel key={p.id} className="p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <Link to={`/docs/${p.docId}`} className="truncate text-sm font-medium text-ink hover:underline">
                    {p.docTitle}
                  </Link>
                  <p className="font-mono text-2xs text-ink-faint">
                    {t('docs.proposals.meta', { version: p.baseVersion, time: relativeTime(p.createdAt) })}
                  </p>
                  {p.summary && <p className="mt-1 text-sm text-ink-muted">{p.summary}</p>}
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button size="sm" variant="primary" disabled={busy} onClick={() => approve.mutate(p.id)}>
                    {t('docs.proposals.approve')}
                  </Button>
                  <Button size="sm" variant="danger" disabled={busy} onClick={() => reject.mutate(p.id)}>
                    {t('docs.proposals.reject')}
                  </Button>
                </div>
              </div>
              <details
                className="mt-3"
                onToggle={(e) => setOpen((o) => ({ ...o, [p.id]: e.currentTarget.open }))}
              >
                <summary className="cursor-pointer text-xs text-ink-muted">{t('docs.proposals.viewContent')}</summary>
                {open[p.id] && <ProposalContent id={p.id} />}
              </details>
            </Panel>
          ))}
          {pageCount > 1 && (
            <Pagination page={page} pageCount={pageCount} onPage={setPage} className="justify-center pt-1" />
          )}
        </div>
      )}
    </div>
  );
}
