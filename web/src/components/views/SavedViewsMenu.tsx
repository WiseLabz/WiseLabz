/**
 * Saved views dropdown: apply, save, or delete named filter sets for one list
 * surface (services/changes/alerts). Filters are opaque to the backend — this
 * component owns the shape and just JSON round-trips whatever `filters` it's
 * given.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AnimatePresence, motion } from 'motion/react';
import * as Popover from '@radix-ui/react-popover';
import {
  getGetSavedViewsQueryKey,
  getSavedViews,
  postSavedViews,
  deleteSavedViewsSavedViewId,
} from '../../api/generated/saved-views/saved-views';
import type { SavedViewSurface } from '../../api/model';
import { Button, IconButton } from '../ui/Button';
import { ChevronDownIcon, XIcon, PlusIcon } from '../icons';

interface SavedViewsMenuProps<TFilters> {
  surface: SavedViewSurface;
  filters: TFilters;
  onApply: (filters: TFilters) => void;
  className?: string;
}

export function SavedViewsMenu<TFilters>({
  surface,
  filters,
  onApply,
  className,
}: SavedViewsMenuProps<TFilters>) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');

  const queryKey = getGetSavedViewsQueryKey({ surface });
  const { data: views } = useQuery({
    queryKey,
    queryFn: () => getSavedViews({ surface }),
  });

  const invalidate = () => qc.invalidateQueries({ queryKey });

  const save = useMutation({
    mutationFn: () =>
      postSavedViews({
        surface,
        name: name.trim(),
        filters: filters as Record<string, unknown>,
      }),
    onSuccess: () => {
      setName('');
      invalidate();
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteSavedViewsSavedViewId(id),
    onSuccess: invalidate,
  });

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <Button variant="secondary" size="md" className={className}>
          {t('savedViews.title')} <ChevronDownIcon size={14} />
        </Button>
      </Popover.Trigger>

      <AnimatePresence>
        {open && (
          <Popover.Portal forceMount>
            <Popover.Content
              asChild
              align="end"
              sideOffset={8}
              onOpenAutoFocus={(e) => e.preventDefault()}
            >
              <motion.div
                initial={{ opacity: 0, y: -6, scale: 0.98 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                exit={{ opacity: 0, y: -6, scale: 0.98 }}
                transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
                className="z-(--z-dropdown) w-72 overflow-hidden rounded-sm border border-line bg-surface-overlay shadow-(--shadow-pop)"
              >
                <div className="max-h-72 overflow-y-auto p-1">
                  {(views ?? []).length === 0 && (
                    <p className="px-2.5 py-4 text-center text-sm text-ink-faint">
                      {t('savedViews.empty')}
                    </p>
                  )}
                  {(views ?? []).map((v) => (
                    <div
                      key={v.id}
                      className="group flex items-center gap-1 rounded-md px-1 hover:bg-surface-raised"
                    >
                      <button
                        onClick={() => {
                          try {
                            onApply(JSON.parse(v.filters) as TFilters);
                          } catch {
                            // ponytail: malformed filters JSON — ignore rather than crash the menu.
                          }
                          setOpen(false);
                        }}
                        className="flex-1 truncate px-1.5 py-2 text-left text-sm text-ink"
                      >
                        {v.name}
                      </button>
                      <IconButton
                        label={t('savedViews.delete', { name: v.name })}
                        onClick={() => remove.mutate(v.id)}
                        className="opacity-0 group-hover:opacity-100 hover:bg-err-tint hover:text-err"
                      >
                        <XIcon size={13} />
                      </IconButton>
                    </div>
                  ))}
                </div>

                <div className="flex items-center gap-1.5 border-t border-line-soft p-1.5">
                  <input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && name.trim() && !save.isPending) save.mutate();
                    }}
                    placeholder={t('savedViews.namePlaceholder')}
                    className="h-8 min-w-0 flex-1 rounded-sm border border-line-soft bg-canvas-sunken px-2 text-sm text-ink outline-none placeholder:text-ink-faint"
                  />
                  <IconButton
                    label={t('savedViews.save')}
                    disabled={!name.trim() || save.isPending}
                    onClick={() => save.mutate()}
                    className="shrink-0 hover:bg-signal-tint hover:text-signal"
                  >
                    <PlusIcon size={14} />
                  </IconButton>
                </div>
              </motion.div>
            </Popover.Content>
          </Popover.Portal>
        )}
      </AnimatePresence>
    </Popover.Root>
  );
}
