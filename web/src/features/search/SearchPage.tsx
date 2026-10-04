import { useEffect, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useGetSearch } from '../../api/generated/search/search';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import type { GetSearchType } from '../../api/model';
import { EmptyState, ErrorState, SkeletonRows } from '../../components/ui/states';
import { RunbookPanel } from '../../components/runbook/RunbookPanel';

const fieldClass = 'rounded-md border border-line-soft bg-canvas px-3 py-2 text-sm text-ink';

export function SearchPage() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const query = params.get('q') ?? '';
  const [debouncedQuery, setDebouncedQuery] = useState(query.trim());
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query.trim()), 300);
    return () => clearTimeout(timer);
  }, [query]);
  const rawType = params.get('type');
  const type: GetSearchType | undefined =
    rawType === 'doc' || rawType === 'runbook' || rawType === 'entity' ? rawType : undefined;
  const connector = params.get('connector') || undefined;
  const rawKind = params.get('kind') ?? '';
  const kind = rawKind.trim().toLowerCase() || undefined;
  const [debouncedKind, setDebouncedKind] = useState(kind);
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedKind(kind), 300);
    return () => clearTimeout(timer);
  }, [kind]);
  const enabled =
    query.trim().length >= 2 && debouncedQuery === query.trim() && debouncedKind === kind;
  const search = useGetSearch(
    { q: debouncedQuery, type, connector, kind: debouncedKind, limit: 100 },
    {
      query: { enabled },
    }
  );
  const { data: connectors = [] } = useGetConnectors();
  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    next.delete('runbook');
    setParams(next, { replace: true });
  };
  const data = enabled ? search.data : undefined;
  const hasResults = Boolean(
    data && data.docs.length + data.runbooks.length + data.entities.length
  );
  const selectedRunbook = params.get('runbook');
  const runbookRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (selectedRunbook) {
      runbookRef.current?.scrollIntoView({ block: 'start' });
      runbookRef.current?.focus();
    }
  }, [selectedRunbook]);
  return (
    <div className="space-y-5">
      <h1 className="text-xl font-semibold text-ink">{t('search.title')}</h1>
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-1 flex-col gap-1 text-xs text-ink-muted">
          {t('search.query')}
          <input
            className={fieldClass}
            type="search"
            value={query}
            placeholder={t('search.placeholder')}
            onChange={(e) => setFilter('q', e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs text-ink-muted">
          {t('search.type')}
          <select
            className={fieldClass}
            value={type ?? ''}
            onChange={(e) => setFilter('type', e.target.value)}
          >
            <option value="">{t('search.allTypes')}</option>
            <option value="doc">{t('search.docs')}</option>
            <option value="runbook">{t('search.runbooks')}</option>
            <option value="entity">{t('search.entities')}</option>
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs text-ink-muted">
          {t('search.connector')}
          <select
            className={fieldClass}
            value={connector ?? ''}
            onChange={(e) => setFilter('connector', e.target.value)}
          >
            <option value="">{t('search.allConnectors')}</option>
            {connectors.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs text-ink-muted">
          {t('search.kind')}
          <input
            className={fieldClass}
            value={rawKind}
            placeholder={t('search.allKinds')}
            onChange={(e) => setFilter('kind', e.target.value)}
          />
        </label>
      </div>
      {selectedRunbook && (
        <div ref={runbookRef} tabIndex={-1} aria-label={t('search.runbooks')}>
          <RunbookPanel runbookId={selectedRunbook} />
        </div>
      )}
      <div aria-live="polite">
        {query.trim().length < 2 ? (
          <EmptyState title={t('search.prompt')} />
        ) : !enabled || search.isLoading ? (
          <SkeletonRows />
        ) : search.isError ? (
          <ErrorState title={t('search.loadError')} onRetry={() => void search.refetch()} />
        ) : !hasResults ? (
          <EmptyState title={t('search.empty')} />
        ) : (
          <div className="space-y-6">
            {(['docs', 'runbooks'] as const).map(
              (group) =>
                data &&
                data[group].length > 0 && (
                  <section key={group} aria-label={t(`search.${group}`)}>
                    <h2 className="mb-2 text-sm font-semibold text-ink">{t(`search.${group}`)}</h2>
                    <ul className="space-y-2">
                      {data[group].map((hit) => (
                        <li key={hit.id} className="rounded-md border border-line-soft p-3">
                          <Link
                            className="text-sm text-accent-secondary-bright"
                            to={
                              group === 'docs'
                                ? `/docs/${encodeURIComponent(hit.id)}`
                                : `/search?${new URLSearchParams({ ...Object.fromEntries(params), runbook: hit.id })}`
                            }
                          >
                            {hit.title}
                          </Link>
                          <p className="mt-1 text-xs text-ink-muted">{hit.snippet}</p>
                        </li>
                      ))}
                    </ul>
                  </section>
                )
            )}
            {data && data.entities.length > 0 && (
              <section aria-label={t('search.entities')}>
                <h2 className="mb-2 text-sm font-semibold text-ink">{t('search.entities')}</h2>
                <ul className="space-y-2">
                  {data.entities.map((hit, index) => (
                    <li
                      key={`${hit.connectorId}-${index}`}
                      className="rounded-md border border-line-soft p-3"
                    >
                      <Link
                        className="text-sm text-accent-secondary-bright"
                        to={
                          hit.docId
                            ? `/docs/${encodeURIComponent(hit.docId)}`
                            : `/services/${encodeURIComponent(hit.connectorId)}`
                        }
                      >
                        {hit.name}
                      </Link>
                      <p className="mt-1 text-xs text-ink-muted">
                        {[
                          hit.connectorName,
                          hit.kind,
                          hit.externalId,
                          hit.ip,
                          hit.hostname,
                          hit.mac,
                          ...hit.aliases,
                        ]
                          .filter(Boolean)
                          .join(' · ')}
                      </p>
                    </li>
                  ))}
                </ul>
              </section>
            )}
            {data &&
              [data.docs, data.runbooks, data.entities].some((group) => group.length === 100) && (
                <p className="text-xs text-ink-muted">{t('search.limited', { count: 100 })}</p>
              )}
          </div>
        )}
      </div>
    </div>
  );
}
