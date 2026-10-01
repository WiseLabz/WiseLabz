/** Settings → Profile → change password (local accounts). */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation } from '@tanstack/react-query';
import { postMePassword } from '../../api/generated/me/me';
import { Button } from '../../components/ui/Button';
import { toast } from '../../lib/toast';
import { Section, Field, TextInput } from './parts';

export function ChangePassword() {
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
