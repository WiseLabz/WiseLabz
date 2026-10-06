import { useState } from 'react';
import type { FormEvent, ReactNode } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { toast } from '../../lib/toast';
import {
  getEntitiesId,
  getGetEntityOverridesQueryKey,
  useGetEntitiesId,
  usePostEntityOverrides,
} from '../../api/generated/search/search';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import type { EntityEndpoint, EntityFinding, EntityMember, EntityOverride } from '../../api/model';
import { Markdown } from '../../components/docs/Markdown';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { Dialog } from '../../components/ui/Dialog';
import { EmptyState, ErrorState, SkeletonRows } from '../../components/ui/states';
import { EntityPicker } from '../../components/manager/EntityPicker';
import { useIsInstanceAdmin } from '../../hooks/useRole';
import { fullDate } from '../../lib/time';

const valueText = (value: unknown) =>
  value === undefined ? '—' : typeof value === 'string' ? value : JSON.stringify(value);

/** An endpoint links to its own page only when the API says the caller can open it. */
function Endpoint({ endpoint }: { endpoint: EntityEndpoint }) {
  return endpoint.entityId ? (
    <Link
      className="text-accent-secondary-bright"
      to={`/entities/${encodeURIComponent(endpoint.entityId)}`}
    >
      {endpoint.name}
    </Link>
  ) : (
    <>{endpoint.name}</>
  );
}

