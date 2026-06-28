/**
 * A dashboard cell. Not a card — it's a region in the instrument grid: flat
 * canvas, a quiet mono header, and hairlines (provided by the parent grid's
 * gap) doing the separation. Drag handle + remove appear only in edit mode.
 */
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { motion, useDragControls } from 'motion/react';
import { IconButton } from '../ui/Button';
import { GripIcon, XIcon } from '../icons';

export function WidgetFrame({
  title,
  icon,
  action,
  editing,
  onRemove,
  dragControls,
  children,
}: {
  title: string;
  icon?: ReactNode;
  action?: ReactNode;
  editing?: boolean;
  onRemove?: () => void;
  dragControls?: ReturnType<typeof useDragControls>;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex h-full flex-col  bg-canvas">
      <div className="flex items-center justify-between gap-3 border-b border-line-soft px-4 py-2.5">
        <div className="flex items-center gap-2 text-ink-faint">
          <span className="text-signal">{icon}</span>
          <h3 className="font-mono text-2xs uppercase tracking-[0.16em] text-ink-muted">
            {title}
          </h3>
        </div>
        {editing ? (
          <div className="flex items-center gap-0.5">
            <button
              aria-label={t('dashboard.dragReorder')}
              onPointerDown={(e) => dragControls?.start(e)}
              className="flex h-8 w-8 cursor-grab items-center justify-center rounded-sm text-ink-faint hover:bg-surface-raised active:cursor-grabbing"
            >
              <GripIcon size={16} />
            </button>
            <IconButton label={`${t('common.remove')} ${title}`} onClick={onRemove}>
              <XIcon size={15} />
            </IconButton>
          </div>
        ) : (
          action
        )}
      </div>
      <motion.div layout="position" className="flex min-h-0 flex-1 flex-col">
        {children}
      </motion.div>
    </div>
  );
}
