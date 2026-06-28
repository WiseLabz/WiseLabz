/**
 * Word-level intra-line diff for the doc DiffViewer. Given a paired removed/added
 * line, splits each side into segments marked changed / unchanged so the viewer
 * can highlight exactly what moved inside the line (JetBrains-style). Uses jsdiff
 * (`diffWordsWithSpace`) — battle-tested word tokenizing and Unicode handling.
 */
import { diffWordsWithSpace } from 'diff';

export interface WordSegment {
  text: string;
  /** true when this segment is an add (on the after line) or a delete (before line) */
  changed: boolean;
}

export interface LineWordDiff {
  del: WordSegment[];
  add: WordSegment[];
}

/**
 * Diff two single lines into highlight segments for each side. The `del` segments
 * describe the "before" line (unchanged + removed words); `add` the "after" line
 * (unchanged + added words).
 */
export function wordDiff(before: string, after: string): LineWordDiff {
  const parts = diffWordsWithSpace(before, after);
  const del: WordSegment[] = [];
  const add: WordSegment[] = [];
  for (const p of parts) {
    if (p.added) {
      add.push({ text: p.value, changed: true });
    } else if (p.removed) {
      del.push({ text: p.value, changed: true });
    } else {
      del.push({ text: p.value, changed: false });
      add.push({ text: p.value, changed: false });
    }
  }
  return { del, add };
}
