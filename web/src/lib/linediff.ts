/**
 * Minimal line-level diff (LCS) for the doc-format DiffViewer. Inputs are small
 * generated docs, so an O(n·m) table is fine and keeps the result exact.
 */
export type DiffLineType = 'same' | 'add' | 'del';

export interface DiffLine {
  type: DiffLineType;
  text: string;
  /** line number in the "before" doc (undefined for additions) */
  before?: number;
  /** line number in the "after" doc (undefined for deletions) */
  after?: number;
}

export function lineDiff(beforeSrc: string, afterSrc: string): DiffLine[] {
  const a = beforeSrc.replace(/\n$/, '').split('\n');
  const b = afterSrc.replace(/\n$/, '').split('\n');
  const n = a.length;
  const m = b.length;

  // LCS length table
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }

  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ type: 'same', text: a[i], before: i + 1, after: j + 1 });
      i++;
      j++;
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      out.push({ type: 'del', text: a[i], before: i + 1 });
      i++;
    } else {
      out.push({ type: 'add', text: b[j], after: j + 1 });
      j++;
    }
  }
  while (i < n) {
    out.push({ type: 'del', text: a[i], before: i + 1 });
    i++;
  }
  while (j < m) {
    out.push({ type: 'add', text: b[j], after: j + 1 });
    j++;
  }
  return out;
}

export function diffStats(lines: DiffLine[]) {
  let added = 0;
  let removed = 0;
  for (const l of lines) {
    if (l.type === 'add') added++;
    else if (l.type === 'del') removed++;
  }
  return { added, removed };
}
