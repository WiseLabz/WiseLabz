/**
 * Settings → Audit log (operator-only). Paginated viewer over the audit
 * trail (docs/AUDIT.md), with action/targetType/date-range filters and a
 * CSV/JSON export that streams straight from GET /system/audit/export.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useGetSystemAudit } from '../../api/generated/system/system';
import { GetSystemAuditExportFormat } from '../../api/model';
import { AXIOS_INSTANCE } from '../../api/axios-instance';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { toast } from '../../lib/toast';
import { SubHeader, Section, Field, TextInput, Select } from './parts';
import { HistoryIcon } from '../../components/icons';

const PAGE_SIZE = 25;

/** Reads a filename from a Content-Disposition header, falling back to a default. */
function filenameFromContentDisposition(header: string | undefined, fallback: string): string {
  const match = header?.match(/filename="?([^"；;]+)"?/);
  return match?.[1] ?? fallback;
}

export function AuditPage() {
  const { t } = useTranslation();

  const [page, setPage] = useState(1);
  const [action, setAction] = useState('');
  const [targetType, setTargetType] = useState('');
  const [createdAfter, setCreatedAfter] = useState('');
  const [createdBefore, setCreatedBefore] = useState('');
  const [format, setFormat] = useState<GetSystemAuditExportFormat>(GetSystemAuditExportFormat.json);
  const [exporting, setExporting] = useState(false);

  const filters = {
    action: action || undefined,
    targetType: targetType || undefined,
    createdAfter: createdAfter ? new Date(createdAfter).toISOString() : undefined,
    createdBefore: createdBefore ? new Date(createdBefore).toISOString() : undefined,
  };

  const { data, isLoading, isError, refetch } = useGetSystemAudit({
    ...filters,
    page,
    pageSize: PAGE_SIZE,
  });

  const handleExport = async () => {
    setExporting(true);
    try {
      const res = await AXIOS_INSTANCE.get('/system/audit/export', {
        params: { ...filters, format },
        responseType: 'blob',
      });
      const filename = filenameFromContentDisposition(
        res.headers['content-disposition'] as string | undefined,
        `wiselabz-audit.${format}`
      );
      const url = URL.createObjectURL(res.data as Blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch {
      toast.error(t('settings.audit.exportError'));
    } finally {
      setExporting(false);
    }
  };

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;

  return (
    <div>
      <SubHeader title={t('settings.audit.title')} description={t('settings.audit.subtitle')} />

      <Section title={t('settings.audit.filtersTitle')}>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Field label={t('settings.audit.action')} htmlFor="audit-action">
            <TextInput
              id="audit-action"
              placeholder={t('settings.audit.anyAction')}
              value={action}
              onChange={(e) => {
                setAction(e.target.value);
                setPage(1);
              }}
            />
          </Field>
          <Field label={t('settings.audit.targetType')} htmlFor="audit-target-type">
            <TextInput
              id="audit-target-type"
              placeholder={t('settings.audit.anyTargetType')}
              value={targetType}
              onChange={(e) => {
                setTargetType(e.target.value);
                setPage(1);
              }}
            />
          </Field>
          <Field label={t('settings.audit.createdAfter')} htmlFor="audit-created-after">
            <TextInput
              id="audit-created-after"
              type="date"
              value={createdAfter}
              onChange={(e) => {
                setCreatedAfter(e.target.value);
                setPage(1);
              }}
            />
          </Field>
          <Field label={t('settings.audit.createdBefore')} htmlFor="audit-created-before">
            <TextInput
              id="audit-created-before"
              type="date"
              value={createdBefore}
              onChange={(e) => {
                setCreatedBefore(e.target.value);
                setPage(1);
              }}
            />
          </Field>
        </div>
      </Section>

      <Panel>
        <PanelHeader
          title={t('settings.audit.listTitle')}
          icon={<HistoryIcon size={14} />}
          count={data?.total}
          action={
            <div className="flex items-center gap-2">
              <Select
                aria-label={t('settings.audit.exportFormat')}
                value={format}
                onChange={(e) => setFormat(e.target.value as GetSystemAuditExportFormat)}
                className="!w-auto"
              >
                <option value={GetSystemAuditExportFormat.json}>JSON</option>
                <option value={GetSystemAuditExportFormat.csv}>CSV</option>
              </Select>
              <Button
                variant="secondary"
                size="sm"
                disabled={exporting}
                onClick={() => void handleExport()}
              >
                {t('settings.audit.exportButton')}
              </Button>
            </div>
          }
        />

        {isLoading ? (
          <SkeletonRows rows={8} />
        ) : isError || !data ? (
          <ErrorState description={t('settings.audit.loadError')} onRetry={() => refetch()} />
        ) : data.items.length === 0 ? (
          <EmptyState title={t('settings.audit.empty')} />
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-line-soft text-2xs font-mono text-ink-faint">
                    <th className="px-4 py-2 font-normal">{t('settings.audit.columnAction')}</th>
                    <th className="px-4 py-2 font-normal">{t('settings.audit.columnTargetType')}</th>
                    <th className="px-4 py-2 font-normal">{t('settings.audit.columnActor')}</th>
                    <th className="px-4 py-2 font-normal">{t('settings.audit.columnCreatedAt')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line-soft">
                  {data.items.map((r) => (
                    <tr key={r.id}>
                      <td className="px-4 py-2 font-mono text-ink">{r.action}</td>
                      <td className="px-4 py-2 font-mono text-ink-muted">
                        {r.targetType || '—'}
                        {r.targetId ? `/${r.targetId}` : ''}
                      </td>
                      <td className="px-4 py-2 text-ink-muted">
                        {r.actorUserId || '—'} ({r.actorRole})
                      </td>
                      <td className="px-4 py-2 font-mono text-2xs text-ink-faint">
                        {new Date(r.createdAt).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="flex items-center justify-between gap-3 border-t border-line-soft px-4 py-2.5">
              <span className="font-mono text-2xs text-ink-faint">
                {page} / {totalPages}
              </span>
              <div className="flex gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  {t('common.previous')}
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                >
                  {t('common.next')}
                </Button>
              </div>
            </div>
          </>
        )}
      </Panel>
    </div>
  );
}
