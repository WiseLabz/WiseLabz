import { describe, expect, it } from 'vitest';
import { cn } from './cn';

describe('cn', () => {
  it('combines class strings', () => {
    expect(cn('px-2', 'py-1')).toContain('px-2');
    expect(cn('px-2', 'py-1')).toContain('py-1');
  });

  it('handles conditional classes', () => {
    const result = cn('base', true && 'included', false && 'excluded');
    expect(result).toContain('base');
    expect(result).toContain('included');
    expect(result).not.toContain('excluded');
  });

  it('resolves Tailwind conflicts', () => {
    // Later value should override earlier one
    const result = cn('px-2', 'px-4');
    expect(result).toContain('px-4');
    expect(result).not.toContain('px-2');
  });

  it('handles objects with conditional classes', () => {
    const result = cn({
      'text-red-500': true,
      'text-blue-500': false,
    });
    expect(result).toContain('text-red-500');
    expect(result).not.toContain('text-blue-500');
  });

  it('handles arrays of classes', () => {
    const result = cn(['px-2', 'py-1'], 'ml-4');
    expect(result).toContain('px-2');
    expect(result).toContain('py-1');
    expect(result).toContain('ml-4');
  });

  it('handles empty inputs', () => {
    const result = cn('');
    expect(result).toBe('');
  });

  it('handles multiple Tailwind conflicts with last value winning', () => {
    const result = cn('text-sm', 'text-lg', 'text-base');
    expect(result).toContain('text-base');
    expect(result).not.toContain('text-lg');
    expect(result).not.toContain('text-sm');
  });

  it('handles undefined and null', () => {
    const result = cn('px-2', undefined, null, 'py-1');
    expect(result).toContain('px-2');
    expect(result).toContain('py-1');
  });
});
