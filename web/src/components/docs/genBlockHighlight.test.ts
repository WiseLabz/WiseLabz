import { afterEach, describe, expect, it } from 'vitest';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { genBlockHighlight } from './genBlockHighlight';

describe('genBlockHighlight', () => {
  let view: EditorView | undefined;
  afterEach(() => view?.destroy());

  it('tints generated block lines and dims their markers', () => {
    const doc = [
      'Human intro',
      '<!-- wl:gen key="snap.status" h="3fa9c0d1e2b4" -->',
      'healthy',
      '<!-- /wl:gen -->',
      'Human outro',
    ].join('\n');
    view = new EditorView({
      state: EditorState.create({ doc, extensions: [genBlockHighlight('generated')] }),
      parent: document.body,
    });
    const lines = [...view.contentDOM.querySelectorAll('.cm-line')];
    expect(lines.map((l) => l.className.includes('cm-gen-marker'))).toEqual([false, true, false, true, false]);
    expect(lines.map((l) => l.className.includes('cm-gen-line'))).toEqual([false, false, true, false, false]);
    expect(lines[2].getAttribute('title')).toBe('generated');
  });
});
