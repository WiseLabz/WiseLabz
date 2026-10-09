import { isMap, isScalar, isSeq, type Node } from 'yaml';
import { recipeFormat, type RecipeFormatField, type RecipeFormatNode } from './recipeFormat';
import { parseRecipeDocument } from './recipeDocument';

export type RecipeErrorTarget = {
  path: (string | number)[];
  id: string;
  from: number;
  to: number;
  line: number;
};

export function recipeFieldId(path: (string | number)[]): string {
  return `recipe-field-${path.map((part) => String(part).replace(/-/g, '--')).join('-')}`;
}

// Locations name recipe nodes, not validation rules. Map names can themselves contain dots.
export function resolveRecipeError(text: string, field: string): RecipeErrorTarget | undefined {
  const location = field.replace(/^config\./, '').replace(/^recipe\.?/, '');
  if (!/^(config\.)?recipe(?:[.[]|$)/.test(field)) return undefined;
  const { document, error } = parseRecipeDocument(text);
  if (error || !document?.contents) return undefined;
  const path: (string | number)[] = [];
  let node: Node | null = document.contents as Node;
  let remaining = location;
  // Below a block value (an endpoint or action body) the location names data, not rows: keep walking for the
  // line, but report the body itself as the row.
  let insideBlock = false;
  while (remaining) {
    remaining = remaining.replace(/^\./, '');
    if (isSeq(node)) {
      const match = /^\[(\d+)\]/.exec(remaining);
      if (!match) return undefined;
      const index = Number(match[1]);
      if (!insideBlock) path.push(index);
      node = node.items[index] as Node | null;
      remaining = remaining.slice(match[0].length);
    } else if (isMap(node)) {
      // Longest match handles names such as attributes.service.status and query keys with dots.
      const key = node.items.map((pair) => isScalar(pair.key) ? String(pair.key.value) : '')
        .filter((candidate) => candidate && (remaining === candidate || remaining.startsWith(`${candidate}.`) || remaining.startsWith(`${candidate}[`)))
        .sort((a, b) => b.length - a.length)[0];
      if (!key) {
        if (insideBlock) break;
        const missing = remaining.match(/^[^.[\]]+/)?.[0];
        if (!missing || !knownField(path, missing)) return undefined;
        path.push(missing);
        remaining = remaining.slice(missing.length);
        while (remaining.startsWith('.')) {
          remaining = remaining.slice(1);
          const next = remaining.match(/^[^.[\]]+/)?.[0];
          if (!next || !knownField(path, next)) return undefined;
          path.push(next);
          remaining = remaining.slice(next.length);
        }
        if (remaining) return undefined;
        break;
      }
      if (!insideBlock) {
        if (!knownField(path, key)) return undefined;
        path.push(key);
        insideBlock = isBlockField(path);
      }
      node = node.get(key, true) as Node | null;
      remaining = remaining.slice(key.length);
    } else return undefined;
  }
  const range = node?.range ?? document.contents.range;
  if (!range) return undefined;
  const from = range[0];
  return { path, id: recipeFieldId(path), from, to: Math.max(from + 1, range[1]), line: text.slice(0, from).split('\n').length };
}

function isBlockField(path: (string | number)[]): boolean {
  return fieldAt(path).field?.kind === 'block';
}

// Walk the same table that draws rows and reports unknown keys.
function knownField(path: (string | number)[], key: string): boolean {
  const { format, field } = fieldAt(path);
  if (field?.kind === 'dynamic-map') return true;
  return !!format?.fields[key];
}

function fieldAt(path: (string | number)[]): { format?: RecipeFormatNode; field?: RecipeFormatField } {
  let format: RecipeFormatNode | undefined = recipeFormat;
  let field: RecipeFormatField | undefined;
  for (const part of path) {
    if (typeof part === 'number') {
      format = field?.item;
      continue;
    }
    if (field?.kind === 'dynamic-map') {
      format = field.item;
      field = undefined;
      continue;
    }
    field = format?.fields[part];
    if (!field) return {};
    format = field.item;
  }
  return { format, field };
}
