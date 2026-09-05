/** Compact relative time ("3m", "2h", "4d") + absolute formatters. */
const UNITS: [limit: number, div: number, suffix: string][] = [
  [60_000, 1000, 's'],
  [3_600_000, 60_000, 'm'],
  [86_400_000, 3_600_000, 'h'],
  [604_800_000, 86_400_000, 'd'],
  [2_592_000_000, 604_800_000, 'w'],
];

export function relativeTime(iso: string | null | undefined, now = Date.now()): string {
  if (!iso) return 'never';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '—';
  // abs() so this also reads future timestamps (e.g. a scheduled next-run) correctly,
  // not just the past ones ("ago") it was originally written for.
  const delta = Math.abs(now - t);
  if (delta < 5000) return 'now';
  for (const [limit, div, suffix] of UNITS) {
    if (delta < limit) return `${Math.floor(delta / div)}${suffix}`;
  }
  return `${Math.floor(delta / 2_592_000_000)}mo`;
}

/** Humanize a duration in ms ("340ms", "1.2s", "3m 5s"). */
export function durationLabel(ms: number | null | undefined): string {
  if (ms == null || Number.isNaN(ms)) return '—';
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  const m = Math.floor(ms / 60_000);
  const s = Math.round((ms % 60_000) / 1000);
  return `${m}m ${s}s`;
}

export function clockTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

export function fullDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}
