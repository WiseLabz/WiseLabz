/**
 * Shared contract between OIDCStepUp (the opener, which starts the popup)
 * and AuthCallbackPage (the popup, which lands on /auth/callback) for OIDC
 * step-up re-authentication (#279 part 3).
 *
 * The popup reuses the normal OIDC login redirect URL — no separate IdP
 * redirect URI needed — so AuthCallbackPage must tell a step-up popup apart
 * from an ordinary login redirect. A sessionStorage flag set by the opener
 * right before it opens the popup does that without changing the backend's
 * state format: sessionStorage is per-tab in most browsers, but a popup
 * opened with `window.open` from a tab shares that tab's session storage,
 * while a plain login redirects the top-level tab itself (never a popup),
 * so the flag it left behind is never observed by an unrelated tab.
 */

/** sessionStorage key the opener sets before calling window.open(). */
export const OIDC_STEP_UP_FLAG = 'wiselabz:oidc-step-up-pending';

/** postMessage type the popup sends back to the opener on success. */
export const OIDC_STEP_UP_MESSAGE = 'wiselabz:oidc-elevate';

export interface OIDCStepUpMessage {
  type: typeof OIDC_STEP_UP_MESSAGE;
  code: string;
  state: string;
}

export function isOIDCStepUpMessage(data: unknown): data is OIDCStepUpMessage {
  return (
    typeof data === 'object' &&
    data !== null &&
    (data as { type?: unknown }).type === OIDC_STEP_UP_MESSAGE &&
    typeof (data as { code?: unknown }).code === 'string' &&
    typeof (data as { state?: unknown }).state === 'string'
  );
}
