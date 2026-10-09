import { act, renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { useMutatingOp } from './useMutatingOp';

const excerpt = 'RESPONSE-BODY-SECRET';

function renderOp(fails: boolean) {
  const client = new QueryClient();
  const executeFn = async () => {
    if (fails) {
      throw {
        response: { status: 502, data: { code: 'connector_error', statusCode: 409, excerpt } },
        request: { responseText: excerpt },
      };
    }
    return { statusCode: 200, excerpt };
  };
  const hook = renderHook(
    () =>
      useMutatingOp({
        previewFn: async () => ({
          targetService: 'API',
          estimatedDowntimeSeconds: 0,
          dependentServices: [],
        }),
        executeFn,
      }),
    {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    }
  );
  return { ...hook, client };
}

function cachedStates(client: QueryClient) {
  return JSON.stringify(
    client
      .getMutationCache()
      .getAll()
      .map((mutation) => mutation.state)
  );
}

describe('action response privacy', () => {
  it.each([false, true])(
    'keeps success/failure excerpt %s solely in dialog state',
    async (fails) => {
      const { result, client, unmount } = renderOp(fails);
      await act(async () => {
        await result.current.mutate.mutateAsync(null).catch(() => undefined);
      });
      expect(
        JSON.stringify(fails ? result.current.failureData : result.current.resultData)
      ).toContain(excerpt);
      expect(cachedStates(client)).not.toContain(excerpt);
      expect(cachedStates(client)).not.toContain('responseText');

      act(() => {
        if (fails) result.current.setPreviewOpen(false);
        else result.current.clearResult();
      });
      expect(result.current.resultData).toBeUndefined();
      expect(result.current.failureData).toBeUndefined();
      expect(cachedStates(client)).not.toContain(excerpt);
      unmount();
      expect(cachedStates(client)).not.toContain(excerpt);
      client.clear();
    }
  );
});

describe('failed action', () => {
  it('closes the spent elevation confirm but keeps the preview and failure visible', async () => {
    const { result, client, unmount } = renderOp(true);
    act(() => {
      result.current.open({ skipPreview: true });
      result.current.setConfirmOpen(true);
    });
    expect(result.current.confirmOpen).toBe(true);
    await act(async () => {
      await result.current.mutate.mutateAsync('token').catch(() => undefined);
    });
    expect(result.current.confirmOpen).toBe(false);
    expect(result.current.previewOpen).toBe(true);
    expect(JSON.stringify(result.current.failureData)).toContain(excerpt);
    unmount();
    client.clear();
  });
});
