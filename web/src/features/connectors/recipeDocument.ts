import {
  Document,
  isAlias,
  isNode,
  isMap,
  isScalar,
  isSeq,
  LineCounter,
  parseAllDocuments,
  parseDocument,
  type Node,
  type Pair,
  type YAMLMap,
  type YAMLSeq,
} from 'yaml';

export type RecipePath = string | readonly (string | number)[];

export type RecipeDocumentIssue = {
  kind: 'syntax' | 'documents' | 'root' | 'anchors' | 'aliases' | 'merge';
  message: string;
  line?: number;
  column?: number;
};

export type RecipeDocumentResult = {
  document: Document | null;
  source: string;
  error?: RecipeDocumentIssue;
  unsupported?: RecipeDocumentIssue;
};

export type RecipeEditOperation =
  | { type: 'set'; path: RecipePath; value: unknown }
  | { type: 'delete'; path: RecipePath }
  | { type: 'append'; path: RecipePath; value: unknown }
  | { type: 'remove'; path: RecipePath; index: number }
  | { type: 'move'; path: RecipePath; index: number; to: number }
  | { type: 'rename'; path: RecipePath; to: string }
  | { type: 'block'; path: RecipePath; text: string };

type SourcePatch = { start: number; end: number; text: string };

/** Parses the source text while retaining node ranges for targeted edits. */
export function parseRecipeDocument(source: string): RecipeDocumentResult {
  let documents: ReturnType<typeof parseAllDocuments>;
  const lineCounter = new LineCounter();
  try {
    documents = parseAllDocuments(source, { keepSourceTokens: true, lineCounter });
  } catch (error) {
    return { document: null, source, error: syntaxIssue(error) };
  }

  if (documents.length === 0 && isBlankSource(source)) {
    return {
      document: null,
      source,
      unsupported: { kind: 'root', message: 'The recipe root must be a YAML mapping.' },
    };
  }

  if (documents.some((document) => document.errors.length)) {
    const issue = documents.flatMap((document) => document.errors)[0];
    const position = issue.linePos?.[0];
    return {
      document: documents[0] ?? null,
      source,
      error: {
        kind: 'syntax',
        message: withoutPosition(issue.message),
        ...(position ? { line: position.line, column: position.col } : {}),
      },
    };
  }

  const document = documents[0] ?? null;
  if (documents.length !== 1) {
    return {
      document,
      source,
      unsupported: issueAt(source, 'documents', 'The recipe must contain exactly one YAML document.', documents[1]?.range?.[0]),
    };
  }
  if (!document || !isMap(document.contents)) {
    return {
      document,
      source,
      unsupported: issueAt(source, 'root', 'The recipe root must be a YAML mapping.', document?.contents?.range?.[0]),
    };
  }

  const unsupported = unsupportedNodeIssue(document.contents, source);
  return { document, source, ...(unsupported ? { unsupported } : {}) };
}

/** Returns the original text for a parsed, not-yet-edited document. */
export function serializeRecipeDocument(parsed: RecipeDocumentResult): string {
  return parsed.source;
}

