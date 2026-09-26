import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import '../../i18n';
import { StepUp } from './StepUp';

const { elevateMock, beginWebAuthnMock, startAuthenticationMock } = vi.hoisted(() => ({
  elevateMock: vi.fn(), beginWebAuthnMock: vi.fn(), startAuthenticationMock: vi.fn(),
}));

vi.mock('@simplewebauthn/browser', () => ({ startAuthentication: startAuthenticationMock }));

let methods: string[] = ['password'];

vi.mock('../../api/generated/auth/auth', () => ({
  useGetAuthElevateMethods: () => ({ data: { methods }, isLoading: false }),
  postAuthElevate: elevateMock,
  postAuthElevateWebauthnBegin: beginWebAuthnMock,
}));

function renderStepUp(onElevated = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return {
    onElevated,
    ...render(
      <QueryClientProvider client={queryClient}>
        <StepUp action="connector.delete" onElevated={onElevated} />
      </QueryClientProvider>
    ),
  };
}

describe('StepUp mode switch (#279)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(cleanup);

  it('renders a password field when the caller has no confirmed factor', () => {
    methods = ['password'];
    renderStepUp();
    expect(screen.getByLabelText(/confirm your password/i)).toBeInTheDocument();
  });

  it('renders an authenticator code field, with a link to switch to a recovery code, once a factor is confirmed', () => {
    methods = ['totp', 'recovery'];
    renderStepUp();
    expect(screen.getByLabelText(/authenticator code/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /use a recovery code/i }));
    expect(screen.getByLabelText(/recovery code/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /use authenticator app instead/i }));
    expect(screen.getByLabelText(/authenticator code/i)).toBeInTheDocument();
  });

  it('submits the totp field to POST /auth/elevate and reports the token', async () => {
    methods = ['totp', 'recovery'];
    elevateMock.mockResolvedValue({ token: 'elev-token', expiresAt: '2025-01-01T00:01:00Z' });
    const { onElevated } = renderStepUp();

    fireEvent.change(screen.getByLabelText(/authenticator code/i), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: /verify/i }));

    await waitFor(() =>
      expect(elevateMock).toHaveBeenCalledWith({ totp: '123456', action: 'connector.delete' })
    );
    await waitFor(() => expect(onElevated).toHaveBeenCalledWith('elev-token'));
  });

  it('submits the password field to POST /auth/elevate when no factor is confirmed', async () => {
    methods = ['password'];
    elevateMock.mockResolvedValue({ token: 'elev-token', expiresAt: '2025-01-01T00:01:00Z' });
    renderStepUp();

    fireEvent.change(screen.getByLabelText(/confirm your password/i), { target: { value: 'hunter2' } });
    fireEvent.click(screen.getByRole('button', { name: /verify/i }));

    await waitFor(() =>
      expect(elevateMock).toHaveBeenCalledWith({ password: 'hunter2', action: 'connector.delete' })
    );
  });

  it('defaults to WebAuthn and treats browser cancellation as a quiet fallback', async () => {
    methods = ['totp', 'recovery', 'webauthn'];
    beginWebAuthnMock.mockResolvedValue({ publicKey: { challenge: 'challenge' } });
    const cancelled = new Error('cancelled');
    cancelled.name = 'NotAllowedError';
    startAuthenticationMock.mockRejectedValue(cancelled);
    renderStepUp();

    fireEvent.click(screen.getByRole('button', { name: /verify/i }));
    await waitFor(() => expect(startAuthenticationMock).toHaveBeenCalledWith({ optionsJSON: { challenge: 'challenge' } }));
    expect(elevateMock).not.toHaveBeenCalled();
    expect(screen.queryByText(/re-authentication failed/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /use authenticator app/i }));
    expect(screen.getByLabelText(/authenticator code/i)).toBeInTheDocument();
  });
});
