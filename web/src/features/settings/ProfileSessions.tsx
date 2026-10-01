/** Settings → Profile → active sessions. */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getGetMeSessionsQueryKey,
  getMeSessions,
  deleteMeSessionsSessionId,
} from '../../api/generated/me/me';
import type { Session } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { TimeAgo } from '../../components/ui/TimeAgo';
import { ToneTag } from '../../components/ui/ToneTag';
import { toast } from '../../lib/toast';
import { Section } from './parts';
import { UserIcon } from '../../components/icons';

export function SessionsSection() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: getGetMeSessionsQueryKey(),
    queryFn: () => getMeSessions(),
  });
  const [revoke, setRevoke] = useState<Session | null>(null);

  const doRevoke = useMutation({
    mutationFn: (id: string) => deleteMeSessionsSessionId(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetMeSessionsQueryKey() });
      toast.success(t('settings.profile.sessionRevoked'));
      setRevoke(null);
    },
    onError: () => toast.error(t('settings.profile.sessionRevokeError')),
  });

  return (
    <Section
      title={t('settings.profile.sessionsTitle')}
      description={t('settings.profile.sessionsDesc')}
    >
      {isLoading ? (
        <SkeletonRows rows={3} className="p-0" />
      ) : isError || !data ? (
        <ErrorState description={t('settings.profile.sessionsError')} onRetry={() => refetch()} />
      ) : data.length === 0 ? (
        <EmptyState icon={<UserIcon size={18} />} title={t('settings.profile.noSessions')} />
      ) : (
        <ul className="divide-y divide-line-soft">
          {data.map((s) => (
            <li key={s.id} className="flex items-center gap-4 py-3 first:pt-0 last:pb-0">
              <div className="min-w-0 flex-1">
                <p className="flex items-center gap-2 text-sm text-ink">
                  <span className="truncate">
                    {s.userAgent ?? t('settings.profile.unknownDevice')}
                  </span>
                  {s.current && <ToneTag tone="ok" label={t('common.current')} />}
                </p>
                <p className="mt-0.5 flex items-center gap-1.5 font-mono text-2xs text-ink-faint">
                  <span>{s.ip ?? '—'}</span>
                  <span>·</span>
                  <span>
                    {t('settings.profile.lastSeen')} <TimeAgo at={s.lastSeenAt} />
                  </span>
                </p>
              </div>
              {!s.current && (
                <Button
                  variant="danger"
                  size="sm"
                  disabled={doRevoke.isPending}
                  onClick={() => setRevoke(s)}
                >
                  {t('settings.profile.revoke')}
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={revoke !== null}
        onClose={() => setRevoke(null)}
        onConfirm={() => revoke && doRevoke.mutate(revoke.id)}
        tone="danger"
        title={t('settings.profile.revokeTitle')}
        description={t('settings.profile.revokeConfirm')}
        confirmLabel={t('settings.profile.revoke')}
        cancelLabel={t('common.cancel')}
        confirmDisabled={doRevoke.isPending}
      />
    </Section>
  );
}
