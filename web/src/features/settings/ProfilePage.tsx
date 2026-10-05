/**
 * Settings → Profile. Edit display name / email, change password (local accounts),
 * and review + revoke active sessions. Available to every authenticated user.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { getGetMeQueryKey, patchMe } from '../../api/generated/me/me';
import { useGetMe } from '../../api/generated/me/me';
import { UserDigestCadence } from '../../api/model/userDigestCadence';
import { Button } from '../../components/ui/Button';
import { ToneTag } from '../../components/ui/ToneTag';
import { toast } from '../../lib/toast';
import { SubHeader, Section, Field, TextInput, Select } from './parts';
import { ApiKeysSection } from './ProfileApiKeys';
import { SecuritySection } from './ProfileSecurity';
import { ChangePassword } from './ProfilePassword';
import { SessionsSection } from './ProfileSessions';

export function ProfilePage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: me } = useGetMe();

  const [displayName, setDisplayName] = useState('');
  const [email, setEmail] = useState('');
  const [digestCadence, setDigestCadence] =
    useState<(typeof UserDigestCadence)[keyof typeof UserDigestCadence]>('off');
  const [digestTimezone, setDigestTimezone] = useState('');
  // Adjust state during render (React-blessed alternative to a syncing effect):
  // re-seed whenever the query yields a fresh reference, e.g. after an invalidate.
  const [seeded, setSeeded] = useState<typeof me | null>(null);
  if (me && me !== seeded) {
    setSeeded(me);
    setDisplayName(me.displayName ?? '');
    setEmail(me.email ?? '');
    setDigestCadence(me.digestCadence ?? 'off');
    setDigestTimezone(me.digestTimezone || Intl.DateTimeFormat().resolvedOptions().timeZone);
  }

  const saveProfile = useMutation({
    mutationFn: () => patchMe({ displayName, email, digestCadence, digestTimezone }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetMeQueryKey() });
      toast.success(t('settings.profile.saved'));
    },
    onError: () => toast.error(t('settings.profile.saveError')),
  });

  const isLocal = me?.authSource === 'local';
  const dirty = me
    ? displayName !== (me.displayName ?? '') ||
      email !== (me.email ?? '') ||
      digestCadence !== (me.digestCadence ?? 'off') ||
      digestTimezone !== (me.digestTimezone ?? '')
    : false;

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
              <ToneTag tone={me?.role === 'admin' ? 'signal' : 'idle'} label={me?.role ?? '—'} />
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
          <Field label={t('settings.profile.digestCadence')} htmlFor="profile-digest-cadence">
            <Select
              id="profile-digest-cadence"
              value={digestCadence}
              onChange={(e) =>
                setDigestCadence(
                  e.target.value as (typeof UserDigestCadence)[keyof typeof UserDigestCadence]
                )
              }
            >
              <option value={UserDigestCadence.off}>
                {t('settings.profile.digestCadenceOff')}
              </option>
              <option value={UserDigestCadence.daily}>
                {t('settings.profile.digestCadenceDaily')}
              </option>
              <option value={UserDigestCadence.weekly}>
                {t('settings.profile.digestCadenceWeekly')}
              </option>
            </Select>
          </Field>
          <Field
            label={t('settings.profile.digestTimezone')}
            htmlFor="profile-digest-timezone"
            hint={t('settings.profile.digestTimezoneHint')}
          >
            <TextInput
              id="profile-digest-timezone"
              value={digestTimezone}
              onChange={(e) => setDigestTimezone(e.target.value)}
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

      {isLocal && <SecuritySection />}

      <ApiKeysSection />

      <SessionsSection />
    </div>
  );
}
