/**
 * Compact pager — prev/next IconButton controls flanking a mono `NN / NN`
 * indicator. Controls disable at the bounds; numerals are tabular so the
 * indicator never jitters as the page count changes.
 */
import { IconButton } from './Button';
import { cn } from '../../lib/cn';

interface PaginationProps {
  page: number;
  pageCount: number;
  onPage: (page: number) => void;
  className?: string;
  prevLabel?: string;
  nextLabel?: string;
}

function Chevron({ dir }: { dir: 'left' | 'right' }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={dir === 'left' ? 'M15 18l-6-6 6-6' : 'M9 18l6-6-6-6'} />
    </svg>
  );
}

export function Pagination({
  page,
  pageCount,
  onPage,
  className,
  prevLabel = 'Previous page',
  nextLabel = 'Next page',
}: PaginationProps) {
  const atStart = page <= 1;
  const atEnd = page >= pageCount;
  return (
    <nav className={cn('flex items-center gap-1.5', className)} aria-label="Pagination">
      <IconButton label={prevLabel} disabled={atStart} onClick={() => onPage(page - 1)}>
        <Chevron dir="left" />
      </IconButton>
      <span
        className="nums px-1 font-mono text-xs text-ink-muted"
        aria-live="polite"
        aria-atomic="true"
      >
        {page} <span className="text-ink-faint">/</span> {pageCount}
      </span>
      <IconButton label={nextLabel} disabled={atEnd} onClick={() => onPage(page + 1)}>
        <Chevron dir="right" />
      </IconButton>
    </nav>
  );
}
