import { describe, expect, it, vi } from 'vitest';
import {
  autocompletion,
  CompletionContext,
  currentCompletions,
  startCompletion,
} from '@codemirror/autocomplete';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { markdown } from '@codemirror/lang-markdown';
import { wikilinkCompletion } from './wikilinkCompletion';

const { getSearchMock } = vi.hoisted(() => ({ getSearchMock: vi.fn() }));
vi.mock('../../api/generated/search/search', () => ({ getSearch: getSearchMock }));

describe('wikilink completion', () => {
  it('offers docs and entities and inserts the selected Markdown link', async () => {
    const source = 'See [[pve';
    const state = EditorState.create({ doc: source, extensions: [markdown()] });
    getSearchMock.mockImplementation(({ type }: { type: string }) =>
      Promise.resolve({
        docs: type === 'doc' ? [{ id: 'doc-1', title: 'Proxmox guide' }] : [],
        entities: type === 'entity' ? [{ entityId: 'entity-1', name: 'pve1', kind: 'vm' }] : [],
        runbooks: [],
      })
    );
    const result = await wikilinkCompletion(new CompletionContext(state, source.length, false));

    if (!result) throw new Error('Expected wikilink completions');
    expect(result.options.map(({ label }) => label)).toEqual(['Proxmox guide', 'pve1']);
    // The default fuzzy filter would drop every option because `from` covers `[[`.
    expect(result.filter).toBe(false);
    expect(result.validFor).toBeUndefined();
    expect(getSearchMock).toHaveBeenNthCalledWith(1, { q: 'pve', type: 'doc', limit: 10 });
    expect(getSearchMock).toHaveBeenNthCalledWith(2, { q: 'pve', type: 'entity', limit: 10 });
    const changes = vi.fn();
    const apply = result?.options[0].apply;
    if (typeof apply === 'function') {
      apply(
        {
          state: { sliceDoc: (from: number, to: number) => source.slice(from, to) },
          dispatch: changes,
        } as never,
        result.options[0],
        result.from,
        source.length
      );
    }
    expect(changes).toHaveBeenCalledWith({
      changes: { from: 4, to: 9, insert: '[Proxmox guide](/docs/doc-1)' },
      selection: { anchor: 32 },
    });
  });

  it.each([
    ['fenced code', '```md\n[[pve', undefined],
    ['inline code', '`[[pve`', 6],
    ['quoted fence', '> ~~~\n> [[pve\n> ~~~', '> ~~~\n> [[pve'.length],
    ['list code', '- example:\n\n      [[pve', undefined],
    [
      'exact code delimiters',
      '``example ``` lone ` [[pve trailing``',
      '``example ``` lone ` [[pve'.length,
    ],
    ['embed', '![[pve', undefined],
    ['escaped input', '\\[[pve', undefined],
    ['generated content', '<!-- wl:gen key="x" -->\n[[pve', undefined],
  ])('does not offer completion inside %s', async (_name, source, cursor) => {
    getSearchMock.mockClear();
    const text = source as string;
    const position = (cursor as number | undefined) ?? text.length;
    const state = EditorState.create({ doc: text, extensions: [markdown()] });
    expect(await wikilinkCompletion(new CompletionContext(state, position, false))).toBeNull();
    expect(getSearchMock).not.toHaveBeenCalled();
  });

  it('escapes brackets and rendering punctuation in the inserted label', async () => {
    const source = '[[abc';
    const state = EditorState.create({ doc: source, extensions: [markdown()] });
    getSearchMock.mockImplementation(({ type }: { type: string }) =>
      Promise.resolve({
        docs: type === 'doc' ? [{ id: 'doc-1', title: 'a_b *c* `d` <e> | [f] \\g' }] : [],
        entities: [],
        runbooks: [],
      })
    );
    const result = await wikilinkCompletion(new CompletionContext(state, source.length, false));
    const apply = result?.options[0].apply;
    if (!result || typeof apply !== 'function') throw new Error('Expected wikilink completions');
    const changes = vi.fn();
    apply(
      { state: { sliceDoc: () => '' }, dispatch: changes } as never,
      result.options[0],
      result.from,
      source.length
    );
    expect(changes.mock.calls[0][0].changes.insert).toBe(
      '[a\\_b \\*c\\* \\`d\\` \\<e> \\| \\[f\\] \\\\g](/docs/doc-1)'
    );
  });

  it('keeps options visible in a real editor while typing after the brackets', async () => {
    getSearchMock.mockImplementation(({ type }: { type: string }) =>
      Promise.resolve({
        docs: type === 'doc' ? [{ id: 'doc-1', title: 'Proxmox guide' }] : [],
        entities: [],
        runbooks: [],
      })
    );
    const parent = document.body.appendChild(document.createElement('div'));
    const view = new EditorView({
      parent,
      extensions: [markdown(), autocompletion({ override: [wikilinkCompletion] })],
    });
    try {
      view.dispatch({
        changes: { from: 0, insert: '[[pve' },
        selection: { anchor: 5 },
        userEvent: 'input.type',
      });
      startCompletion(view);
      await vi.waitFor(() =>
        expect(currentCompletions(view.state).map(({ label }) => label)).toEqual(['Proxmox guide'])
      );
    } finally {
      view.destroy();
      parent.remove();
    }
  });
});
