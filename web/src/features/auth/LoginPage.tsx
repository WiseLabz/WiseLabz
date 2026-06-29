/**
 * Login. Local username/password plus any enabled OIDC providers. The access token
 * returned by login lives in memory (authStore); on success we route to the page the
 * user was headed for, or the dashboard.
 */
import { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../../store/auth';
import { useGetAuthProviders } from '../../api/generated/auth/auth';
import { Button } from '../../components/ui/Button';
import { toast } from '../../lib/toast';

const inputClass =
  'h-10 w-full rounded-sm border border-[var(--color-line)] bg-[var(--color-canvas-sunken)] px-3 text-sm text-[var(--color-ink)] outline-none transition-colors focus-visible:border-[var(--color-signal-soft)]';
const labelClass = 'mb-1.5 block text-2xs uppercase tracking-wider text-[var(--color-ink-faint)]';

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation() as { state?: { from?: string } };
  const login = useAuth((s) => s.login);
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
      navigate(from, { replace: true });
    } catch {
      toast.error(t('auth.loginFailed'));
      setBusy(false);
    }
  }

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-surface shadow-(--shadow-pop)">
      <div className="flex items-center gap-3 border-b border-line-soft px-6 py-5">
        <div className="flex h-9 w-9 items-center justify-center rounded-md bg-signal shadow-(--shadow-raised)">
          <span className="font-mono text-base font-bold text-signal-ink">W</span>
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
                <span className="font-mono text-2xs uppercase tracking-wider text-ink-faint">
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
