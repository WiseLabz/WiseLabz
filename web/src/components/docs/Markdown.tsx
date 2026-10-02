/**
 * Markdown renderer for WiseLabz's generated docs, on react-markdown + remark-gfm
 * (tables, strikethrough, task lists, autolinks) instead of the old hand-rolled
 * line-scanner. `components` maps each element to the same Tailwind classes the
 * previous renderer used, so existing docs render pixel-identical; gfm adds
 * correctness (nested lists, escaping, ordered lists) the old parser didn't have.
 * Inline vs. block code is told apart with a CSS `:not(pre)` selector rather than
 * a JS heuristic, since react-markdown's `code` renderer no longer reports it.
 */
import { memo, isValidElement, useState } from 'react';
import ReactMarkdown, { defaultUrlTransform, type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { remarkStripGenMarkers } from '../../lib/genMarkers';
import { Mermaid } from './Mermaid';
import { Dialog } from '../ui/Dialog';
import type { DocAttachment } from '../../api/model';

const headingClass: Record<'h1' | 'h2' | 'h3' | 'h4', string> = {
  h1: 'mt-1 mb-3 font-mono text-2xl font-semibold tracking-tight text-balance text-ink',
  h2: 'mb-2 mt-6 font-mono text-lg font-semibold tracking-tight text-ink',
  h3: 'mb-1.5 mt-5 font-mono text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-ink-muted)] text-ink',
  h4: 'mb-1.5 mt-5 font-mono text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-ink-muted)] text-ink',
};

const components: Components = {
  h1: ({ children }) => <h1 className={headingClass.h1}>{children}</h1>,
  h2: ({ children }) => <h2 className={headingClass.h2}>{children}</h2>,
  h3: ({ children }) => <h3 className={headingClass.h3}>{children}</h3>,
  h4: ({ children }) => <h4 className={headingClass.h4}>{children}</h4>,
  p: ({ children }) => (
    <p className="my-2.5 text-sm leading-relaxed text-ink-muted text-pretty">{children}</p>
  ),
  strong: ({ children }) => <strong className="font-semibold text-ink">{children}</strong>,
  blockquote: ({ children }) => (
    <blockquote className="my-3 flex gap-2.5 rounded-sm border border-line-soft bg-canvas-sunken px-4 py-2.5 text-sm text-ink-muted">
      <span className="mt-1.5 h-1.5 w-1.5 shrink-0 bg-accent-primary" />
      <div>{children}</div>
    </blockquote>
  ),
  ul: ({ children }) => <ul className="my-3 space-y-1.5 pl-1">{children}</ul>,
  ol: ({ children }) => (
    <ol className="my-3 space-y-1.5 pl-5 text-sm text-ink-muted [list-style:decimal]">
      {children}
    </ol>
  ),
  li: ({ children, className }) => {
    // Task-list items (remark-gfm) get a "task-list-item" className and render
    // their own checkbox — keep those bare, dot-prefix everything else.
    if (className?.includes('task-list-item')) {
      return <li className="flex items-center gap-2 text-sm text-ink-muted">{children}</li>;
    }
    return (
      <li className="flex gap-2.5 text-sm text-ink-muted">
        <span className="mt-2 h-1 w-1 shrink-0 rounded-full bg-accent-primary" />
        <span>{children}</span>
      </li>
    );
  },
  pre: ({ children }) => {
    // A fenced ```mermaid block renders through <Mermaid>, which owns its
    // own frame — skip the generic <pre> wrapper for it.
    const child = isValidElement<{ className?: string }>(children) ? children : null;
    if (child?.props.className?.includes('language-mermaid')) {
      return <>{children}</>;
    }
    return (
      <pre className="my-3 overflow-x-auto rounded-lg border border-line-soft bg-canvas-sunken p-3 font-mono text-xs leading-relaxed text-ink-muted">
        {children}
      </pre>
    );
  },
  code: ({ children, className }) => {
    if (className?.includes('language-mermaid')) {
      return <Mermaid chart={String(children).replace(/\n$/, '')} />;
    }
    return (
      <code className="rounded bg-canvas-sunken px-1.5 py-0.5 font-mono text-[0.85em] text-accent-primary-bright [pre_&]:rounded-none [pre_&]:bg-transparent [pre_&]:px-0 [pre_&]:py-0 [pre_&]:text-[1em] [pre_&]:text-inherit">
        {children}
      </code>
    );
  },
  table: ({ children }) => (
    <div className="my-3 overflow-x-auto rounded-lg border border-line-soft">
      <table className="w-full border-collapse text-sm">{children}</table>
    </div>
  ),
  tr: ({ children }) => <tr className="transition-colors hover:bg-surface-raised">{children}</tr>,
  th: ({ children }) => (
    <th className="border-b border-line-soft bg-canvas-sunken px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-ink-muted">
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td className="border-b border-line-soft px-3 py-2 text-ink last:border-0 [tr:last-child_&]:border-0">
      {children}
    </td>
  ),
};

function AttachmentPDF({
  attachment,
  children,
}: {
  attachment: DocAttachment;
  children: React.ReactNode;
}) {
  const [preview, setPreview] = useState(false);
  return (
    <span className="my-2 inline-flex max-w-full flex-col gap-2 rounded border border-line-soft p-3">
      <span>{children || attachment.filename}</span>
      <span className="flex gap-3">
        <a href={attachment.url} target="_blank" rel="noopener noreferrer">
          Open PDF
        </a>
        <button type="button" onClick={() => setPreview(!preview)} aria-expanded={preview}>
          {preview ? 'Hide preview' : 'Preview PDF'}
        </button>
      </span>
      {preview && (
        <iframe src={attachment.url} title={attachment.filename} className="h-96 w-full" />
      )}
    </span>
  );
}

export const Markdown = memo(function Markdown({
  source,
  attachments = [],
}: {
  source: string;
  attachments?: DocAttachment[];
}) {
  const [image, setImage] = useState<{ url: string; alt: string } | null>(null);
  const owned = new Map(attachments.map((a) => [a.id, a]));
  const byURL = new Map(attachments.filter((a) => a.url).map((a) => [a.url, a]));
  const transform = (url: string) => {
    if (url.startsWith('attachment:')) return owned.get(url.slice(11))?.url ?? 'attachment:missing';
    return defaultUrlTransform(url);
  };
  return (
    <div className="max-w-[68ch]">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkStripGenMarkers]}
        urlTransform={transform}
        components={{
          ...components,
          img: ({ src, alt = '' }) => {
            if (src === 'attachment:missing')
              return (
                <span role="img" aria-label={alt}>
                  Attachment unavailable: {alt}
                </span>
              );
            const url = typeof src === 'string' ? src : '';
            const attachment = byURL.get(url);
            if (attachment?.contentType === 'application/pdf')
              return <AttachmentPDF attachment={attachment}>{alt}</AttachmentPDF>;
            return (
              <button
                type="button"
                onClick={() => setImage({ url, alt })}
                aria-label={`Enlarge ${alt}`}
              >
                <img src={src} alt={alt} className="max-w-full rounded" loading="lazy" />
              </button>
            );
          },
          a: ({ href, children }) => {
            if (href === 'attachment:missing')
              return <span>Attachment unavailable: {children}</span>;
            const attachment = byURL.get(href);
            if (attachment?.contentType === 'application/pdf')
              return <AttachmentPDF attachment={attachment}>{children}</AttachmentPDF>;
            return (
              <a href={href} className="text-accent-primary underline">
                {children}
              </a>
            );
          },
        }}
      >
        {source}
      </ReactMarkdown>
      <Dialog
        open={image !== null}
        onClose={() => setImage(null)}
        title={image?.alt || 'Image preview'}
        size="lg"
      >
        {image && (
          <img src={image.url} alt={image.alt} className="max-h-[80vh] w-full object-contain" />
        )}
        <button type="button" onClick={() => setImage(null)}>
          Close image
        </button>
      </Dialog>
    </div>
  );
});