export function EntityDetailPage() {
  const { t } = useTranslation();
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const isAdmin = useIsInstanceAdmin();

  const entity = useGetEntitiesId(id);
  const connectors = useGetConnectors({ query: { enabled: isAdmin } });

  const [detachMember, setDetachMember] = useState<EntityMember | null>(null);
  const [mergeMember, setMergeMember] = useState<EntityMember | null>(null);
  const [targetConnectorId, setTargetConnectorId] = useState('');
  const [targetRef, setTargetRef] = useState('');
  const [note, setNote] = useState('');

  const postOverride = usePostEntityOverrides();

  const handleVisibilityAndNavigate = async (
    targetEntityId: string | undefined,
    successMsg: string,
    notVisibleMsg: string
  ) => {
    if (!targetEntityId) {
      toast.success(notVisibleMsg);
      void entity.refetch();
      return;
    }
    try {
      await getEntitiesId(targetEntityId);
      toast.success(successMsg);
      if (targetEntityId === id) {
        void entity.refetch();
      } else {
        navigate(`/entities/${encodeURIComponent(targetEntityId)}`);
      }
    } catch (err) {
      if (isAxiosError(err) && err.response?.status === 404) {
        toast.success(notVisibleMsg);
        void entity.refetch();
      } else {
        toast.success(successMsg);
        if (targetEntityId === id) {
          void entity.refetch();
        } else {
          navigate(`/entities/${encodeURIComponent(targetEntityId)}`);
        }
      }
    }
  };

  const invalidateEntityCaches = () => {
    void queryClient.invalidateQueries({ queryKey: getGetEntityOverridesQueryKey() });
    void queryClient.invalidateQueries({
      predicate: (q) =>
        typeof q.queryKey[0] === 'string' && q.queryKey[0].startsWith('/entities/'),
    });
  };

  const handleOverrideSuccess = async (
    member: EntityMember,
    res: EntityOverride,
    messages: { success: string; notVisible: string },
    resetForm: () => void
  ) => {
    invalidateEntityCaches();
    resetForm();
    const targetMember =
      res.members.find(
        (item) =>
          item.connectorId === member.connectorId &&
          item.kind === member.kind &&
          item.ref === member.ref
      ) ?? res.members[0];
    if (res.state === 'dormant') {
      toast.info(t('entities.overrides.savedDormant'));
      void entity.refetch();
      return;
    }
    await handleVisibilityAndNavigate(
      targetMember?.entityId,
      messages.success,
      messages.notVisible
    );
  };

  const handleOverrideError = (
    err: unknown,
    errorMessage: string,
    resetForm: () => void
  ) => {
    if (isAxiosError(err) && err.response?.status === 409) {
      toast.error(t('entities.overrides.conflictError'));
    } else if (isAxiosError(err) && err.response?.status && err.response.status < 500) {
      toast.error(errorMessage);
    } else {
      invalidateEntityCaches();
      void entity.refetch();
      resetForm();
      toast.error(errorMessage);
    }
  };

  const handleDetachConfirm = () => {
    const m = detachMember;
    if (!m) return;
    postOverride.mutate(
      {
        data: {
          action: 'detach',
          connectorId: m.connectorId,
          kind: m.kind,
          ref: m.ref,
        },
      },
      {
        onSuccess: (res) =>
          handleOverrideSuccess(
            m,
            res,
            {
              success: t('entities.overrides.detachSuccess'),
              notVisible: t('entities.overrides.detachSuccessNotVisible'),
            },
            () => setDetachMember(null)
          ),
        onError: (err) =>
          handleOverrideError(err, t('entities.overrides.detachError'), () =>
            setDetachMember(null)
          ),
      }
    );
  };

  const openMerge = (m: EntityMember) => {
    setMergeMember(m);
    setTargetConnectorId(m.connectorId || connectors.data?.[0]?.id || '');
    setTargetRef('');
    setNote('');
  };

  const isSameMember = Boolean(
    mergeMember &&
      targetConnectorId === mergeMember.connectorId &&
      targetRef === mergeMember.ref
  );

  const handleMergeSubmit = (e: FormEvent) => {
    e.preventDefault();
    const m = mergeMember;
    if (!m || !targetConnectorId || !targetRef || isSameMember) return;
    postOverride.mutate(
      {
        data: {
          action: 'merge',
          connectorId: m.connectorId,
          kind: m.kind,
          ref: m.ref,
          otherConnectorId: targetConnectorId,
          otherKind: m.kind,
          otherRef: targetRef,
          note: note.trim() || undefined,
        },
      },
      {
        onSuccess: (res) =>
          handleOverrideSuccess(
            m,
            res,
            {
              success: t('entities.overrides.mergeSuccess'),
              notVisible: t('entities.overrides.mergeSuccessNotVisible'),
            },
            () => {
              setMergeMember(null);
              setTargetRef('');
              setNote('');
            }
          ),
        onError: (err) =>
          handleOverrideError(err, t('entities.overrides.mergeError'), () => {
            setMergeMember(null);
            setTargetRef('');
            setNote('');
          }),
      }
    );
  };

  if (entity.isLoading) {
    return (
      <Panel className="p-5">
        <SkeletonRows rows={5} />
      </Panel>
    );
  }
  if (isAxiosError(entity.error) && entity.error.response?.status === 404) {
    return (
      <Panel className="min-h-[40vh]">
        <ErrorState title={t('entities.notFound')} />
      </Panel>
    );
  }
  if (entity.isError || !entity.data) {
    return (
      <Panel className="min-h-[40vh]">
        <ErrorState title={t('entities.loadError')} onRetry={() => void entity.refetch()} />
      </Panel>
    );
  }

  const data = entity.data;
  const findingMeta = (finding: EntityFinding) =>
    `${t(`status.severity.${finding.severity}`, { defaultValue: finding.severity })} · ${t(`entities.findingStatus.${finding.status}`, { defaultValue: finding.status })}`;
  const section = (title: string, children: ReactNode) => (
    <Panel>
      <PanelHeader title={title} />
      <div className="space-y-3 p-4">{children}</div>
    </Panel>
  );

  return (
    <div className="mx-auto max-w-275 space-y-5 px-6 py-6">
      <div className="flex items-center justify-between">
        <Link className="text-xs text-ink-muted hover:text-ink" to="/search">
          ← {t('search.title')}
        </Link>
        {isAdmin && (
          <Link
            className="text-xs text-ink-muted hover:text-ink"
            to="/entities/overrides"
          >
            {t('entities.overrides.manageOverrides')} →
          </Link>
        )}
      </div>

      <header>
        <p className="font-mono text-xs text-ink-muted">{data.kind}</p>
        <h1 className="text-xl font-semibold text-ink">{data.name}</h1>
      </header>

      {data.gone && (
        <p
          role="status"
          className="rounded-md border border-warn/30 bg-warn-tint px-4 py-3 text-sm text-warn"
        >
          {t('entities.gone')}
        </p>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        {section(
          t('entities.members'),
          data.members.length ? (
            data.members.map((m) => (
              <div
                key={`${m.connectorId}:${m.kind}:${m.ref}`}
                className="flex flex-col gap-2 border-b border-line-soft pb-3 last:border-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between sm:gap-3"
              >
                <span className="text-sm text-ink">
                  {m.name} <span className="font-mono text-xs text-ink-muted">· {m.kind}</span>
                  {m.goneAt && (
                    <span className="block text-xs text-warn">
                      {t('entities.goneSince', { date: fullDate(m.goneAt) })}
                    </span>
                  )}
                </span>
                <span className="flex shrink-0 flex-wrap items-center gap-2 text-xs">
                  <Link
                    className="text-accent-secondary-bright"
                    to={`/services/${encodeURIComponent(m.connectorId)}`}
                  >
                    {m.connectorName}
                  </Link>
                  {m.docId && (
                    <Link
                      className="text-accent-secondary-bright"
                      to={`/docs/${encodeURIComponent(m.docId)}`}
                    >
                      {t('entities.connectorDoc')}
                    </Link>
                  )}
                  {isAdmin && (
                    <span className="flex items-center gap-1.5 pl-1">
                      <Button
                        size="sm"
                        variant="ghost"
                        aria-label={t('entities.overrides.mergeActionAria', {
                          name: m.name || m.ref,
                          connector: m.connectorName || m.connectorId,
                        })}
                        onClick={() => openMerge(m)}
                      >
                        {t('entities.overrides.mergeAction')}
                      </Button>
                      <Button
                        size="sm"
                        variant="danger"
                        aria-label={t('entities.overrides.detachActionAria', {
                          name: m.name || m.ref,
                          connector: m.connectorName || m.connectorId,
                        })}
                        onClick={() => setDetachMember(m)}
                      >
                        {t('entities.overrides.detachAction')}
                      </Button>
                    </span>
                  )}
                </span>
              </div>
            ))
          ) : (
            <EmptyState title={t('entities.empty')} />
          )
        )}

        {section(
          t('entities.relatedByIp'),
          data.relatedByIp.length ? (
            data.relatedByIp.map((edge, i) => (
              <p
                key={`${edge.from.connectorId}:${edge.to.connectorId}:${i}`}
                className="text-sm text-ink"
              >
                <Endpoint endpoint={edge.from} /> ↔ <Endpoint endpoint={edge.to} />{' '}
                <span className="text-xs text-ink-muted">· {edge.reason}</span>
              </p>
            ))
          ) : (
            <EmptyState title={t('entities.empty')} />
          )
        )}

        {section(
          t('entities.neighbors'),
          data.neighbors.length ? (
            data.neighbors.map((edge, i) => (
              <p
                key={`${edge.kind}:${edge.from.connectorId}:${edge.to.connectorId}:${i}`}
                className="text-sm text-ink"
              >
                <Endpoint endpoint={edge.from} /> → <Endpoint endpoint={edge.to} />{' '}
                <span className="font-mono text-xs text-ink-muted">· {edge.kind}</span>
                {edge.detail && (
                  <span className="text-xs text-ink-muted"> · {edge.detail}</span>
                )}
              </p>
            ))
          ) : (
            <EmptyState title={t('entities.empty')} />
          )
        )}

        {section(
          t('entities.history'),
          data.history.length ? (
            data.history.map((item, i) => (
              <div
                key={`${item.at}:${item.key}:${item.field}:${i}`}
                className="border-b border-line-soft pb-2 text-sm last:border-0"
              >
                <p className="text-ink">
                  <span className="font-mono">{item.field}</span> · {item.change}
                </p>
                <p className="mt-1 text-xs text-ink-muted">
                  {fullDate(item.at)} · {valueText(item.old)} → {valueText(item.new)}
                </p>
              </div>
            ))
          ) : (
            <EmptyState title={t('entities.empty')} />
          )
        )}

        {section(
          t('entities.findings'),
          data.findings.length ? (
            data.findings.map((finding) => (
              <div key={finding.id} className="text-sm">
                <p className="text-ink">{finding.title}</p>
                <p className="text-xs text-ink-muted">{findingMeta(finding)}</p>
              </div>
            ))
          ) : (
            <EmptyState title={t('entities.empty')} />
          )
        )}

        {section(
          t('entities.connectorFindings'),
          data.onReportingConnectors.length ? (
            data.onReportingConnectors.map((finding) => (
              <div key={finding.id} className="text-sm">
                <p className="text-ink">{finding.title}</p>
                <p className="text-xs text-ink-muted">{findingMeta(finding)}</p>
              </div>
            ))
          ) : (
            <EmptyState title={t('entities.empty')} />
          )
        )}

        {section(
          t('entities.runbooks'),
          data.runbooks.length ? (
            data.runbooks.map((runbook) => (
              <article key={`${runbook.id}:${runbook.step.id}`}>
                <h3 className="text-sm font-medium text-ink">{runbook.title}</h3>
                <p className="text-xs text-ink-muted">
                  {runbook.step.title} · {runbook.step.verb}
                </p>
                <Markdown source={runbook.body} />
              </article>
            ))
          ) : (
            <EmptyState title={t('entities.empty')} />
          )
        )}
      </div>

      <ConfirmDialog
        open={Boolean(detachMember)}
        onClose={() => setDetachMember(null)}
        onConfirm={handleDetachConfirm}
        title={t('entities.overrides.detachTitle')}
        description={
          detachMember
            ? t('entities.overrides.detachConfirm', {
                name: detachMember.name || detachMember.ref,
              })
            : ''
        }
        confirmLabel={t('entities.overrides.detachAction')}
        cancelLabel={t('common.cancel', { defaultValue: 'Cancel' })}
        tone="danger"
        confirmDisabled={postOverride.isPending}
      />

      {mergeMember && (
        <Dialog
          open={Boolean(mergeMember)}
          onClose={() => {
            setMergeMember(null);
            setTargetRef('');
            setTargetConnectorId('');
            setNote('');
          }}
          title={t('entities.overrides.mergeTitle')}
        >
          <form onSubmit={handleMergeSubmit} className="space-y-4">
            <p className="text-sm text-ink-muted">
              {t('entities.overrides.mergeSubtitle', { name: mergeMember.name })}
            </p>

            <div>
              <span className="mb-1 block text-2xs text-ink-faint">
                {t('entities.overrides.targetConnector')}
              </span>
              <select
                aria-label={t('entities.overrides.targetConnector')}
                value={targetConnectorId}
                onChange={(e) => {
                  setTargetConnectorId(e.target.value);
                  setTargetRef('');
                }}
                className="h-8 w-full appearance-none rounded-sm border border-line bg-surface pl-2.5 pr-7 text-xs text-ink outline-none focus-visible:border-accent-primary-soft"
              >
                {connectors.data?.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </div>

            {targetConnectorId && (
              <EntityPicker
                connectorId={targetConnectorId}
                value={targetRef}
                onChange={setTargetRef}
                kind={mergeMember.kind}
                placeholder={t('entities.overrides.selectEntity')}
                label={t('entities.overrides.targetEntity')}
                hideWholeService
              />
            )}

            {isSameMember && (
              <p role="alert" className="text-2xs text-err">
                {t('entities.overrides.sameMemberError')}
              </p>
            )}

            <div>
              <label htmlFor="override-note" className="mb-1 block text-2xs text-ink-faint">
                {t('entities.overrides.noteLabel')}
              </label>
              <textarea
                id="override-note"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                maxLength={1000}
                placeholder={t('entities.overrides.notePlaceholder')}
                rows={3}
                className="w-full rounded-sm border border-line bg-surface p-2 text-xs text-ink outline-none placeholder:text-ink-faint focus-visible:border-accent-primary-soft"
              />
            </div>

            <div className="flex items-center justify-end gap-2 pt-2">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => {
                  setMergeMember(null);
                  setTargetRef('');
                  setTargetConnectorId('');
                  setNote('');
                }}
              >
                {t('common.cancel', { defaultValue: 'Cancel' })}
              </Button>
              <Button
                type="submit"
                variant="primary"
                size="sm"
                disabled={!targetRef || isSameMember || postOverride.isPending}
              >
                {t('entities.overrides.mergeAction')}
              </Button>
            </div>
          </form>
        </Dialog>
      )}
    </div>
  );
}
