/**
 * Template editor (Phase 7). Edit a template's identity, what it applies to
 * (connector category / type), and its ordered sections — then preview-generate:
 * resolve the `{{placeholder}}` tokens against a sample connector's last synced
 * snapshot and render the result, either as the finished doc (Markdown) or as a
 * diff (source → resolved) so it's obvious what each placeholder filled in.
 *
 * Edits are local until Save (PUT). Preview runs against the last *saved*
 * template, so a "save to preview" hint shows while there are unsaved changes.
 * Mutations are role-gated in the UI and the route is operator-gated in App.tsx.
 */
import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getGetTemplatesQueryKey,
  getGetTemplatesTemplateIdQueryKey,
  postTemplatesTemplateIdPreview,
  putTemplatesTemplateId,
  useGetTemplatesTemplateId,
} from '../../api/generated/templates/templates';
import { ConnectorCategory } from '../../api/model';
import type { DocVersion, Template, TemplateInput, TemplateSection } from '../../api/model';
import { renderTemplate, previewConnectors } from '../../data/templates.fixtures';
import { Button, IconButton } from '../../components/ui/Button';
import { Panel, PanelHeader } from '../../components/ui/Panel';
import { Skeleton, SkeletonRows, ErrorState } from '../../components/ui/states';
import { RoleGate } from '../../components/ui/RoleGate';
import { Markdown } from '../../components/docs/Markdown';
import { DocDiff } from '../../components/diff/DiffViewer';
import { toast } from '../../lib/toast';
import { cn } from '../../lib/cn';
import {
  ArrowRightIcon,
  CheckIcon,
  ChevronDownIcon,
  FileTextIcon,
  PlusIcon,
  SparklesIcon,
  XIcon,
  DiffIcon,
} from '../../components/icons';

const CATEGORIES = Object.values(ConnectorCategory);

type Draft = {
  name: string;
  description: string;
  category: string; // '' = any
  type: string;
  sections: TemplateSection[];
};

function toDraft(tpl: Template): Draft {
  return {
    name: tpl.name,
    description: tpl.description ?? '',
    category: tpl.appliesTo?.category ?? '',
    type: tpl.appliesTo?.type ?? '',
    sections: [...tpl.sections].sort((a, b) => a.order - b.order).map((s) => ({ ...s })),
  };
}

function toInput(d: Draft): TemplateInput {
  return {
    name: d.name.trim(),
    description: d.description.trim(),
    appliesTo: {
      ...(d.category ? { category: d.category as ConnectorCategory } : {}),
      ...(d.type.trim() ? { type: d.type.trim() } : {}),
    },
    sections: d.sections.map((s, i) => ({ ...s, order: i + 1 })),
  };
}

