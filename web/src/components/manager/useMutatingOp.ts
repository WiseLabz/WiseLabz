/**
 * State machine for a dry-run-preview + elevation-confirm connector lifecycle
 * mutation (restart/start/stop) — the same shape ADR 0001/0002 give all
 * three, whether triggered directly on a connector (ServiceDetailPage) or
 * indirectly through a runbook step (RunbookPanel, #282). The caller supplies
 * the actual preview/execute calls (closing over the connector id, entityRef,
 * or runbook/step id it needs); {@link MutatingOpDialogs} in `LifecycleOp.tsx`
 * renders the dialogs this drives. Split into its own module (rather than
 * living alongside the component) so this hook doesn't break fast refresh for
 * that file.
 */
import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import type { RestartPreview } from '../../api/model';

export function useMutatingOp({
  previewFn,
  executeFn,
  onSuccess,
}: {
  previewFn: () => Promise<RestartPreview>;
  executeFn: (token: string | null) => Promise<unknown>;
  onSuccess?: () => void;
}) {
  const [previewOpen, setPreviewOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [resultData, setResultData] = useState<unknown>();
  const [failureData, setFailureData] = useState<unknown>();

  const preview = useMutation({ mutationFn: previewFn });

  const mutate = useMutation({
    mutationFn: async (token: string | null) => {
      setFailureData(undefined);
      try {
        const result = await executeFn(token);
        setResultData(result);
        return withoutExcerpt(result);
      } catch (error) {
        const response = (error as { response?: { data?: unknown; status?: number } } | null)
          ?.response;
        setFailureData(response?.data);
        // Axios errors also retain the raw response on request.responseText.
        // Cache only a new error and the safe response data, never the transport.
        if (response?.data && typeof response.data === 'object' && 'excerpt' in response.data) {
          throw Object.assign(new Error('Action request failed'), {
            response: { status: response.status, data: withoutExcerpt(response.data) },
          });
        }
        throw error;
      }
    },
    onSuccess: () => {
      setConfirmOpen(false);
      setPreviewOpen(false);
      onSuccess?.();
    },
    // The elevation token is single-use: close the confirm so the failure block
    // in the preview dialog is visible and the next confirm mints a fresh token.
    onError: () => setConfirmOpen(false),
  });

  const open = (options?: { skipPreview?: boolean }) => {
    preview.reset();
    mutate.reset();
    setResultData(undefined);
    setFailureData(undefined);
    setPreviewOpen(true);
    if (!options?.skipPreview) preview.mutate();
  };

  const rerunPreview = () => {
    preview.mutate();
  };

  const clearResult = () => setResultData(undefined);

  return {
    previewOpen,
    setPreviewOpen: (open: boolean) => {
      setPreviewOpen(open);
      if (!open) setFailureData(undefined);
    },
    confirmOpen,
    setConfirmOpen,
    preview,
    mutate,
    resultData,
    failureData,
    clearResult,
    open,
    rerunPreview,
  };
}

function withoutExcerpt(value: unknown): unknown {
  if (!value || typeof value !== 'object' || !('excerpt' in value)) return value;
  const copy: Record<string, unknown> = { ...value };
  delete copy.excerpt;
  return copy;
}

export type MutatingOp = ReturnType<typeof useMutatingOp>;
