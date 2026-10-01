import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('../api/generated/auth/auth', async (orig) => ({
  ...(await orig<object>()),
  postAuthRefresh: vi.fn(),
}));

import { postAuthRefresh } from '../api/generated/auth/auth';
import { refreshSession, useAuth } from './auth';

const refresh = vi.mocked(postAuthRefresh);
afterEach(() => refresh.mockReset());

describe('refreshSession', () => {
  it('shares one in-flight refresh across concurrent callers', async () => {
    refresh.mockRejectedValue({ response: { status: 500 } });
    const [a, b] = await Promise.all([refreshSession(), refreshSession()]);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect([a, b]).toEqual([false, false]);
  });

  it('keeps the session on network/5xx errors', async () => {
    useAuth.setState({ status: 'authenticated' });
    refresh.mockRejectedValue(new Error('network'));
    await refreshSession();
    expect(useAuth.getState().status).toBe('authenticated');
  });

  it('clears the session when the refresh token is rejected', async () => {
    useAuth.setState({ status: 'authenticated' });
    refresh.mockRejectedValue({ response: { status: 401 } });
    await refreshSession();
    expect(useAuth.getState().status).not.toBe('authenticated');
  });
});
