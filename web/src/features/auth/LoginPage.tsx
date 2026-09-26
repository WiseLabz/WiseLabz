/**
 * Login. Local username/password plus any enabled OIDC providers. The access token
 * returned by login lives in memory (authStore); on success we route to the page the
 * user was headed for, or the dashboard.
 */
import { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../../store/auth';
import { postAuthLoginMfaWebauthnBegin, useGetAuthProviders } from '../../api/generated/auth/auth';
import type { WebAuthnResponse } from '../../api/model';
import { authenticateWebAuthn, isWebAuthnCancel } from '../../lib/webauthn';
import { Button } from '../../components/ui/Button';
import { toast } from '../../lib/toast';

const inputClass =
  'h-10 w-full rounded-sm border border-[var(--color-line)] bg-[var(--color-canvas-sunken)] px-3 text-sm text-[var(--color-ink)] outline-none transition-colors focus-visible:border-[var(--color-accent-primary-soft)]';
const labelClass = 'mb-1.5 block text-2xs text-[var(--color-ink-faint)]';

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as { state?: { from?: string } };
  const login = useAuth((s) => s.login);
  const mfaTicket = useAuth((s) => s.mfaTicket);
  const { data: providers } = useGetAuthProviders();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);

  const from = location.state?.from ?? '/dashboard';
  const oidc = providers?.oidc ?? [];
  const localEnabled = providers?.localEnabled ?? true;

  async function submit() {
    if (!username || !password) return;
    setBusy(true);
    try {
      await login(username, password);
      // A successful password step with no confirmed factor lands here with
      // a real session; login() only sets mfaTicket when a factor is required
      // (see LoginMfaStep below), so useAuth().status flips to 'authenticated'
      // and the router itself takes over from RequireAuth.
      if (!useAuth.getState().mfaTicket) {
        navigate(from, { replace: true });
      }
    } catch {
      toast.error(t('auth.loginFailed'));
    } finally {
      setBusy(false);
    }
  }

  if (mfaTicket) {
    return <LoginMfaStep onDone={() => navigate(from, { replace: true })} />;
  }

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-surface shadow-(--shadow-pop)">
      <div className="flex items-center gap-3 border-b border-line-soft px-6 py-5">
        <div className="flex h-9 w-9 items-center justify-center rounded-md bg-accent-primary shadow-(--shadow-raised)">
          <span className="font-mono text-base font-bold text-accent-primary-ink">W</span>
        </div>
        <div>
          <p className="text-sm font-semibold text-ink">{t('app.name')}</p>
          <p className="font-mono text-2xs text-ink-faint">{t('auth.signInPrompt')}</p>
        </div>
      </div>

      <div className="px-6 py-5">
        {localEnabled && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void submit();
            }}
            className="space-y-3.5"
          >
            <div>
              <label htmlFor="username" className={labelClass}>
                {t('auth.username')}
              </label>
              <input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                autoFocus
                className={inputClass}
              />
            </div>
            <div>
              <label htmlFor="password" className={labelClass}>
                {t('auth.password')}
              </label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                className={inputClass}
              />
            </div>
            <Button
              type="submit"
              variant="primary"
              size="md"
              disabled={busy || !username || !password}
              className="w-full justify-center"
            >
              {busy ? t('auth.signingIn') : t('auth.signIn')}
            </Button>
          </form>
        )}

        {oidc.length > 0 && (
          <>
            {localEnabled && (
              <div className="my-4 flex items-center gap-3">
                <span className="h-px flex-1 bg-line-soft" />
                <span className="font-mono text-2xs text-ink-faint">
                  {t('auth.or')}
                </span>
                <span className="h-px flex-1 bg-line-soft" />
              </div>
            )}
            <div className="space-y-2">
              {oidc.map((p) => (
                <Button
                  key={p.id}
                  variant="secondary"
                  size="md"
                  className="w-full justify-center"
                  onClick={() => window.location.assign(p.authUrl)}
                >
                  {t('auth.continueWith', { provider: p.displayName })}
                </Button>
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

/** Second step of a login that required a second factor (#279). */
function LoginMfaStep({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const submitMfa = useAuth((s) => s.submitMfa);
  const cancelMfa = useAuth((s) => s.cancelMfa);
  const ticket = useAuth((s) => s.mfaTicket);
  const methods = useAuth((s) => s.mfaMethods);
  const [useRecoveryCode, setUseRecoveryCode] = useState(!methods.includes('totp'));
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);

  async function submit() {
    if (!code) return;
    setBusy(true);
    setError(false);
    try {
      await submitMfa(useRecoveryCode ? { recoveryCode: code } : { totp: code });
      onDone();
    } catch {
      setError(true);
    } finally {
      setBusy(false);
    }
  }

  async function authenticateWithSecurityKey() {
    if (!ticket) return;
    setBusy(true);
    setError(false);
    try {
      const options = await postAuthLoginMfaWebauthnBegin({ ticket });
      const assertion = await authenticateWebAuthn(options);
      await submitMfa({ webauthn: assertion as unknown as WebAuthnResponse });
      onDone();
    } catch (err) {
      if (!isWebAuthnCancel(err)) setError(true);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-surface shadow-(--shadow-pop)">
      <div className="flex items-center gap-3 border-b border-line-soft px-6 py-5">
        <div className="flex h-9 w-9 items-center justify-center rounded-md bg-accent-primary shadow-(--shadow-raised)">
          <span className="font-mono text-base font-bold text-accent-primary-ink">W</span>
        </div>
        <div>
          <p className="text-sm font-semibold text-ink">
            {t('auth.mfa.title', { defaultValue: 'Two-factor authentication' })}
          </p>
          <p className="font-mono text-2xs text-ink-faint">
            {useRecoveryCode
              ? t('auth.mfa.recoveryPrompt', { defaultValue: 'Enter a recovery code' })
              : t('auth.mfa.totpPrompt', { defaultValue: 'Enter your authenticator code' })}
          </p>
        </div>
      </div>

      <div className="px-6 py-5">
        {methods.includes('webauthn') && (
          <Button type="button" variant="primary" size="md" disabled={busy} className="mb-4 w-full justify-center" onClick={() => void authenticateWithSecurityKey()}>
            {t('auth.mfa.useSecurityKey', { defaultValue: 'Use security key' })}
          </Button>
        )}
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
          className="space-y-3.5"
        >
          <div>
            <label htmlFor="mfa-code" className={labelClass}>
              {useRecoveryCode
                ? t('auth.mfa.recoveryCode', { defaultValue: 'Recovery code' })
                : t('auth.mfa.code', { defaultValue: 'Authenticator code' })}
            </label>
            <input
              id="mfa-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              inputMode={useRecoveryCode ? undefined : 'numeric'}
              autoComplete="one-time-code"
              autoFocus
              className={inputClass}
            />
            {error && (
              <p className="mt-1.5 text-2xs text-err">
                {t('auth.mfa.invalidCode', { defaultValue: 'That code is not valid. Try again.' })}
              </p>
            )}
          </div>
          <Button
            type="submit"
            variant="primary"
            size="md"
            disabled={busy || !code}
            className="w-full justify-center"
          >
            {busy
              ? t('auth.signingIn')
              : t('auth.mfa.verify', { defaultValue: 'Verify' })}
          </Button>
        </form>

        <div className="mt-4 flex items-center justify-between text-2xs">
          {methods.includes('totp') && <button
            type="button"
            className="text-ink-faint underline-offset-2 hover:text-ink hover:underline"
            onClick={() => {
              setUseRecoveryCode((v) => !v);
              setCode('');
              setError(false);
            }}
          >
            {useRecoveryCode
              ? t('auth.mfa.useAuthenticator', { defaultValue: 'Use authenticator app instead' })
              : t('auth.mfa.useRecoveryCode', { defaultValue: 'Use a recovery code' })}
          </button>}
          <button
            type="button"
            className="text-ink-faint underline-offset-2 hover:text-ink hover:underline"
            onClick={cancelMfa}
          >
            {t('common.cancel')}
          </button>
        </div>
      </div>
    </div>
  );
}