export function TemplateEditorPage() {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const templateId = id ?? '';
  const { data, isLoading, isError, refetch } = useGetTemplatesTemplateId(templateId);

  const [draft, setDraft] = useState<Draft | null>(null);

  // Seed the editable draft once the template loads (and re-seed if it changes).
  // Adjusting state during render is the React-blessed alternative to an effect:
  // re-seed whenever the query yields a fresh reference.
  const [seeded, setSeeded] = useState<typeof data | null>(null);
  if (data && data !== seeded) {
    setSeeded(data);
    setDraft(toDraft(data));
  }

  const dirty = useMemo(() => {
    if (!data || !draft) return false;
    return JSON.stringify(toInput(draft)) !== JSON.stringify(toInput(toDraft(data)));
  }, [data, draft]);

  const save = useMutation({
    mutationFn: () => putTemplatesTemplateId(templateId, toInput(draft!)),
    onSuccess: (updated) => {
      queryClient.setQueryData(getGetTemplatesTemplateIdQueryKey(templateId), updated);
      queryClient.invalidateQueries({ queryKey: getGetTemplatesQueryKey() });
      toast.success(t('templates.toastSaved'));
    },
    onError: () => toast.error(t('templates.toastSaveError')),
  });

  if (isLoading || (!draft && !isError)) {
    return (
      <div className="mx-auto max-w-275 px-6 py-6">
        <Panel className="p-6">
          <Skeleton className="mb-4 h-7 w-1/3" />
          <SkeletonRows rows={6} />
        </Panel>
      </div>
    );
  }
  if (isError || !data || !draft) {
    return (
      <div className="mx-auto max-w-275 px-6 py-6">
        <Panel className="min-h-[40vh]">
          <ErrorState description={t('templates.editorLoadError')} onRetry={() => refetch()} />
        </Panel>
      </div>
    );
  }

  const update = (patch: Partial<Draft>) => setDraft((d) => (d ? { ...d, ...patch } : d));

  const updateSection = (idx: number, patch: Partial<TemplateSection>) =>
    update({ sections: draft.sections.map((s, i) => (i === idx ? { ...s, ...patch } : s)) });

  const addSection = () =>
    update({
      sections: [
        ...draft.sections,
        {
          title: t('templates.newSectionTitle'),
          order: draft.sections.length + 1,
          body: '',
        },
      ],
    });

  const removeSection = (idx: number) =>
    update({ sections: draft.sections.filter((_, i) => i !== idx) });

  const moveSection = (idx: number, dir: -1 | 1) => {
    const next = idx + dir;
    if (next < 0 || next >= draft.sections.length) return;
    const copy = [...draft.sections];
    [copy[idx], copy[next]] = [copy[next], copy[idx]];
    update({ sections: copy });
  };

  return (
    <div className="mx-auto max-w-275 px-6 py-6">
      <button
        onClick={() => navigate('/templates')}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('templates.backToList')}
      </button>

      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="truncate text-xl font-semibold tracking-tight text-ink">
            {draft.name || t('templates.untitled')}
          </h1>
          <p className="font-mono text-2xs text-ink-faint">{data.id}</p>
        </div>
        <RoleGate
          fallback={
            <span className="font-mono text-2xs text-ink-faint">{t('templates.readOnly')}</span>
          }
        >
          <div className="flex items-center gap-2">
            {dirty && (
              <span className="flex items-center gap-1.5 font-mono text-2xs text-warn">
                <span className="h-1.5 w-1.5 bg-warn" aria-hidden="true" />
                {t('templates.unsaved')}
              </span>
            )}
            <Button
              variant="primary"
              size="md"
              disabled={!dirty || !draft.name.trim() || save.isPending}
              onClick={() => save.mutate()}
            >
              <CheckIcon size={15} />
              {save.isPending ? t('templates.saving') : t('templates.save')}
            </Button>
          </div>
        </RoleGate>
      </header>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        {/* ── Left: edit ─────────────────────────────────────────────── */}
        <div className="space-y-4">
          <Panel>
            <PanelHeader title={t('templates.detailsHeader')} icon={<FileTextIcon size={13} />} />
            <div className="space-y-3 p-4">
              <Labeled label={t('templates.nameLabel')} required>
                <input
                  value={draft.name}
                  onChange={(e) => update({ name: e.target.value })}
                  className={inputCls}
                />
              </Labeled>
              <Labeled label={t('templates.descriptionLabel')}>
                <textarea
                  value={draft.description}
                  onChange={(e) => update({ description: e.target.value })}
                  rows={2}
                  placeholder={t('templates.descriptionPlaceholder')}
                  className={cn(inputCls, 'h-auto resize-y py-2 leading-relaxed')}
                />
              </Labeled>
            </div>
          </Panel>

          <Panel>
            <PanelHeader title={t('templates.appliesHeader')} icon={<ArrowRightIcon size={13} />} />
            <div className="space-y-3 p-4">
              <p className="text-xs leading-relaxed text-ink-muted">{t('templates.appliesHelp')}</p>
              <Labeled label={t('templates.categoryLabel')}>
                <div className="relative">
                  <select
                    value={draft.category}
                    onChange={(e) => update({ category: e.target.value })}
                    className={cn(inputCls, 'appearance-none pr-8')}
                  >
                    <option value="">{t('templates.anyCategory')}</option>
                    {CATEGORIES.map((c) => (
                      <option key={c} value={c}>
                        {t(`templates.category.${c}`, { defaultValue: c })}
                      </option>
                    ))}
                  </select>
                  <ChevronDownIcon
                    size={14}
                    className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-ink-faint"
                  />
                </div>
              </Labeled>
              <Labeled label={t('templates.typeLabel')}>
                <input
                  value={draft.type}
                  onChange={(e) => update({ type: e.target.value })}
                  placeholder={t('templates.typePlaceholder')}
                  className={inputCls}
                />
              </Labeled>
            </div>
          </Panel>

          <Panel>
            <PanelHeader
              title={t('templates.sectionsHeader')}
              icon={<FileTextIcon size={13} />}
              count={draft.sections.length}
              action={
                <RoleGate>
                  <Button variant="secondary" size="sm" onClick={addSection}>
                    <PlusIcon size={13} />
                    {t('templates.addSection')}
                  </Button>
                </RoleGate>
              }
            />
            <div className="space-y-3 p-4">
              {draft.sections.length === 0 && (
                <p className="py-4 text-center text-xs text-ink-faint">
                  {t('templates.noSections')}
                </p>
              )}
              {draft.sections.map((s, idx) => (
                <SectionEditor
                  key={idx}
                  section={s}
                  index={idx}
                  count={draft.sections.length}
                  onChange={(patch) => updateSection(idx, patch)}
                  onMove={(dir) => moveSection(idx, dir)}
                  onRemove={() => removeSection(idx)}
                />
              ))}
            </div>
          </Panel>
        </div>

        {/* ── Right: preview ─────────────────────────────────────────── */}
        <PreviewPanel template={data} type={draft.type} dirty={dirty} />
      </div>
    </div>
  );
}