/** Applies one Document API mutation and patches only its source node or block. */
export function editRecipeDocument(source: string, requested: RecipeEditOperation): string {
  const parsed = parseRecipeDocument(source);
  if (parsed.error) throw new Error(parsed.error.message);
  if (parsed.unsupported && !(parsed.unsupported.kind === 'root' && isBlankSource(source))) {
    throw new Error(parsed.unsupported.message);
  }

  const document = parsed.document ?? new Document();
  if (!document.contents) document.contents = document.createNode({}) as YAMLMap;
  const before = document.clone();
  const { operation, path, value } = liftNullAncestor(
    before.contents,
    requested,
    pathSegments(requested.path),
    requested.type === 'block' ? parseBlock(requested.text) : requested.type === 'set' || requested.type === 'append' ? requested.value : undefined,
  );
  const beforeNode = getNode(before.contents, path);
  if (operation.type === 'delete' && !beforeNode) return source;
  if ((operation.type === 'set' || operation.type === 'block') && beforeNode && sameValue(beforeNode.toJSON(), value)) return source;
  if (operation.type === 'move' && operation.index === operation.to) return source;

  switch (operation.type) {
    case 'set':
    case 'block':
      document.setIn(path, value);
      break;
    case 'delete':
      document.deleteIn(path);
      break;
    case 'rename': {
      const parent = getNode(document.contents, path.slice(0, -1));
      const pair = isMap(parent) ? findPair(parent, path[path.length - 1]) : undefined;
      const newKey = operation.to;
      if (!isMap(parent) || !pair || !newKey || !isScalar(pair.key)) return source;
      if (String(pair.key.value) === newKey) return source;
      if (parent.has(newKey)) throw new Error(`A recipe entry named ${newKey} already exists.`);
      pair.key.value = newKey;
      break;
    }
    case 'append': {
      const sequence = getNode(document.contents, path);
      if (isSeq(sequence)) document.addIn(path, value);
      else document.setIn(path, [value]);
      break;
    }
    case 'remove': {
      const sequence = getNode(document.contents, path);
      if (!isSeq(sequence) || operation.index < 0 || operation.index >= sequence.items.length) return source;
      sequence.items.splice(operation.index, 1);
      break;
    }
    case 'move': {
      const sequence = getNode(document.contents, path);
      if (!isSeq(sequence) || operation.index < 0 || operation.index >= sequence.items.length) return source;
      if (operation.to < 0 || operation.to >= sequence.items.length) return source;
      const [item] = sequence.items.splice(operation.index, 1);
      sequence.items.splice(operation.to, 0, item);
      break;
    }
  }

  const patch = sourcePatch(source, before.contents, operation, value, path);
  if (!patch) throw new Error(unpatchable);
  const patched = source.slice(0, patch.start) + patch.text + source.slice(patch.end);
  // A text with no final line break stays that way: text inserted at its very end does not add one.
  if (patch.end >= source.length && !source.endsWith('\n') && !isBlankSource(source)) {
    const unterminated = source.slice(0, patch.start) + patch.text.replace(/\r?\n$/, '');
    if (unterminated !== patched && readsAs(unterminated, document)) return unterminated;
  }
  // Safety net: never hand back text that does not read as the mutation the Document API applied.
  if (!readsAs(patched, document)) throw new Error(unpatchable);
  return patched;
}

const unpatchable = 'The requested recipe edit cannot be applied as a targeted source change.';

/** True when the text parses and holds the same value as the mutated document. */
function readsAs(text: string, expected: Document): boolean {
  try {
    const parsed = parseRecipeDocument(text);
    if (parsed.error) return false;
    return sameValue(emptyAsNull(parsed.document?.toJS() ?? null), emptyAsNull(expected.toJS()));
  } catch {
    return false;
  }
}

/** An emptied block mapping reads back as null, so an empty collection and null count as equal. */
function emptyAsNull(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(emptyAsNull);
  if (value && typeof value === 'object') {
    const entries = Object.entries(value);
    return entries.length ? Object.fromEntries(entries.map(([key, child]) => [key, emptyAsNull(child)])) : null;
  }
  return value;
}

/**
 * A null value (`auth:`, `auth: ~`) has no children to edit, so an edit below it, or an append to it,
 * becomes a set that replaces the null with the collection the edit needs.
 */
function liftNullAncestor(
  root: Node | null | undefined,
  operation: RecipeEditOperation,
  path: unknown[],
  value: unknown,
): { operation: RecipeEditOperation; path: unknown[]; value: unknown } {
  const unchanged = { operation, path, value };
  if (operation.type !== 'set' && operation.type !== 'block' && operation.type !== 'append') return unchanged;
  const leaf = operation.type === 'append' ? [value] : value;
  for (let depth = 1; depth <= path.length; depth += 1) {
    const node = getNode(root, path.slice(0, depth));
    if (!node) break;
    if (isScalar(node) && node.value === null && (depth < path.length || operation.type === 'append')) {
      let nested = leaf;
      for (const segment of path.slice(depth).reverse()) {
        if (typeof segment !== 'string') return unchanged;
        nested = { [segment]: nested };
      }
      const target = path.slice(0, depth) as (string | number)[];
      return { operation: { type: 'set', path: target, value: nested }, path: target, value: nested };
    }
    if (!isMap(node) && !isSeq(node)) break;
  }
  return unchanged;
}

