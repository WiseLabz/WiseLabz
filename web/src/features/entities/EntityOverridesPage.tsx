import { useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { toast } from '../../lib/toast';
import {
  getEntitiesId,
  useGetEntityOverrides,
  useDeleteEntityOverridesId,
  getGetEntityOverridesQueryKey,
} from '../../api/generated/search/search';
import { useGetUsers } from '../../api/generated/users/users';
import type { EntityOverride, EntityOverrideMember } from '../../api/model';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { ToneTag } from '../../components/ui/ToneTag';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { EmptyState, ErrorState, SkeletonRows } from '../../components/ui/states';
import { fullDate } from '../../lib/time';

function MemberItem({ member }: { member: EntityOverrideMember }) {
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      {member.entityId ? (
        <Link
          className="font-medium text-accent-secondary-bright hover:underline"
          to={`/entities/${encodeURIComponent(member.entityId)}`}
        >
          {member.name || member.ref}
        </Link>
      ) : (
        <span className="font-medium text-ink">{member.name || member.ref}</span>
      )}
      <span className="font-mono text-xs text-ink-muted">· {member.kind}</span>
      <span className="text-xs text-ink-faint">
        ({member.connectorName || member.connectorId})
      </span>
    </div>
  );
}

export function EntityOverridesPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [toRemove, setToRemove] = useState<EntityOverride | null>(null);

  const usersQuery = useGetUsers();
  const usersById = useMemo(() => {
    const map = new Map<string, string>();
    for (const u of usersQuery.data ?? []) {
      map.set(u.id, u.displayName || u.username);
    }
    return map;
  }, [usersQuery.data]);

  const overrides = useGetEntityOverrides();
  const deleteMutation = useDeleteEntityOverridesId({
    mutation: {
      onSuccess: async (res) => {
        void queryClient.invalidateQueries({ queryKey: getGetEntityOverridesQueryKey() });
        void queryClient.invalidateQueries({ queryKey: ['/entities'] });
        setToRemove(null);

        const targetEntityId = res?.members?.[0]?.entityId;
        if (!targetEntityId) {
          toast.success(t('entities.overrides.removeSuccessNotVisible'));
          return;
        }

        try {
          await getEntitiesId(targetEntityId);
          toast.success(t('entities.overrides.removeSuccess'), {
            action: {
              label: t('notify.view'),
              onClick: () => navigate(`/entities/${encodeURIComponent(targetEntityId)}`),
            },
          });
        } catch (err) {
          if (isAxiosError(err) && err.response?.status === 404) {
            toast.success(t('entities.overrides.removeSuccessNotVisible'));
          } else {
            toast.success(t('entities.overrides.removeSuccess'), {
              action: {
                label: t('notify.view'),
                onClick: () => navigate(`/entities/${encodeURIComponent(targetEntityId)}`),
              },
            });
          }
        }
      },
      onError: () => {
        toast.error(t('entities.overrides.removeError'));
      },
    },
  });

  if (overrides.isLoading) {
    return (
      <div className="mx-auto max-w-275 space-y-5 px-6 py-6">
        <Link className="text-xs text-ink-muted hover:text-ink" to="/search">
          ← {t('search.title')}
        </Link>
        <Panel className="p-5">
          <SkeletonRows rows={5} />
        </Panel>
      </div>
    );
  }

  if (overrides.isError || !overrides.data) {
    return (
      <div className="mx-auto max-w-275 space-y-5 px-6 py-6">
        <Link className="text-xs text-ink-muted hover:text-ink" to="/search">
          ← {t('search.title')}
        </Link>
        <Panel className="min-h-[40vh]">
          <ErrorState
            title={t('entities.overrides.loadError')}
            onRetry={() => void overrides.refetch()}
          />
        </Panel>
      </div>
    );
  }

  const data = overrides.data;

  return (
    <div className="mx-auto max-w-275 space-y-5 px-6 py-6">
      <Link className="text-xs text-ink-muted hover:text-ink" to="/search">
        ← {t('search.title')}
      </Link>
      <header>
        <h1 className="text-xl font-semibold text-ink">{t('entities.overrides.pageTitle')}</h1>
        <p className="mt-1 text-sm text-ink-muted">{t('entities.overrides.pageSubtitle')}</p>
      </header>

      {data.length === 0 ? (
        <Panel className="min-h-[40vh]">
          <EmptyState
            title={t('entities.overrides.emptyTitle')}
            description={t('entities.overrides.emptyDescription')}
          />
        </Panel>
      ) : (
        <Panel>
          <PanelHeader
            title={t('entities.overrides.listHeading')}
            count={data.length}
          />
          <div className="divide-y divide-line-soft">
            {data.map((override) => (
              <div
                key={override.id}
                className="flex flex-col gap-3 p-4 sm:flex-row sm:items-start sm:justify-between"
              >
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-mono text-xs uppercase font-medium text-ink">
                      {t(`entities.overrides.action_${override.action}`, {
                        defaultValue: override.action,
                      })}
                    </span>
                    <ToneTag
                      tone={override.state === 'active' ? 'ok' : 'idle'}
                      label={
                        override.state === 'active'
                          ? t('entities.overrides.stateActive')
                          : t('entities.overrides.stateDormant')
                      }
                    />
                  </div>

                  <div className="space-y-1">
                    {override.members.map((m) => (
                      <MemberItem
                        key={`${m.connectorId}:${m.kind}:${m.ref}`}
                        member={m}
                      />
                    ))}
                  </div>

                  {override.note && (
                    <p className="text-xs italic text-ink-muted">{override.note}</p>
                  )}

                  <div className="flex flex-wrap items-center gap-3 text-2xs text-ink-faint">
                    <span>
                      {t('entities.overrides.createdBy', {
                        user: usersById.get(override.createdBy) || override.createdBy,
                      })}
                    </span>
                    <span>·</span>
                    <span>{t('entities.overrides.createdAt', { date: fullDate(override.createdAt) })}</span>
                  </div>
                </div>

                <div className="shrink-0 pt-1">
                  <Button
                    size="sm"
                    variant="danger"
                    aria-label={t('entities.overrides.removeActionAria', {
                      name: override.members.map((m) => m.name || m.ref).join(', '),
                    })}
                    onClick={() => setToRemove(override)}
                  >
                    {t('entities.overrides.removeAction')}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </Panel>
      )}

      {toRemove && (
        <ConfirmDialog
          open={Boolean(toRemove)}
          onClose={() => setToRemove(null)}
          onConfirm={() => {
            if (toRemove) deleteMutation.mutate({ id: toRemove.id });
          }}
          title={t('entities.overrides.removeTitle')}
          description={t('entities.overrides.removeConfirm')}
          confirmLabel={t('entities.overrides.removeAction')}
          cancelLabel={t('common.cancel', { defaultValue: 'Cancel' })}
          tone="danger"
          confirmDisabled={deleteMutation.isPending}
        />
      )}
    </div>
  );
}
