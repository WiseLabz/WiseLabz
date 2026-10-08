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

  const preview = useMutation({ mutationFn: previewFn });

  const mutate = useMutation({
    mutationFn: (token: string | null) => executeFn(token),
    onSuccess: (result) => {
      setResultData(result);
      setConfirmOpen(false);
      setPreviewOpen(false);
      onSuccess?.();
    },
  });

  const open = (options?: { skipPreview?: boolean }) => {
    preview.reset();
    setResultData(undefined);
    setPreviewOpen(true);
    if (!options?.skipPreview) preview.mutate();
  };

  const rerunPreview = () => {
    preview.mutate();
  };

  const clearResult = () => setResultData(undefined);

  return {
    previewOpen,
    setPreviewOpen,
    confirmOpen,
    setConfirmOpen,
    preview,
    mutate,
    resultData,
    clearResult,
    open,
    rerunPreview,
  };
}

export type MutatingOp = ReturnType<typeof useMutatingOp>;
