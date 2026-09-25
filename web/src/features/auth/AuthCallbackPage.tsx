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
import { OIDC_STEP_UP_FLAG, OIDC_STEP_UP_MESSAGE } from './oidcStepUpFlow';

// consumeOIDCStepUpFlag reads and clears the opener's flag in one step, so a
// popup the user closed without completing never leaves a stale flag behind
// to misclassify a later, ordinary login redirect in the same tab.
function consumeOIDCStepUpFlag(): boolean {
  try {
    const pending = sessionStorage.getItem(OIDC_STEP_UP_FLAG) === '1';
    sessionStorage.removeItem(OIDC_STEP_UP_FLAG);
    return pending;
  } catch {
    return false;
  }
}

export function AuthCallbackPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const loginOidc = useAuth((s) => s.loginOidc);
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return; // StrictMode double-invoke guard
    ran.current = true;

    const code = params.get('code') ?? '';
    const state = params.get('state') ?? '';

    // Read (and always clear) the flag first: a flag left behind by a popup
    // the user closed without completing must not misclassify a later,
    // ordinary login redirect in the same tab.
    const stepUpPending = consumeOIDCStepUpFlag();

    // OIDC step-up (#279 part 3): this page loaded inside the popup
    // OIDCStepUp opened, not the top-level login tab. Hand the code/state
    // back to the opener — it holds the access token this exchange needs —
    // and never run the login exchange here.
    if (stepUpPending && window.opener) {
      if (code && state) {
        window.opener.postMessage({ type: OIDC_STEP_UP_MESSAGE, code, state }, window.location.origin);
      }
      window.close();
      return;
    }

    const providerId = params.get('providerId') ?? '';

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
