import { describe, expect, it, vi } from 'vitest';
import { CompletionContext } from '@codemirror/autocomplete';
import { EditorState } from '@codemirror/state';
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
});
