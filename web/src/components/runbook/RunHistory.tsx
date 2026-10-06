import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useListRunbookRuns } from '../../api/generated/runbooks/runbooks';
import type { RunbookRun } from '../../api/model';
import { Button } from '../ui/Button';
import { Pagination } from '../ui/Pagination';
import { Panel, PanelHeader } from '../ui/Panel';
import { Skeleton } from '../ui/states';
import { HistoryIcon } from '../icons';
import { RunStateTag } from './RunDetail';

type RunT = ReturnType<typeof useTranslation>['t'];

const PAGE_SIZE = 10;

export function RunHistory({
  runbookId,
  onSelectRun,
}: {
  runbookId: string;
  onSelectRun: (id: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const [pagination, setPagination] = useState({ runbookId, page: 1 });
  const page = pagination.runbookId === runbookId ? pagination.page : 1;
  const { data, isLoading, isError, refetch } = useListRunbookRuns(runbookId, {
    page,
    pageSize: PAGE_SIZE,
  });

  const pageCount = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;

  return (
    <section aria-label={t('runbooks.runs.historyTitle')}>
      <Panel>
        <PanelHeader
          title={t('runbooks.runs.historyTitle')}
          icon={<HistoryIcon size={14} />}
          count={data?.total}
        />

        {isLoading && (
          <div
            role="status"
            aria-label={t('runbooks.runs.loadingHistory')}
            className="space-y-3 p-4"
          >
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        )}

        {isError && (
          <div role="alert" className="space-y-3 p-4">
            <p className="text-sm text-err">{t('runbooks.runs.historyLoadError')}</p>
            <Button size="sm" onClick={() => void refetch()}>
              {t('runbooks.runs.retry')}
            </Button>
          </div>
        )}

        {data && data.items.length === 0 && (
          <p className="p-4 text-sm text-ink-muted">{t('runbooks.runs.historyEmpty')}</p>
        )}

        {data && data.items.length > 0 && (
          <>
            <ol className="divide-y divide-line-soft" aria-label={t('runbooks.runs.historyTitle')}>
              {data.items.map((run) => (
                <HistoryRow
                  key={run.id}
                  run={run}
                  language={i18n.language}
                  onSelectRun={onSelectRun}
                  t={t}
                />
              ))}
            </ol>
            {pageCount > 1 && (
              <div className="flex justify-end border-t border-line-soft px-3 py-2">
                <Pagination
                  page={page}
                  pageCount={pageCount}
                  onPage={(nextPage) => setPagination({ runbookId, page: nextPage })}
                  prevLabel={t('runbooks.runs.previousPage')}
                  nextLabel={t('runbooks.runs.nextPage')}
                />
              </div>
            )}
          </>
        )}
      </Panel>
    </section>
  );
}

function HistoryRow({
  run,
  language,
  onSelectRun,
  t,
}: {
  run: RunbookRun;
  language: string;
  onSelectRun: (id: string) => void;
  t: RunT;
}) {
  const href = `/runbook-runs/${encodeURIComponent(run.id)}`;
  return (
    <li>
      <a
        href={href}
        onClick={(event) => {
          if (
            event.defaultPrevented ||
            event.button !== 0 ||
            event.metaKey ||
            event.ctrlKey ||
            event.shiftKey ||
            event.altKey
          ) {
            return;
          }
          event.preventDefault();
          onSelectRun(run.id);
        }}
        className="flex items-center justify-between gap-3 px-4 py-3 transition-colors hover:bg-surface-raised/50 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent-primary"
      >
        <span className="min-w-0">
          <span className="block truncate text-sm font-medium text-ink">{run.runbookTitle}</span>
          <span className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-2xs text-ink-faint">
            <code>{run.id}</code>
            <time dateTime={run.startedAt}>{formatTimestamp(run.startedAt, language)}</time>
          </span>
        </span>
        <span aria-live="polite" aria-atomic="true" className="shrink-0">
          <RunStateTag state={run.state} label={t(`runbooks.runs.status.${run.state}`)} />
        </span>
      </a>
    </li>
  );
}

function formatTimestamp(value: string, language: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat(language, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}
