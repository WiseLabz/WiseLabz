import { useEffect, useId, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import {
  deleteDocsImportPull,
  getDocsImportPull,
  postDocsImportPull,
} from '../../api/generated/docs/docs';
import { DocPullRequestSource } from '../../api/model';
import type { DocImportPreview, DocPullRequest } from '../../api/model';
import { elevationOptions, useStepUpMutation } from '../manager/useStepUpMutation';
import { Button } from '../ui/Button';
import { toast } from '../../lib/toast';

const pullQueryKey = ['docs-import-pull'];

/** The server's message for a rejected start request (400), if it sent one. */
function startErrorMessage(err: unknown): string | undefined {
  if (!isAxiosError(err) || err.response?.status !== 400) return undefined;
  const message = (err.response.data as { message?: unknown } | undefined)?.message;
  return typeof message === 'string' && message ? message : undefined;
}

/**
 * Pull form and progress view shared by the wiki sources. Credentials stay in
 * this form and the POST body, never the query key or storage. Wiki.js has one
 * API key, sent as the token secret, and no token id.
 */
export function WikiPull({
  source,
  onReady,
}: {
  source: DocPullRequestSource;
  onReady: (preview: DocImportPreview) => void;
}) {
  const isWikiJs = source === DocPullRequestSource.wikijs;
  const k = isWikiJs ? 'docs.import.pullWikijs' : 'docs.import.pull';
  const { t } = useTranslation();
  const id = useId();
  const watching = useRef(false);
  const queryClient = useQueryClient();
  const [url, setUrl] = useState('');
  const [tokenId, setTokenId] = useState('');
  const [tokenSecret, setTokenSecret] = useState('');
  const [skipTlsVerify, setSkipTlsVerify] = useState(false);
  const current = useQuery({
    queryKey: pullQueryKey,
    queryFn: () => getDocsImportPull(),
    retry: false,
    refetchInterval: (query) =>
      query.state.status !== 'error' && query.state.data?.state === 'fetching' ? 1000 : false,
    staleTime: 0,
  });
  const start = useStepUpMutation<DocPullRequest, Awaited<ReturnType<typeof postDocsImportPull>>>({
    action: 'docs.import.pull',
    mutationFn: (request, token) => postDocsImportPull(request, elevationOptions(token)),
    onSuccess: (job) => {
      watching.current = true;
      setTokenId('');
      setTokenSecret('');
      queryClient.setQueryData(pullQueryKey, job);
      void queryClient.invalidateQueries({ queryKey: pullQueryKey });
    },
    onError: (err) => {
      setTokenId('');
      setTokenSecret('');
      if (isAxiosError(err) && err.response?.status === 409) {
        // Another pull is running: show its progress instead of the form.
        void queryClient.invalidateQueries({ queryKey: pullQueryKey });
      }
      toast.error(startErrorMessage(err) ?? t('docs.import.pull.startError'));
    },
  });
  const cancel = useMutation({
    mutationFn: () => deleteDocsImportPull(),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: pullQueryKey }),
    onError: () => toast.error(t('docs.import.pull.cancelError')),
  });
  // A failed read (404 when no pull exists) means no current job, not stale data.
  const job = current.isError ? undefined : current.data;
  const jobK =
    job?.source === DocPullRequestSource.wikijs ? 'docs.import.pullWikijs' : 'docs.import.pull';
  useEffect(() => {
    if (job?.state === 'fetching') watching.current = true;
    if (job?.state === 'ready' && job.preview && watching.current) {
      watching.current = false;
      onReady(job.preview);
    }
  }, [job, onReady]);

  if (job?.state === 'fetching') {
    return (
      <div className="flex flex-col gap-4">
        <p className="text-sm font-medium text-ink">{t(`${jobK}.jobSource`)}</p>
        <p role="status" aria-live="polite">
          {t(`${jobK}.progress`, { done: job.done, total: job.total })}
        </p>
        <progress
          className="w-full"
          aria-label={t(`${jobK}.progressLabel`)}
          value={job.total > 0 ? job.done : undefined}
          max={job.total || 1}
        />
        <Button
          type="button"
          variant="secondary"
          disabled={cancel.isPending}
          onClick={() => cancel.mutate()}
        >
          {cancel.isPending ? t('docs.import.pull.cancelling') : t('docs.import.pull.cancel')}
        </Button>
      </div>
    );
  }
  return (
    <>
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          start.mutate(
            isWikiJs
              ? { source, url, tokenSecret, skipTlsVerify }
              : { source, url, tokenId, tokenSecret, skipTlsVerify }
          );
        }}
      >
        <p className="text-sm text-ink-muted">{t(`${k}.intro`)}</p>
        {job && <p className="text-sm font-medium text-ink">{t(`${jobK}.jobSource`)}</p>}
        {job?.state === 'ready' && job.preview && (
          <Button type="button" variant="secondary" onClick={() => onReady(job.preview!)}>
            {t(`${jobK}.review`)}
          </Button>
        )}
        {job?.state === 'failed' && (
          <p role="alert" className="text-sm text-danger">
            {job.error || t(`${jobK}.failed`)}
          </p>
        )}
        {job?.state === 'cancelled' && (
          <p role="status" className="text-sm text-ink-muted">
            {t(`${jobK}.cancelled`)}
          </p>
        )}
        <label className="flex flex-col gap-1 text-sm" htmlFor={`${id}-url`}>
          {t(`${k}.url`)}
          <input
            id={`${id}-url`}
            type="url"
            required
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            className="rounded border border-line-soft bg-canvas-sunken p-2"
            autoComplete="off"
          />
        </label>
        {!isWikiJs && (
          <label className="flex flex-col gap-1 text-sm" htmlFor={`${id}-token-id`}>
            {t('docs.import.pull.tokenId')}
            <input
              id={`${id}-token-id`}
              required
              value={tokenId}
              onChange={(e) => setTokenId(e.target.value)}
              className="rounded border border-line-soft bg-canvas-sunken p-2"
              autoComplete="off"
            />
          </label>
        )}
        <label className="flex flex-col gap-1 text-sm" htmlFor={`${id}-token-secret`}>
          {t(`${k}.tokenSecret`)}
          <input
            id={`${id}-token-secret`}
            type="password"
            required
            value={tokenSecret}
            onChange={(e) => setTokenSecret(e.target.value)}
            className="rounded border border-line-soft bg-canvas-sunken p-2"
            autoComplete="new-password"
          />
        </label>
        <label className="flex items-center gap-2 text-sm" htmlFor={`${id}-skip-tls`}>
          <input
            id={`${id}-skip-tls`}
            type="checkbox"
            checked={skipTlsVerify}
            onChange={(e) => setSkipTlsVerify(e.target.checked)}
          />
          {t('docs.import.pull.skipTls')}
        </label>
        <Button type="submit" disabled={start.isPending || current.isLoading}>
          {start.isPending ? t('docs.import.pull.starting') : t('docs.import.pull.start')}
        </Button>
      </form>
      {start.dialog}
    </>
  );
}
