/**
 * Relative timestamp — renders the compact "3m"/"2h"/"4d" form from lib/time and
 * keeps it fresh on a quiet interval. Wrapped in a <time> with the machine ISO in
 * `dateTime` and the full absolute date in `title`. Tabular nums so it never
 * jitters as the value ticks.
 */
import { useEffect, useState } from 'react';
import { cn } from '../../lib/cn';
import { fullDate, relativeTime } from '../../lib/time';

interface TimeAgoProps {
  at: string | number;
  className?: string;
}

export function TimeAgo({ at, className }: TimeAgoProps) {
  const iso = typeof at === 'number' ? new Date(at).toISOString() : at;
  const [, tick] = useState(0);

  useEffect(() => {
    const id = setInterval(() => tick((n) => n + 1), 30_000);
    return () => clearInterval(id);
  }, []);

  return (
    <time dateTime={iso} title={fullDate(iso)} className={cn('nums', className)}>
      {relativeTime(iso)}
    </time>
  );
}
