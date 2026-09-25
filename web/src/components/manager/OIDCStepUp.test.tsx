/**
 * #279 part 3: the OIDC step-up popup flow. Covers the two things that are
 * easy to get wrong in a postMessage-based flow — accepting a message from
 * the wrong origin, and the browser blocking the popup outright — plus the
 * happy path end to end.
 */
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { OIDCStepUp } from './OIDCStepUp';
import { OIDC_STEP_UP_MESSAGE } from '../../features/auth/oidcStepUpFlow';

const postAuthElevateOidcBegin = vi.fn();
const postAuthElevateOidcComplete = vi.fn();

vi.mock('../../api/generated/auth/auth', () => ({
  postAuthElevateOidcBegin: (...args: unknown[]) => postAuthElevateOidcBegin(...args),
  postAuthElevateOidcComplete: (...args: unknown[]) => postAuthElevateOidcComplete(...args),
}));

function fakePopup() {
  return {
    closed: false,
    close: vi.fn(),
    location: { href: '' },
  };
}

describe('OIDCStepUp', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  beforeEach(() => {
    postAuthElevateOidcBegin.mockReset();
    postAuthElevateOidcComplete.mockReset();
  });

  it('shows a blocked-popup message when window.open returns null', async () => {
    vi.spyOn(window, 'open').mockReturnValue(null);
    const onElevated = vi.fn();
    render(<OIDCStepUp action="connector.delete" onElevated={onElevated} />);

    fireEvent.click(screen.getByRole('button'));

    expect(await screen.findByText(/popup was blocked/i)).toBeInTheDocument();
    expect(postAuthElevateOidcBegin).not.toHaveBeenCalled();
  });

  it('ignores a postMessage from an unexpected origin', async () => {
    const popup = fakePopup();
    vi.spyOn(window, 'open').mockReturnValue(popup as unknown as Window);
    postAuthElevateOidcBegin.mockResolvedValue({ authUrl: 'https://idp.example.com/authorize' });
    const onElevated = vi.fn();
    render(<OIDCStepUp action="connector.delete" onElevated={onElevated} />);

    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(popup.location.href).toBe('https://idp.example.com/authorize'));

    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://attacker.example.com',
        data: { type: OIDC_STEP_UP_MESSAGE, code: 'evil-code', state: 'evil-state' },
      }),
    );

    // Give any (incorrect) handling a tick to run.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(postAuthElevateOidcComplete).not.toHaveBeenCalled();
    expect(onElevated).not.toHaveBeenCalled();
  });

  it('completes elevation on a same-origin message', async () => {
    const popup = fakePopup();
    vi.spyOn(window, 'open').mockReturnValue(popup as unknown as Window);
    postAuthElevateOidcBegin.mockResolvedValue({ authUrl: 'https://idp.example.com/authorize' });
    postAuthElevateOidcComplete.mockResolvedValue({ token: 'elevation-token', expiresAt: '2026-01-01T00:00:00Z' });
    const onElevated = vi.fn();
    render(<OIDCStepUp action="connector.delete" providerName="Authentik" onElevated={onElevated} />);

    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(popup.location.href).toBe('https://idp.example.com/authorize'));

    window.dispatchEvent(
      new MessageEvent('message', {
        origin: window.location.origin,
        data: { type: OIDC_STEP_UP_MESSAGE, code: 'good-code', state: 'good-state' },
      }),
    );

    await waitFor(() => expect(onElevated).toHaveBeenCalledWith('elevation-token'));
    expect(postAuthElevateOidcComplete).toHaveBeenCalledWith({ code: 'good-code', state: 'good-state' });
    expect(popup.close).toHaveBeenCalled();
  });
});
