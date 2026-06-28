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
  const delta = now - t;
  if (delta < 5000) return 'now';
  for (const [limit, div, suffix] of UNITS) {
    if (delta < limit) return `${Math.floor(delta / div)}${suffix}`;
  }
  return `${Math.floor(delta / 2_592_000_000)}mo`;
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