function parseBlock(text: string): unknown {
  const document = parseDocument(text, { keepSourceTokens: true });
  if (document.errors.length || !document.contents) {
    throw new Error(document.errors[0]?.message ?? 'The block must contain a YAML value.');
  }
  const unsupported = unsupportedNodeIssue(document.contents, text);
  if (unsupported) throw new Error(unsupported.message);
  return document.toJS();
}

function isBlankSource(source: string): boolean {
  return source.replace(/^\uFEFF/, '').trim() === '';
}

function sourcePatch(
  source: string,
  beforeRoot: Node | null | undefined,
  operation: RecipeEditOperation,
  value: unknown,
  path: unknown[],
): SourcePatch | undefined {
  const oldNode = getNode(beforeRoot, path);
  // Scalars written inside a flow collection must be quoted when they hold flow indicators such as `{` or `,`.
  const inFlow = inFlowContext(beforeRoot, path);
  if (operation.type === 'set' || operation.type === 'block') {
    if (oldNode?.range) {
      const parent = getNode(beforeRoot, path.slice(0, -1));
      const pair = isMap(parent) ? findPair(parent, path[path.length - 1]) : undefined;
      const encoded = renderYaml(value, inFlow).replace(/\n$/, '');
      const composite = value !== null && typeof value === 'object';
      if (composite && inFlow) {
        return { start: oldNode.range[0], end: oldNode.range[2], text: encoded };
      }
      if (!composite && !encoded.includes('\n')) {
        // A key with no value (`name:`) has an empty range right after the colon.
        const bare = oldNode.range[0] === oldNode.range[1] && !/\s/.test(source[oldNode.range[0] - 1] ?? ' ');
        // A block scalar's range ends with its line break; keep it so the next line is not joined.
        const lineBreaks = source.slice(oldNode.range[0], oldNode.range[1]).match(/(?:\r?\n)*$/)?.[0].length ?? 0;
        // A comment right after the colon (`key: # note`) starts where the empty value does; keep a space before it.
        const beforeComment = oldNode.range[0] === oldNode.range[1] && source[oldNode.range[0]] === '#' ? ' ' : '';
        return { start: oldNode.range[0], end: oldNode.range[1] - lineBreaks, text: `${bare ? ' ' : ''}${encoded}${beforeComment}` };
      }
      if (composite && isScalar(oldNode) && oldNode.value === null && pair?.key?.range && isMap(parent) && !parent.flow) {
        const patch = nullValuePatch(source, pair.key.range, encoded);
        if (patch) return patch;
      }
      if (pair && pair.key && pair.value && isMap(parent) && !parent.flow) {
        const colon = source.indexOf(':', pair.key.range?.[1] ?? -1);
        if (colon >= 0 && colon < oldNode.range[0]) {
          const indent = `${' '.repeat(keyColumn(source, pair.key.range?.[0] ?? oldNode.range[0]) + 2)}`;
          const lineEnding = eolAt(source, colon + 1);
          return {
            start: colon + 1,
            end: oldNode.range[2],
            text: `${lineEnding}${indentBlock(encoded, indent, lineEnding)}${lineEnding}`,
          };
        }
      }
      return {
        start: oldNode.range[0],
        end: oldNode.range[2],
        text: indentBlock(encoded, ' '.repeat(lineIndent(source, oldNode.range[0])), eolAt(source, oldNode.range[0])),
      };
    }
    return appendMissingPath(source, beforeRoot, path, value);
  }

  if (operation.type === 'delete') {
    const parent = getNode(beforeRoot, path.slice(0, -1));
    const pair = isMap(parent) ? findPair(parent, path[path.length - 1]) : undefined;
    if (pair && isMap(parent)) return deletePairPatch(source, parent, pair);
    return undefined;
  }

  if (operation.type === 'rename') {
    const parent = getNode(beforeRoot, path.slice(0, -1));
    const pair = isMap(parent) ? findPair(parent, path[path.length - 1]) : undefined;
    const newKey = operation.to;
    if (!pair?.key?.range || !newKey) return undefined;
    return { start: pair.key.range[0], end: pair.key.range[1], text: renderYaml(newKey, inFlow).replace(/\n$/, '') };
  }

  if (operation.type === 'append') {
    const sequence = getNode(beforeRoot, path);
    if (!isSeq(sequence)) return appendMissingPath(source, beforeRoot, path, [value]);
    if (sequence.flow) {
      if (sequence.items.length === 0) {
        const parent = getNode(beforeRoot, path.slice(0, -1));
        const pair = isMap(parent) ? findPair(parent, path[path.length - 1]) : undefined;
        if (pair?.key?.range && pair.value?.range && isMap(parent) && !parent.flow) {
          const colon = source.indexOf(':', pair.key.range[1]);
          if (colon >= 0) {
            const indentation = ' '.repeat(keyColumn(source, pair.key.range[0]) + 2);
            const rendered = renderYaml([value]).replace(/\n$/, '');
            const lineEnding = eolAt(source, colon + 1);
            return {
              start: colon + 1,
              end: pair.value.range[2],
              text: `${lineEnding}${indentBlock(rendered, indentation, lineEnding)}${lineEnding}`,
            };
          }
        }
      }
      const range = sequence.range;
      const close = range ? source.lastIndexOf(']', range[1] - 1) : -1;
      if (close < 0) return undefined;
      // Insert right after the last item: the bracket may sit alone on its own line.
      const insertion = (sequence.items[sequence.items.length - 1] as Node | undefined)?.range?.[1] ?? close;
      return {
        start: insertion,
        end: insertion,
        text: `${sequence.items.length ? ', ' : ' '}${renderYaml(value, true).trim()}${sequence.items.length ? '' : ' '}`,
      };
    }
    const insertion = sequence.items.length
      ? (sequence.items[sequence.items.length - 1] as Node).range?.[2] ?? sequence.range?.[2] ?? source.length
      : sequence.range?.[0] ?? source.length;
    const indent = sequenceIndent(source, sequence);
    const rendered = renderYaml([value]).replace(/\n$/, '');
    const lineEnding = eolAt(source, insertion);
    const prefix = insertion > 0 && !source.slice(0, insertion).endsWith('\n') ? lineEnding : '';
    return { start: insertion, end: insertion, text: `${prefix}${indentBlock(rendered, ' '.repeat(indent), lineEnding)}${lineEnding}` };
  }

  if (operation.type === 'remove' || operation.type === 'move') {
    const sequence = getNode(beforeRoot, path);
    if (!isSeq(sequence)) return undefined;
    if (sequence.flow) {
      return flowSequencePatch(source, sequence, operation);
    }
    const spans = sequenceSpans(source, sequence);
    if (operation.type === 'remove') {
      const span = spans[operation.index];
      if (sequence.items.length === 1) return emptySequencePatch(source, beforeRoot, path, sequence, spans[0]);
      return span ? dropPrecedingBreak(source, { start: span.start, end: span.end, text: '' }) : undefined;
    }
    const reordered = [...spans];
    const [span] = reordered.splice(operation.index, 1);
    if (!span) return undefined;
    reordered.splice(operation.to, 0, span);
    const start = Math.min(...spans.map((item) => item.start));
    const end = Math.max(...spans.map((item) => item.end));
    // The last span has no line break when the text has none; give every span one while joining,
    // then drop the final one again so a file without a final newline stays that way.
    const lineEnding = eolAt(source, end);
    const joined = reordered.map((item) => (item.text.endsWith('\n') ? item.text : item.text + lineEnding)).join('');
    return { start, end, text: source.slice(start, end).endsWith('\n') ? joined : joined.slice(0, -lineEnding.length) };
  }

  return undefined;
}

