/**
 * Generic tone tag — a square marker + a caller-supplied word on a matching tint,
 * so meaning survives grayscale and color-blindness (status is never color alone).
 * Small and tight (rounded-sm). Use for arbitrary tone+label tags where the label
 * is not an API `Severity` (for that, prefer `SeverityTag` from StatusDot). The
 * marker is the shared StatusDot marker; the palette comes from `status.ts`.
 */
import { cn } from '../../lib/cn';
import { StatusMarker } from './StatusDot';
import { toneColor, type Tone } from './status';

interface ToneTagProps {
  tone: Tone;
  label: string;
  className?: string;
}

export function ToneTag({ tone, label, className }: ToneTagProps) {
  const { fg, tint } = toneColor[tone];
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-sm px-1.5 py-0.5 font-mono text-2xs font-medium',
        className,
      )}
      style={{ color: fg, backgroundColor: tint }}
    >
      <StatusMarker tone={tone} />
      {label}
    </span>
  );
}
