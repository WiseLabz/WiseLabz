import { describe, expect, it } from 'vitest';
import { lineDiff, diffStats, type DiffLine } from './linediff';

describe('lineDiff', () => {
  it('returns empty diff for identical strings', () => {
    const result = lineDiff('hello\nworld', 'hello\nworld');
    expect(result).toEqual([
      { type: 'same', text: 'hello', before: 1, after: 1 },
      { type: 'same', text: 'world', before: 2, after: 2 },
    ]);
  });

  it('detects added lines', () => {
    const result = lineDiff('hello', 'hello\nworld');
    expect(result).toContainEqual({ type: 'same', text: 'hello', before: 1, after: 1 });
    expect(result).toContainEqual({ type: 'add', text: 'world', after: 2 });
  });

  it('detects deleted lines', () => {
    const result = lineDiff('hello\nworld', 'hello');
    expect(result).toContainEqual({ type: 'same', text: 'hello', before: 1, after: 1 });
    expect(result).toContainEqual({ type: 'del', text: 'world', before: 2 });
  });

  it('handles trailing newlines consistently', () => {
    const withNewline = lineDiff('hello\n', 'hello');
    const withoutNewline = lineDiff('hello', 'hello');
    expect(withNewline).toEqual(withoutNewline);
  });

  it('handles empty before string', () => {
    const result = lineDiff('', 'hello\nworld');
    const adds = result.filter((l) => l.type === 'add');
    expect(adds).toHaveLength(2);
    expect(adds.map((l) => l.text)).toEqual(['hello', 'world']);
  });

  it('handles empty after string', () => {
    const result = lineDiff('hello\nworld', '');
    const dels = result.filter((l) => l.type === 'del');
    expect(dels).toHaveLength(2);
    expect(dels.map((l) => l.text)).toEqual(['hello', 'world']);
  });

  it('handles completely replaced content', () => {
    const result = lineDiff('apple\nbanana', 'orange\ngrape');
    expect(result).toHaveLength(4);
    expect(result.filter((l) => l.type === 'del')).toHaveLength(2);
    expect(result.filter((l) => l.type === 'add')).toHaveLength(2);
  });

  it('finds longest common subsequence', () => {
    const result = lineDiff('a\nb\nc\nd', 'a\nx\nc\nd');
    const same = result.filter((l) => l.type === 'same');
    expect(same.map((l) => l.text)).toEqual(['a', 'c', 'd']);
  });

  it('assigns correct line numbers', () => {
    const result = lineDiff('a\nb\nc', 'x\na\ny\nb\nz\nc');
    const beforeLines = result.filter((l) => l.before !== undefined).map((l) => l.before);
    const afterLines = result.filter((l) => l.after !== undefined).map((l) => l.after);
    expect(Math.max(...beforeLines)).toBe(3);
    expect(Math.max(...afterLines)).toBe(6);
  });
});

describe('diffStats', () => {
  it('counts additions and deletions', () => {
    const lines: DiffLine[] = [
      { type: 'same', text: 'a', before: 1, after: 1 },
      { type: 'add', text: 'b', after: 2 },
      { type: 'add', text: 'c', after: 3 },
      { type: 'del', text: 'd', before: 2 },
    ];
    expect(diffStats(lines)).toEqual({ added: 2, removed: 1 });
  });

  it('handles empty diff', () => {
    expect(diffStats([])).toEqual({ added: 0, removed: 0 });
  });

  it('ignores same lines', () => {
    const lines: DiffLine[] = [
      { type: 'same', text: 'a', before: 1, after: 1 },
      { type: 'same', text: 'b', before: 2, after: 2 },
    ];
    expect(diffStats(lines)).toEqual({ added: 0, removed: 0 });
  });
});
