import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import '../../i18n';
import { LoginPage } from './LoginPage';
import { useAuth } from '../../store/auth';

const { postAuthLoginMock, postAuthLoginMfaMock, beginWebAuthnMock, startAuthenticationMock } = vi.hoisted(() => ({
  postAuthLoginMock: vi.fn(),
  postAuthLoginMfaMock: vi.fn(),
  beginWebAuthnMock: vi.fn(),
  startAuthenticationMock: vi.fn(),
}));

vi.mock('@simplewebauthn/browser', () => ({ startAuthentication: startAuthenticationMock }));

vi.mock('../../api/generated/auth/auth', () => ({
  useGetAuthProviders: () => ({ data: { localEnabled: true, oidc: [] } }),
  postAuthLogin: postAuthLoginMock,
  postAuthLoginMfa: postAuthLoginMfaMock,
  postAuthLoginMfaWebauthnBegin: beginWebAuthnMock,
  postAuthLogout: vi.fn(),
  postAuthOidcCallback: vi.fn(),
  postAuthRefresh: vi.fn().mockRejectedValue(new Error('no session')),
}));

function renderLoginPage() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <LoginPage />
    </MemoryRouter>
  );
}

describe('LoginPage two-factor step (#279)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuth.setState({ status: 'anonymous', user: null, mfaTicket: null, mfaMethods: [] });
  });

  afterEach(cleanup);

  it('swaps to the code step when login returns mfaRequired', async () => {
    postAuthLoginMock.mockResolvedValue({ mfaRequired: true, ticket: 'tix-1', methods: ['totp', 'recovery'] });
    renderLoginPage();

    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'alice' } });
    fireEvent.change(screen.getByLabelText(/^password$/i), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => expect(screen.getByLabelText(/authenticator code/i)).toBeInTheDocument());
    expect(postAuthLoginMfaMock).not.toHaveBeenCalled();
  });

  it('toggles between the authenticator code and a recovery code', () => {
    useAuth.setState({ mfaTicket: 'tix-1', mfaMethods: ['totp', 'recovery'] });
    renderLoginPage();

    expect(screen.getByLabelText(/authenticator code/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /use a recovery code/i }));
    expect(screen.getByLabelText(/recovery code/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /use authenticator app instead/i }));
    expect(screen.getByLabelText(/authenticator code/i)).toBeInTheDocument();
  });

  it('submits the TOTP code via POST /auth/login/mfa and clears the pending ticket on success', async () => {
    postAuthLoginMfaMock.mockResolvedValue({
      accessToken: 'tok',
      expiresIn: 900,
      user: {
        id: 'u1',
        username: 'alice',
        role: 'viewer',
        authSource: 'local',
        createdAt: '2025-01-01T00:00:00Z',
      },
    });
    useAuth.setState({ mfaTicket: 'tix-1', mfaMethods: ['totp', 'recovery'] });
    renderLoginPage();

    fireEvent.change(screen.getByLabelText(/authenticator code/i), { target: { value: '123456' } });
    fireEvent.click(screen.getByRole('button', { name: /verify/i }));

    await waitFor(() =>
      expect(postAuthLoginMfaMock).toHaveBeenCalledWith({ ticket: 'tix-1', totp: '123456' })
    );
    await waitFor(() => expect(useAuth.getState().mfaTicket).toBeNull());
    expect(useAuth.getState().status).toBe('authenticated');
  });

  it('shows an error and keeps the ticket when the code is rejected', async () => {
    postAuthLoginMfaMock.mockRejectedValue(new Error('invalid code'));
    useAuth.setState({ mfaTicket: 'tix-1', mfaMethods: ['totp', 'recovery'] });
    renderLoginPage();

    fireEvent.change(screen.getByLabelText(/authenticator code/i), { target: { value: '000000' } });
    fireEvent.click(screen.getByRole('button', { name: /verify/i }));

    await waitFor(() => expect(screen.getByText(/not valid/i)).toBeInTheDocument());
    expect(useAuth.getState().mfaTicket).toBe('tix-1');
  });

  it('offers the security key before codes and quietly handles cancellation', async () => {
    useAuth.setState({ mfaTicket: 'tix-1', mfaMethods: ['recovery', 'webauthn'] });
    beginWebAuthnMock.mockResolvedValue({ publicKey: { challenge: 'challenge' } });
    const cancelled = new Error('cancelled');
    cancelled.name = 'NotAllowedError';
    startAuthenticationMock.mockRejectedValue(cancelled);
    renderLoginPage();

    fireEvent.click(screen.getByRole('button', { name: /use security key/i }));
    await waitFor(() => expect(startAuthenticationMock).toHaveBeenCalledWith({ optionsJSON: { challenge: 'challenge' } }));
    expect(postAuthLoginMfaMock).not.toHaveBeenCalled();
    expect(screen.queryByText(/not valid/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText(/recovery code/i)).toBeInTheDocument();
  });
});
