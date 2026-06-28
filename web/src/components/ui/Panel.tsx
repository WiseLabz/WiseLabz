/**
 * A framed region in the soft-dark register: a raised surface with a soft hairline
 * and a layered depth shadow (rounded-but-tight, 12px). Replaces the flat
 * sharp-Blueprint panel. Header stays a quiet mono label, optionally with a Swiss
 * `/ NN` count, never an uppercase marketing eyebrow.
 */
import type { ReactNode } from 'react';
import { cn } from '../../lib/cn';

export function Panel({
  className,
  children,
  ...rest
}: { className?: string; children: ReactNode } & React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'flex flex-col overflow-hidden rounded-lg border border-[var(--color-line-soft)]',
        'bg-[var(--color-surface)] shadow-[var(--shadow-panel)]',
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}

export function PanelHeader({
  title,
  icon,
  count,
  action,
  className,
}: {
  title: string;
  icon?: ReactNode;
  count?: number | string;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'flex items-center justify-between gap-3 border-b border-[var(--color-line-soft)] px-4 py-2.5',
        className,
      )}
    >
      <div className="flex items-center gap-2 text-[var(--color-ink-faint)]">
        <span className="text-[var(--color-signal)]">{icon}</span>
        <h3 className="font-mono text-2xs uppercase tracking-[0.16em] text-[var(--color-ink-muted)]">
          {title}
          {count !== undefined && (
            <span className="ml-1.5 text-[var(--color-ink-faint)]">/ {count}</span>
          )}
        </h3>
      </div>
      {action}
    </div>
  );
}