/** A removal that reaches the end of a text with no final line break also takes the break before it, so none appears. */
function dropPrecedingBreak(source: string, patch: SourcePatch): SourcePatch {
  if (patch.end < source.length || source.endsWith('\n')) return patch;
  const lead = source.slice(0, patch.start).match(/\r?\n$/)?.[0].length ?? 0;
  return { ...patch, start: patch.start - lead };
}

/** Replaces a null value (`key:` or `key: ~`) with a block collection below the key, keeping a comment on the key line. */
function nullValuePatch(source: string, keyRange: readonly number[], encoded: string): SourcePatch | undefined {
  const colon = source.indexOf(':', keyRange[1]);
  if (colon < 0) return undefined;
  const lineEnding = eolAt(source, colon);
  const newline = source.indexOf('\n', colon);
  const lineBreak = newline < 0 ? source.length : newline;
  const lineEnd = source[lineBreak - 1] === '\r' ? lineBreak - 1 : lineBreak;
  const rest = source.slice(colon + 1, lineEnd).match(/^[ \t]*(?:(?:~|null|Null|NULL)[ \t]*)?(#.*)?$/);
  if (!rest) return undefined;
  const indent = ' '.repeat(keyColumn(source, keyRange[0]) + 2);
  return { start: colon + 1, end: lineEnd, text: `${rest[1] ? ` ${rest[1]}` : ''}${lineEnding}${indentBlock(encoded, indent, lineEnding)}` };
}

/**
 * The line ending for text inserted at `offset`: that of the line the insertion follows, or splits when it
 * lands inside a line. A text with no line break uses LF.
 */
function eolAt(source: string, offset: number): string {
  const endingOf = (breakIndex: number) => (source[breakIndex - 1] === '\r' ? '\r\n' : '\n');
  const before = offset > 0 ? source.lastIndexOf('\n', offset - 1) : -1;
  if (before >= 0 && before === offset - 1) return endingOf(before);
  const after = source.indexOf('\n', offset);
  if (after >= 0) return endingOf(after);
  return before >= 0 ? endingOf(before) : '\n';
}

/** True when the node at `path`, or any collection above it, is written in flow style (`{...}` or `[...]`). */
function inFlowContext(root: Node | null | undefined, path: readonly unknown[]): boolean {
  for (let depth = 0; depth < path.length; depth += 1) {
    const container = getNode(root, path.slice(0, depth));
    if ((isMap(container) || isSeq(container)) && container.flow) return true;
  }
  return false;
}

function emptySequencePatch(
  source: string,
  root: Node | null | undefined,
  path: unknown[],
  sequence: YAMLSeq,
  item: { start: number; end: number; text: string } | undefined,
): SourcePatch | undefined {
  const parent = getNode(root, path.slice(0, -1));
  const pair = isMap(parent) ? findPair(parent, path[path.length - 1]) : undefined;
  if (!pair?.key?.range || !sequence.range) return undefined;
  const colon = source.indexOf(':', pair.key.range[1]);
  if (colon < 0) return undefined;
  const rowStart = item?.start ?? lineStart(source, sequence.range[0]);
  const header = source.slice(colon + 1, rowStart);
  // Comment lines written above the only item stay where they are: the form does not delete user text.
  const firstItem = sequence.items[0] as Node | undefined;
  const kept = firstItem?.range ? source.slice(rowStart, lineStart(source, firstItem.range[0])) : '';
  const last = sequence.items[sequence.items.length - 1] as Node;
  const end = header.includes('#') ? last.range?.[2] ?? item?.end ?? source.length : sequence.range[2];
  // `[]` goes on the key line ahead of any comment there; a text without a final break keeps none.
  let stay = header + kept;
  if (!source.slice(rowStart, end).endsWith('\n')) stay = stay.replace(/\r?\n$/, '');
  return { start: colon + 1, end, text: ` []${stay}` };
}

function appendMissingPath(source: string, root: Node | null | undefined, path: unknown[], value: unknown): SourcePatch | undefined {
  if (!path.length) return { start: 0, end: source.length, text: renderYaml(value) };
  let parentPath: unknown[] = [];
  let missingAt = 0;
  while (missingAt < path.length && getNode(root, [...parentPath, path[missingAt]])) {
    parentPath = [...parentPath, path[missingAt]];
    missingAt += 1;
  }
  const parent = getNode(root, parentPath);
  if (!isMap(parent)) return undefined;

  const relative = path.slice(missingAt);
  let nested: unknown = value;
  for (let index = relative.length - 1; index >= 1; index -= 1) {
    const segment = relative[index];
    const previous = relative[index - 1];
    if (typeof previous === 'number') {
      const list: unknown[] = [];
      list[previous] = nested;
      nested = list;
    } else {
      nested = { [String(segment)]: nested };
    }
  }
  const key = relative[0];
  if (key === undefined || typeof key === 'number') return undefined;
  return insertMapPair(source, parent, String(key), nested);
}

function insertMapPair(source: string, map: YAMLMap, key: string, value: unknown): SourcePatch {
  const pairText = renderYaml({ [key]: value }).replace(/\n$/, '');
  if (map.flow) {
    const range = map.range;
    if (!range) return { start: source.length, end: source.length, text: `${eolAt(source, source.length)}${pairText}` };
    const close = source.lastIndexOf('}', range[1] - 1);
    // Insert right after the last pair: the brace may sit alone on its own line.
    const lastPair = map.items[map.items.length - 1] as Pair<Node, Node> | undefined;
    const lastEnd = Math.max(lastPair?.key?.range?.[1] ?? -1, lastPair?.value?.range?.[1] ?? -1);
    const insertion = lastEnd >= 0 ? lastEnd : close >= 0 ? close : range[1];
    const contents = map.items.length ? `, ${flowPair(key, value)}` : ` ${flowPair(key, value)} `;
    return { start: insertion, end: insertion, text: contents };
  }
  if (!map.items.length) {
    const range = map.range;
    const insertion = range?.[0] ?? source.length;
    const indent = range ? lineIndent(source, range[0]) : 0;
    const lineEnding = eolAt(source, insertion);
    return { start: insertion, end: insertion, text: `${indentBlock(pairText, ' '.repeat(indent), lineEnding)}${lineEnding}` };
  }
  const last = map.items[map.items.length - 1] as Pair<Node, Node>;
  const insertion = last.value && typeof last.value === 'object' && 'range' in last.value
    ? ((last.value as Node).range?.[2] ?? source.length)
    : source.length;
  const indent = keyColumn(source, (last.key as Node).range?.[0] ?? 0);
  const lineEnding = eolAt(source, insertion);
  const prefix = insertion > 0 && !source.slice(0, insertion).endsWith('\n') ? lineEnding : '';
  return {
    start: insertion,
    end: insertion,
    text: `${prefix}${indentBlock(pairText, ' '.repeat(indent), lineEnding)}${lineEnding}`,
  };
}

function deletePairPatch(source: string, map: YAMLMap, pair: Pair<Node, Node>): SourcePatch {
  if (map.flow && map.range) {
    const pairIndex = map.items.indexOf(pair);
    if (map.items.length === 1) {
      return { start: pair.key?.range?.[0] ?? map.range[0], end: pair.value?.range?.[2] ?? map.range[1], text: '' };
    }
    const start = pair.key?.range?.[0] ?? map.range[0];
    const end = pair.value?.range?.[2] ?? start;
    const next = map.items[pairIndex + 1] as Pair<Node, Node> | undefined;
    if (next) {
      const comma = source.indexOf(',', end);
      return { start, end: comma >= 0 && comma < (next.key?.range?.[0] ?? source.length) ? comma + 1 : end, text: '' };
    }
    const previous = map.items[pairIndex - 1] as Pair<Node, Node> | undefined;
    const previousEnd = previous?.value?.range?.[1] ?? previous?.key?.range?.[1];
    return { start: previousEnd === undefined ? start : separatorStart(source, previousEnd, start), end, text: '' };
  }
  const keyStart = pair.key?.range?.[0] ?? 0;
  // A key written on the dash line of a sequence item (`- name: x`): keep the dash.
  if (/^[ \t]*(?:-[ \t]+)+$/.test(source.slice(lineStart(source, keyStart), keyStart))) {
    const next = map.items[map.items.indexOf(pair) + 1] as Pair<Node, Node> | undefined;
    if (next?.key?.range) return { start: keyStart, end: next.key.range[0], text: '' };
    return { start: keyStart, end: pair.value?.range?.[1] ?? pair.key?.range?.[1] ?? keyStart, text: '{}' };
  }
  // Keep a leading byte order mark when the first line goes away.
  const start = Math.max(lineStart(source, keyStart), source.startsWith('\uFEFF') ? 1 : 0);
  let end = pair.value?.range?.[2] ?? pair.key?.range?.[2] ?? start;
  // A key with no value ends before its line break; take the break so no blank line is left behind.
  if (end > 0 && source[end - 1] !== '\n') {
    const lineEnd = source.indexOf('\n', end);
    if (!source.slice(end, lineEnd < 0 ? source.length : lineEnd).trim()) end = lineEnd < 0 ? source.length : lineEnd + 1;
  }
  return dropPrecedingBreak(source, { start, end, text: '' });
}

/**
 * Where removing the last entry of a flow collection starts: at the comma after the previous entry, so none is left
 * dangling. A comment between that comma and the entry stays, and so does the comma (a trailing one is valid YAML).
 */
function separatorStart(source: string, previousEnd: number, nodeStart: number): number {
  const comma = source.indexOf(',', previousEnd);
  if (comma < 0 || comma >= nodeStart) return nodeStart;
  return source.slice(comma, nodeStart).includes('#') ? nodeStart : comma;
}

function flowSequencePatch(source: string, sequence: YAMLSeq, operation: Extract<RecipeEditOperation, { type: 'remove' | 'move' }>): SourcePatch | undefined {
  const nodes = sequence.items as Node[];
  const selected = nodes[operation.index];
  if (!selected?.range) return undefined;
  if (operation.type === 'remove') {
    if (nodes.length === 1) return { start: selected.range[0], end: selected.range[1], text: '' };
    if (operation.index < nodes.length - 1) {
      const next = nodes[operation.index + 1];
      const comma = source.indexOf(',', selected.range[1]);
      return { start: selected.range[0], end: comma >= 0 && comma < (next.range?.[0] ?? source.length) ? comma + 1 : selected.range[1], text: '' };
    }
    const previous = nodes[operation.index - 1];
    return { start: previous?.range ? separatorStart(source, previous.range[1], selected.range[0]) : selected.range[0], end: selected.range[1], text: '' };
  }

  const first = nodes[0]?.range?.[0];
  const last = nodes[nodes.length - 1]?.range?.[1];
  if (first === undefined || last === undefined) return undefined;
  const tokens = nodes.map((item) => source.slice(item.range?.[0] ?? 0, item.range?.[1] ?? 0));
  const separators = nodes.slice(0, -1).map((item, index) => source.slice(item.range?.[1] ?? 0, nodes[index + 1].range?.[0] ?? 0));
  const [token] = tokens.splice(operation.index, 1);
  tokens.splice(operation.to, 0, token);
  return { start: first, end: last, text: tokens.map((part, index) => `${part}${separators[index] ?? ''}`).join('') };
}

function sequenceSpans(source: string, sequence: YAMLSeq): Array<{ start: number; end: number; text: string }> {
  const sequenceIndentation = sequenceIndent(source, sequence);
  const starts = sequence.items.map((item) => {
    const range = (item as Node).range;
    let start = lineStart(source, range?.[0] ?? 0);
    while (start > 0) {
      const previousEnd = start;
      const previousStart = lineStart(source, previousEnd - 1);
      const previousLine = source.slice(previousStart, previousEnd).replace(/[\r\n]+$/, '');
      const commentIndent = previousLine.match(/^ */)?.[0].length ?? 0;
      if (!previousLine.trim().startsWith('#') || commentIndent < sequenceIndentation) break;
      start = previousStart;
    }
    return start;
  });
  return sequence.items.map((item, index) => {
    const range = (item as Node).range;
    const start = starts[index];
    const next = sequence.items[index + 1] as Node | undefined;
    const end = next ? starts[index + 1] : range?.[2] ?? start;
    return { start, end, text: source.slice(start, end) };
  });
}

function sequenceIndent(source: string, sequence: YAMLSeq): number {
  const first = sequence.items[0] as Node | undefined;
  if (first?.range) return lineIndent(source, first.range[0]);
  return sequence.range ? lineIndent(source, sequence.range[0]) : 0;
}

function pathSegments(path: RecipePath): unknown[] {
  if (typeof path !== 'string') return [...path];
  const parts: unknown[] = [];
  const pattern = /(?:^|\.)([^.[\]]+)|\[(\d+)\]/g;
  for (const match of path.matchAll(pattern)) parts.push(match[2] === undefined ? match[1] : Number(match[2]));
  return parts;
}

function getNode(root: unknown, path: readonly unknown[]): Node | undefined {
  let current = root;
  for (const segment of path) {
    if (isMap(current)) current = findPair(current, segment)?.value;
    else if (isSeq(current) && typeof segment === 'number') current = current.items[segment];
    else return undefined;
  }
  return isNode(current) ? current : undefined;
}

function findPair(map: YAMLMap, key: unknown): Pair<Node, Node> | undefined {
  return map.items.find((item): item is Pair<Node, Node> => {
    const pair = item as Pair<Node, Node>;
    return isScalar(pair.key) && String(pair.key.value) === String(key);
  });
}

function flowPair(key: string, value: unknown): string {
  const keyText = renderYaml(key, true).trim();
  const valueText = renderYaml(value, true).trim();
  return `${keyText}: ${valueText}`;
}

function renderYaml(value: unknown, flow = false): string {
  if (flow && (value === null || typeof value !== 'object')) {
    // Render the scalar as the only item of a flow sequence so the library quotes flow indicators, then unwrap it.
    return renderYaml([value], true).replace(/^\[ ?/, '').replace(/ ?\]\n?$/, '');
  }
  const document = new Document(value);
  if (flow && isMap(document.contents)) document.contents.flow = true;
  if (flow && isSeq(document.contents)) document.contents.flow = true;
  return document.toString({ indent: 2, lineWidth: 0 });
}

function lineStart(source: string, offset: number): number {
  return source.lastIndexOf('\n', Math.max(0, offset - 1)) + 1;
}

function lineIndent(source: string, offset: number): number {
  const start = lineStart(source, offset);
  const prefix = source.slice(start, offset);
  return prefix.match(/^ */)?.[0].length ?? 0;
}

/** The column a mapping key starts at, counting the dashes in front of a key written on a sequence item's line. */
function keyColumn(source: string, offset: number): number {
  const prefix = source.slice(lineStart(source, offset), offset);
  return /^ *(?:- +)+$/.test(prefix) ? prefix.length : lineIndent(source, offset);
}

function indentBlock(value: string, indent: string, lineEnding: string): string {
  return value.replace(/\r?\n/g, lineEnding).split(lineEnding).map((line) => `${indent}${line}`).join(lineEnding);
}

function sameValue(left: unknown, right: unknown): boolean {
  try {
    return JSON.stringify(left) === JSON.stringify(right);
  } catch {
    return Object.is(left, right);
  }
}

function issueAt(source: string, kind: RecipeDocumentIssue['kind'], message: string, offset?: number): RecipeDocumentIssue {
  if (offset === undefined) return { kind, message };
  const before = source.slice(0, offset);
  const line = before.split(/\r\n|\r|\n/).length;
  const column = offset - Math.max(before.lastIndexOf('\n'), before.lastIndexOf('\r'));
  return { kind, message, line, column };
}

/** The parser appends `at line N, column M:` and a source snippet; the issue carries the line separately. */
function withoutPosition(message = ''): string {
  return message.replace(/\s+at line \d+, column \d+:[\s\S]*$/, '');
}

function syntaxIssue(error: unknown): RecipeDocumentIssue {
  const candidate = error as { message?: string; linePos?: Array<{ line: number; col: number }> };
  const position = candidate.linePos?.[0];
  return {
    kind: 'syntax',
    message: withoutPosition(candidate.message) || 'Invalid YAML.',
    ...(position ? { line: position.line, column: position.col } : {}),
  };
}

function unsupportedNodeIssue(node: Node, source: string): RecipeDocumentIssue | undefined {
  const seen = new Set<object>();
  const visit = (current: unknown): RecipeDocumentIssue | undefined => {
    if (!current || typeof current !== 'object' || seen.has(current)) return undefined;
    seen.add(current);
    if (isAlias(current)) return issueAt(source, 'aliases', 'Aliases are not supported in the Form view.', current.range?.[0]);
    const yamlNode = current as Node & { anchor?: string; tag?: string };
    if (yamlNode.anchor) return issueAt(source, 'anchors', 'Anchors are not supported in the Form view.', yamlNode.range?.[0]);
    if (isMap(current)) {
      for (const item of current.items) {
        const key = item.key as Node | undefined;
        if (isScalar(key) && key.value === '<<' && key.type === 'PLAIN') {
          return issueAt(source, 'merge', 'Merge keys are not supported in the Form view.', key.range?.[0]);
        }
        const keyIssue = visit(item.key);
        if (keyIssue) return keyIssue;
        const valueIssue = visit(item.value);
        if (valueIssue) return valueIssue;
      }
    } else if (isSeq(current)) {
      for (const item of current.items) {
        const issue = visit(item);
        if (issue) return issue;
      }
    }
    return undefined;
  };
  return visit(node);
}
