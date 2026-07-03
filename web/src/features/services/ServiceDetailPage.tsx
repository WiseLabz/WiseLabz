/**
 * Service detail (`/services/:id`) — the "live picture + operate on it" surface for
 * one connector. Four panels: live raw-data snapshot (status + last sync), the
 * linked service doc, service-scoped change history, and a recent activity timeline
 * fed by the live store. Operator controls (sync / enable-disable / edit / remove)
 * are inline and role-gated; the destructive remove runs the full blast-radius +
 * step-up + type-to-confirm flow via <ConfirmDestructive/>.
 */
import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetConnectorsConnectorId,
  useGetConnectorsConnectorIdData,
  putConnectorsConnectorIdEnabled,
  getGetConnectorsQueryKey,
} from '../../api/generated/connectors/connectors';
import { useGetChanges } from '../../api/generated/changes/changes';
import {
  useGetDocsServiceConnectorId,
  getGetDocsServiceConnectorIdQueryKey,
  postDocsGenerate,
} from '../../api/generated/docs/docs';
import { useGetTemplates } from '../../api/generated/templates/templates';
import { useLive } from '../../store/live';
import { useCanMutate } from '../../hooks/useRole';
import { runSync } from '../../lib/runSync';
import { relativeTime } from '../../lib/time';
import { StatusPill, SeverityTag } from '../../components/ui/StatusDot';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Dialog } from '../../components/ui/Dialog';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { Markdown } from '../../components/docs/Markdown';
import { ConfirmDestructive } from '../../components/manager/ConfirmDestructive';
import {
  ArrowRightIcon,
  SyncIcon,
  FileTextIcon,
  DiffIcon,
  SparklesIcon,
  ChevronDownIcon,
} from '../../components/icons';
import type { ServiceStatus } from '../../api/model';

export function ServiceDetailPage() {
  const { t } = useTranslation();
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canMutate = useCanMutate();

  const connector = useGetConnectorsConnectorId(id);
  const overrides = useLive((s) => s.statusOverrides);
  const activity = useLive((s) => s.activity);
  const [removing, setRemoving] = useState(false);

  const toggleEnabled = useMutation({
    mutationFn: (enabled: boolean) => putConnectorsConnectorIdEnabled(id, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetConnectorsQueryKey() });
      void connector.refetch();
    },
  });

  if (connector.isLoading) {
    return (
      <div className="mx-auto max-w-275 px-6 py-6">
        <Panel className="p-6">
          <SkeletonRows rows={6} />
        </Panel>
      </div>
    );
  }
  if (connector.isError || !connector.data) {
    return (
      <div className="mx-auto max-w-275 px-6 py-6">
        <Panel className="min-h-[40vh]">
          <ErrorState
            description={t('services.detail.notFound')}
            onRetry={() => connector.refetch()}
          />
        </Panel>
      </div>
    );
  }

  const c = connector.data;
  const status = (overrides[c.id] ?? c.status) as ServiceStatus;

  return (
    <div className="mx-auto max-w-275 px-6 py-6">
      <button
        onClick={() => navigate('/services')}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('services.detail.back')}
      </button>

      {/* Header + operator controls */}
      <header className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold tracking-tight text-ink">{c.name}</h1>
            <StatusPill status={status} />
          </div>
          <p className="mt-1 font-mono text-2xs text-ink-faint">
            {c.type} · {c.url?.replace(/^https?:\/\//, '')} ·{' '}
            {t('services.detail.lastSync', { time: relativeTime(c.lastSyncAt) })}
          </p>
        </div>
        {canMutate && (
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => runSync(c.id)}
              disabled={!c.enabled}
            >
              <SyncIcon size={14} /> {t('common.sync')}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => toggleEnabled.mutate(!c.enabled)}
              disabled={toggleEnabled.isPending}
            >
              {c.enabled ? t('common.disable') : t('common.enable')}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => navigate(`/connectors/${c.id}/edit`)}>
              {t('common.edit')}
            </Button>
            <Button size="sm" variant="danger" onClick={() => setRemoving(true)}>
              {t('common.remove')}
            </Button>
          </div>
        )}
      </header>

      <div className="grid gap-4 lg:grid-cols-[1fr_320px]">
        <div className="space-y-4">
          <SnapshotPanel id={c.id} />
          <ServiceChangesPanel id={c.id} />
        </div>
        <div className="space-y-4">
          <LinkedDocPanel connectorId={c.id} connectorType={c.type} />
          <ActivityPanel activity={activity} />
        </div>
      </div>

      <ConfirmDestructive
        open={removing}
        connectorId={c.id}
        connectorName={c.name}
        onClose={() => setRemoving(false)}
        onConfirmed={() => {
          setRemoving(false);
          navigate('/services');
        }}
      />
    </div>
  );
}

