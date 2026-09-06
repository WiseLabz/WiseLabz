/**
 * A framed region in the soft-dark register: a hairline border at rest
 * (rounded-but-tight, 12px), no shadow — depth is reserved for genuinely
 * raised/floating surfaces (dialogs, dropdowns). Header stays a quiet
 * sentence-case mono label, optionally with a rack-unit tag and a Swiss
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
        'flex flex-col overflow-hidden rounded-lg border border-line-soft',
        'bg-surface',
        className
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
  unit,
  count,
  action,
  className,
}: {
  title: string;
  icon?: ReactNode;
  /** Rack-unit tag (e.g. "U1", "U2") shown before the title. */
  unit?: string;
  count?: number | string;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'flex items-center justify-between gap-3 border-b border-line-soft px-4 py-2.5',
        className
      )}
    >
      <div className="flex items-center gap-2 text-ink-faint">
        <span className="text-accent-primary">{icon}</span>
        <h3 className="font-mono text-sm text-ink-muted">
          {unit && <span className="mr-1.5 text-accent-secondary">{unit}</span>}
          {title}
          {count !== undefined && <span className="ml-1.5 text-ink-faint">/ {count}</span>}
        </h3>
      </div>
      {action}
    </div>
  );
}
