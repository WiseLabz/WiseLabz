import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { isDeepStrictEqual } from 'node:util';
import { isMap, isScalar, isSeq, Lexer, type Node, type Pair, type YAMLMap, type YAMLSeq } from 'yaml';
import { describe, expect, it } from 'vitest';
import { editRecipeDocument, parseRecipeDocument, type RecipeEditOperation } from './recipeDocument';

// Contract: an edit through editRecipeDocument changes only the text it was asked to change.
// Line-based checks apply to block YAML. Edits inside a flow collection are checked at the token
// level (a flow line must be rewritten as a whole, so line checks cannot hold). Every failure is
// collected per source and reported with the operation and the actual output.

const repositoryRoot = resolve(process.cwd(), '..');
const recipesDirectory = `${repositoryRoot}/docs/connectors/recipes`;
const recipeFormat = readFileSync(`${repositoryRoot}/docs/connectors/RECIPE_FORMAT.md`, 'utf8');
const BOM = '﻿';
const NEW = 'zz-new';
const ADDED = 'zz-added';
// `{` and `}` are flow indicators: a path like this must be quoted inside a flow mapping and stay plain in a block one.
const PLACEHOLDER = '/items/{external_id}/rescan';
// Strings a plain scalar cannot always hold, in either context: flow indicators, `: `, ` #`, indicators at the start.
const AWKWARD = ['a, b', '[x]', '{x}', 'x: y', 'a #b', '', '- x', '"q"', "it's", '*star', '? key', '@at', 'null', 'true', '42'];

type Path = (string | number)[];
type Kind = 'set' | 'set-typed' | 'add-key' | 'delete' | 'remove' | 'move' | 'append';
type Planned = { label: string; kind: Kind; operation: RecipeEditOperation; expected: unknown; newValue?: string };

