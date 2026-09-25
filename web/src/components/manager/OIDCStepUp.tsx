/**
 * OIDC step-up re-authentication (#279 part 3): for a user whose account
 * signs in through an IdP and has no password to confirm with StepUp's
 * password/totp modes. Opens a popup that re-prompts the IdP for
 * credentials, and turns the code it comes back with into the same
 * short-lived elevation token POST /auth/elevate would issue.
 */
import {useEffect, useRef, useState} from 'react';
import {postAuthElevateOidcBegin, postAuthElevateOidcComplete} from '../../api/generated/auth/auth';
import {Button} from '../ui/Button';
import {
  OIDC_STEP_UP_FLAG,
  isOIDCStepUpMessage,
} from '../../features/auth/oidcStepUpFlow';

type Status = 'idle' | 'waiting' | 'completing' | 'error-blocked' | 'error-failed' | 'error-timeout';

// How long to wait for the popup to finish before giving up.
const TIMEOUT_MS = 3 * 60 * 1000;
// How often to check whether the popup was closed by the user.
const POLL_MS = 500;

export function OIDCStepUp({
                              action,
                              providerName,
                              onElevated,
                            }: {
  action: string;
  providerName?: string;
  onElevated: (token: string) => void;
}) {
  const [status, setStatus] = useState<Status>('idle');
  const popupRef = useRef<Window | null>(null);
  const cleanupRef = useRef<() => void>(() => {});

  // Always close any open popup and clear listeners/timers on unmount.
  useEffect(() => () => cleanupRef.current(), []);

  const start = () => {
    // Open synchronously in the click handler so browsers don't treat it as
    // an unrequested popup; navigate it once /begin resolves.
    const popup = window.open('about:blank', 'wiselabz-oidc-step-up', 'width=520,height=680');
    if (!popup) {
      setStatus('error-blocked');
      return;
    }
    popupRef.current = popup;
    setStatus('waiting');

    try {
      sessionStorage.setItem(OIDC_STEP_UP_FLAG, '1');
    } catch {
      // Best-effort; if storage is unavailable the callback page falls back
      // to treating this as a login redirect, which fails loudly rather
      // than silently, so we still try to proceed.
    }

    let settled = false;
    const finish = (fn: () => void) => {
      if (settled) return;
      settled = true;
      cleanupRef.current();
      fn();
    };

    const onMessage = (event: MessageEvent) => {
      if (event.origin !== window.location.origin) return;
      if (!isOIDCStepUpMessage(event.data)) return;
      finish(() => {
        setStatus('completing');
        popup.close();
        postAuthElevateOidcComplete({code: event.data.code, state: event.data.state})
          .then((res) => onElevated(res.token))
          .catch(() => setStatus('error-failed'));
      });
    };
    window.addEventListener('message', onMessage);

    const closedCheck = window.setInterval(() => {
      if (popup.closed) {
        finish(() => setStatus('error-failed'));
      }
    }, POLL_MS);

    const timeout = window.setTimeout(() => {
      finish(() => {
        popup.close();
        setStatus('error-timeout');
      });
    }, TIMEOUT_MS);

    cleanupRef.current = () => {
      window.removeEventListener('message', onMessage);
      window.clearInterval(closedCheck);
      window.clearTimeout(timeout);
    };

    postAuthElevateOidcBegin({action})
      .then((res) => {
        if (popup.closed) return;
        popup.location.href = res.authUrl;
      })
      .catch(() => {
        finish(() => {
          popup.close();
          setStatus('error-failed');
        });
      });
  };

  const label = providerName ? `Re-authenticate with ${providerName}` : 'Re-authenticate with your identity provider';

  return (
    <div className="rounded-md border border-line-soft bg-canvas-sunken p-3">
      <p className="mb-1.5 text-2xs text-ink-faint">
        Your account signs in through an identity provider, so confirm this in a popup instead of a
        password.
      </p>
      <Button
        type="button"
        variant="secondary"
        size="md"
        disabled={status === 'waiting' || status === 'completing'}
        onClick={start}
      >
        {status === 'waiting' ? 'Waiting for provider…' : status === 'completing' ? 'Verifying…' : label}
      </Button>
      {status === 'error-blocked' && (
        <p className="mt-1.5 text-2xs text-err">
          The popup was blocked. Allow popups for this site and try again.
        </p>
      )}
      {status === 'error-timeout' && (
        <p className="mt-1.5 text-2xs text-err">Re-authentication timed out. Try again.</p>
      )}
      {status === 'error-failed' && (
        <p className="mt-1.5 text-2xs text-err">
          Re-authentication failed or the popup was closed. Try again.
        </p>
      )}
    </div>
  );
}
