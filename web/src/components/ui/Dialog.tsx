/**
 * Modal Dialog on the native <dialog> element so it escapes overflow/stacking
 * contexts via the top layer. Raised surface, soft hairline, depth shadow.
 * Closes on Escape (native cancel) and backdrop click; focus is trapped while
 * open and restored to the trigger on close (both native behaviours). Enter/exit
 * is a quiet opacity/scale transition — covered by the global [data-motion='off']
 * rule, so it self-disables when motion is turned off.
 */
import { useEffect, useId, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { cn } from '../../lib/cn';

type Size = 'sm' | 'md' | 'lg';

interface DialogProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
  size?: Size;
}

const sizes: Record<Size, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-2xl',
};

export function Dialog({ open, onClose, title, children, size = 'md' }: DialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  // `render` keeps the dialog mounted through the exit transition; `visible`
  // drives the opacity/scale animation state.
  const [render, setRender] = useState(open);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    // Defer through rAF so the state change isn't synchronous-in-effect, and so
    // the exit transition has a frame to start before unmount.
    const id = requestAnimationFrame(() => {
      if (open) setRender(true);
      else setVisible(false);
    });
    return () => cancelAnimationFrame(id);
  }, [open]);

  useEffect(() => {
    const dlg = dialogRef.current;
    if (!dlg || !render) return;
    if (!dlg.open) dlg.showModal();
    const id = requestAnimationFrame(() => setVisible(true));
    return () => cancelAnimationFrame(id);
  }, [render]);

  if (!render) return null;

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={title ? titleId : undefined}
      onCancel={(e) => {
        // Escape key — intercept the native close so the parent owns `open`.
        e.preventDefault();
        onClose();
      }}
      className={cn(
        'fixed inset-0 m-0 h-full max-h-none w-full max-w-none bg-transparent p-0',
        'z-(--z-modal) overflow-y-auto',
        'backdrop:z-(--z-overlay) backdrop:bg-canvas/70'
      )}
    >
      <div
        role="presentation"
        onClick={(e) => {
          if (e.target === e.currentTarget) onClose();
        }}
        className="flex min-h-full items-center justify-center p-4"
      >
        <div
          onTransitionEnd={() => {
            if (!visible) {
              dialogRef.current?.close();
              setRender(false);
            }
          }}
          className={cn(
            'w-full origin-center rounded-lg border border-line-soft',
            'bg-surface shadow-(--shadow-pop)',
            'transition-[opacity,transform] duration-150 ease-out-quart',
            visible ? 'translate-y-0 scale-100 opacity-100' : 'translate-y-1 scale-95 opacity-0',
            sizes[size]
          )}
        >
          {title && (
            <header className="border-b border-line-soft px-5 py-3.5">
              <h2 id={titleId} className="font-mono text-sm font-medium text-ink">
                {title}
              </h2>
            </header>
          )}
          <div className="px-5 py-4 text-sm text-ink">{children}</div>
        </div>
      </div>
    </dialog>
  );
}
