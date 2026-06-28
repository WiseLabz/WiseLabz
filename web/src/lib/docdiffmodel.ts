/**
 * Row model for the JetBrains-style doc diff. Turns two document revisions into
 * aligned rows that both the unified and side-by-side views render from a single
 * source of truth:
 *   1. line-level LCS (lib/linediff)
 *   2. pair adjacent delete/add runs into `mod` rows + compute intra-line word
 *      highlighting (lib/worddiff)
 *   3. collapse long unchanged runs into foldable gaps (context fold), which also
 *      bounds rendered rows on large docs.
 */
import { lineDiff, diffStats, type DiffLine } from './linediff';
import { wordDiff, type WordSegment } from './worddiff';

export interface DiffRowUnit {
  type: 'same' | 'mod' | 'del' | 'add';
  /** before-side line (absent for a pure add) */
  left?: DiffLine;
  /** after-side line (absent for a pure delete) */
  right?: DiffLine;
  /** word segments for the before line (mod rows only) */
  leftWords?: WordSegment[];
  /** word segments for the after line (mod rows only) */
  rightWords?: WordSegment[];
}

export type DocRow =
  | { kind: 'unit'; unit: DiffRowUnit }
  | { kind: 'gap'; units: DiffRowUnit[]; count: number };

export interface DocDiffModel {
  rows: DocRow[];
  stats: { added: number; removed: number };
}

/** Pair delete/add runs into aligned row units, with word highlighting on pairs. */
function toUnits(lines: DiffLine[]): DiffRowUnit[] {
  const units: DiffRowUnit[] = [];
  let i = 0;
  while (i < lines.length) {
    const t = lines[i].type;
    if (t === 'same') {
      units.push({ type: 'same', left: lines[i], right: lines[i] });
      i++;
      continue;
    }
    // Gather a maximal del-run immediately followed by an add-run; pair them.
    let d = i;
    while (d < lines.length && lines[d].type === 'del') d++;
    let a = d;
    while (a < lines.length && lines[a].type === 'add') a++;
    const dels = lines.slice(i, d);
    const adds = lines.slice(d, a);
    const paired = Math.min(dels.length, adds.length);
    for (let k = 0; k < paired; k++) {
      const wd = wordDiff(dels[k].text, adds[k].text);
      units.push({
        type: 'mod',
        left: dels[k],
        right: adds[k],
        leftWords: wd.del,
        rightWords: wd.add,
      });
    }
    for (let k = paired; k < dels.length; k++) units.push({ type: 'del', left: dels[k] });
    for (let k = paired; k < adds.length; k++) units.push({ type: 'add', right: adds[k] });
    i = a;
  }
  return units;
}

/** Collapse runs of `same` units longer than the kept context into foldable gaps. */
function fold(units: DiffRowUnit[], context: number, minFold: number): DocRow[] {
  const rows: DocRow[] = [];
  let i = 0;
  while (i < units.length) {
    if (units[i].type !== 'same') {
      rows.push({ kind: 'unit', unit: units[i] });
      i++;
      continue;
    }
    let j = i;
    while (j < units.length && units[j].type === 'same') j++;
    const run = units.slice(i, j);
    const atStart = i === 0;
    const atEnd = j === units.length;
    const lead = atStart ? 0 : context;
    const tail = atEnd ? 0 : context;
    if (run.length > lead + tail + minFold) {
      const head = run.slice(0, lead);
      const foot = run.slice(run.length - tail);
      const hidden = run.slice(head.length, run.length - foot.length);
      for (const u of head) rows.push({ kind: 'unit', unit: u });
      rows.push({ kind: 'gap', units: hidden, count: hidden.length });
      for (const u of foot) rows.push({ kind: 'unit', unit: u });
    } else {
      for (const u of run) rows.push({ kind: 'unit', unit: u });
    }
    i = j;
  }
  return rows;
}

export function buildDocDiff(
  before: string,
  after: string,
  { context = 3, minFold = 2 }: { context?: number; minFold?: number } = {},
): DocDiffModel {
  const lines = lineDiff(before, after);
  const units = toUnits(lines);
  return { rows: fold(units, context, minFold), stats: diffStats(lines) };
}
