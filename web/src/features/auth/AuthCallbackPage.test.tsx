/**
 * #279 part 3: AuthCallbackPage's step-up branch. This page is shared by two
 * flows — a normal OIDC login redirect (top-level tab) and the popup
 * OIDCStepUp opens — and must tell them apart without running the login
 * exchange when it's really a step-up popup handing its code back to the
 * opener.
 */
import { cleanup, render, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthCallbackPage } from './AuthCallbackPage';
import { useAuth } from '../../store/auth';
import { OIDC_STEP_UP_MESSAGE } from './oidcStepUpFlow';

function renderCallback(search: string) {
  return render(
    <MemoryRouter initialEntries={[`/auth/callback${search}`]}>
      <Routes>
        <Route path="/auth/callback" element={<AuthCallbackPage />} />
        <Route path="/dashboard" element={<p>dashboard</p>} />
        <Route path="/login" element={<p>login</p>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AuthCallbackPage', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    Object.defineProperty(window, 'opener', { value: null, configurable: true });
    useAuth.setState({ status: 'unknown', user: null });
  });

  it('hands the code/state to the opener and closes, without running the login exchange', async () => {
    const postMessage = vi.fn();
    Object.defineProperty(window, 'opener', { value: { postMessage } as unknown as Window, configurable: true });
    const close = vi.spyOn(window, 'close').mockImplementation(() => {});
    const loginOidc = vi.fn().mockResolvedValue(undefined);
    useAuth.setState({ loginOidc } as never);

    renderCallback('?code=popup-code&state=wiselabz%3Aoidc-step-up%3Apopup-state');

    await waitFor(() => expect(postMessage).toHaveBeenCalled());
    expect(postMessage).toHaveBeenCalledWith(
      { type: OIDC_STEP_UP_MESSAGE, code: 'popup-code', state: 'wiselabz:oidc-step-up:popup-state' },
      window.location.origin,
    );
    expect(close).toHaveBeenCalled();
    expect(loginOidc).not.toHaveBeenCalled();
  });

  it('runs the ordinary login exchange when state has no step-up marker', async () => {
    const postMessage = vi.fn();
    Object.defineProperty(window, 'opener', { value: { postMessage } as unknown as Window, configurable: true });
    const loginOidc = vi.fn().mockResolvedValue(undefined);
    useAuth.setState({ loginOidc } as never);

    renderCallback('?providerId=authentik&code=login-code&state=login-state');

    await waitFor(() =>
      expect(loginOidc).toHaveBeenCalledWith({
        providerId: 'authentik',
        code: 'login-code',
        state: 'login-state',
      }),
    );
    expect(postMessage).not.toHaveBeenCalled();
  });

  it('does not run a login exchange for marked state without an opener', () => {
    Object.defineProperty(window, 'opener', { value: null, configurable: true });
    const loginOidc = vi.fn();
    useAuth.setState({ loginOidc } as never);
    const close = vi.spyOn(window, 'close').mockImplementation(() => {});
    renderCallback('?providerId=authentik&code=popup-code&state=wiselabz%3Aoidc-step-up%3Apopup-state');
    expect(loginOidc).not.toHaveBeenCalled();
    expect(close).toHaveBeenCalled();
  });
});
