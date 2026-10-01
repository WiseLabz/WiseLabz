/** State prefix issued by /auth/elevate/oidc/begin, preserved through the IdP redirect. */
export const OIDC_STEP_UP_STATE_PREFIX = 'wiselabz:oidc-step-up:';

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
