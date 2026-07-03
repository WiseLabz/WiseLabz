/**
 * Templates list — the doc-generation control surface (Phase 7). Operators see
 * every template, create new ones, open the editor, or delete. Each template
 * declares what it `appliesTo` (a connector category and optionally a type), so
 * the lab knows which template renders which service's doc. Mutating actions are
 * role-gated in the UI (RoleGate) and the route itself is operator-gated in
 * App.tsx; the server still enforces the real boundary.
 */
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { motion } from 'motion/react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  deleteTemplatesTemplateId,
  getGetTemplatesQueryKey,
  postTemplates,
  useGetTemplates,
} from '../../api/generated/templates/templates';
import type { Template } from '../../api/model';
import { Button, IconButton } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { Dialog } from '../../components/ui/Dialog';
import { ElevationConfirm } from '../../components/manager/ElevationConfirm';
import { RoleGate } from '../../components/ui/RoleGate';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { toast } from '../../lib/toast';
import { LayersIcon, PlusIcon, XIcon, ArrowRightIcon } from '../../components/icons';

export function TemplatesPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetTemplates();

  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [toDelete, setToDelete] = useState<Template | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: getGetTemplatesQueryKey() });

  const create = useMutation({
    mutationFn: (name: string) =>
      postTemplates({
        name,
        description: '',
        appliesTo: {},
        sections: [
          {
            title: t('templates.defaultSectionTitle'),
            order: 1,
            body: '',
          },
        ],
      }),
    onSuccess: (tpl) => {
      invalidate();
      setCreating(false);
      setNewName('');
      toast.success(t('templates.toastCreated'));
      navigate(`/templates/${tpl.id}`);
    },
    onError: () => toast.error(t('templates.toastCreateError')),
  });

  const remove = useMutation({
    mutationFn: ({ id, token }: { id: string; token: string | null }) =>
      deleteTemplatesTemplateId(id, token ? { headers: { 'X-Elevation-Token': token } } : undefined),
    onSuccess: () => {
      invalidate();
      setToDelete(null);
      toast.success(t('templates.toastDeleted'));
    },
    onError: () => toast.error(t('templates.toastDeleteError')),
  });

  return (
    <div className="mx-auto max-w-225 px-6 py-6">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-ink">{t('templates.title')}</h1>
          <p className="text-sm text-ink-muted">{t('templates.subtitle')}</p>
        </div>
        <RoleGate>
          <Button variant="primary" size="md" onClick={() => setCreating(true)}>
            <PlusIcon size={15} />
            {t('templates.new')}
          </Button>
        </RoleGate>
      </header>

      <Panel>
        {isLoading ? (
          <SkeletonRows rows={5} />
        ) : isError || !data ? (
          <ErrorState description={t('templates.loadError')} onRetry={() => refetch()} />
        ) : data.length === 0 ? (
          <EmptyState
            icon={<LayersIcon size={20} />}
            title={t('templates.emptyTitle')}
            description={t('templates.emptyDesc')}
            action={
              <RoleGate>
                <Button variant="secondary" size="sm" onClick={() => setCreating(true)}>
                  <PlusIcon size={14} />
                  {t('templates.new')}
                </Button>
              </RoleGate>
            }
          />
        ) : (
          data.map((tpl, idx) => (
            <motion.div
              key={tpl.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.03, duration: 0.25 }}
              className="group flex items-center gap-4 border-b border-line-soft px-4 py-3 transition-colors last:border-0 hover:bg-surface-raised"
            >
              <button
                onClick={() => navigate(`/templates/${tpl.id}`)}
                className="flex min-w-0 flex-1 items-center gap-3 text-left"
              >
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-canvas-sunken text-ink-faint">
                  <LayersIcon size={15} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-ink">{tpl.name}</span>
                  <span className="flex items-center gap-1.5 font-mono text-2xs text-ink-faint">
                    <AppliesToLabel template={tpl} />
                    <span>·</span>
                    <span>
                      {t('templates.sectionCount', {
                        count: tpl.sections.length,
                      })}
                    </span>
                  </span>
                </span>
              </button>

              <RoleGate>
                <IconButton
                  label={t('templates.deleteLabel')}
                  onClick={() => setToDelete(tpl)}
                  className="opacity-0 transition-opacity hover:text-err group-hover:opacity-100 focus-visible:opacity-100"
                >
                  <XIcon size={15} />
                </IconButton>
              </RoleGate>
              <ArrowRightIcon
                size={15}
                className="shrink-0 text-line-strong transition-colors group-hover:text-ink-muted"
              />
            </motion.div>
          ))
        )}
      </Panel>

      {/* Create dialog */}
      <Dialog
        open={creating}
        onClose={() => setCreating(false)}
        title={t('templates.createTitle')}
        size="sm"
      >
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (newName.trim()) create.mutate(newName.trim());
          }}
        >
          <label className="block">
            <span className="mb-1 block text-2xs uppercase tracking-wider text-ink-faint">
              {t('templates.nameLabel')}
            </span>
            <input
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder={t('templates.namePlaceholder')}
              className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none placeholder:text-ink-faint focus-visible:border-signal-soft"
            />
          </label>
          <div className="mt-5 flex items-center justify-end gap-2">
            <Button type="button" variant="ghost" size="sm" onClick={() => setCreating(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              variant="primary"
              size="sm"
              disabled={!newName.trim() || create.isPending}
            >
              {create.isPending ? t('templates.creating') : t('templates.create')}
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Delete confirm */}
      <ElevationConfirm
        open={!!toDelete}
        onClose={() => setToDelete(null)}
        resourceName={toDelete?.name ?? ''}
        action="template.delete"
        title={t('templates.deleteTitle')}
        description={t('templates.deleteDesc', {
          name: toDelete?.name ?? '',
        })}
        confirmLabel={remove.isPending ? t('templates.deleting') : t('templates.delete')}
        isPending={remove.isPending}
        onConfirm={(token) => {
          if (toDelete) return remove.mutateAsync({ id: toDelete.id, token });
        }}
      />
    </div>
  );
}

/** Compact "applies to" label: category · type, or "any service" when unset. */
export function AppliesToLabel({ template }: { template: Template }) {
  const { t } = useTranslation();
  const cat = template.appliesTo?.category;
  const type = template.appliesTo?.type;
  if (!cat && !type) {
    return <span>{t('templates.appliesAny')}</span>;
  }
  return (
    <span className="text-signal-bright">
      {cat
        ? t(`templates.category.${cat}`, { defaultValue: cat })
        : t('templates.appliesAnyCategory')}
      {type ? ` / ${type}` : ''}
    </span>
  );
}
