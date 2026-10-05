import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest';
import type { AuthSession, User } from '../api/model';

vi.mock('../api/generated/auth/auth', async (orig) => ({
  ...(await orig<object>()),
  postAuthLogin: vi.fn(),
  postAuthLoginMfa: vi.fn(),
  postAuthLogout: vi.fn(),
  postAuthOidcCallback: vi.fn(),
  postAuthRefresh: vi.fn(),
}));

vi.mock('../api/axios-instance', () => ({
  setAccessToken: vi.fn(),
  setRefreshHandler: vi.fn(),
}));

vi.mock('../app/queryClient', () => ({
  queryClient: {
    clear: vi.fn(),
  },
}));

import {
  postAuthLogin,
  postAuthLoginMfa,
  postAuthLogout,
  postAuthOidcCallback,
  postAuthRefresh,
} from '../api/generated/auth/auth';
import { setAccessToken } from '../api/axios-instance';
import { queryClient } from '../app/queryClient';
import { useAuth } from './auth';

const mockRefresh = vi.mocked(postAuthRefresh);
const mockLogin = vi.mocked(postAuthLogin);
const mockMfa = vi.mocked(postAuthLoginMfa);
const mockLogout = vi.mocked(postAuthLogout);
const mockOidc = vi.mocked(postAuthOidcCallback);
const mockSetAccessToken = vi.mocked(setAccessToken);
const mockClearQueryClient = vi.mocked(queryClient.clear);

const mockSession: AuthSession = {
  accessToken: 'test-token',
  expiresIn: 3600,
  user: {
    id: 'user123',
    username: 'testuser',
    email: 'test@example.com',
    role: 'admin',
    authSource: 'local',
    createdAt: '2026-10-01T00:00:00Z',
  } as User,
};

