/**
 * Confirmation prompt built on Dialog. Copy-agnostic: every string (title,
 * description, button labels) is supplied by the caller. `tone='danger'` routes
 * the confirm action through the danger Button variant for destructive ops.
 */
import type { ReactNode } from 'react';
import { Button } from './Button';
import { Dialog } from './Dialog';

interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: ReactNode;
  description?: ReactNode;
  confirmLabel: ReactNode;
  cancelLabel: ReactNode;
  tone?: 'default' | 'danger';
  confirmDisabled?: boolean;
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel,
  cancelLabel,
  tone = 'default',
  confirmDisabled = false,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onClose={onClose} title={title} size="sm">
      {description && <p className="text-sm leading-relaxed text-ink-muted">{description}</p>}
      <div className="mt-5 flex items-center justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onClose}>
          {cancelLabel}
        </Button>
        <Button
          variant={tone === 'danger' ? 'danger' : 'primary'}
          size="sm"
          onClick={onConfirm}
          disabled={confirmDisabled}
        >
          {confirmLabel}
        </Button>
      </div>
    </Dialog>
  );
}
