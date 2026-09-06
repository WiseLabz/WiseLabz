/**
 * A dashboard cell. Not a card — it's a region in the instrument grid: flat
 * canvas, a quiet mono header, and hairlines (provided by the parent grid's
 * gap) doing the separation. Drag handle + remove appear only in edit mode.
 */
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { motion, useDragControls } from 'motion/react';
import { IconButton } from '../ui/Button';
import { PanelHeader } from '../ui/Panel';
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
    <div className="flex h-full flex-col bg-canvas">
      <PanelHeader
        title={title}
        icon={icon}
        action={
          editing ? (
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
          )
        }
      />
      <motion.div layout="position" className="flex min-h-0 flex-1 flex-col">
        {children}
      </motion.div>
    </div>
  );
}