const fixtures: Array<[string, string]> = [
  ['fixture: comments', '# leading\nversion: 1 # inline\n# between\ncategory: media\n# trailing\n'],
  ['fixture: blank lines', 'version: 1\n\nauth:\n  mode: header\n\n  name: Authorization\n\nendpoints:\n  - name: one\n    path: /one\n\n  - name: two\n    path: /two\n'],
  ['fixture: flow', 'version: 1\ncategory: {name: media, tags: [a, b]}\nauth: {mode: none}\nvalues: [one, two, three]\n'],
  ['fixture: quoted keys', '"version": 1\n\'category\': "media"\n"auth":\n  \'mode\': \'none\'\n'],
  ['fixture: quotes and spaces', "version: '1'   \ncategory: \"media\"  \nname: 'x # not a comment'\n"],
  ['fixture: crlf', 'version: 1\r\ncategory: media\r\nendpoints:\r\n  - name: one\r\n    path: /one\r\n'],
  ['fixture: bom', `${BOM}version: 1\ncategory: media\n`],
  ['fixture: bom crlf', `${BOM}version: 1\r\ncategory: "media" # note\r\nauth:\r\n  mode: none\r\n`],
  ['fixture: block scalar', 'version: 1\ndescription: |\n  line one\n  line two\n\nkeep: yes\n'],
  ['fixture: block scalar no blank', 'a: |\n  x\nb: 2\n'],
  ['fixture: null values', 'version: 1\nname:\ncategory: media\nbody: ~\nextra:\n'],
  ['fixture: multi-entry map', 'auth:\n  mode: header\n  name: Authorization\n  prefix: "Bearer "\ncategory: media\n'],
  ['fixture: comments in sequences', 'items:\n  # first\n  - name: first # trailing\n  # second\n  - name: second\n  # end\nafter: 1\n'],
  ['fixture: no final newline', 'a: 1\nitems:\n  - name: x\n  - name: y'],
  ['fixture: header comment on a sequence key', 'items: # list\n  - name: x\nafter: 1\n'],
  ['fixture: single-key items with a sibling', 'items:\n  - name: x\n  - name: y\nafter: 1\n'],
  ['fixture: single-key item with a sibling', 'items:\n  - name: x\nafter: 1\n'],
  ['fixture: null-valued parents', 'auth:\nversion: 1\nextra: # none\nlast: ~\n'],
  ['fixture: multi-line flow sequence', 'values: [\n  one,\n  two\n]\nafter: 1\n'],
  ['fixture: multi-line flow map', 'auth: {\n  mode: none,\n  name: x\n}\nafter: 1\n'],
  ['fixture: mixed line endings', 'version: 1\ncategory: media\r\nauth:\r\n  mode: none\r\nendpoints:\n  - name: one\r\n    path: /one\n'],
  ['fixture: single item with a comment above it', 'endpoints:\n  # the main list\n  - name: one\n    path: /one # inline\nafter: 1\n'],
  ['fixture: crlf, no final newline', 'version: 1\r\ncategory: media\r\nauth:\r\n  mode: none'],
  [
    'fixture: flow mappings in a block sequence, tab in a comment, non-ASCII',
    'items:\n  - {name: "é", path: /a} \t# tab \tcomment ü\n  - {name: b, path: /b}\nafter: 1\n',
  ],
  [
    'fixture: actions, block style with comments between them',
    [
      'version: 1',
      'category: containers_paas',
      'auth: {mode: none}',
      'endpoints:',
      '  - name: containers',
      '    path: /api/containers',
      '    items: items',
      '    entity:',
      '      kind: container',
      '      name: name',
      '      external_id: id',
      '      attributes:',
      '        node: {path: node}',
      '      actions:',
      '        # restart the container',
      '        restart:',
      '          method: POST',
      '          path: /api/containers/{external_id}/restart',
      '          label: Restart',
      '',
      '        # rescan its volumes',
      '        rescan:',
      '          method: POST',
      '          path: /api/nodes/{attr.node}/containers/{external_id}/rescan',
      '          query: {source: operator}',
      '          body: {reason: "Rescan {attr.node}"}',
      '          downtime_seconds: 0',
      '# service level',
      'actions:',
      '  restart:',
      '    method: POST',
      '    path: /api/system/restart',
      '',
    ].join('\n'),
  ],
  [
    'fixture: actions, flow style',
    [
      'version: 1',
      'category: other',
      'auth: {mode: none}',
      'endpoints:',
      '  - name: items',
      '    path: /api/items',
      '    items: items',
      '    entity: {kind: item, name: name, external_id: id, actions: {rescan: {method: POST, path: "/items/{external_id}/rescan"}, stop: {method: DELETE, path: \'/items/{external_id}\'}}}',
      'actions: {restart: {method: POST, path: /api/restart, label: "Restart, now"}}',
      '',
    ].join('\n'),
  ],
  [
    'fixture: actions, multi-line flow mapping',
    'actions: {\n  restart: {method: POST, path: /api/restart},\n  rescan: {method: POST, path: "/a/{b}"}\n}\nversion: 1\n',
  ],
  [
    'fixture: actions, quoted keys and a flow mapping spread over lines in a block entity',
    [
      'version: 1',
      'endpoints:',
      '  - name: items',
      '    path: /api/items',
      '    entity:',
      '      kind: item',
      '      name: name',
      '      external_id: id',
      '      actions: {',
      '        "rescan": {method: POST, path: "/a/{external_id}"},  # first',
      '        stop: {method: DELETE, path: /s}',
      '      }',
      '',
    ].join('\n'),
  ],
];

function lines(text: string): string[] {
  return text.replace(/^\uFEFF/, '').split(/\r?\n/);
}

/** An emptied mapping may read back as null; the two mean the same to the recipe parser. */
function emptyAsNull(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(emptyAsNull);
  if (value && typeof value === 'object') {
    const entries = Object.entries(value);
    return entries.length ? Object.fromEntries(entries.map(([key, child]) => [key, emptyAsNull(child)])) : null;
  }
  return value;
}

/** Counts comment tokens in YAML text; the lexer emits each comment as one token starting with '#'. */
function commentCount(text: string): number {
  return [...new Lexer().lex(text)].filter((token) => token.startsWith('#')).length;
}

function hasCrlf(text: string): boolean {
  return text.includes('\r\n');
}

function hasBareLf(text: string): boolean {
  return /(^|[^\r])\n/.test(text);
}

