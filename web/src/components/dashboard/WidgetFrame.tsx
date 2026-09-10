/**
 * A dashboard cell. Not a card — it's a region in the instrument grid: flat
 * canvas, a quiet mono header, and hairlines (provided by the parent grid's
 * gap) doing the separation.
 */
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { motion } from 'motion/react';
import { IconButton } from '../ui/Button';
import { PanelHeader } from '../ui/Panel';
import { SyncIcon, PauseIcon, PlayIcon } from '../icons';

export function WidgetFrame({
  title,
  icon,
  onRefresh,
  pollingEnabled,
  onTogglePolling,
  children,
}: {
  title: string;
  icon?: ReactNode;
  onRefresh?: () => void;
  pollingEnabled?: boolean;
  onTogglePolling?: () => void;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex h-full flex-col bg-canvas">
      <PanelHeader
        title={title}
        icon={icon}
        action={
          onRefresh ? (
            <div className="flex items-center gap-0.5">
              <IconButton label={`${t('common.refresh')} ${title}`} onClick={onRefresh}>
                <SyncIcon size={15} />
              </IconButton>
              <IconButton
                label={pollingEnabled ? t('common.pause') : t('common.resume')}
                onClick={onTogglePolling}
              >
                {pollingEnabled ? <PauseIcon size={15} /> : <PlayIcon size={15} />}
              </IconButton>
            </div>
          ) : undefined
        }
      />
      <motion.div layout="position" className="flex min-h-0 flex-1 flex-col">
        {children}
      </motion.div>
    </div>
  );
}
