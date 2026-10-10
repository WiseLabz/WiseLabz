import { useId, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import { postDocsImport, postDocsImportImportIdCommit } from '../../api/generated/docs/docs';
import { DocPullRequestSource, PostDocsImportBodySource } from '../../api/model';
import type { DocImportIssue, DocImportNode, DocImportPreview } from '../../api/model';
import { Dialog } from '../ui/Dialog';
import { Button } from '../ui/Button';
import { toast } from '../../lib/toast';
import { WikiPull } from './WikiPull';

/** SourcePicker values for the pull sources; the Wiki.js export zip owns the plain `wikijs`. */
const WIKIJS_API = 'wikijs-api';
type SourcePick =
  PostDocsImportBodySource | typeof DocPullRequestSource.bookstack | typeof WIKIJS_API;

function pullSource(pick: SourcePick): DocPullRequestSource | undefined {
  if (pick === DocPullRequestSource.bookstack) return DocPullRequestSource.bookstack;
  return pick === WIKIJS_API ? DocPullRequestSource.wikijs : undefined;
}

/** Admin-only docs import: pick a source, upload, preview, confirm. */
export function ImportDocsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return open ? <ImportDocsForm onClose={onClose} /> : null;
}

function ImportDocsForm({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const fieldId = useId();
  const [source, setSource] = useState<SourcePick>(PostDocsImportBodySource.markdown);
  const pull = pullSource(source);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<DocImportPreview | null>(null);
  const upload = useMutation({
    mutationFn: (f: File) =>
      postDocsImport({
        file: f,
        source: pull ? PostDocsImportBodySource.markdown : (source as PostDocsImportBodySource),
      }),
    onSuccess: setPreview,
    onError: () => toast.error(t('docs.import.uploadError')),
  });
  const commit = useMutation({
    mutationFn: (id: string) => postDocsImportImportIdCommit(id),
    onSuccess: (docs) => {
      void queryClient.invalidateQueries();
      toast.success(t('docs.import.done', { count: docs.length }));
      onClose();
      if (docs[0]) navigate(`/docs/${docs[0].docId}`);
    },
    onError: () => toast.error(t('docs.import.commitError')),
  });

  return (
    <Dialog open onClose={onClose} size="lg" title={t('docs.import.title')}>
      {preview ? (
        <PreviewPane
          preview={preview}
          pending={commit.isPending}
          onBack={() => setPreview(null)}
          onConfirm={() => commit.mutate(preview.id)}
        />
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1 text-sm">
            <label htmlFor={`${fieldId}-source`}>{t('docs.import.source')}</label>
            <select
              id={`${fieldId}-source`}
              value={source}
              className="rounded-md border border-line-soft bg-canvas px-2 py-1 text-sm text-ink"
              onChange={(e) => setSource(e.target.value as SourcePick)}
            >
              <option value={PostDocsImportBodySource.markdown}>
                {t('docs.import.sourceMarkdown')}
              </option>
              <option value={PostDocsImportBodySource.wikijs}>
                {t('docs.import.sourceWikijs')}
              </option>
              <option value={DocPullRequestSource.bookstack}>{t('docs.import.bookstack')}</option>
              <option value={WIKIJS_API}>{t('docs.import.wikijsApi')}</option>
            </select>
          </div>
          {pull ? (
            // Keyed so switching sources never carries one form's credentials over.
            <WikiPull key={source} source={pull} onReady={setPreview} />
          ) : (
            <form
              className="flex flex-col gap-4"
              onSubmit={(e) => {
                e.preventDefault();
                if (file) upload.mutate(file);
              }}
            >
              <p className="text-sm text-ink-muted">
                {source === PostDocsImportBodySource.wikijs
                  ? t('docs.import.introWikijs')
                  : t('docs.import.intro')}
              </p>
              <div className="flex flex-col gap-1 text-sm">
                <label htmlFor={`${fieldId}-file`}>
                  {source === PostDocsImportBodySource.wikijs
                    ? t('docs.import.fileWikijs')
                    : t('docs.import.file')}
                </label>
                <input
                  id={`${fieldId}-file`}
                  type="file"
                  accept=".zip,application/zip"
                  aria-describedby={`${fieldId}-limits`}
                  className="text-sm text-ink"
                  onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                />
                <span id={`${fieldId}-limits`} className="text-2xs text-ink-faint">
                  {t('docs.import.limits')}
                </span>
              </div>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="secondary" onClick={onClose}>
                  {t('common.cancel')}
                </Button>
                <Button type="submit" disabled={!file || upload.isPending}>
                  {upload.isPending ? t('docs.import.uploading') : t('docs.import.upload')}
                </Button>
              </div>
            </form>
          )}
        </div>
      )}
    </Dialog>
  );
}

function PreviewPane({
  preview,
  pending,
  onBack,
  onConfirm,
}: {
  preview: DocImportPreview;
  pending: boolean;
  onBack: () => void;
  onConfirm: () => void;
}) {
  const { t } = useTranslation();
  const { data: connectors = [] } = useGetConnectors();
  const names = new Map(connectors.map((c) => [c.id, c.name]));
  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-ink">
        {t('docs.import.summary', {
          docs: preview.docCount,
          attachments: preview.attachmentCount,
          links: preview.mappings.length,
        })}
      </p>
      {preview.docCount === 0 ? (
        <p className="text-sm text-ink-muted">{t('docs.import.empty')}</p>
      ) : (
        <section aria-label={t('docs.import.tree')}>
          <h3 className="mb-1 text-xs font-medium text-ink-muted">{t('docs.import.tree')}</h3>
          <div className="max-h-64 overflow-y-auto rounded-md border border-line-soft bg-canvas-sunken p-2">
            <ImportTree nodes={preview.tree} connectorNames={names} />
          </div>
        </section>
      )}
      {preview.collisions.length > 0 && (
        <section>
          <h3 className="mb-1 text-xs font-medium text-ink-muted">{t('docs.import.collisions')}</h3>
          <ul className="text-xs text-ink">
            {preview.collisions.map((c) => (
              <li key={c.path}>{t('docs.import.collision', c)}</li>
            ))}
          </ul>
        </section>
      )}
      <IssueList
        label={t('docs.import.warnings', { count: preview.warnings.length })}
        issues={preview.warnings}
        open
      />
      <IssueList
        label={t('docs.import.skipped', { count: preview.skipped.length })}
        issues={preview.skipped}
      />
      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={onBack} disabled={pending}>
          {t('docs.import.back')}
        </Button>
        <Button type="button" onClick={onConfirm} disabled={pending || preview.docCount === 0}>
          {pending
            ? t('docs.import.committing')
            : t('docs.import.confirm', { count: preview.docCount })}
        </Button>
      </div>
    </div>
  );
}

