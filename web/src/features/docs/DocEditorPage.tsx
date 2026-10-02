/**
 * Freehand + AI-assisted doc editor (`/docs/:docId/edit`). CodeMirror 6 on the
 * left, live Markdown preview on the right; save writes a new version
 * (with stale-version detection). If the doc is regenerated elsewhere while editing, a
 * "newer version available" banner offers an explicit reload before saving.
 *
 * AI assist is batched (no streaming, per the plan): "Suggest update" calls the
 * server once, then renders the proposed revision as a review-diff. Accept applies it
 * to the editor and marks the draft as AI-drafted (provenance); Reject discards it.
 * Operator-gated (the route guards, and the save button respects role too).
 */
import { useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { skipToken, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import CodeMirror from '@uiw/react-codemirror';
import { markdown } from '@codemirror/lang-markdown';
import { EditorView } from '@codemirror/view';
import {
  useGetDocsDocId,
  putDocsDocId,
  postDocsDocIdAiSuggest,
  postDocsDocIdLock,
  postDocsDocIdLockRelease,
  getGetDocsDocIdQueryKey,
  getGetDocsDocIdVersionsQueryKey,
  getGetDocsTreeQueryKey,
} from '../../api/generated/docs/docs';
import type { DocAiSuggestionPayload } from '../../types/ws';
import { useConnectorRole, useIsInstanceAdmin } from '../../hooks/useRole';
import { useAuth } from '../../store/auth';
import { useLive } from '../../store/live';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Skeleton, SkeletonRows, ErrorState } from '../../components/ui/states';
import { Markdown } from '../../components/docs/Markdown';
import { genBlockHighlight } from '../../components/docs/genBlockHighlight';
import { DocDiff } from '../../components/diff/DiffViewer';
import { toast } from '../../lib/toast';
import { fullDate, relativeTime } from '../../lib/time';
import {
  ArrowRightIcon,
  SparklesIcon,
  CheckIcon,
  XIcon,
  FileTextIcon,
} from '../../components/icons';

// Compact CodeMirror theme that sits on the app's dark chrome instead of the
// library's default white surface.
const cmTheme = EditorView.theme(
  {
    '&': { backgroundColor: 'transparent', color: 'var(--color-ink)', fontSize: '13px' },
    '.cm-content': {
      fontFamily: 'var(--font-mono, monospace)',
      caretColor: 'var(--color-accent-primary)',
    },
    '.cm-gutters': {
      backgroundColor: 'transparent',
      color: 'var(--color-ink-faint)',
      border: 'none',
    },
    '.cm-activeLine': { backgroundColor: 'var(--color-surface-raised)' },
    '.cm-activeLineGutter': { backgroundColor: 'var(--color-surface-raised)' },
    '&.cm-focused': { outline: 'none' },
    '.cm-selectionBackground, ::selection': { backgroundColor: 'var(--color-accent-primary-tint)' },
  },
  { dark: true }
);

type Provenance = 'manual' | 'ai-draft';

export function DocEditorPage() {
  const { docId } = useParams();
  return <DocEditor key={docId} />;
}

function DocEditor() {
  const { t } = useTranslation();
  const { docId = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const isInstanceAdmin = useIsInstanceAdmin();

  const userId = useAuth((s) => s.user?.id);
  const docLock = useLive((s) => s.docLocks[docId]);

  const doc = useGetDocsDocId(docId);
  const extensions = useMemo(
    () => [
      markdown(),
      cmTheme,
      EditorView.lineWrapping,
      EditorView.contentAttributes.of({ 'aria-label': t('docs.editor.markdownLabel') }),
      genBlockHighlight(t('docs.editor.generatedHint')),
    ],
    [t]
  );
  // Service docs are gated on that connector's operator role; the lab
  // overview doc has no owning connector, so it falls back to instance-admin.
  const connectorRole = useConnectorRole(doc.data?.serviceId ?? undefined);
  const canMutate = doc.data?.serviceId ? connectorRole === 'operator' : isInstanceAdmin;

  const [draft, setDraft] = useState<string | null>(null);
  const [provenance, setProvenance] = useState<Provenance>('manual');
  const [requestId, setRequestId] = useState<string | null>(null);
  const aiResult = useQuery<DocAiSuggestionPayload>({
    queryKey: ['doc-ai-suggestion', docId, requestId],
    queryFn: skipToken,
  });
  const suggestion =
    aiResult.data?.status === 'complete' ? (aiResult.data.fullContent ?? null) : null;
  const [baseContent, setBaseContent] = useState<string | null>(null);
  const [lockHeld, setLockHeld] = useState(false);
  // State (not refs) so `newerAvailable` can derive from them during render.
  const [baseVersion, setBaseVersion] = useState<number | null>(null);
  const [justSaved, setJustSaved] = useState(false);
  const lockAcquiredRef = useRef(false);
  const mountedRef = useRef(true);

  // Seed the draft once the doc loads; capture the version the edit is based on.
  // Adjusting state during render is the React-blessed alternative to an effect.
  const [seeded, setSeeded] = useState(false);
  if (doc.data && !seeded) {
    setSeeded(true);
    setDraft(doc.data.content);
    setBaseContent(doc.data.content);
    setBaseVersion(doc.data.currentVersion);
  }

  const dirty = draft !== null && doc.data != null && draft !== baseContent;
  const deferredDraft = useDeferredValue(draft ?? '');

  const dirtyRef = useRef(dirty);
  useEffect(() => {
    dirtyRef.current = dirty;
  }, [dirty]);

  // Warn on tab close with unsaved edits; release lock on unload.
  useEffect(() => {
    mountedRef.current = true;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (dirtyRef.current) {
        e.preventDefault();
      }
      if (lockAcquiredRef.current) {
        postDocsDocIdLockRelease(docId).catch(() => {});
      }
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => {
      mountedRef.current = false;
      window.removeEventListener('beforeunload', onBeforeUnload);
      if (lockAcquiredRef.current) {
        postDocsDocIdLockRelease(docId).catch(() => {});
      }
    };
  }, [docId]);

  const save = useMutation({
    mutationFn: () =>
      putDocsDocId(docId, {
        content: draft ?? '',
        baseVersion: baseVersion ?? undefined,
        trigger: provenance === 'ai-draft' ? 'ai' : 'manual',
      }),
    onSuccess: (updated) => {
      setJustSaved(true);
      setBaseVersion(updated.currentVersion);
      if (lockAcquiredRef.current) {
        postDocsDocIdLockRelease(docId).catch(() => {});
        lockAcquiredRef.current = false;
      }
      queryClient.invalidateQueries({ queryKey: getGetDocsDocIdQueryKey(docId) });
      queryClient.invalidateQueries({ queryKey: getGetDocsDocIdVersionsQueryKey(docId) });
      queryClient.invalidateQueries({ queryKey: getGetDocsTreeQueryKey() });
      toast.success(t('docs.editor.saved', { version: updated.currentVersion }));
      navigate(`/docs/${docId}`);
    },
    onError: (error) => {
      // Stale baseVersion: server rejected with 409, current doc is untouched.
      // Refetch so `newerAvailable` flips true and the existing "load latest"
      // banner (below) becomes the recovery path — the draft stays intact.
      if (isAxiosError(error) && error.response?.status === 409) {
        queryClient.invalidateQueries({ queryKey: getGetDocsDocIdQueryKey(docId) });
        toast.error(t('docs.editor.saveConflict'));
        return;
      }
      toast.error(t('docs.editor.saveError'));
    },
  });

  const suggest = useMutation({
    mutationFn: () => postDocsDocIdAiSuggest(docId, { prompt: 'improve' }),
    onMutate: () => setRequestId(null),
    onSuccess: (response) => setRequestId(response.requestId),
    onError: () => toast.error(t('docs.editor.aiError')),
  });

  // A newer version landed (e.g. a regen) while editing and it isn't our own save.
  const newerAvailable =
    doc.data != null && baseVersion != null && doc.data.currentVersion > baseVersion && !justSaved;

  const lockedByOther = !!docLock && docLock.userId !== userId;
  const acquireLock = useMutation({
    mutationFn: () => postDocsDocIdLock(docId),
    onSuccess: () => {
      if (!mountedRef.current) {
        postDocsDocIdLockRelease(docId).catch(() => {});
        return;
      }
      useLive.getState().setDocLock(docId, undefined);
      lockAcquiredRef.current = true;
      setLockHeld(true);
    },
    onError: (error) => {
      lockAcquiredRef.current = false;
      setLockHeld(false);
      if (isAxiosError(error) && error.response?.status === 409) {
        const holder = error.response.data;
        if (holder?.userId && holder?.expiresAt) useLive.getState().setDocLock(docId, holder);
      }
      toast.error(t('docs.editor.lockError'));
    },
  });
  const renewLock = acquireLock.mutate;
  useEffect(() => {
    if (!lockHeld) return;
    const heartbeat = setInterval(() => renewLock(), 30_000);
    return () => clearInterval(heartbeat);
  }, [lockHeld, renewLock]);

  useEffect(() => {
    if (!requestId) return;
    if (aiResult.data?.status === 'complete' && typeof aiResult.data.fullContent === 'string')
      return;
    if (aiResult.data?.status === 'error' || aiResult.data?.status === 'complete') {
      toast.error(t('docs.editor.aiError'));
      return;
    }
    // The server bounds its provider call at two minutes; allow delivery time.
    const timeout = setTimeout(() => {
      setRequestId(null);
      toast.error(t('docs.editor.aiTimeout'));
    }, 150_000);
    return () => clearTimeout(timeout);
  }, [requestId, aiResult.data?.status, aiResult.data?.fullContent, t]);

  const canEdit = canMutate && lockHeld && !lockedByOther && !acquireLock.isPending;
  const aiPending =
    suggest.isPending ||
    (requestId !== null &&
      suggestion === null &&
      aiResult.data?.status !== 'error' &&
      aiResult.data?.status !== 'complete');

  if (doc.isLoading) {
    return (
      <div className="mx-auto max-w-330 px-6 py-6">
        <Panel className="p-6">
          <Skeleton className="mb-4 h-7 w-1/3" />
          <SkeletonRows rows={8} />
        </Panel>
      </div>
    );
  }
  if (doc.isError || !doc.data) {
    return (
      <div className="mx-auto max-w-330 px-6 py-6">
        <Panel className="min-h-[50vh]">
          <ErrorState description={t('docs.docLoadError')} onRetry={() => doc.refetch()} />
        </Panel>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-330 px-6 py-6">
      <button
        onClick={() => navigate(`/docs/${docId}`)}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('docs.editor.back')}
      </button>

      {/* Toolbar */}
      <header className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <FileTextIcon size={18} className="text-accent-primary" />
          <h1 className="text-lg font-semibold tracking-tight text-ink">{doc.data.title}</h1>
          {provenance === 'ai-draft' && (
            <span className="inline-flex items-center gap-1 rounded bg-accent-primary-tint px-1.5 py-0.5 text-2xs font-medium text-accent-primary">
              <SparklesIcon size={11} /> {t('docs.editor.aiDrafted')}
            </span>
          )}
          {doc.data.lastSyncedAt && (
            <span className="text-2xs text-ink-faint" title={fullDate(doc.data.lastSyncedAt)}>
              {t('docs.editor.synced', { when: relativeTime(doc.data.lastSyncedAt) })}
            </span>
          )}
          {dirty && (
            <span className="inline-flex items-center gap-1 text-2xs text-ink-faint">
              <span className="h-1.5 w-1.5 rounded-sm bg-warn" aria-hidden />{' '}
              {t('docs.editor.unsaved')}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {canMutate && !lockHeld && (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => acquireLock.mutate()}
              disabled={acquireLock.isPending}
            >
              {acquireLock.isPending
                ? t('docs.editor.acquiringLock')
                : t('docs.editor.startEditing')}
            </Button>
          )}
          <Button
            variant="secondary"
            size="sm"
            onClick={() => suggest.mutate()}
            disabled={aiPending || !canEdit}
          >
            <SparklesIcon size={14} />
            {aiPending ? t('docs.editor.aiThinking') : t('docs.editor.aiSuggest')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setDraft(baseContent);
              setProvenance('manual');
            }}
            disabled={!dirty || save.isPending}
          >
            {t('docs.editor.discard')}
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={() => save.mutate()}
            disabled={!dirty || save.isPending || !canEdit}
          >
            <CheckIcon size={14} />
            {save.isPending ? t('docs.editor.saving') : t('docs.editor.save')}
          </Button>
        </div>
      </header>

      {newerAvailable && (
        <div className="mb-4 flex items-center justify-between gap-3 rounded-md border border-warn bg-warn-tint px-3 py-2 text-xs text-warn">
          <span>{t('docs.editor.newerBanner', { version: doc.data.currentVersion })}</span>
          <button
            onClick={() => {
              setDraft(doc.data!.content);
              setBaseVersion(doc.data!.currentVersion);
              setBaseContent(doc.data!.content);
              setRequestId(null);
              setProvenance('manual');
            }}
            className="rounded-sm font-medium underline-offset-2 hover:underline"
          >
            {t('docs.editor.loadLatest')}
          </button>
        </div>
      )}

      {docLock && docLock.userId !== userId && (
        <div className="mb-4 rounded-md border border-info bg-info-tint px-3 py-2 text-xs text-info">
          <span>
            {t('docs.editor.lockBanner', {
              name: docLock.userName || t('docs.editor.unknownLockHolder'),
            })}
          </span>
        </div>
      )}

      {/* AI review-diff panel */}
      {suggestion !== null && (
        <Panel className="mb-4 p-4">
          <div className="mb-3 flex items-center justify-between">
            <span className="flex items-center gap-2 text-sm font-semibold text-ink">
              <SparklesIcon size={15} className="text-accent-primary" />
              {t('docs.editor.aiReviewTitle')}
            </span>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" onClick={() => setRequestId(null)}>
                <XIcon size={14} /> {t('docs.editor.aiReject')}
              </Button>
              <Button
                variant="primary"
                size="sm"
                disabled={!canEdit}
                onClick={() => {
                  setDraft(suggestion);
                  setProvenance('ai-draft');
                  setRequestId(null);
                  toast.success(t('docs.editor.aiAccepted'));
                }}
              >
                <CheckIcon size={14} /> {t('docs.editor.aiAccept')}
              </Button>
            </div>
          </div>
          <DocDiff
            before={draft ?? ''}
            after={suggestion}
            baseLabel={t('docs.editor.aiDiffCurrent')}
            headLabel={t('docs.editor.aiDiffProposed')}
            headTrigger="ai"
            label={t('docs.editor.aiDiffLabel')}
          />
        </Panel>
      )}

      {/* Editor + preview */}
      <div className="grid gap-4 lg:grid-cols-2">
        <Panel className="overflow-hidden">
          <div className="border-b border-line-soft px-3 py-2 text-2xs text-ink-faint">
            {t('docs.editor.markdownLabel')}
          </div>
          <CodeMirror
            value={draft ?? ''}
            onChange={(v) => {
              if (canEdit) setDraft(v);
            }}
            extensions={extensions}
            basicSetup={{ lineNumbers: true, foldGutter: false, highlightActiveLine: true }}
            editable={canEdit}
            className="min-h-[60vh] text-sm"
          />
        </Panel>
        <Panel className="overflow-hidden">
          <div className="border-b border-line-soft px-3 py-2 text-2xs text-ink-faint">
            {t('docs.editor.previewLabel')}
          </div>
          <div className="max-h-[70vh] overflow-y-auto px-5 py-4">
            <Markdown source={deferredDraft} />
          </div>
        </Panel>
      </div>
    </div>
  );
}
