/**
 * Settings → Profile. Edit display name / email, change password (local accounts),
 * and review + revoke active sessions. Available to every authenticated user.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getGetMeQueryKey,
  getGetMeSessionsQueryKey,
  patchMe,
  postMePassword,
  getMeSessions,
  deleteMeSessionsSessionId,
} from '../../api/generated/me/me';
import { useGetMe } from '../../api/generated/me/me';
import type { Session } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { TimeAgo } from '../../components/ui/TimeAgo';
import { ToneTag } from '../../components/ui/ToneTag';
import { toast } from '../../lib/toast';
import { SubHeader, Section, Field, TextInput } from './parts';
import { UserIcon } from '../../components/icons';

export function ProfilePage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: me } = useGetMe();

  const [displayName, setDisplayName] = useState('');
  const [email, setEmail] = useState('');
  // Adjust state during render (React-blessed alternative to a syncing effect):
  // re-seed whenever the query yields a fresh reference, e.g. after an invalidate.
  const [seeded, setSeeded] = useState<typeof me | null>(null);
  if (me && me !== seeded) {
    setSeeded(me);
    setDisplayName(me.displayName ?? '');
    setEmail(me.email ?? '');
  }

  const saveProfile = useMutation({
    mutationFn: () => patchMe({ displayName, email }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetMeQueryKey() });
      toast.success(t('settings.profile.saved'));
    },
    onError: () => toast.error(t('settings.profile.saveError')),
  });

  const isLocal = me?.authSource === 'local';
  const dirty = me ? displayName !== (me.displayName ?? '') || email !== (me.email ?? '') : false;

  return (
    <div>
      <SubHeader title={t('settings.profile.title')} description={t('settings.profile.subtitle')} />

      <Section
        title={t('settings.profile.detailsTitle')}
        description={me?.authSource === 'oidc' ? t('settings.profile.oidcNote') : undefined}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t('settings.profile.username')}>
            <TextInput value={me?.username ?? ''} readOnly disabled />
          </Field>
          <Field label={t('settings.profile.role')}>
            <div className="flex h-9.5 items-center">
              <ToneTag tone={me?.role === 'operator' ? 'signal' : 'idle'} label={me?.role ?? '—'} />
            </div>
          </Field>
          <Field label={t('settings.profile.displayName')} htmlFor="profile-name">
            <TextInput
              id="profile-name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </Field>
          <Field label={t('settings.profile.email')} htmlFor="profile-email">
            <TextInput
              id="profile-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </Field>
        </div>
        <div className="mt-4 flex justify-end">
          <Button
            variant="primary"
            size="sm"
            disabled={!dirty || saveProfile.isPending}
            onClick={() => saveProfile.mutate()}
          >
            {t('common.save')}
          </Button>
        </div>
      </Section>

      {isLocal && <ChangePassword />}

      <SessionsSection />
    </div>
  );
}

function ChangePassword() {
  const { t } = useTranslation();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');

  const change = useMutation({
    mutationFn: () => postMePassword({ currentPassword: current, newPassword: next }),
    onSuccess: () => {
      toast.success(t('settings.profile.passwordChanged'));
      setCurrent('');
      setNext('');
      setConfirm('');
    },
    onError: () => toast.error(t('settings.profile.passwordError')),
  });

  const mismatch = next.length > 0 && confirm.length > 0 && next !== confirm;
  const canSubmit = current.length > 0 && next.length >= 8 && next === confirm && !change.isPending;

  return (
    <Section
      title={t('settings.profile.passwordTitle')}
      description={t('settings.profile.passwordDesc')}
    >
      <form
        className="grid gap-4 sm:grid-cols-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) change.mutate();
        }}
      >
        <Field label={t('settings.profile.currentPassword')} htmlFor="pw-current">
          <TextInput
            id="pw-current"
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
          />
        </Field>
        <Field label={t('settings.profile.newPassword')} htmlFor="pw-new">
          <TextInput
            id="pw-new"
            type="password"
            autoComplete="new-password"
            value={next}
            onChange={(e) => setNext(e.target.value)}
          />
        </Field>
        <Field
          label={t('settings.profile.confirmPassword')}
          htmlFor="pw-confirm"
          hint={mismatch ? t('settings.profile.passwordMismatch') : undefined}
        >
          <TextInput
            id="pw-confirm"
            type="password"
            autoComplete="new-password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
          />
        </Field>
        <div className="sm:col-span-3 flex justify-end">
          <Button type="submit" variant="primary" size="sm" disabled={!canSubmit}>
            {t('settings.profile.updatePassword')}
          </Button>
        </div>
      </form>
    </Section>
  );
}

function SessionsSection() {
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