beforeEach(() => {
  useAuth.setState({
    status: 'unknown',
    user: null,
    mfaTicket: null,
    mfaMethods: [],
  });
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('useAuth', () => {
  describe('bootstrap', () => {
    it('sets authenticated status on successful refresh', async () => {
      mockRefresh.mockResolvedValue(mockSession);
      await useAuth.getState().bootstrap();
      expect(useAuth.getState().status).toBe('authenticated');
      expect(useAuth.getState().user).toEqual(mockSession.user);
    });

    it('sets anonymous status on refresh failure', async () => {
      mockRefresh.mockRejectedValue(new Error('Network error'));
      await useAuth.getState().bootstrap();
      expect(useAuth.getState().status).toBe('anonymous');
      expect(useAuth.getState().user).toBeNull();
    });

    it('sets access token on successful bootstrap', async () => {
      mockRefresh.mockResolvedValue(mockSession);
      await useAuth.getState().bootstrap();
      expect(mockSetAccessToken).toHaveBeenCalledWith('test-token');
    });

    it('clears access token on failed bootstrap', async () => {
      mockRefresh.mockRejectedValue(new Error('Unauthorized'));
      await useAuth.getState().bootstrap();
      expect(mockSetAccessToken).toHaveBeenCalledWith(null);
    });
  });

  describe('login', () => {
    it('authenticates on successful login', async () => {
      mockLogin.mockResolvedValue(mockSession);
      await useAuth.getState().login('user@example.com', 'password123');
      expect(useAuth.getState().status).toBe('authenticated');
      expect(useAuth.getState().user).toEqual(mockSession.user);
    });

    it('sets MFA ticket when MFA is required', async () => {
      mockLogin.mockResolvedValue({
        mfaRequired: true,
        ticket: 'mfa-ticket-123',
        methods: ['totp', 'recovery'],
      });
      await useAuth.getState().login('user@example.com', 'password123');
      expect(useAuth.getState().status).toBe('unknown');
      expect(useAuth.getState().mfaTicket).toBe('mfa-ticket-123');
      expect(useAuth.getState().mfaMethods).toEqual(['totp', 'recovery']);
    });

    it('does not set access token when MFA is required', async () => {
      mockLogin.mockResolvedValue({
        mfaRequired: true,
        ticket: 'mfa-ticket-123',
        methods: ['totp'],
      });
      await useAuth.getState().login('user@example.com', 'password123');
      expect(mockSetAccessToken).not.toHaveBeenCalled();
    });

    it('calls postAuthLogin with credentials', async () => {
      mockLogin.mockResolvedValue(mockSession);
      await useAuth.getState().login('testuser', 'pass123');
      expect(mockLogin).toHaveBeenCalledWith({ username: 'testuser', password: 'pass123' });
    });
  });

  describe('submitMfa', () => {
    beforeEach(() => {
      useAuth.setState({
        mfaTicket: 'pending-ticket',
        mfaMethods: ['totp'],
      });
    });

    it('completes login with TOTP code', async () => {
      mockMfa.mockResolvedValue(mockSession);
      await useAuth.getState().submitMfa({ totp: '123456' });
      expect(useAuth.getState().status).toBe('authenticated');
      expect(useAuth.getState().mfaTicket).toBeNull();
    });

    it('completes login with recovery code', async () => {
      mockMfa.mockResolvedValue(mockSession);
      await useAuth.getState().submitMfa({ recoveryCode: 'RECOVERY-001' });
      expect(mockMfa).toHaveBeenCalledWith({
        ticket: 'pending-ticket',
        recoveryCode: 'RECOVERY-001',
      });
    });

    it('throws error when no MFA ticket is pending', async () => {
      useAuth.setState({ mfaTicket: null });
      await expect(useAuth.getState().submitMfa({ totp: '123456' })).rejects.toThrow(
        'no pending MFA login'
      );
    });

    it('clears MFA state on successful submission', async () => {
      mockMfa.mockResolvedValue(mockSession);
      await useAuth.getState().submitMfa({ totp: '123456' });
      expect(useAuth.getState().mfaTicket).toBeNull();
      expect(useAuth.getState().mfaMethods).toEqual([]);
    });
  });

  describe('cancelMfa', () => {
    it('clears MFA ticket and methods', () => {
      useAuth.setState({
        mfaTicket: 'some-ticket',
        mfaMethods: ['totp', 'recovery_code'],
      });
      useAuth.getState().cancelMfa();
      expect(useAuth.getState().mfaTicket).toBeNull();
      expect(useAuth.getState().mfaMethods).toEqual([]);
    });
  });

  describe('loginOidc', () => {
    it('authenticates via OIDC callback', async () => {
      mockOidc.mockResolvedValue(mockSession);
      await useAuth.getState().loginOidc({ providerId: 'okta', code: 'auth-code', state: 'state-value' });
      expect(useAuth.getState().status).toBe('authenticated');
      expect(useAuth.getState().user).toEqual(mockSession.user);
    });

    it('sets access token on successful OIDC login', async () => {
      mockOidc.mockResolvedValue(mockSession);
      await useAuth.getState().loginOidc({ providerId: 'okta', code: 'auth-code', state: 'state-value' });
      expect(mockSetAccessToken).toHaveBeenCalledWith('test-token');
    });
  });

  describe('logout', () => {
    beforeEach(() => {
      useAuth.setState({
        status: 'authenticated',
        user: mockSession.user,
      });
    });

    it('clears session and query client on successful logout', async () => {
      mockLogout.mockResolvedValue(undefined);
      await useAuth.getState().logout();
      expect(useAuth.getState().status).toBe('anonymous');
      expect(useAuth.getState().user).toBeNull();
      expect(mockClearQueryClient).toHaveBeenCalled();
    });

    it('clears session even when logout API fails', async () => {
      mockLogout.mockRejectedValue(new Error('Network error'));
      await useAuth.getState().logout();
      expect(useAuth.getState().status).toBe('anonymous');
      expect(useAuth.getState().user).toBeNull();
      expect(mockClearQueryClient).toHaveBeenCalled();
    });

    it('clears access token', async () => {
      mockLogout.mockResolvedValue(undefined);
      await useAuth.getState().logout();
      expect(mockSetAccessToken).toHaveBeenCalledWith(null);
    });

    it('does not throw on logout API failure', async () => {
      mockLogout.mockRejectedValue({ response: { status: 500 } });
      await expect(useAuth.getState().logout()).resolves.not.toThrow();
    });

    it('clears MFA state during logout', async () => {
      useAuth.setState({
        mfaTicket: 'pending-ticket',
        mfaMethods: ['totp'],
      });
      mockLogout.mockResolvedValue(undefined);
      await useAuth.getState().logout();
      expect(useAuth.getState().mfaTicket).toBeNull();
      expect(useAuth.getState().mfaMethods).toEqual([]);
    });
  });
});
