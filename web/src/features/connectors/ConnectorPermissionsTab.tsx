/**
 * Per-connector permissions (#240 PR1): who has viewer/operator access to
 * this one connector. Instance-admin only — the server gates the underlying
 * endpoints the same way, this just keeps non-admins from seeing a section
 * they can't use.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useGetUsers } from '../../api/generated/users/users';
import {
  useGetConnectorsConnectorIdPermissions,
  putConnectorsConnectorIdPermissionsUserId,
  deleteConnectorsConnectorIdPermissionsUserId,
  getGetConnectorsConnectorIdPermissionsQueryKey,
} from '../../api/generated/connectors/connectors';
import type { GetConnectorsConnectorIdPermissions200Item } from '../../api/model';
import { useIsInstanceAdmin } from '../../hooks/useRole';
import { Panel } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { toast } from '../../lib/toast';

export function ConnectorPermissionsTab({ connectorId }: { connectorId: string }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const isInstanceAdmin = useIsInstanceAdmin();
  const grants = useGetConnectorsConnectorIdPermissions(connectorId, {
    query: { enabled: isInstanceAdmin },
  });
  const users = useGetUsers({ query: { enabled: isInstanceAdmin } });
  const [addingUserId, setAddingUserId] = useState('');

  const upsert = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: 'viewer' | 'operator' }) =>
      putConnectorsConnectorIdPermissionsUserId(connectorId, userId, { role }),
    onSuccess: () => {
      setAddingUserId('');
      queryClient.invalidateQueries({
        queryKey: getGetConnectorsConnectorIdPermissionsQueryKey(connectorId),
      });
    },
    onError: () =>
      toast.error(t('connectors.permissions.error')),
  });

  const remove = useMutation({
    mutationFn: (userId: string) =>
      deleteConnectorsConnectorIdPermissionsUserId(connectorId, userId),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: getGetConnectorsConnectorIdPermissionsQueryKey(connectorId),
      }),
  });

  if (!isInstanceAdmin) return null;

  const userById = new Map((users.data ?? []).map((u) => [u.id, u]));
  // Only a 'manual' grant blocks adding a user here — a user with only an
  // 'oidc'-sourced grant (#279 part 3) can still get a manual grant added
  // alongside it (GetUserConnectorRole takes the higher of the two).
  const manuallyGrantedUserIds = new Set(
    (grants.data ?? []).filter((g) => g.source !== 'oidc').map((g) => g.userId)
  );
  const addableUsers = (users.data ?? []).filter((u) => !manuallyGrantedUserIds.has(u.id));

  const onAdd = () => {
    if (!addingUserId) return;
    upsert.mutate({ userId: addingUserId, role: 'viewer' });
  };

  return (
    <Panel className="mt-4 p-5">
      <h2 className="mb-1 text-sm font-semibold text-ink">{t('connectors.permissions.title')}</h2>
      <p className="mb-4 text-xs text-ink-muted">{t('connectors.permissions.subtitle')}</p>

      {grants.isLoading ? (
        <SkeletonRows rows={3} />
      ) : grants.isError ? (
        <ErrorState description={t('connectors.permissions.loadError')} onRetry={() => grants.refetch()} />
      ) : (grants.data ?? []).length === 0 ? (
        <EmptyState title={t('connectors.permissions.emptyTitle')} />
      ) : (
        <ul className="mb-4 divide-y divide-line-soft">
          {(grants.data ?? []).map((g: GetConnectorsConnectorIdPermissions200Item) => {
            /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
            const user = userById.get((g as any).userId);
            // 'oidc' grants are synced from the user's IdP group at login
            // (#279 part 3) and are read-only here — editing them would be
            // silently overwritten at the user's next login anyway.
            /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
            const isSSO = (g as any).source === 'oidc';
            return (
              /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
              <li key={(g as any).id} className="flex items-center justify-between gap-3 py-2.5">
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5">
                    {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                    <p className="truncate text-sm text-ink">{user?.displayName || user?.username || (g as any).userId}</p>
                    {isSSO && (
                      <span
                        title={t('connectors.permissions.viaSsoHint')}
                        className="shrink-0 rounded-full bg-canvas px-1.5 py-0.5 text-2xs text-ink-faint"
                      >
                        {t('connectors.permissions.viaSso')}
                      </span>
                    )}
                  </div>
                  {user?.username && <p className="truncate text-2xs text-ink-faint">{user.username}</p>}
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {isSSO ? (
                    <span className="rounded-md border border-line-soft px-2 py-1 text-xs text-ink-muted">
                      {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                      {(g as any).role === 'operator' ? t('connectors.permissions.operator') : t('connectors.permissions.viewer')}
                    </span>
                  ) : (
                    <>
                      {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                      <select value={(g as any).role}
                        onChange={(e) => {
                          /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
                          const userId = (g as any).userId;
                          upsert.mutate({
                            userId,
                            role: e.target.value as 'viewer' | 'operator',
                          });
                        }}
                        className="rounded-md border border-line-soft bg-canvas px-2 py-1 text-xs text-ink"
                      >
                        <option value="viewer">{t('connectors.permissions.viewer')}</option>
                        <option value="operator">{t('connectors.permissions.operator')}</option>
                      </select>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={remove.isPending}
                        onClick={() => {
                          /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
                          remove.mutate((g as any).userId);
                        }}
                      >
                        {t('connectors.permissions.revoke')}
                      </Button>
                    </>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}

      <div className="flex items-center gap-2 border-t border-line-soft pt-3">
        <select
          value={addingUserId}
          onChange={(e) => setAddingUserId(e.target.value)}
          className="min-w-0 flex-1 rounded-md border border-line-soft bg-canvas px-2 py-1.5 text-xs text-ink"
        >
          <option value="">{t('connectors.permissions.addUserPlaceholder')}</option>
          {addableUsers.map((u) => (
            <option key={u.id} value={u.id}>
              {u.displayName || u.username}
            </option>
          ))}
        </select>
        <Button variant="secondary" size="sm" disabled={!addingUserId || upsert.isPending} onClick={onAdd}>
          {t('connectors.permissions.addUser')}
        </Button>
      </div>
    </Panel>
  );
}
