/**
 * OIDC redirect landing. Reads providerId/code/state from the query, exchanges them
 * for a session, then routes to the dashboard. On failure, back to login with a
 * toast. (Mock: the provider's authUrl points straight here with a stub code.)
 */
import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../../store/auth';
import { toast } from '../../lib/toast';
import { Splash } from './guards';

export function AuthCallbackPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const loginOidc = useAuth((s) => s.loginOidc);
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return; // StrictMode double-invoke guard
    ran.current = true;

    const providerId = params.get('providerId') ?? '';
    const code = params.get('code') ?? '';
    const state = params.get('state') ?? '';

    if (!providerId || !code) {
      toast.error(t('auth.oidcFailed'));
      navigate('/login', { replace: true });
      return;
    }

    void loginOidc({ providerId, code, state })
      .then(() => navigate('/dashboard', { replace: true }))
      .catch(() => {
        toast.error(t('auth.oidcFailed'));
        navigate('/login', { replace: true });
      });
  }, [params, loginOidc, navigate, t]);

  return <Splash />;
}
