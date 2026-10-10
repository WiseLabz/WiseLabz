import { useEffect, useId, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import {
  deleteDocsImportPull,
  getDocsImportPull,
  postDocsImportPull,
} from '../../api/generated/docs/docs';
import type { DocImportPreview, DocPullRequest } from '../../api/model';
import { elevationOptions, useStepUpMutation } from '../manager/useStepUpMutation';
import { Button } from '../ui/Button';
import { toast } from '../../lib/toast';

const pullQueryKey = ['docs-import-pull'];

/** Credentials stay in this form and the POST body, never the query key or storage. */
export function BookStackPull({ onReady }: { onReady: (preview: DocImportPreview) => void }) {
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
    refetchInterval: (query) => (query.state.data?.state === 'fetching' ? 1000 : false),
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
    onError: () => toast.error(t('docs.import.pull.startError')),
  });
  const cancel = useMutation({
    mutationFn: () => deleteDocsImportPull(),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: pullQueryKey }),
    onError: () => toast.error(t('docs.import.pull.cancelError')),
  });
  const job = current.data;
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
        <p role="status" aria-live="polite">
          {t('docs.import.pull.progress', { done: job.done, total: job.total })}
        </p>
        <progress
          className="w-full"
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
          start.mutate({ source: 'bookstack', url, tokenId, tokenSecret, skipTlsVerify });
        }}
      >
        <p className="text-sm text-ink-muted">{t('docs.import.pull.intro')}</p>
        {job?.state === 'ready' && job.preview && (
          <Button type="button" variant="secondary" onClick={() => onReady(job.preview!)}>
            {t('docs.import.pull.review')}
          </Button>
        )}
        {job?.state === 'failed' && (
          <p role="alert" className="text-sm text-danger">
            {job.error || t('docs.import.pull.failed')}
          </p>
        )}
        {job?.state === 'cancelled' && (
          <p role="status" className="text-sm text-ink-muted">
            {t('docs.import.pull.cancelled')}
          </p>
        )}
        <label className="flex flex-col gap-1 text-sm" htmlFor={`${id}-url`}>
          {t('docs.import.pull.url')}
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
        <label className="flex flex-col gap-1 text-sm" htmlFor={`${id}-token-secret`}>
          {t('docs.import.pull.tokenSecret')}
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