function SnapshotPanel({ id }: { id: string }) {
  const { t } = useTranslation();
  const data = useGetConnectorsConnectorIdData(id);

  return (
    <Panel className="p-5">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-ink">{t('services.detail.snapshot')}</h2>
        {data.data && (
          <span className="font-mono text-2xs text-ink-faint">
            {t('services.detail.fetchedAt', { time: relativeTime(data.data.fetchedAt) })}
          </span>
        )}
      </div>
      {data.isLoading ? (
        <SkeletonRows rows={4} />
      ) : data.isError || !data.data ? (
        <ErrorState
          description={t('services.detail.snapshotError')}
          onRetry={() => data.refetch()}
        />
      ) : data.data.sections.length === 0 ? (
        <EmptyState
          title={t('services.detail.neverSyncedTitle')}
          description={t('services.detail.neverSyncedDesc')}
        />
      ) : (
        <div className="space-y-3">
          {[...data.data.sections]
            .sort((a, b) => a.order - b.order)
            .map((s) => (
              <details
                key={s.title}
                open
                className="rounded-md border border-line-soft bg-canvas-sunken p-3"
              >
                <summary className="cursor-pointer text-xs font-semibold text-ink">
                  {s.title}
                </summary>
                <div className="mt-2 text-sm">
                  <Markdown source={s.content} />
                </div>
              </details>
            ))}
        </div>
      )}
    </Panel>
  );
}

