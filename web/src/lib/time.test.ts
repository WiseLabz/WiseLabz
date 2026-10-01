import { describe, expect, it } from 'vitest';
import { relativeTime, durationLabel, clockTime, fullDate } from './time';

describe('relativeTime', () => {
  it('returns "never" for null or undefined', () => {
    expect(relativeTime(null)).toBe('never');
    expect(relativeTime(undefined)).toBe('never');
  });

  it('returns "—" for invalid dates', () => {
    expect(relativeTime('invalid-date')).toBe('—');
  });

  it('returns "now" for timestamps within 5 seconds', () => {
    const now = Date.now();
    const recent = new Date(now - 2000).toISOString();
    expect(relativeTime(recent, now)).toBe('now');
  });

  it('shows seconds for durations under 60 seconds', () => {
    const now = Date.now();
    const past = new Date(now - 30_000).toISOString();
    expect(relativeTime(past, now)).toBe('30s');
  });

  it('shows minutes for durations under 60 minutes', () => {
    const now = Date.now();
    const past = new Date(now - 5 * 60_000).toISOString();
    expect(relativeTime(past, now)).toBe('5m');
  });

  it('shows hours for durations under 24 hours', () => {
    const now = Date.now();
    const past = new Date(now - 3 * 3_600_000).toISOString();
    expect(relativeTime(past, now)).toBe('3h');
  });

  it('shows days for durations under 7 days', () => {
    const now = Date.now();
    const past = new Date(now - 4 * 86_400_000).toISOString();
    expect(relativeTime(past, now)).toBe('4d');
  });

  it('shows weeks for durations under 30 days', () => {
    const now = Date.now();
    const past = new Date(now - 2 * 604_800_000).toISOString();
    expect(relativeTime(past, now)).toBe('2w');
  });

  it('shows months for durations 30+ days', () => {
    const now = Date.now();
    const past = new Date(now - 60 * 86_400_000).toISOString();
    expect(relativeTime(past, now)).toBe('2mo');
  });

  it('handles future timestamps correctly', () => {
    const now = Date.now();
    const future = new Date(now + 10 * 60_000).toISOString();
    expect(relativeTime(future, now)).toBe('10m');
  });

  it('uses Date.now() by default', () => {
    const iso = new Date().toISOString();
    expect(relativeTime(iso)).toBe('now');
  });
});

describe('durationLabel', () => {
  it('returns "—" for null or undefined', () => {
    expect(durationLabel(null)).toBe('—');
    expect(durationLabel(undefined)).toBe('—');
  });

  it('returns "—" for NaN', () => {
    expect(durationLabel(NaN)).toBe('—');
  });

  it('formats milliseconds for durations under 1 second', () => {
    expect(durationLabel(500)).toBe('500ms');
    expect(durationLabel(0)).toBe('0ms');
    expect(durationLabel(999)).toBe('999ms');
  });

  it('formats seconds for durations under 1 minute', () => {
    expect(durationLabel(1000)).toBe('1.0s');
    expect(durationLabel(5500)).toBe('5.5s');
    expect(durationLabel(59999)).toBe('60.0s');
  });

  it('formats minutes and seconds for durations 1+ minute', () => {
    expect(durationLabel(60_000)).toBe('1m 0s');
    expect(durationLabel(90_000)).toBe('1m 30s');
    expect(durationLabel(125_000)).toBe('2m 5s');
    expect(durationLabel(3_600_000)).toBe('60m 0s');
  });

  it('rounds seconds correctly', () => {
    expect(durationLabel(60_500)).toBe('1m 1s');
    expect(durationLabel(60_499)).toBe('1m 0s');
  });
});

describe('clockTime', () => {
  it('returns "—" for invalid dates', () => {
    expect(clockTime('invalid-date')).toBe('—');
  });

  it('formats valid ISO timestamps as clock time', () => {
    const result = clockTime('2026-10-01T14:30:45Z');
    expect(result).toMatch(/\d{2}:\d{2}/);
  });

  it('respects locale', () => {
    const result = clockTime('2026-10-01T09:05:00Z');
    expect(result).toContain(':');
  });
});

describe('fullDate', () => {
  it('returns "—" for invalid dates', () => {
    expect(fullDate('invalid-date')).toBe('—');
  });

  it('formats valid ISO timestamps with month, day, and time', () => {
    const result = fullDate('2026-10-01T14:30:45Z');
    expect(result).toMatch(/Oct.*\d{1,2}.*\d{2}:\d{2}/);
  });

  it('includes time in HH:MM format', () => {
    const result = fullDate('2026-10-01T09:05:00Z');
    expect(result).toMatch(/\d{1,2}:\d{2}/);
  });
});
