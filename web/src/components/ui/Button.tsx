/**
 * Button vocabulary — mono label, sharp corners, one signal accent for primary.
 * Full state coverage (hover, active, focus-visible, disabled). Flat: a single
 * solid fill or a single hairline border, never border + glow.
 */
import { forwardRef } from 'react';
import type { ButtonHTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';
type Size = 'sm' | 'md';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

const base =
  'inline-flex items-center justify-center gap-2 rounded-sm font-mono font-medium whitespace-nowrap ' +
  'transition-[background-color,border-color,color,transform] duration-150 ease-[var(--ease-snap)] ' +
  'active:translate-y-px disabled:pointer-events-none disabled:opacity-40 select-none';

const sizes: Record<Size, string> = {
  sm: 'h-7 px-2.5 text-xs',
  md: 'h-9 px-3.5 text-sm',
};

const variants: Record<Variant, string> = {
  primary:
    'bg-[var(--color-signal)] text-[var(--color-signal-ink)] hover:bg-[var(--color-signal-bright)]',
  secondary:
    'bg-transparent text-[var(--color-ink)] border border-[var(--color-line-strong)] ' +
    'hover:border-[var(--color-signal-soft)] hover:bg-[var(--color-surface)]',
  ghost:
    'text-[var(--color-ink-muted)] hover:text-[var(--color-ink)] hover:bg-[var(--color-surface)]',
  danger:
    'bg-transparent text-[var(--color-err)] border border-[var(--color-err)]/40 ' +
    'hover:bg-[var(--color-err-tint)]',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'secondary', size = 'md', className, ...rest }, ref) => (
    <button ref={ref} className={cn(base, sizes[size], variants[variant], className)} {...rest} />
  )
);
Button.displayName = 'Button';

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string;
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ label, className, children, ...rest }, ref) => (
    <button
      ref={ref}
      aria-label={label}
      title={label}
      className={cn(
        'inline-flex h-8 w-8 items-center justify-center rounded-sm text-ink-muted',
        'transition-colors duration-150 hover:bg-surface hover:text-ink',
        'active:translate-y-px disabled:opacity-40',
        className
      )}
      {...rest}
    >
      {children}
    </button>
  )
);
IconButton.displayName = 'IconButton';
