/**
 * Auth store. The access token lives IN MEMORY only (held by axios-instance, never
 * localStorage); the refresh token rides an HttpOnly cookie. On a 401 the axios
 * interceptor performs ONE silent refresh — we register that handler here. Mock-
 * backed for now (MSW): login/refresh/logout/oidc hit the curated handlers; the
 * real Go backend implements the same contract later.
 */
import { create } from 'zustand';
import type { AuthSession, OidcCallbackRequest, User } from '../api/model';
import {
  postAuthLogin,
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
  /** Resolve the session once on app load (silent refresh from the cookie). */
  bootstrap: () => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  loginOidc: (req: OidcCallbackRequest) => Promise<void>;
  logout: () => Promise<void>;
}

function apply(session: AuthSession): { status: Status; user: User } {
  setAccessToken(session.accessToken);
  return { status: 'authenticated', user: session.user };
}

function clear(): { status: Status; user: null } {
  setAccessToken(null);
  return { status: 'anonymous', user: null };
}

export const useAuth = create<AuthState>((set) => ({
  status: 'unknown',
  user: null,

  async bootstrap() {
    try {
      set(apply(await postAuthRefresh()));
    } catch {
      set(clear());
    }
  },

  async login(username, password) {
    set(apply(await postAuthLogin({ username, password })));
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