const inputCls =
  'h-9 w-full rounded-sm border border-[var(--color-line)] bg-[var(--color-surface)] px-2.5 text-sm text-[var(--color-ink)] outline-none placeholder:text-[var(--color-ink-faint)] focus-visible:border-[var(--color-signal-soft)]';

function Labeled({
  label,
  required,
  children,
}: {
  label: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-2xs uppercase tracking-wider text-ink-faint">
        {label}
        {required && <span className="text-err"> *</span>}
      </span>
      {children}
    </label>
  );
}

function SectionEditor({
  section,
  index,
  count,
  onChange,
  onMove,
  onRemove,
}: {
  section: TemplateSection;
  index: number;
  count: number;
  onChange: (patch: Partial<TemplateSection>) => void;
  onMove: (dir: -1 | 1) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="rounded-lg border border-line-soft bg-canvas-sunken p-3">
      <div className="mb-2 flex items-center gap-2">
        <span className="nums font-mono text-2xs text-ink-faint">{index + 1}</span>
        <input
          value={section.title}
          onChange={(e) => onChange({ title: e.target.value })}
          aria-label={t('templates.sectionTitleLabel')}
          className="h-8 min-w-0 flex-1 rounded-sm border border-line bg-surface px-2.5 text-sm font-medium text-ink outline-none focus-visible:border-signal-soft"
        />
        <RoleGate>
          <div className="flex items-center">
            <IconButton
              label={t('templates.moveUp')}
              disabled={index === 0}
              onClick={() => onMove(-1)}
              className="h-7 w-7"
            >
              <ChevronDownIcon size={14} className="rotate-180" />
            </IconButton>
            <IconButton
              label={t('templates.moveDown')}
              disabled={index === count - 1}
              onClick={() => onMove(1)}
              className="h-7 w-7"
            >
              <ChevronDownIcon size={14} />
            </IconButton>
            <IconButton
              label={t('templates.removeSection')}
              onClick={onRemove}
              className="h-7 w-7 hover:text-err"
            >
              <XIcon size={14} />
            </IconButton>
          </div>
        </RoleGate>
      </div>
      <textarea
        value={section.body}
        onChange={(e) => onChange({ body: e.target.value })}
        rows={4}
        aria-label={t('templates.sectionBodyLabel')}
        placeholder={t('templates.sectionBodyPlaceholder')}
        className="w-full resize-y rounded-sm border border-line bg-surface px-2.5 py-2 font-mono text-xs leading-relaxed text-ink outline-none placeholder:text-ink-faint focus-visible:border-signal-soft"
      />
    </div>
  );
}

type PreviewTab = 'rendered' | 'diff';

