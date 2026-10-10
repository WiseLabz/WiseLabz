import {
  type Completion,
  type CompletionContext,
  type CompletionResult,
} from '@codemirror/autocomplete';
import { syntaxTree } from '@codemirror/language';
import { getSearch } from '../../api/generated/search/search';

// Escapes brackets and the punctuation that would change how the label renders.
const escapeLabel = (label: string) => label.replace(/[\\[\]`*_<|]/g, '\\$&');

function isInCode(context: CompletionContext) {
  type SyntaxNodeLike = { name: string; parent: SyntaxNodeLike | null };
  let node: SyntaxNodeLike | null = syntaxTree(context.state).resolveInner(context.pos, -1);
  while (node) {
    if (['CodeText', 'InlineCode', 'FencedCode', 'CodeBlock'].includes(node.name)) return true;
    node = node.parent;
  }
  return false;
}

export async function wikilinkCompletion(
  context: CompletionContext
): Promise<CompletionResult | null> {
  const before = context.state.sliceDoc(0, context.pos);
  const match = /\[\[([^\]\n]*)$/.exec(before);
  if (!match) return null;
  const start = context.pos - match[0].length;
  if (before[start - 1] === '!' || before[start - 1] === '\\') return null;
  if (isInCode(context)) return null;
  const generatedStart = before.lastIndexOf('<!-- wl:gen');
  const generatedEnd = before.lastIndexOf('<!-- /wl:gen -->');
  if (generatedStart > generatedEnd) return null;
  const query = match[1].split('|', 1)[0].trim();
  if (query.length < 2) return null;

  try {
    const [docs, entities] = await Promise.all([
      getSearch({ q: query, type: 'doc', limit: 10 }),
      getSearch({ q: query, type: 'entity', limit: 10 }),
    ]);
    const options = [
      ...docs.docs.map((hit) => ({
        title: hit.title,
        href: `/docs/${encodeURIComponent(hit.id)}`,
        type: 'doc',
      })),
      ...entities.entities
        .filter((hit) => hit.entityId)
        .map((hit) => ({
          title: hit.name,
          href: `/entities/${encodeURIComponent(hit.entityId)}`,
          type: hit.kind,
        })),
    ];
    return {
      from: context.pos - match[0].length,
      options: options.map((option): Completion => ({
        label: option.title,
        detail: option.type,
        apply: (view, _completion, from, to) => {
          const close = view.state.sliceDoc(to, to + 2) === ']]' ? 2 : 0;
          const insert = `[${escapeLabel(option.title)}](${option.href})`;
          view.dispatch({
            changes: { from, to: to + close, insert },
            selection: { anchor: from + insert.length },
          });
        },
      })),
      // `from` includes the leading `[[`, which the default fuzzy filter would
      // match against the labels and drop everything; re-query per keystroke.
      filter: false,
    };
  } catch {
    return null;
  }
}
