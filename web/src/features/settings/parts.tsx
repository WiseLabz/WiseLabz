/**
 * Shared building blocks for the Settings sub-pages — kept here so every sub-page
 * has one consistent field / toggle / section vocabulary that matches DESIGN.md
 * (mono labels, hairline inputs, signal only for the active toggle, status as
 * shape+word). No new global primitives; these are local compositions.
 */
import { forwardRef } from 'react';
import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from 'react';
import { motion } from 'motion/react';
import { Panel } from '../../components/ui/Panel';
import { cn } from '../../lib/cn';

/** Page heading shared by every sub-page. */
export function SubHeader({ title, description }: { title: string; description?: string }) {
  return (
    <header className="mb-5">
      <h1 className="text-xl font-semibold tracking-tight text-ink">{title}</h1>
      {description && <p className="mt-0.5 text-sm text-ink-muted">{description}</p>}
    </header>
  );
}

/** A titled card region. */
export function Section({
  title,
  description,
  action,
  children,
  className,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Panel className={cn('mb-4 p-5', className)}>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-ink">{title}</h2>
          {description && (
            <p className="mt-0.5 text-xs leading-relaxed text-ink-muted">{description}</p>
          )}
        </div>
        {action}
      </div>
      {children}
    </Panel>
  );
}

/** Labelled form field wrapping any control. */
export function Field({
  label,
  hint,
  htmlFor,
  children,
  className,
}: {
  label: string;
  hint?: string;
  htmlFor?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <label htmlFor={htmlFor} className={cn('block', className)}>
      <span className="mb-1.5 block font-mono text-2xs uppercase tracking-[0.16em] text-ink-faint">
        {label}
      </span>
      {children}
      {hint && <span className="mt-1 block text-2xs leading-relaxed text-ink-faint">{hint}</span>}
    </label>
  );
}

const controlBase =
  'w-full rounded-md border border-[var(--color-line-strong)] bg-[var(--color-canvas-sunken)] ' +
  'px-3 py-2 text-sm text-[var(--color-ink)] placeholder:text-[var(--color-ink-faint)] ' +
  'transition-colors hover:border-[var(--color-line)] disabled:opacity-50';

export const TextInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...rest }, ref) => (
    <input ref={ref} className={cn(controlBase, className)} {...rest} />
  )
);
TextInput.displayName = 'TextInput';

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, children, ...rest }, ref) => (
    <select ref={ref} className={cn(controlBase, 'font-mono', className)} {...rest}>
      {children}
    </select>
  )
);
Select.displayName = 'Select';

/** Accessible on/off switch (role=switch) with a spring thumb that honours motion settings. */
export function Toggle({
  checked,
  onChange,
  label,
  disabled,
  size = 'md',
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label: string;
  disabled?: boolean;
  size?: 'sm' | 'md';
}) {
  const w = size === 'sm' ? 'h-4 w-8' : 'h-5 w-9';
  const knob = size === 'sm' ? 'h-3 w-3' : 'h-4 w-4';
  const travel = size === 'sm' ? '17px' : '18px';
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn('relative shrink-0 rounded-full transition-colors disabled:opacity-50', w)}
      style={{ backgroundColor: checked ? 'var(--color-signal)' : 'var(--color-line-strong)' }}
    >
      <motion.span
        layout
        transition={{ type: 'spring', stiffness: 600, damping: 34 }}
        className={cn('absolute top-0.5 rounded-full bg-white', knob)}
        style={{ left: checked ? travel : '2px' }}
      />
    </button>
  );
}

/** A labelled toggle row — label/description left, switch right. */
export function ToggleRow({
  title,
  description,
  checked,
  onChange,
  disabled,
}: {
  title: string;
  description?: string;
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-lg border border-line-soft p-3.5">
      <div className="flex-1">
        <p className="text-sm font-medium text-ink">{title}</p>
        {description && (
          <p className="mt-0.5 text-2xs leading-relaxed text-ink-muted">{description}</p>
        )}
      </div>
      <div className="mt-0.5">
        <Toggle checked={checked} onChange={onChange} label={title} disabled={disabled} />
      </div>
    </div>
  );
}
