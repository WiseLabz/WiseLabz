/**
 * "My Share Links" (#240 PR2) — every read-only doc share link the current
 * user created, with revoke. Row layout mirrors ConnectorPermissionsTab.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetDocsShareLinks,
  deleteDocsShareLinksId,
  getGetDocsShareLinksQueryKey,
} from '../../api/generated/docs/docs';
import type { GetDocsShareLinks200Item } from '../../api/model';
import { Panel } from '../../components/ui/Panel';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { toast } from '../../lib/toast';
import { fullDate } from '../../lib/time';

export function ShareLinksPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: links = [], isLoading, isError, refetch } = useGetDocsShareLinks();
  // Lazy init: read "now" once at mount, not on every render
  // (react-hooks/purity forbids calling Date.now() during render).
  const [nowIso] = useState(() => new Date().toISOString());

  const revoke = useMutation({
    mutationFn: (id: string) => deleteDocsShareLinksId(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getGetDocsShareLinksQueryKey() }),
    onError: () =>
      toast.error(t('settings.shareLinks.revokeError', { defaultValue: 'Could not revoke link' })),
  });

  const onRevoke = (id: string) => {
    revoke.mutate(id);
  };

  return (
    <Panel className="p-5">
      <h2 className="mb-1 text-sm font-semibold text-ink">
        {t('settings.shareLinks.title', { defaultValue: 'Share links' })}
      </h2>
      <p className="mb-4 text-xs text-ink-muted">
        {t('settings.shareLinks.subtitle', {
          defaultValue: 'Read-only links you created into the doc tree. Revoke one to cut off access immediately.',
        })}
      </p>

      {isLoading ? (
        <SkeletonRows rows={3} />
      ) : isError ? (
        <ErrorState
          description={t('settings.shareLinks.loadError', { defaultValue: 'Could not load share links' })}
          onRetry={() => refetch()}
        />
      ) : links.length === 0 ? (
        <EmptyState title={t('settings.shareLinks.empty', { defaultValue: 'No share links yet' })} />
      ) : (
        <ul className="divide-y divide-line-soft">
          {links.map((link: GetDocsShareLinks200Item) => {
            /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
            const revoked = !!(link as any).revokedAt;
            /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
            const expired = !revoked && (link as any).expiresAt <= nowIso;
            return (
              /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
              <li key={(link as any).id} className="flex items-center justify-between gap-3 py-2.5">
                <div className="min-w-0">
                  {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                  <p className="truncate font-mono text-sm text-ink">{(link as any).docTreeRoot}</p>
                  <p className="truncate text-2xs text-ink-faint">
                    {t('settings.shareLinks.created', { defaultValue: 'Created' })}{' '}
                    {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                    {fullDate((link as any).createdAt)}
                    {' · '}
                    {revoked
                      ? t('settings.shareLinks.revoked', { defaultValue: 'Revoked' })
                      : expired
                        ? t('settings.shareLinks.expired', { defaultValue: 'Expired' })
                        : (() => {
                            /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
                            return `${t('settings.shareLinks.expires', { defaultValue: 'Expires' })} ${fullDate((link as any).expiresAt)}`;
                          })()}
                  </p>
                </div>
                {!revoked && !expired && (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={revoke.isPending}
                    onClick={() => {
                      /* eslint-disable-next-line @typescript-eslint/no-explicit-any */
                      onRevoke((link as any).id);
                    }}
                    className="shrink-0"
                  >
                    {t('settings.shareLinks.revoke', { defaultValue: 'Revoke' })}
                  </Button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </Panel>
  );
}
