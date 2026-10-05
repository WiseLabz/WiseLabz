import { Link, useParams } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useGetEntitiesId } from '../../api/generated/search/search';
import { Markdown } from '../../components/docs/Markdown';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { EmptyState, ErrorState, SkeletonRows } from '../../components/ui/states';

const valueText = (value: unknown) =>
  value === undefined ? '—' : typeof value === 'string' ? value : JSON.stringify(value);

export function EntityDetailPage() {
  const { t } = useTranslation();
  const { id = '' } = useParams();
  const entity = useGetEntitiesId(id);

  if (entity.isLoading) return <Panel className="p-5"><SkeletonRows rows={5} /></Panel>;
  if (entity.isError || !entity.data) {
    return <Panel className="min-h-[40vh]"><ErrorState title={t(entity.isError ? 'entities.loadError' : 'entities.notFound')} onRetry={() => void entity.refetch()} /></Panel>;
  }

  const data = entity.data;
  const section = (title: string, children: ReactNode) => (
    <Panel>
      <PanelHeader title={title} />
      <div className="space-y-3 p-4">{children}</div>
    </Panel>
  );

  return (
    <div className="mx-auto max-w-275 space-y-5 px-6 py-6">
      <Link className="text-xs text-ink-muted hover:text-ink" to="/search">← {t('search.title')}</Link>
      <header>
        <p className="font-mono text-xs text-ink-muted">{data.kind}</p>
        <h1 className="text-xl font-semibold text-ink">{data.name}</h1>
      </header>
      {data.gone && <p role="status" className="rounded-md border border-warn/30 bg-warn-tint px-4 py-3 text-sm text-warn">{t('entities.gone')}</p>}
      <div className="grid gap-4 lg:grid-cols-2">
        {section(t('entities.members'), data.members.length ? data.members.map((m) => (
          <div key={`${m.connectorId}:${m.kind}:${m.ref}`} className="flex items-center justify-between gap-3 text-sm">
            <span className="text-ink">{m.name} <span className="font-mono text-xs text-ink-muted">· {m.kind}</span></span>
            <Link className="font-mono text-xs text-accent-secondary-bright" to={`/services/${encodeURIComponent(m.connectorId)}`}>{m.connectorId}</Link>
          </div>
        )) : <EmptyState title={t('entities.empty')} />)}
        {section(t('entities.relatedByIp'), data.relatedByIp.length ? data.relatedByIp.map((edge, i) => (
          <p key={`${edge.from.connectorId}:${edge.to.connectorId}:${i}`} className="text-sm text-ink">{edge.from.name} ↔ {edge.to.name} <span className="text-xs text-ink-muted">· {edge.reason}</span></p>
        )) : <EmptyState title={t('entities.empty')} />)}
        {section(t('entities.neighbors'), data.neighbors.length ? data.neighbors.map((edge, i) => (
          <p key={`${edge.kind}:${edge.from.connectorId}:${edge.to.connectorId}:${i}`} className="text-sm text-ink">{edge.from.name} → {edge.to.name} <span className="font-mono text-xs text-ink-muted">· {edge.kind}</span></p>
        )) : <EmptyState title={t('entities.empty')} />)}
        {section(t('entities.history'), data.history.length ? data.history.map((item, i) => (
          <div key={`${item.at}:${item.key}:${item.field}:${i}`} className="border-b border-line-soft pb-2 text-sm last:border-0">
            <p className="text-ink"><span className="font-mono">{item.field}</span> · {item.change}</p>
            <p className="mt-1 text-xs text-ink-muted">{item.at} · {valueText(item.old)} → {valueText(item.new)}</p>
          </div>
        )) : <EmptyState title={t('entities.empty')} />)}
        {section(t('entities.findings'), data.findings.length ? data.findings.map((finding) => (
          <div key={finding.id} className="text-sm"><p className="text-ink">{finding.title}</p><p className="text-xs text-ink-muted">{finding.severity} · {finding.status}</p></div>
        )) : <EmptyState title={t('entities.empty')} />)}
        {section(t('entities.connectorFindings'), data.onReportingConnectors.length ? data.onReportingConnectors.map((finding) => (
          <div key={finding.id} className="text-sm"><p className="text-ink">{finding.title}</p><p className="text-xs text-ink-muted">{finding.severity} · {finding.status}</p></div>
        )) : <EmptyState title={t('entities.empty')} />)}
        {section(t('entities.runbooks'), data.runbooks.length ? data.runbooks.map((runbook) => (
          <article key={`${runbook.id}:${runbook.step.id}`}><h3 className="text-sm font-medium text-ink">{runbook.title}</h3><p className="text-xs text-ink-muted">{runbook.step.title} · {runbook.step.verb}</p><Markdown source={runbook.body} /></article>
        )) : <EmptyState title={t('entities.empty')} />)}
      </div>
    </div>
  );
}
