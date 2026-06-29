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
      toast.success(t('settings.profile.saved', { defaultValue: 'Profile updated.' }));
    },
    onError: () =>
      toast.error(t('settings.profile.saveError', { defaultValue: 'Could not save profile.' })),
  });

  const isLocal = me?.authSource === 'local';
  const dirty = me ? displayName !== (me.displayName ?? '') || email !== (me.email ?? '') : false;

  return (
    <div>
      <SubHeader
        title={t('settings.profile.title', { defaultValue: 'Profile' })}
        description={t('settings.profile.subtitle', {
          defaultValue: 'Your account details and active sessions.',
        })}
      />

      <Section
        title={t('settings.profile.detailsTitle', { defaultValue: 'Account details' })}
        description={
          me?.authSource === 'oidc'
            ? t('settings.profile.oidcNote', {
                defaultValue:
                  'Signed in via your identity provider; some fields are managed there.',
              })
            : undefined
        }
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t('settings.profile.username', { defaultValue: 'Username' })}>
            <TextInput value={me?.username ?? ''} readOnly disabled />
          </Field>
          <Field label={t('settings.profile.role', { defaultValue: 'Role' })}>
            <div className="flex h-9.5 items-center">
              <ToneTag tone={me?.role === 'operator' ? 'signal' : 'idle'} label={me?.role ?? '—'} />
            </div>
          </Field>
          <Field
            label={t('settings.profile.displayName', { defaultValue: 'Display name' })}
            htmlFor="profile-name"
          >
            <TextInput
              id="profile-name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </Field>
          <Field
            label={t('settings.profile.email', { defaultValue: 'Email' })}
            htmlFor="profile-email"
          >
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
            {t('common.save', { defaultValue: 'Save' })}
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
      toast.success(t('settings.profile.passwordChanged', { defaultValue: 'Password changed.' }));
      setCurrent('');
      setNext('');
      setConfirm('');
    },
    onError: () =>
      toast.error(
        t('settings.profile.passwordError', { defaultValue: 'Could not change password.' })
      ),
  });

  const mismatch = next.length > 0 && confirm.length > 0 && next !== confirm;
  const canSubmit = current.length > 0 && next.length >= 8 && next === confirm && !change.isPending;

  return (
    <Section
      title={t('settings.profile.passwordTitle', { defaultValue: 'Change password' })}
      description={t('settings.profile.passwordDesc', {
        defaultValue: 'Use at least 8 characters.',
      })}
    >
      <form
        className="grid gap-4 sm:grid-cols-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) change.mutate();
        }}
      >
        <Field
          label={t('settings.profile.currentPassword', { defaultValue: 'Current password' })}
          htmlFor="pw-current"
        >
          <TextInput
            id="pw-current"
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
          />
        </Field>
        <Field
          label={t('settings.profile.newPassword', { defaultValue: 'New password' })}
          htmlFor="pw-new"
        >
          <TextInput
            id="pw-new"
            type="password"
            autoComplete="new-password"
            value={next}
            onChange={(e) => setNext(e.target.value)}
          />
        </Field>
        <Field
          label={t('settings.profile.confirmPassword', { defaultValue: 'Confirm password' })}
          htmlFor="pw-confirm"
          hint={
            mismatch
              ? t('settings.profile.passwordMismatch', { defaultValue: 'Passwords do not match.' })
              : undefined
          }
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
            {t('settings.profile.updatePassword', { defaultValue: 'Update password' })}
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
      toast.success(t('settings.profile.sessionRevoked', { defaultValue: 'Session revoked.' }));
      setRevoke(null);
    },
    onError: () =>
      toast.error(
        t('settings.profile.sessionRevokeError', { defaultValue: 'Could not revoke session.' })
      ),
  });

  return (
    <Section
      title={t('settings.profile.sessionsTitle', { defaultValue: 'Active sessions' })}
      description={t('settings.profile.sessionsDesc', {
        defaultValue: 'Devices currently signed in to your account.',
      })}
    >
      {isLoading ? (
        <SkeletonRows rows={3} className="p-0" />
      ) : isError || !data ? (
        <ErrorState
          description={t('settings.profile.sessionsError', {
            defaultValue: 'Could not load sessions.',
          })}
          onRetry={() => refetch()}
        />
      ) : data.length === 0 ? (
        <EmptyState
          icon={<UserIcon size={18} />}
          title={t('settings.profile.noSessions', { defaultValue: 'No active sessions' })}
        />
      ) : (
        <ul className="divide-y divide-line-soft">
          {data.map((s) => (
            <li key={s.id} className="flex items-center gap-4 py-3 first:pt-0 last:pb-0">
              <div className="min-w-0 flex-1">
                <p className="flex items-center gap-2 text-sm text-ink">
                  <span className="truncate">
                    {s.userAgent ??
                      t('settings.profile.unknownDevice', { defaultValue: 'Unknown device' })}
                  </span>
                  {s.current && (
                    <ToneTag tone="ok" label={t('common.current', { defaultValue: 'current' })} />
                  )}
                </p>
                <p className="mt-0.5 flex items-center gap-1.5 font-mono text-2xs text-ink-faint">
                  <span>{s.ip ?? '—'}</span>
                  <span>·</span>
                  <span>
                    {t('settings.profile.lastSeen', { defaultValue: 'last seen' })}{' '}
                    <TimeAgo at={s.lastSeenAt} />
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
                  {t('settings.profile.revoke', { defaultValue: 'Revoke' })}
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
        title={t('settings.profile.revokeTitle', { defaultValue: 'Revoke session?' })}
        description={t('settings.profile.revokeConfirm', {
          defaultValue: 'That device will be signed out and need to log in again.',
        })}
        confirmLabel={t('settings.profile.revoke', { defaultValue: 'Revoke' })}
        cancelLabel={t('common.cancel', { defaultValue: 'Cancel' })}
        confirmDisabled={doRevoke.isPending}
      />
    </Section>
  );
}