/** `key: ~` and `key: null` lose their explicit null token when children are added below the key. */
function withoutNullToken(line: string): string {
  return line.replace(/^([^#]*?:)[ \t]*(?:~|null|Null|NULL)[ \t]*(#.*)?$/, (_match, head: string, comment?: string) =>
    comment ? `${head} ${comment}` : head,
  );
}

/**
 * The original lines an add-key or append must keep. When the edit adds children to a null value, the key
 * line may lose its explicit null token; that one line is compared without it, everything else stays strict.
 */
function keptLines(source: string, planned: Planned): string[] {
  const original = lines(source);
  if (planned.kind !== 'add-key' && planned.kind !== 'append') return original;
  const path = planned.operation.path as Path;
  const { node, keyOffset } = locate(source, planned.kind === 'add-key' ? path.slice(0, -1) : path);
  if (!isScalar(node) || node.value !== null || keyOffset === undefined) return original;
  const keyLine = source.slice(0, keyOffset).split('\n').length - 1;
  return original.map((line, index) => (index === keyLine ? withoutNullToken(line) : line));
}

/** Each inserted line takes the line ending of the line above it (the first line's ending at the very top, LF without breaks). */
function addedLineEndings(result: string, kept: boolean[]): string | undefined {
  const raw = result.replace(/^\uFEFF/, '').split(/(?<=\n)/);
  const endingOf = (line: string | undefined) => (line?.endsWith('\r\n') ? '\r\n' : line?.endsWith('\n') ? '\n' : undefined);
  const firstEnding = endingOf(raw[0]) ?? '\n';
  for (let index = 0; index < raw.length; index += 1) {
    if (kept[index] || raw[index].trim() === '') continue;
    const own = endingOf(raw[index]);
    if (own === undefined) continue; // the last line of a text without a final break has none
    const expected = index === 0 ? firstEnding : endingOf(raw[index - 1]) ?? '\n';
    if (own !== expected) return `inserted line has ${JSON.stringify(own)} but the line above uses ${JSON.stringify(expected)}`;
  }
  return undefined;
}

/** Greedy in-order match of `sub` inside `big`; returns which `big` lines matched, or undefined if not a subsequence. */
function matchedInOrder(sub: string[], big: string[]): boolean[] | undefined {
  const matched = big.map(() => false);
  let cursor = 0;
  for (const line of sub) {
    while (cursor < big.length && big[cursor] !== line) cursor += 1;
    if (cursor >= big.length) return undefined;
    matched[cursor] = true;
    cursor += 1;
  }
  return matched;
}

/** True when the characters that differ between `before` and `after` lie inside [start, end) of `before`. */
function confined(before: string, after: string, start: number, end: number): boolean {
  if (before === after) return true;
  let prefix = 0;
  while (prefix < before.length && prefix < after.length && before[prefix] === after[prefix]) prefix += 1;
  let suffix = 0;
  while (
    suffix < before.length - prefix &&
    suffix < after.length - prefix &&
    before[before.length - 1 - suffix] === after[after.length - 1 - suffix]
  ) {
    suffix += 1;
  }
  return prefix >= start && before.length - suffix <= end;
}

function parsedJS(text: string): unknown {
  return parseRecipeDocument(text).document?.toJS() ?? null;
}

/** Walks the source AST along `path`, returning each node on the way plus the key offset of the last mapping step. */
function locate(source: string, path: Path): { chain: unknown[]; keyOffset?: number; node?: unknown } {
  let node: unknown = parseRecipeDocument(source).document?.contents;
  const chain: unknown[] = [];
  let keyOffset: number | undefined;
  for (const segment of path) {
    if (isMap(node)) {
      const pair = node.items.find((item) => isScalar(item.key) && String(item.key.value) === String(segment)) as
        | Pair<Node, Node>
        | undefined;
      keyOffset = pair?.key?.range?.[0];
      node = pair?.value;
    } else if (isSeq(node)) {
      node = node.items[segment as number];
    } else {
      node = undefined;
    }
    chain.push(node);
  }
  return { chain, keyOffset, node };
}

/* eslint-disable @typescript-eslint/no-explicit-any -- the helpers below walk arbitrary parsed YAML */
function at(root: unknown, path: RecipeEditOperation['path']): any {
  return (path as Path).reduce<any>((node, key) => node[key], root);
}

function expectedAfter(js: unknown, operation: RecipeEditOperation): unknown {
  const copy: any = structuredClone(js);
  const parent = operation.type === 'set' || operation.type === 'delete' ? at(copy, (operation.path as Path).slice(0, -1)) : undefined;
  const last = (operation.path as Path)[(operation.path as Path).length - 1];
  if (operation.type === 'set') parent[last] = operation.value;
  else if (operation.type === 'delete') delete parent[last];
  else if (operation.type === 'remove') at(copy, operation.path).splice(operation.index, 1);
  else if (operation.type === 'move') {
    const items = at(copy, operation.path);
    const [item] = items.splice(operation.index, 1);
    items.splice(operation.to, 0, item);
  } else if (operation.type === 'append') at(copy, operation.path).push(operation.value);
  return copy;
}

/** The parsed value with `value` stored at `path`, whatever was there before. */
function withValue(js: unknown, path: Path, value: unknown): unknown {
  if (!path.length) return value;
  const copy: any = structuredClone(js);
  at(copy, path.slice(0, -1))[path[path.length - 1]] = value;
  return copy;
}
/* eslint-enable @typescript-eslint/no-explicit-any */

/** Plans every edit for a parsed source; `expected` is the JS value the edit must produce. */
function planOperations(js: unknown): Planned[] {
  const planned: Planned[] = [];
  const add = (label: string, kind: Kind, operation: RecipeEditOperation): void => {
    planned.push({ label, kind, operation, expected: expectedAfter(js, operation) });
  };
  const visit = (value: unknown, path: Path): void => {
    if (Array.isArray(value)) {
      addSequenceOperations(value.length, path, add);
      value.forEach((item, index) => visit(item, [...path, index]));
    } else if (value && typeof value === 'object') {
      for (const [key, child] of Object.entries(value)) {
        const childPath = [...path, key];
        add(`delete ${JSON.stringify(childPath)}`, 'delete', { type: 'delete', path: childPath });
        visit(child, childPath);
      }
    } else if (path.length) {
      add(`set ${JSON.stringify(path)} = ${JSON.stringify(NEW)}`, 'set', { type: 'set', path, value: NEW });
      add(`set ${JSON.stringify(path)} = 42`, 'set-typed', { type: 'set', path, value: 42 });
      add(`set ${JSON.stringify(path)} = true`, 'set-typed', { type: 'set', path, value: true });
      add(`set ${JSON.stringify(path)} = a path with a placeholder`, 'set', { type: 'set', path, value: PLACEHOLDER });
      for (const awkward of AWKWARD) {
        add(`set ${JSON.stringify(path)} = ${JSON.stringify(awkward)}`, 'set-typed', { type: 'set', path, value: awkward });
      }
    }
  };
  visit(js, []);
  return planned;
}

/** Plans a new key for every block mapping, and a new child key or item for every null value inside one. */
function planAddedKeys(source: string): Planned[] {
  const js = parsedJS(source);
  const planned: Planned[] = [];
  const add = (label: string, kind: Kind, operation: RecipeEditOperation, expected: unknown, newValue = NEW): void => {
    planned.push({ label, kind, operation, expected, newValue });
  };
  const visit = (node: unknown, path: Path): void => {
    if (isSeq(node)) {
      node.items.forEach((item, index) => visit(item, [...path, index]));
    } else if (isMap(node)) {
      const added = [...path, ADDED];
      if (!node.flow) {
        add(`add key ${JSON.stringify(added)}`, 'add-key', { type: 'set', path: added, value: NEW }, withValue(js, added, NEW));
      }
      add(
        `add key ${JSON.stringify(added)} = a path with a placeholder`,
        'add-key',
        { type: 'set', path: added, value: PLACEHOLDER },
        withValue(js, added, PLACEHOLDER),
        PLACEHOLDER,
      );
      for (const pair of node.items) {
        if (!isScalar(pair.key)) continue;
        const childPath = [...path, String(pair.key.value)];
        if (!node.flow && isScalar(pair.value) && pair.value.value === null) {
          // A null value (`auth:`) has no children yet: adding one replaces it with a mapping or a sequence.
          const child = [...childPath, ADDED];
          add(`add key ${JSON.stringify(child)}`, 'add-key', { type: 'set', path: child, value: NEW }, withValue(js, childPath, { [ADDED]: NEW }));
          add(`append ${JSON.stringify(childPath)}`, 'append', { type: 'append', path: childPath, value: NEW }, withValue(js, childPath, [NEW]));
        }
        visit(pair.value, childPath);
      }
    }
  };
  visit(parseRecipeDocument(source).document?.contents, []);
  return planned;
}

function addSequenceOperations(
  length: number,
  path: Path,
  add: (label: string, kind: Kind, operation: RecipeEditOperation) => void,
): void {
  const large = length > 6;
  for (let index = 0; index < length; index += 1) {
    add(`remove ${JSON.stringify(path)}[${index}]`, 'remove', { type: 'remove', path, index });
    for (let to = 0; to < length; to += 1) {
      if (to === index) continue;
      if (large && to !== 0 && to !== length - 1 && to !== index + 1) continue;
      add(`move ${JSON.stringify(path)} ${index}->${to}`, 'move', { type: 'move', path, index, to });
    }
  }
  add(`append ${JSON.stringify(path)}`, 'append', { type: 'append', path, value: NEW });
}

/** Returns a human-readable reason when the edit breaks the contract, otherwise undefined. */
function checkEdit(source: string, planned: Planned): string | undefined {
  let result: string;
  try {
    result = editRecipeDocument(source, planned.operation);
  } catch (error) {
    return `threw: ${(error as Error).message}`;
  }
  const snippet = `result=${JSON.stringify(result).slice(0, 220)}`;
  const parsed = parseRecipeDocument(result);
  if (parsed.error) return `result does not parse (${parsed.error.message.split('\n')[0]}) ${snippet}`;

  const actual = parsedJS(result);
  if (!isDeepStrictEqual(emptyAsNull(actual), emptyAsNull(planned.expected))) {
    return `value mismatch: got ${JSON.stringify(actual)?.slice(0, 160)} want ${JSON.stringify(planned.expected)?.slice(0, 160)} ${snippet}`;
  }
  if (result.startsWith(BOM) !== source.startsWith(BOM)) return `BOM changed ${snippet}`;
  if ((planned.kind === 'add-key' || planned.kind === 'append') && !source.endsWith('\n') && result.endsWith('\n')) {
    return `gained a final newline ${snippet}`;
  }
  // Deleting the last line of a text with no final newline also drops the break before it, so none is left.
  const droppedLastBreak = planned.kind === 'delete' && !source.endsWith('\n') && !hasCrlf(result) && !hasBareLf(result);
  if (!droppedLastBreak && (hasCrlf(result) !== hasCrlf(source) || hasBareLf(result) !== hasBareLf(source))) {
    return `line ending style changed ${snippet}`;
  }

  const commentsLost = commentCount(source) - commentCount(result);
  const located = locate(source, planned.operation.path as Path);
  // A set or delete edits its parent container; a remove, move or append edits the sequence at the path itself.
  const containers = planned.kind === 'remove' || planned.kind === 'move' || planned.kind === 'append' ? located.chain : located.chain.slice(0, -1);
  const flow = containers.find((node) => (isMap(node) || isSeq(node)) && node.flow === true) as YAMLMap | YAMLSeq | undefined;
  if (flow?.range) {
    if (!confined(source, result, flow.range[0], flow.range[2])) return `text outside the flow collection changed ${snippet}`;
    return commentsLost === 0 ? undefined : `comment count changed ${snippet}`;
  }
  const container = located.node;
  if (planned.kind === 'remove' && isSeq(container) && container.items.length === 1 && located.keyOffset !== undefined) {
    // Emptying a block sequence must put `[]` on the key line (a bare `key:` reads back as null), so the
    // key line and the removed item are the only text allowed to change.
    if (!confined(source, result, located.keyOffset, container.range?.[2] ?? source.length)) return `text outside the sequence changed ${snippet}`;
    // Comments above the first item are user text and stay; only those inside the removed item may go.
    const item = container.items[0] as Node;
    const insideItem = item.range ? commentCount(source.slice(item.range[0], container.range?.[2] ?? source.length)) : 0;
    return commentsLost > insideItem ? `a comment above the only item was dropped ${snippet}` : undefined;
  }

  const before = lines(source);
  const after = lines(result);
  const scalar = located.node;
  if ((planned.kind === 'set' || planned.kind === 'set-typed') && isScalar(scalar) && scalar.range) {
    // A multi-line scalar (block scalar) is replaced as a unit, so the line count may change.
    const old = source.slice(scalar.range[0], scalar.range[1]).replace(/(?:\r?\n)+$/, '');
    if (/\n/.test(old)) {
      if (!confined(source, result, scalar.range[0], scalar.range[1])) return `text outside the scalar changed ${snippet}`;
      return commentsLost === 0 ? undefined : `comment count changed ${snippet}`;
    }
  }
  if (planned.kind === 'delete' && located.keyOffset !== undefined) {
    // The first key of a sequence item shares its line with the dash: the dash stays and the next key joins it.
    const lineStartOffset = source.lastIndexOf('\n', located.keyOffset - 1) + 1;
    if (/^[ \t]*(?:-[ \t]+)+$/.test(source.slice(lineStartOffset, located.keyOffset))) {
      const owner = located.chain[located.chain.length - 2] as YAMLMap | undefined;
      if (!owner?.range || !confined(source, result, located.keyOffset, owner.range[2])) return `text outside the item changed ${snippet}`;
      return undefined;
    }
  }
  if (planned.kind === 'add-key') {
    // Every original line stays in order; the only new text is the new key on a line of its own.
    const kept = matchedInOrder(keptLines(source, planned), after);
    if (!kept) return `original lines not kept in order ${snippet}`;
    if (commentsLost !== 0) return `comment count changed ${snippet}`;
    const added = after.filter((_, index) => !kept[index]).filter((line) => line.trim() !== '');
    if (added.length !== 1 || added[0].trim() !== `${ADDED}: ${planned.newValue ?? NEW}`) return `added lines are not only the new key ${snippet}`;
    return addedLineEndings(result, kept);
  }
  if (planned.kind === 'set' || planned.kind === 'set-typed') {
    if (commentsLost !== 0) return `comment count changed ${snippet}`;
    if (before.length !== after.length) return `line count ${before.length} -> ${after.length} ${snippet}`;
    const changed = before.filter((line, index) => line !== after[index]).length;
    return changed > 1 ? `${changed} lines changed ${snippet}` : undefined;
  }

  if (planned.kind === 'append') {
    if (!matchedInOrder(keptLines(source, planned), after)) return `original lines not kept in order ${snippet}`;
    return commentsLost !== 0 ? `comment count changed ${snippet}` : undefined;
  }

  if (planned.kind === 'move') {
    if (!isDeepStrictEqual([...before].sort(), [...after].sort())) return `lines are not a permutation of the original ${snippet}`;
    return commentsLost !== 0 ? `comment count changed ${snippet}` : undefined;
  }

  // delete / remove: the result must be the original minus whole lines.
  const kept = matchedInOrder(after, before);
  if (!kept) return `result introduces or rewrites a line ${snippet}`;
  const removed = before.filter((_, index) => !kept[index]).join('\n');
  if (commentCount(removed) !== commentsLost) {
    return `comments removed (${commentsLost}) do not match removed lines (${commentCount(removed)}) ${snippet}`;
  }
  return undefined;
}

function auditSource(source: string): string[] {
  const failures: string[] = [];
  for (const planned of [...planOperations(parsedJS(source)), ...planAddedKeys(source)]) {
    const reason = checkEdit(source, planned);
    if (reason) failures.push(`${planned.label} :: ${reason}`);
  }
  return failures;
}

const shipped = readdirSync(recipesDirectory)
  .filter((name) => name.endsWith('.yaml'))
  .sort()
  .map((name): [string, string] => [`recipes/${name}`, readFileSync(`${recipesDirectory}/${name}`, 'utf8')]);
const reference = [...recipeFormat.matchAll(/```yaml\r?\n([\s\S]*?)```/g)].map(
  ([, example], index): [string, string] => [`RECIPE_FORMAT.md yaml block ${index + 1}`, example],
);
const corpus = [...shipped, ...reference, ...fixtures].filter(([, source]) => {
  const parsed = parseRecipeDocument(source);
  return !parsed.error && !parsed.unsupported && parsed.document !== null;
});

describe('recipe edits change only the requested text', () => {
  it('keeps every hand-written fixture in the corpus', () => {
    const skipped = fixtures.filter(([name]) => !corpus.some(([kept]) => kept === name)).map(([name]) => name);
    expect(skipped).toEqual([]);
  });

  it.each(corpus)('%s', (_name, source) => {
    const failures = auditSource(source);
    expect(failures.length, failures.join('\n')).toBe(0);
  });
});
