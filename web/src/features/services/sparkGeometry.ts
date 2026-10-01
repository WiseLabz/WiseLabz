import type { UptimeHistory } from '../../api/model';

export const W = 280;
export const H = 48;

export interface SparkGeometry {
  x: (iso: string) => number;
  cell: number;
  segments: { x: number; y: number }[][];
  maxLatency: number;
}

/**
 * Lay buckets out by start time over [windowStart, windowEnd] so empty
 * stretches show as gaps, and split the latency line wherever a bucket is
 * missing or has no latency (never plotted as 0).
 */
export function sparkGeometry(history: UptimeHistory): SparkGeometry {
  const start = Date.parse(history.windowStart);
  const range = Math.max(Date.parse(history.windowEnd) - start, 1);
  const bucketMs = history.bucketSeconds * 1000;
  const x = (iso: string) => ((Date.parse(iso) - start) / range) * W;
  const cell = (bucketMs / range) * W;
  const maxLatency = Math.max(1, ...history.buckets.map((b) => b.avgLatencyMs ?? 0));

  const segments: { x: number; y: number }[][] = [];
  let prevEnd: number | null = null;
  for (const b of history.buckets) {
    const t = Date.parse(b.start);
    if (b.avgLatencyMs == null) {
      prevEnd = null;
      continue;
    }
    const pt = { x: x(b.start) + cell / 2, y: H - (b.avgLatencyMs / maxLatency) * (H - 4) - 2 };
    if (prevEnd !== null && t === prevEnd) segments[segments.length - 1].push(pt);
    else segments.push([pt]);
    prevEnd = t + bucketMs;
  }
  return { x, cell, segments, maxLatency };
}