function PreviewPanel({
  template,
  type,
  dirty,
}: {
  template: Template;
  type: string;
  dirty: boolean;
}) {
  const { t } = useTranslation();

  // Offer connectors that match the (draft) appliesTo scope, else all samples.
  const candidates = useMemo(() => {
    const matches = previewConnectors.filter((c) => {
      return !(type.trim() && c.type !== type.trim());
    });
    return matches.length ? matches : previewConnectors;
  }, [type]);

  const [connectorId, setConnectorId] = useState<string>(candidates[0]?.id ?? '');
  const [tab, setTab] = useState<PreviewTab>('rendered');
  const [result, setResult] = useState<DocVersion | null>(null);

  // Keep the selection valid as the candidate set narrows with the type filter.
  // Adjust during render rather than in an effect (setting to the same value is a
  // no-op, so this cannot loop).
  if (!candidates.some((c) => c.id === connectorId)) {
    const next = candidates[0]?.id ?? '';
    if (next !== connectorId) setConnectorId(next);
  }

  const preview = useMutation({
    mutationFn: () =>
      postTemplatesTemplateIdPreview(template.id, connectorId ? { connectorId } : {}),
    onSuccess: (doc) => setResult(doc),
    onError: () => toast.error(t('templates.toastPreviewError')),
  });

  const selected = candidates.find((c) => c.id === connectorId) ?? null;
  // Left side of the diff: the raw template source (placeholders intact).
  const source = useMemo(() => renderTemplate(template, selected, false), [template, selected]);

  return (
    <Panel className="lg:sticky lg:top-6 lg:self-start">
      <PanelHeader
        title={t('templates.previewHeader')}
        icon={<SparklesIcon size={13} />}
        action={
          result && (
            <div className="flex items-center gap-0.5 rounded-md border border-line-soft p-0.5">
              <PreviewTabButton
                active={tab === 'rendered'}
                onClick={() => setTab('rendered')}
                icon={<FileTextIcon size={13} />}
              >
                {t('templates.previewRendered')}
              </PreviewTabButton>
              <PreviewTabButton
                active={tab === 'diff'}
                onClick={() => setTab('diff')}
                icon={<DiffIcon size={13} />}
              >
                {t('templates.previewDiff')}
              </PreviewTabButton>
            </div>
          )
        }
      />
      <div className="space-y-3 p-4">
        <div className="flex flex-wrap items-end gap-2">
          <label className="min-w-0 flex-1">
            <span className="mb-1 block text-2xs uppercase tracking-wider text-ink-faint">
              {t('templates.sampleConnector')}
            </span>
            <div className="relative">
              <select
                value={connectorId}
                onChange={(e) => setConnectorId(e.target.value)}
                className={cn(inputCls, 'appearance-none pr-8')}
              >
                {candidates.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.type})
                  </option>
                ))}
              </select>
              <ChevronDownIcon
                size={14}
                className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-ink-faint"
              />
            </div>
          </label>
          <Button
            variant="secondary"
            size="md"
            disabled={preview.isPending}
            onClick={() => preview.mutate()}
          >
            <SparklesIcon size={14} />
            {preview.isPending ? t('templates.generating') : t('templates.generate')}
          </Button>
        </div>

        {dirty && (
          <p className="rounded-md bg-warn-tint px-3 py-2 text-2xs text-warn">
            {t('templates.previewStale')}
          </p>
        )}

        {preview.isPending ? (
          <SkeletonRows rows={6} />
        ) : !result ? (
          <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-line-soft px-6 py-12 text-center">
            <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-surface-raised text-ink-faint">
              <SparklesIcon size={20} />
            </span>
            <p className="text-sm font-medium text-ink">{t('templates.previewEmptyTitle')}</p>
            <p className="max-w-xs text-xs leading-relaxed text-ink-muted">
              {t('templates.previewEmptyDesc')}
            </p>
          </div>
        ) : tab === 'rendered' ? (
          <div className="rounded-lg border border-line-soft bg-canvas-sunken px-4 py-3">
            <Markdown source={result.content} />
          </div>
        ) : (
          <DocDiff
            before={source}
            after={result.content}
            label={t('templates.diffLabel')}
            baseLabel={t('templates.diffBase')}
            headLabel={selected?.name ?? t('templates.diffHead')}
          />
        )}
      </div>
    </Panel>
  );
}

function PreviewTabButton({
  active,
  onClick,
  icon,
  children,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        'flex items-center gap-1.5 rounded px-2 py-1 text-2xs font-medium transition-colors',
        active ? 'bg-surface-raised text-ink' : 'text-ink-faint hover:text-ink'
      )}
    >
      {icon}
      {children}
    </button>
  );
}
