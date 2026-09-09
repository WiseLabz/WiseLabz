/**
 * Collapsible reference panel for the template editor: lists the templateData
 * field paths (e.g. `.Metadata.region`, `.Dependencies[].Kind`) and the filter
 * functions (dateFormat, truncate, toJSON, filterByTitle, join) usable in
 * `{{ .Field | fn }}` section bodies. Backed by GET /api/docs/template-schema.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useGetDocsTemplateSchema } from '../../api/generated/docs/docs';
import { Panel } from '../../components/ui/Panel';
import { Skeleton } from '../../components/ui/states';
import { cn } from '../../lib/cn';
import { ChevronDownIcon, FileTextIcon } from '../../components/icons';
import type { TemplateSchemaFields } from '../../api/model';

/** Flattens the (loosely-typed) fields object into dotted/bracketed paths for display. */
function flattenFields(fields: TemplateSchemaFields | undefined, prefix = ''): string[] {
  if (!fields || typeof fields !== 'object') return [];
  const out: string[] = [];
  for (const [name, raw] of Object.entries(fields as Record<string, unknown>)) {
    const meta = raw as { type?: string; description?: string; items?: { fields?: TemplateSchemaFields } };
    const path = `${prefix}.${name}`;
    const isArrayOfObjects = meta?.type === 'array' && meta.items?.fields;
    out.push(`${path}${isArrayOfObjects ? '[]' : ''} — ${meta?.description ?? meta?.type ?? ''}`);
    if (isArrayOfObjects) {
      out.push(...flattenFields(meta.items!.fields, `${path}[]`));
    }
  }
  return out;
}

export function TemplateSchemaPanel() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const { data, isLoading } = useGetDocsTemplateSchema({ query: { enabled: open } });

  const fieldLines = flattenFields(data?.fields);
  const functions = data?.functions ?? [];

  return (
    <Panel>
      <button
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left"
      >
        <span className="flex items-center gap-2 text-ink-faint">
          <span className="text-accent-primary">
            <FileTextIcon size={13} />
          </span>
          <h3 className="font-mono text-sm text-ink-muted">{t('templates.schemaHeader')}</h3>
        </span>
        <ChevronDownIcon
          size={14}
          className={cn('text-ink-faint transition-transform', open && 'rotate-180')}
        />
      </button>
      {open && (
        <div className="space-y-3 border-t border-line-soft p-4">
          {isLoading ? (
            <Skeleton className="h-24 w-full" />
          ) : (
            <>
              <div>
                <p className="mb-1.5 text-2xs font-medium uppercase tracking-wide text-ink-faint">
                  {t('templates.schemaFields')}
                </p>
                <ul className="space-y-1 font-mono text-2xs text-ink-muted">
                  {fieldLines.map((line) => (
                    <li key={line} className="break-all">
                      {line}
                    </li>
                  ))}
                </ul>
              </div>
              <div>
                <p className="mb-1.5 text-2xs font-medium uppercase tracking-wide text-ink-faint">
                  {t('templates.schemaFunctions')}
                </p>
                <ul className="space-y-1.5">
                  {functions.map((fn) => (
                    <li key={fn.name} className="text-2xs leading-relaxed text-ink-muted">
                      <span className="font-mono font-medium text-ink">{fn.name}</span>
                      {fn.description && <span> — {fn.description}</span>}
                    </li>
                  ))}
                </ul>
              </div>
            </>
          )}
        </div>
      )}
    </Panel>
  );
}
