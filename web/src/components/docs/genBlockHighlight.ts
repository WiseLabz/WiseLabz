/**
 * CodeMirror extension that tints the sync-owned (generated) blocks of a doc
 * and dims their marker lines, so editors can see which text sync manages:
 * edits inside a tinted block are flagged for review on the next sync, text
 * outside is never touched. See lib/genMarkers.ts.
 */
import { RangeSetBuilder } from '@codemirror/state';
import { Decoration, EditorView, ViewPlugin, type DecorationSet, type ViewUpdate } from '@codemirror/view';
import { findGenBlocks } from '../../lib/genMarkers';

function build(view: EditorView, title: string): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>();
  const doc = view.state.doc;
  const marker = Decoration.line({ class: 'cm-gen-marker' });
  const body = Decoration.line({ class: 'cm-gen-line', attributes: { title } });
  for (const b of findGenBlocks(doc.toString())) {
    for (let n = b.openLine; n <= b.closeLine; n++) {
      const line = doc.line(n);
      builder.add(line.from, line.from, n === b.openLine || n === b.closeLine ? marker : body);
    }
  }
  return builder.finish();
}

/** Line decorations for generated blocks; `title` is the hover tooltip. */
export function genBlockHighlight(title: string) {
  return [
    ViewPlugin.fromClass(
      class {
        decorations: DecorationSet;
        constructor(view: EditorView) {
          this.decorations = build(view, title);
        }
        update(u: ViewUpdate) {
          if (u.docChanged) this.decorations = build(u.view, title);
        }
      },
      { decorations: (v) => v.decorations }
    ),
    EditorView.baseTheme({
      '.cm-gen-line': { backgroundColor: 'var(--color-accent-primary-tint)' },
      '.cm-gen-marker': { opacity: '0.45', fontSize: '0.85em' },
    }),
  ];
}
