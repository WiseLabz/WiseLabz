/**
 * Mutation for an endpoint that may demand step-up (X-Elevation-Token). It is
 * tried bare first — whether step-up is on is the server's call — and only on
 * `elevation_required` does the returned dialog ask the caller to re-authenticate
 * before replaying the same variables with a fresh single-use token.
 */
import { useState } from 'react';
import type { ReactNode } from 'react';
import { useMutation } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import { Dialog } from '../ui/Dialog';
import { StepUp } from './StepUp';

export function isElevationRequired(err: unknown): boolean {
  return (
    isAxiosError(err) &&
    (err.response?.data as { code?: string } | undefined)?.code === 'elevation_required'
  );
}

/** Axios options carrying the elevation token, if there is one. */
export const elevationOptions = (token: string | null | undefined) =>
  token ? { headers: { 'X-Elevation-Token': token } } : undefined;

export function useStepUpMutation<V, R>({
  action,
  target,
  mutationFn,
  onSuccess,
  onError,
}: {
  action: string;
  /** Resource the token is bound to, derived from the variables. */
  target?: (vars: V) => string | undefined;
  mutationFn: (vars: V, token: string | null) => Promise<R>;
  onSuccess?: (result: R) => void;
  onError?: (err: unknown) => void;
}): { mutate: (vars: V) => void; isPending: boolean; dialog: ReactNode } {
  const { t } = useTranslation();
  const [pending, setPending] = useState<{ vars: V } | null>(null);

  const mutation = useMutation({
    mutationFn: ({ vars, token }: { vars: V; token: string | null }) => mutationFn(vars, token),
    onSuccess,
    onError: (err, { vars, token }) => {
      if (!token && isElevationRequired(err)) setPending({ vars });
      else onError?.(err);
    },
  });

  const dialog = (
    <Dialog
      open={pending !== null}
      onClose={() => setPending(null)}
      title={t('stepUp.title', { defaultValue: 'Confirm it’s you' })}
      size="sm"
    >
      {pending && (
        <StepUp
          action={action}
          target={target?.(pending.vars)}
          onElevated={(token) => {
            const { vars } = pending;
            setPending(null);
            mutation.mutate({ vars, token });
          }}
        />
      )}
    </Dialog>
  );

  return {
    mutate: (vars) => mutation.mutate({ vars, token: null }),
    isPending: mutation.isPending,
    dialog,
  };
}
