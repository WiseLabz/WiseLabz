/**
 * Auth store. The access token lives IN MEMORY only (held by axios-instance, never
 * localStorage); the refresh token rides an HttpOnly cookie. On a 401 the axios
 * interceptor performs ONE silent refresh — we register that handler here. Mock-
 * backed for now (MSW): login/refresh/logout/oidc hit the curated handlers; the
 * real Go backend implements the same contract later.
 *
 * Two-factor login (#279): postAuthLogin returns either a normal AuthSession or
 * a LoginMfaRequired envelope. In the latter case no session exists yet — we
 * stash the ticket + methods and LoginPage renders a second step that calls
 * submitMfa, which finishes the login via POST /auth/login/mfa.
 */
import { create } from 'zustand';
import type { AuthSession, LoginMfaRequired, OidcCallbackRequest, User, WebAuthnResponse } from '../api/model';
import {
  postAuthLogin,
  postAuthLoginMfa,
  postAuthLogout,
  postAuthOidcCallback,
  postAuthRefresh,
} from '../api/generated/auth/auth';
import { setAccessToken, setRefreshHandler } from '../api/axios-instance';
import { queryClient } from '../app/queryClient';

type Status = 'unknown' | 'authenticated' | 'anonymous';

interface AuthState {
  status: Status;
  user: User | null;
  /** Set instead of a session by POST /auth/login when the user has a confirmed factor. */
  mfaTicket: string | null;
  mfaMethods: string[];
  /** Resolve the session once on app load (silent refresh from the cookie). */
  bootstrap: () => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  /** Finishes a login that returned mfaTicket, with exactly one of totp/recoveryCode. */
  submitMfa: (input: { totp?: string; recoveryCode?: string; webauthn?: WebAuthnResponse }) => Promise<void>;
  /** Abandons the pending MFA step (e.g. "use a different account"). */
  cancelMfa: () => void;
  loginOidc: (req: OidcCallbackRequest) => Promise<void>;
  logout: () => Promise<void>;
}

function apply(session: AuthSession): { status: Status; user: User; mfaTicket: null; mfaMethods: never[] } {
  setAccessToken(session.accessToken);
  return { status: 'authenticated', user: session.user, mfaTicket: null, mfaMethods: [] };
}

function isMfaRequired(result: AuthSession | LoginMfaRequired): result is LoginMfaRequired {
  return 'mfaRequired' in result && result.mfaRequired === true;
}

function clear(): { status: Status; user: null; mfaTicket: null; mfaMethods: never[] } {
  setAccessToken(null);
  return { status: 'anonymous', user: null, mfaTicket: null, mfaMethods: [] };
}

export const useAuth = create<AuthState>((set, get) => ({
  status: 'unknown',
  user: null,
  mfaTicket: null,
  mfaMethods: [],

  async bootstrap() {
    try {
      set(apply(await postAuthRefresh()));
    } catch {
      set(clear());
    }
  },

  async login(username, password) {
    const result = await postAuthLogin({ username, password });
    if (isMfaRequired(result)) {
      set({ mfaTicket: result.ticket, mfaMethods: result.methods });
      return;
    }
    set(apply(result));
  },

  async submitMfa(input) {
    const ticket = get().mfaTicket;
    if (!ticket) throw new Error('no pending MFA login');
    set(apply(await postAuthLoginMfa({ ticket, ...input })));
  },

  cancelMfa() {
    set({ mfaTicket: null, mfaMethods: [] });
  },

  async loginOidc(req) {
    set(apply(await postAuthOidcCallback(req)));
  },

  async logout() {
    try {
      await postAuthLogout();
    } catch {
      // best-effort; clear the client session regardless
    }
    queryClient.clear();
    set(clear());
  },
}));

// Register the interceptor's silent-refresh handler once. Returning true tells the
// interceptor to retry the original request with the new token.
setRefreshHandler(async () => {
  try {
    const session = await postAuthRefresh();
    useAuth.setState(apply(session));
    return true;
  } catch {
    useAuth.setState(clear());
    return false;
  }
});