function ImportTree({
  nodes,
  connectorNames,
}: {
  nodes: DocImportNode[];
  connectorNames: Map<string, string>;
}) {
  const { t } = useTranslation();
  return (
    <ul className="flex flex-col gap-0.5 pl-3 text-sm first:pl-0">
      {nodes.map((n) => (
        <li key={n.docId}>
          <span className="text-ink">{n.title}</span>
          {n.folder && (
            <span className="ml-2 text-2xs text-ink-faint">{t('docs.import.folder')}</span>
          )}
          {n.serviceId && (
            <span className="ml-2 rounded bg-accent-secondary-tint px-1 text-2xs text-accent-secondary-bright">
              {connectorNames.get(n.serviceId) ?? n.serviceId}
            </span>
          )}
          {n.attachmentCount > 0 && (
            <span className="ml-2 text-2xs text-ink-faint">
              {t('docs.import.attachments', { count: n.attachmentCount })}
            </span>
          )}
          {n.children.length > 0 && (
            <ImportTree nodes={n.children} connectorNames={connectorNames} />
          )}
        </li>
      ))}
    </ul>
  );
}

function IssueList({
  label,
  issues,
  open = false,
}: {
  label: string;
  issues: DocImportIssue[];
  open?: boolean;
}) {
  if (issues.length === 0) return null;
  return (
    <details open={open} className="rounded-md border border-line-soft p-2 text-xs">
      <summary className="cursor-pointer text-ink-muted">{label}</summary>
      <ul className="mt-1 max-h-40 overflow-y-auto">
        {issues.map((issue, i) => (
          <li key={`${issue.path}-${i}`} className="text-ink">
            <span className="font-mono text-ink-faint">{issue.path}</span> {issue.message}
          </li>
        ))}
      </ul>
    </details>
  );
}
