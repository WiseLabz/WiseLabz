import { describe, expect, it } from 'vitest';
import { findGenBlocks, isGenMarker } from './genMarkers';

const open = (key: string) => `<!-- wl:gen key="${key}" h="3fa9c0d1e2b4" -->`;
const close = '<!-- /wl:gen -->';

describe('genMarkers', () => {
  it('recognises open and close markers only', () => {
    expect(isGenMarker(open('head'))).toBe(true);
    expect(isGenMarker(close + '\n')).toBe(true);
    expect(isGenMarker('<!-- a normal comment -->')).toBe(false);
    expect(isGenMarker('<!-- wl:gen key="x" h="nothex" -->')).toBe(false);
  });

  it('finds well-formed blocks and ignores malformed ones', () => {
    const content = [open('head'), '# Svc', close, '', 'notes', open('snap.a'), 'unclosed', open('snap.b'), 'b', close].join('\n');
    expect(findGenBlocks(content)).toEqual([
      { key: 'head', openLine: 1, closeLine: 3 },
      { key: 'snap.b', openLine: 8, closeLine: 10 },
    ]);
  });

  it('rejects a block with no body line', () => {
    expect(findGenBlocks([open('x'), close].join('\n'))).toEqual([]);
  });
});