function ServiceChangesPanel({ id }: { id: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const changes = useGetChanges({ serviceId: id, pageSize: 10 });

  return (
    <Panel className="p-5">
      <h2 className="mb-3 text-sm font-semibold text-ink">{t('services.detail.history')}</h2>
      {changes.isLoading ? (
        <SkeletonRows rows={3} />
      ) : changes.isError ? (
        <ErrorState
          description={t('services.detail.historyError')}
          onRetry={() => changes.refetch()}
        />
      ) : !changes.data || changes.data.items.length === 0 ? (
        <EmptyState
          title={t('services.detail.noChangesTitle')}
          description={t('services.detail.noChangesDesc')}
        />
      ) : (
        <ul className="divide-y divide-line-soft">
          {changes.data.items.map((ch) => (
            <li key={ch.id}>
              <button
                onClick={() => navigate(`/changes/${ch.id}`)}
                className="flex w-full items-start gap-3 py-2.5 text-left transition-colors hover:bg-surface-raised"
              >
                <SeverityTag severity={ch.severity} />
                <span className="flex-1">
                  <span className="block text-sm text-ink">{ch.summary}</span>
                  <span className="font-mono text-2xs text-ink-faint">
                    {ch.changeType} · {relativeTime(ch.detectedAt)}
                  </span>
                </span>
                <DiffIcon size={14} className="mt-1 shrink-0 text-ink-faint" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}

function LinkedDocPanel({
  connectorId,
  connectorType,
}: {
  connectorId: string;
  connectorType: string;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canMutate = useCanMutate();
  const doc = useGetDocsServiceConnectorId(connectorId);
  const templates = useGetTemplates();
  const [picking, setPicking] = useState(false);
  const [templateId, setTemplateId] = useState('');

  // Templates with no appliesTo.type apply to any connector; others must match.
  const candidates = (templates.data ?? []).filter(
    (tpl) => !tpl.appliesTo?.type || tpl.appliesTo.type === connectorType
  );

  const generate = useMutation({
    mutationFn: () => postDocsGenerate({ templateId, connectorId }),
    onSuccess: (result) => {
      queryClient.invalidateQueries({
        queryKey: getGetDocsServiceConnectorIdQueryKey(connectorId),
      });
      setPicking(false);
      navigate(`/docs/${result.docId}`);
    },
  });

  const openPicker = () => {
    generate.reset();
    setTemplateId(candidates[0]?.id ?? '');
    setPicking(true);
  };

  return (
    <Panel className="p-5">
      <h2 className="mb-3 text-sm font-semibold text-ink">{t('services.detail.linkedDoc')}</h2>
      {doc.isError || !doc.data ? (
        <div className="space-y-3">
          <p className="text-sm text-ink-muted">{t('services.detail.noDoc')}</p>
          {canMutate && (
            <Button size="sm" variant="secondary" onClick={openPicker}>
              <SparklesIcon size={14} />
              {t('services.detail.generateDoc')}
            </Button>
          )}
        </div>
      ) : (
        <button
          onClick={() => navigate(`/docs/${doc.data.docId}`)}
          className="flex w-full items-center gap-2.5 rounded-md border border-line-soft p-3 text-left transition-colors hover:border-line-strong"
        >
          <FileTextIcon size={16} className="shrink-0 text-signal" />
          <span className="flex-1 text-sm text-ink">{doc.data.title}</span>
          <ArrowRightIcon size={14} className="text-ink-faint" />
        </button>
      )}

      <Dialog
        open={picking}
        onClose={() => setPicking(false)}
        title={t('services.detail.generateDocTitle')}
        size="sm"
      >
        <div className="space-y-3">
          <label className="block">
            <span className="mb-1 block text-2xs uppercase tracking-wider text-ink-faint">
              {t('services.detail.templateLabel')}
            </span>
            <div className="relative">
              <select
                value={templateId}
                onChange={(e) => setTemplateId(e.target.value)}
                disabled={candidates.length === 0}
                className="h-9 w-full appearance-none rounded-sm border border-line bg-surface px-2.5 pr-8 text-sm text-ink outline-none focus-visible:border-signal-soft"
              >
                {candidates.length === 0 ? (
                  <option value="">{t('services.detail.noTemplates')}</option>
                ) : (
                  candidates.map((tpl) => (
                    <option key={tpl.id} value={tpl.id}>
                      {tpl.name}
                    </option>
                  ))
                )}
              </select>
              <ChevronDownIcon
                size={14}
                className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-ink-faint"
              />
            </div>
          </label>

          {generate.isError && (
            <p className="text-xs text-err">{t('services.detail.generateDocError')}</p>
          )}

          <div className="flex items-center justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={() => setPicking(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              disabled={!templateId || generate.isPending}
              onClick={() => generate.mutate()}
            >
              {generate.isPending ? t('services.detail.generating') : t('services.detail.generateDoc')}
            </Button>
          </div>
        </div>
      </Dialog>
    </Panel>
  );
}

function ActivityPanel({
  activity,
}: {
  activity: { id: string; label: string; detail?: string; at: string }[];
}) {
  const { t } = useTranslation();
  return (
    <Panel className="p-5">
      <h2 className="mb-3 text-sm font-semibold text-ink">{t('services.detail.activity')}</h2>
      {activity.length === 0 ? (
        <p className="text-sm text-ink-muted">{t('services.detail.noActivity')}</p>
      ) : (
        <ul className="space-y-2.5">
          {activity.slice(0, 8).map((a) => (
            <li key={a.id} className="flex items-start gap-2.5">
              <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-signal" aria-hidden />
              <span className="flex-1">
                <span className="block text-xs text-ink">{a.label}</span>
                {a.detail && <span className="text-2xs text-ink-faint">{a.detail}</span>}
              </span>
              <span className="font-mono text-2xs text-ink-faint">{relativeTime(a.at)}</span>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}
