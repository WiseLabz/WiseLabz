import { describe, expect, it } from 'vitest';
import { wordDiff } from './worddiff';

describe('wordDiff', () => {
  it('returns empty for identical lines', () => {
    const result = wordDiff('hello world', 'hello world');
    expect(result.del).toEqual([{ text: 'hello world', changed: false }]);
    expect(result.add).toEqual([{ text: 'hello world', changed: false }]);
  });

  it('marks all as deleted when after is empty', () => {
    const result = wordDiff('hello world', '');
    expect(result.del).toEqual([{ text: 'hello world', changed: true }]);
    expect(result.add).toEqual([]);
  });

  it('marks all as added when before is empty', () => {
    const result = wordDiff('', 'hello world');
    expect(result.del).toEqual([]);
    expect(result.add).toEqual([{ text: 'hello world', changed: true }]);
  });

  it('detects word replacements', () => {
    const result = wordDiff('quick brown', 'fast brown');
    // "quick" should be deleted, "fast" should be added, "brown" is unchanged
    const delChanged = result.del.filter((s) => s.changed);
    const addChanged = result.add.filter((s) => s.changed);
    expect(delChanged.map((s) => s.text)).toContain('quick');
    expect(addChanged.map((s) => s.text)).toContain('fast');
  });

  it('preserves common words', () => {
    const result = wordDiff('hello world test', 'hello world example');
    const del = result.del.find((s) => !s.changed && s.text.includes('hello'));
    const add = result.add.find((s) => !s.changed && s.text.includes('hello'));
    expect(del).toBeDefined();
    expect(add).toBeDefined();
  });

  it('handles additions within a line', () => {
    const result = wordDiff('the quick fox', 'the very quick brown fox');
    // "very" and "brown" should be added, others unchanged or deleted
    const addedWords = result.add
      .filter((s) => s.changed)
      .map((s) => s.text.trim())
      .filter((t) => t.length > 0);
    expect(addedWords.some((w) => w.includes('very') || w.includes('brown'))).toBe(true);
  });

  it('handles deletions within a line', () => {
    const result = wordDiff('the very quick brown fox', 'the quick fox');
    const deletedWords = result.del
      .filter((s) => s.changed)
      .map((s) => s.text.trim())
      .filter((t) => t.length > 0);
    expect(deletedWords.some((w) => w.includes('very') || w.includes('brown'))).toBe(true);
  });

  it('maintains synchronized add and del lists for unchanged text', () => {
    const result = wordDiff('unchanged suffix', 'prefix unchanged suffix');
    const delUnchanged = result.del.filter((s) => !s.changed).map((s) => s.text);
    const addUnchanged = result.add.filter((s) => !s.changed).map((s) => s.text);
    // Both should contain "unchanged suffix" (in same or similar positions)
    expect(delUnchanged.join('').includes('unchanged')).toBe(true);
    expect(addUnchanged.join('').includes('unchanged')).toBe(true);
  });

  it('handles punctuation correctly', () => {
    const result = wordDiff('hello, world!', 'hello, universe!');
    // Should recognize "hello," and trailing space as unchanged
    expect(result.del.filter((s) => !s.changed).length).toBeGreaterThan(0);
    expect(result.add.filter((s) => !s.changed).length).toBeGreaterThan(0);
  });
});
