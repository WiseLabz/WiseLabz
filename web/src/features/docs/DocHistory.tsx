/**
 * Revision history + version-to-version diff for a document. Selecting a revision
 * diffs it against the one before it (or against empty for the first revision).
 * The trigger of each revision (ai / template / manual) is shown so the reader
 * knows whether a sync or a human last touched the doc.
 */
import { useMemo, useState } from 'react';
import {
  useGetDocsDocIdVersions,
  useGetDocsDocIdVersionsRev,
} from '../../api/generated/docs/docs';
import { DocDiff } from '../../components/diff/DiffViewer';
import { Skeleton, SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { cn } from '../../lib/cn';
import { relativeTime } from '../../lib/time';
import { HistoryIcon, SparklesIcon, FileTextIcon, UserIcon } from '../../components/icons';
import type { DocVersionMetaTrigger } from '../../api/model';

const TRIGGER: Record<DocVersionMetaTrigger, { label: string; Icon: React.ComponentType<{ size?: number }>; tone: string }> = {
  ai: { label: 'AI', Icon: SparklesIcon, tone: 'var(--color-signal)' },
  template: { label: 'Template', Icon: FileTextIcon, tone: 'var(--color-signal-bright)' },
  manual: { label: 'Manual', Icon: UserIcon, tone: 'var(--color-ink-muted)' },
};

export function DocHistory({
  docId,
  currentVersion,
}: {
  docId: string;
  currentVersion: number;
}) {
  const versions = useGetDocsDocIdVersions(docId);
  // Mounted fresh per doc (DocReader is keyed by docId), so initial = latest rev.
  const [selected, setSelected] = useState<number>(currentVersion);

  const prevRev = selected - 1;
  const current = useGetDocsDocIdVersionsRev(docId, selected, {
    query: { enabled: selected > 0 },
  });
  const previous = useGetDocsDocIdVersionsRev(docId, prevRev, {
    query: { enabled: prevRev > 0, retry: false },
  });

  const metas = versions.data ?? [];

  const beforeText = previous.data?.content ?? '';
  const afterText = current.data?.content ?? '';

  const ready = !current.isLoading && (prevRev <= 0 || !previous.isFetching);
  const hasContent = useMemo(
    () => afterText !== '' || beforeText !== '',
    [afterText, beforeText],
  );

  if (versions.isLoading) return <SkeletonRows rows={4} />;
  if (versions.isError)
    return <ErrorState description="Couldn't load revision history." onRetry={() => versions.refetch()} />;
  if (metas.length === 0)
    return (
      <EmptyState
        icon={<HistoryIcon size={20} />}
        title="No prior revisions"
        description="This document has only its current version. A diff appears here once a sync or an edit creates the next revision."
      />
    );

  return (
    <div className="grid gap-5 lg:grid-cols-[200px_1fr]">
      {/* Version rail */}
      <ol className="flex flex-col gap-1">
        {metas.map((v) => {
          const t = TRIGGER[v.trigger];
          const active = v.rev === selected;
          return (
            <li key={v.rev}>
              <button
                onClick={() => setSelected(v.rev)}
                className={cn(
                  'flex w-full items-center gap-2.5 rounded-md border px-2.5 py-2 text-left transition-colors',
                  active
                    ? 'border-[var(--color-signal-soft)] bg-[var(--color-signal-tint)]'
                    : 'border-transparent hover:bg-[var(--color-surface-raised)]',
                )}
              >
                <span
                  className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md"
                  style={{ color: t.tone, backgroundColor: 'var(--color-canvas-sunken)' }}
                >
                  <t.Icon size={13} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="nums block text-sm font-medium text-[var(--color-ink)]">
                    v{v.rev}
                    {v.rev === currentVersion && (
                      <span className="ml-1.5 text-2xs font-normal text-[var(--color-ink-faint)]">
                        current
                      </span>
                    )}
                  </span>
                  <span className="block text-2xs text-[var(--color-ink-faint)]">
                    {t.label} · {relativeTime(v.createdAt)} ago
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ol>

      {/* Diff */}
      <div className="min-w-0">
        <p className="nums mb-2.5 font-mono text-2xs text-[var(--color-ink-faint)]">
          {prevRev > 0 ? `comparing v${prevRev} → v${selected}` : `v${selected} (first revision)`}
        </p>
        {!ready ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-32 w-full" />
          </div>
        ) : current.isError ? (
          <ErrorState
            title="Revision unavailable"
            description="This revision's content isn't available in the mock dataset. Try v4 → v3 on the pve1 doc to see a full diff."
          />
        ) : !hasContent ? (
          <EmptyState title="No textual changes" description="These revisions are identical." />
        ) : (
          <DocDiff before={beforeText} after={afterText} label={`v${prevRev > 0 ? prevRev : 0} → v${selected}`} />
        )}
      </div>
    </div>
  );
}
