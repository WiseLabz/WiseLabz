import { isMap, isScalar, isSeq, type Document, type Node, type Pair, type YAMLMap } from 'yaml';

export type RecipeFormatKind = 'string' | 'scalar' | 'block' | 'mapping' | 'sequence' | 'dynamic-map';

export type RecipeFormatField = {
  kind: RecipeFormatKind;
  options?: readonly string[];
  item?: RecipeFormatNode;
  itemKind?: 'string' | 'scalar';
};

export type RecipeFormatNode = { fields: Record<string, RecipeFormatField> };

const node = (fields: Record<string, RecipeFormatField>): RecipeFormatNode => ({ fields });
const string = (options?: readonly string[]): RecipeFormatField => ({ kind: 'string', ...(options ? { options } : {}) });
const scalar = (): RecipeFormatField => ({ kind: 'scalar' });
const sequence = (item: RecipeFormatNode): RecipeFormatField => ({ kind: 'sequence', item });
const mapping = (item: RecipeFormatNode): RecipeFormatField => ({ kind: 'mapping', item });

// Keep this table aligned with backend/internal/connector/custom/recipe.go and recipe_action.go; it describes rows, not validation rules.
const dependency = node({ kind: string(['host', 'network', 'storage', 'upstream_service']), path: string(), const: scalar() });
const attribute = node({
  path: string(),
  const: scalar(),
  template: string(),
  type: string(['string', 'number', 'bool', 'boolean', 'list', 'string_array']),
  map: { kind: 'dynamic-map', itemKind: 'scalar' },
  default: scalar(),
});
const pagination = node({
  type: string(['page', 'offset', 'cursor', 'next_link']),
  param: string(),
  size_param: string(),
  size: scalar(),
  start: scalar(),
  cursor_path: string(),
  next_path: string(),
  link_header: scalar(),
});
const action = node({
  method: string(['POST', 'PUT', 'PATCH', 'DELETE']),
  path: string(),
  query: { kind: 'dynamic-map', itemKind: 'string' },
  headers: { kind: 'dynamic-map', itemKind: 'string' },
  body: { kind: 'block' },
  label: string(),
  description: string(),
  downtime_seconds: scalar(),
});
const actions: RecipeFormatField = { kind: 'dynamic-map', item: action };
const entity = node({
  kind: string(),
  name: string(),
  external_id: string(),
  hostname: string(),
  ip: string(),
  mac: string(),
  aliases: string(),
  attributes: { kind: 'dynamic-map', item: attribute },
  actions,
});
const endpoint = node({
  name: string(),
  path: string(),
  method: string(['GET', 'POST']),
  query: { kind: 'dynamic-map', itemKind: 'string' },
  headers: { kind: 'dynamic-map', itemKind: 'string' },
  body: { kind: 'block' },
  items: string(),
  pagination: mapping(pagination),
  entity: mapping(entity),
  dependencies: sequence(dependency),
});

export const recipeFormat = node({
  version: scalar(),
  category: string(['virtualization', 'containers_paas', 'networking', 'dns', 'storage', 'monitoring', 'media', 'other']),
  auth: mapping(node({ mode: string(['none', 'header', 'basic', 'query']), name: string(), prefix: string() })),
  endpoints: sequence(endpoint),
  dependencies: sequence(dependency),
  actions,
});

export type RecipeUnknownNode = { node: YAMLMap; path: RecipePathSegments; keys: string[] };
export type RecipePathSegments = Array<string | number>;

/** Lists unknown keys at each fixed-key mapping while allowing dynamic maps such as attributes and headers. */
export function unknownRecipeKeys(document: Document | null): RecipeUnknownNode[] {
  const contents = document?.contents;
  if (!isMap(contents)) return [];
  const unknown: RecipeUnknownNode[] = [];
  const seen = new Set<object>();

  const visit = (current: Node | null | undefined, format: RecipeFormatNode | undefined, path: RecipePathSegments): void => {
    if (!current || typeof current !== 'object' || seen.has(current)) return;
    seen.add(current);
    if (isMap(current) && format) {
      const keys: string[] = [];
      const children: Array<{ node: Node; field: RecipeFormatField; path: RecipePathSegments }> = [];
      for (const item of current.items) {
        const pair = item as Pair<Node, Node>;
        const key = isScalar(pair.key) ? String(pair.key.value) : String(pair.key);
        const field = format.fields[key];
        if (!field) {
          keys.push(key);
          continue;
        }
        if (pair.value) children.push({ node: pair.value, field, path: [...path, key] });
      }
      if (keys.length) unknown.push({ node: current, path, keys });
      for (const child of children) visitField(child.node, child.field, child.path);
      return;
    }
    if (isMap(current) && !format) {
      for (const item of current.items) {
        const pair = item as Pair<Node, Node>;
        if (pair.value) visit(pair.value, undefined, path);
      }
    }
  };

  const visitField = (current: Node, field: RecipeFormatField, path: RecipePathSegments): void => {
    if (field.kind === 'mapping') visit(current, field.item, path);
    else if (field.kind === 'sequence' && isSeq(current)) {
      current.items.forEach((item, index) => visit(item as Node, field.item, [...path, index]));
    } else if (field.kind === 'dynamic-map' && isMap(current) && field.item) {
      current.items.forEach((item) => {
        const pair = item as Pair<Node, Node>;
        const key = isScalar(pair.key) ? String(pair.key.value) : String(pair.key);
        if (pair.value) visit(pair.value, field.item, [...path, key]);
      });
    }
  };

  visit(contents, recipeFormat, []);
  return unknown;
}
