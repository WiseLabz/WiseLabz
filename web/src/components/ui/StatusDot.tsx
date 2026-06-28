/**
 * Status / severity indicators, instrument style: a crisp square marker (no glow,
 * no ping) paired with a mono word. Square = precise/mechanical; the word means
 * it survives grayscale and color-blindness. Live status gets a quiet pulse only.
 */
import { cn } from '../../lib/cn';
import { statusMeta, severityMeta, toneColor, type Tone } from './status';
import type { ServiceStatus, Severity } from '../../api/model';

function Marker({ tone, live }: { tone: Tone; live?: boolean }) {
  const { fg } = toneColor[tone];
  return (
    <span
      className={cn('inline-block h-[7px] w-[7px] shrink-0', live && 'motion-safe:animate-pulse')}
      style={{ backgroundColor: fg }}
    />
  );
}

export function StatusPill({
  status,
  className,
}: {
  status: ServiceStatus;
  className?: string;
}) {
  const { label, tone } = statusMeta[status];
  return (
    <span
      className={cn('inline-flex items-center gap-2 font-mono text-xs', className)}
      style={{ color: toneColor[tone].fg }}
    >
      <Marker tone={tone} live={status === 'online'} />
      {label}
    </span>
  );
}

export function SeverityTag({
  severity,
  className,
}: {
  severity: Severity;
  className?: string;
}) {
  const { label, tone } = severityMeta[severity];
  const { fg, tint } = toneColor[tone];
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-sm px-1.5 py-0.5 font-mono text-2xs font-medium',
        className,
      )}
      style={{ color: fg, backgroundColor: tint }}
    >
      <span className="h-[6px] w-[6px]" style={{ backgroundColor: fg }} />
      {label}
    </span>
  );
}

export { Marker as StatusMarker };
