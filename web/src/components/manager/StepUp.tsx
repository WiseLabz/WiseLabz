/**
 * Step-up re-authentication for a single destructive action. Fetches which
 * method(s) GET /auth/elevate/methods expects — "password" for a caller
 * without a confirmed second factor, "totp"/"recovery" once they have one
 * (#279 part 1), or "oidc" for an account that signs in through an IdP and
 * has no local password (#279 part 3, delegates to OIDCStepUp's popup flow)
 * — renders the matching input, calls POST /auth/elevate, and hands the
 * short-lived elevation token back to the caller, which replays it on the
 * destructive request. Self-contained: owns its own input and error state,
 * never persists the secret. PR 2 (WebAuthn) adds a further mode.
 */
import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { postAuthElevate, useGetAuthElevateMethods } from '../../api/generated/auth/auth';
import { Button } from '../ui/Button';
import { SkeletonRows } from '../ui/states';
import { OIDCStepUp } from './OIDCStepUp';

type Mode = 'password' | 'totp' | 'recovery';

export function StepUp({
  onElevated,
  action,
}: {
  onElevated: (token: string) => void;
  action: string;
}) {
  const { t } = useTranslation();
  const { data, isLoading } = useGetAuthElevateMethods();
  const [mode, setMode] = useState<Mode | null>(null);
  const [value, setValue] = useState('');

  const methods = data?.methods ?? ['password'];

  // methods is ["password"] or ["totp","recovery"] (see GET /auth/elevate/methods);
  // default to the first once loaded, but let the caller switch to recovery.
  const activeMode = mode ?? (methods.includes('totp') ? 'totp' : 'password');

  // Hooks must run unconditionally on every render, so useMutation is
  // declared before the isLoading/oidc early returns below — it's simply
  // unused in those branches.
  const elevate = useMutation({
    mutationFn: () =>
      postAuthElevate(
        activeMode === 'password'
          ? { password: value, action }
          : activeMode === 'recovery'
            ? { recoveryCode: value, action }
            : { totp: value, action }
      ),
    onSuccess: (res) => onElevated(res.token),
  });

  if (isLoading) {
    return <SkeletonRows rows={1} className="rounded-md border border-line-soft bg-canvas-sunken p-3" />;
  }

  if (methods.includes('oidc')) {
    return <OIDCStepUp action={action} onElevated={onElevated} />;
  }

  const label =
    activeMode === 'password'
      ? t('stepUp.password', { defaultValue: 'Confirm your password' })
      : activeMode === 'recovery'
        ? t('stepUp.recoveryCode', { defaultValue: 'Recovery code' })
        : t('stepUp.authenticatorCode', { defaultValue: 'Authenticator code' });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (value) elevate.mutate();
      }}
      className="rounded-md border border-line-soft bg-canvas-sunken p-3"
    >
      <label htmlFor="step-up-value" className="mb-1.5 block text-2xs text-ink-faint">
        {label}
      </label>
      <div className="flex items-center gap-2">
        <input
          id="step-up-value"
          type={activeMode === 'password' ? 'password' : 'text'}
          inputMode={activeMode === 'totp' ? 'numeric' : undefined}
          autoComplete={activeMode === 'password' ? 'current-password' : 'one-time-code'}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          autoFocus
          className="h-9 flex-1 rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none focus-visible:border-accent-primary-soft"
        />
        <Button type="submit" variant="secondary" size="md" disabled={!value || elevate.isPending}>
          {elevate.isPending
            ? t('stepUp.verifying', { defaultValue: 'Verifying…' })
            : t('stepUp.verify', { defaultValue: 'Verify' })}
        </Button>
      </div>
      {methods.includes('totp') && methods.includes('recovery') && (
        <button
          type="button"
          className="mt-1.5 text-2xs text-ink-faint underline-offset-2 hover:text-ink hover:underline"
          onClick={() => {
            setMode(activeMode === 'recovery' ? 'totp' : 'recovery');
            setValue('');
          }}
        >
          {activeMode === 'recovery'
            ? t('stepUp.useAuthenticator', { defaultValue: 'Use authenticator app instead' })
            : t('stepUp.useRecoveryCode', { defaultValue: 'Use a recovery code instead' })}
        </button>
      )}
      {elevate.isError && (
        <p className="mt-1.5 text-2xs text-err">
          {t('stepUp.failed', {
            defaultValue: 'Re-authentication failed. Check your {{what}} and try again.',
            what:
              activeMode === 'password'
                ? t('stepUp.password', { defaultValue: 'password' })
                : t('stepUp.code', { defaultValue: 'code' }),
          })}
        </p>
      )}
    </form>
  );
}
