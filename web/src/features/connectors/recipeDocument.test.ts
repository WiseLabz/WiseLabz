import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { editRecipeDocument, parseRecipeDocument, serializeRecipeDocument } from './recipeDocument';

const repositoryRoot = resolve(process.cwd(), '..');
const recipesDirectory = `${repositoryRoot}/docs/connectors/recipes`;
const recipeFormat = readFileSync(`${repositoryRoot}/docs/connectors/RECIPE_FORMAT.md`, 'utf8');

describe('recipe document parsing', () => {
  it('keeps every shipped recipe and the format reference examples byte-identical when unedited', () => {
    const shipped = readdirSync(recipesDirectory)
      .filter((name) => name.endsWith('.yaml'))
      .map((name) => readFileSync(`${recipesDirectory}/${name}`, 'utf8'));
    const reference = [...recipeFormat.matchAll(/```yaml\r?\n([\s\S]*?)```/g)].map(([, example]) => example);
    const fixtures = [
      '# a comment stays\nversion: 1\ncategory: media\nauth: { mode: none }\n',
      'version: 1\ncategory: {name: media}\n',
      '"category": media\n',
      '\uFEFFversion: 1\r\ncategory: "media"  \r\n# final comment\r\n',
      'version: 1\ncategory: media   \n',
    ];

    for (const source of [...shipped, ...reference, ...fixtures]) {
      expect(serializeRecipeDocument(parseRecipeDocument(source))).toBe(source);
    }
  });

  it('reports syntax and unsupported document shapes with a line', () => {
    const invalid = parseRecipeDocument('version: 1\ncategory: [\n');
    expect(invalid.error?.kind).toBe('syntax');
    expect(invalid.error?.line).toBeGreaterThan(0);

    for (const [source, kind] of [
      ['- item\n', 'root'],
      ['category: media\n---\ncategory: dns\n', 'documents'],
      ['base: &base {mode: none}\n', 'anchors'],
      ['value: *missing\n', 'aliases'],
      ['defaults: {mode: none}\nrecipe: {<<: defaults}\n', 'merge'],
    ] as const) {
      const parsed = parseRecipeDocument(source);
      expect(parsed.unsupported?.kind).toBe(kind);
      expect(parsed.unsupported?.line).toBeGreaterThan(0);
    }
  });

  it('changes one scalar while preserving comments, quoting and CRLF around it', () => {
    const source = '\uFEFF# keep\r\nversion: 1  \r\ncategory: "media" # category note\r\nauth: {mode: none}\r\n';
    const updated = editRecipeDocument(source, { type: 'set', path: ['category'], value: 'dns' });

    expect(updated).toBe('\uFEFF# keep\r\nversion: 1  \r\ncategory: dns # category note\r\nauth: {mode: none}\r\n');
  });

  it('sets a typed scalar and adds or deletes only the requested mapping key', () => {
    const source = 'version: 1\nauth:\n  mode: header # keep\n  name: Authorization\n';
    const typed = editRecipeDocument(source, { type: 'set', path: ['version'], value: 2 });
    expect(typed).toBe('version: 2\nauth:\n  mode: header # keep\n  name: Authorization\n');

    const added = editRecipeDocument(typed, { type: 'set', path: ['auth', 'prefix'], value: 'Bearer ' });
    expect(added).toBe(`${typed}  prefix: "Bearer "\n`);
    const deleted = editRecipeDocument(added, { type: 'delete', path: ['auth', 'name'] });
    expect(deleted).toBe('version: 2\nauth:\n  mode: header # keep\n  prefix: "Bearer "\n');
  });

  it('renames a mapping key without rewriting the stored value or unknown fields', () => {
    const source = 'auth: {mode: none, custom: {quoted: "keep me"}} # trailing\n';
    const renamed = editRecipeDocument(source, { type: 'rename', path: ['auth', 'custom'], to: 'custom_field' });
    expect(renamed).toBe('auth: {mode: none, custom_field: {quoted: "keep me"}} # trailing\n');

    const updated = editRecipeDocument(source, { type: 'set', path: ['auth', 'prefix'], value: 'Bearer' });
    expect(updated).toBe('auth: {mode: none, custom: {quoted: "keep me"}, prefix: Bearer} # trailing\n');

    const deleted = editRecipeDocument(source, { type: 'delete', path: ['auth', 'custom'] });
    expect(deleted).toBe('auth: {mode: none} # trailing\n');
  });

  it('adds, removes and moves sequence blocks while preserving neighboring text', () => {
    const source = 'endpoints:\n  - name: one\n    path: /one\n  - name: two\n    path: /two\n';
    const appended = editRecipeDocument(source, {
      type: 'append',
      path: ['endpoints'],
      value: { name: 'three', path: '/three' },
    });
    expect(appended).toBe(`${source}  - name: three\n    path: /three\n`);

    const removed = editRecipeDocument(appended, { type: 'remove', path: ['endpoints'], index: 1 });
    expect(removed).toBe('endpoints:\n  - name: one\n    path: /one\n  - name: three\n    path: /three\n');

    const moved = editRecipeDocument(source, { type: 'move', path: ['endpoints'], index: 0, to: 1 });
    expect(moved).toBe('endpoints:\n  - name: two\n    path: /two\n  - name: one\n    path: /one\n');
  });

  it('keeps comments attached to sequence items when removing or moving them', () => {
    const source = 'items:\n  # first item\n  - name: first\n  # second item\n  - name: second\n';
    const removed = editRecipeDocument(source, { type: 'remove', path: ['items'], index: 0 });
    expect(removed).toBe('items:\n  # second item\n  - name: second\n');

    const moved = editRecipeDocument(source, { type: 'move', path: ['items'], index: 0, to: 1 });
    expect(moved).toBe('items:\n  # second item\n  - name: second\n  # first item\n  - name: first\n');
  });

  it('edits flow sequence tokens without reserializing retained items', () => {
    const source = 'values: [one, { custom: "keep # this" }, three]\n';
    expect(editRecipeDocument(source, { type: 'remove', path: ['values'], index: 0 })).toBe('values: [ { custom: "keep # this" }, three]\n');
    expect(editRecipeDocument(source, { type: 'move', path: ['values'], index: 0, to: 2 })).toBe('values: [{ custom: "keep # this" }, three, one]\n');
  });

  it('inserts a parsed block as a two-space-indented YAML node', () => {
    const source = 'version: 1\nbody: null\nkeep: true\n';
    const updated = editRecipeDocument(source, {
      type: 'block',
      path: ['body'],
      text: '{query: {status: active}}',
    });
    expect(updated).toBe('version: 1\nbody:\n  query:\n    status: active\nkeep: true\n');
  });

  it('deletes the first key of a sequence item without dropping its dash', () => {
    const source = 'endpoints:\n  - name: one\n    path: /one\n  - path: /two\n';
    expect(editRecipeDocument(source, { type: 'delete', path: ['endpoints', 0, 'name'] })).toBe('endpoints:\n  - path: /one\n  - path: /two\n');
    expect(editRecipeDocument(source, { type: 'delete', path: ['endpoints', 1, 'path'] })).toBe('endpoints:\n  - name: one\n    path: /one\n  - {}\n');
  });

  it('keeps a leading byte order mark when the first key is deleted', () => {
    expect(editRecipeDocument('\uFEFFversion: 1\ncategory: media\n', { type: 'delete', path: ['version'] })).toBe('\uFEFFcategory: media\n');
  });

  it('sets a key that has no value and replaces a block scalar without joining lines', () => {
    expect(editRecipeDocument('name:\nkeep: 1\n', { type: 'set', path: ['name'], value: 'x' })).toBe('name: x\nkeep: 1\n');
    expect(editRecipeDocument('a: |\n  x\nb: 2\n', { type: 'set', path: ['a'], value: 'y' })).toBe('a: y\nb: 2\n');
  });

  it('removes a key that has no value together with its line', () => {
    expect(editRecipeDocument('a: 1\nname:\nb: 2\n', { type: 'delete', path: ['name'] })).toBe('a: 1\nb: 2\n');
  });

  it('indents a key added to a mapping whose last key sits on a list-item line', () => {
    const source = 'items:\n  - name: x\nafter: 1\n';
    expect(editRecipeDocument(source, { type: 'set', path: ['items', 0, 'path'], value: '/x' })).toBe('items:\n  - name: x\n    path: /x\nafter: 1\n');
  });

  it('keeps a missing final newline missing when removing or moving the last sequence item', () => {
    const source = 'a: 1\nitems:\n  - name: x\n  - name: y';
    expect(editRecipeDocument(source, { type: 'remove', path: ['items'], index: 1 })).toBe('a: 1\nitems:\n  - name: x');
    expect(editRecipeDocument(source, { type: 'remove', path: ['items'], index: 0 })).toBe('a: 1\nitems:\n  - name: y');
    expect(editRecipeDocument(source, { type: 'move', path: ['items'], index: 0, to: 1 })).toBe('a: 1\nitems:\n  - name: y\n  - name: x');
  });

  it('keeps a header comment on the key line when the last sequence item is removed', () => {
    const source = 'items: # list\n  - name: x\nafter: 1\n';
    expect(editRecipeDocument(source, { type: 'remove', path: ['items'], index: 0 })).toBe('items: [] # list\nafter: 1\n');
  });

  it('replaces a null parent when setting below it or appending to it', () => {
    expect(editRecipeDocument('auth:\nversion: 1\n', { type: 'set', path: ['auth', 'mode'], value: 'none' })).toBe('auth:\n  mode: none\nversion: 1\n');
    expect(editRecipeDocument('auth: # note\nversion: 1\n', { type: 'set', path: ['auth', 'mode'], value: 'none' })).toBe('auth: # note\n  mode: none\nversion: 1\n');
    expect(editRecipeDocument('auth:\nversion: 1\n', { type: 'append', path: ['auth'], value: 'x' })).toBe('auth:\n  - x\nversion: 1\n');
  });

  it('replaces a null value with a scalar and keeps the comment and the rest of the line', () => {
    const set = (source: string): string => editRecipeDocument(source, { type: 'set', path: ['extra'], value: 'zz-new' });
    expect(set('auth:\nversion: 1\nextra: # none\nlast: ~\n')).toBe('auth:\nversion: 1\nextra: zz-new # none\nlast: ~\n');
    expect(set('extra:\nlast: 1\n')).toBe('extra: zz-new\nlast: 1\n');
    expect(set('extra: ~\n')).toBe('extra: zz-new\n');
    expect(set('extra: null # c\nz: 1\n')).toBe('extra: zz-new # c\nz: 1\n');
    expect(set('extra: # none\r\nlast: 1\r\n')).toBe('extra: zz-new # none\r\nlast: 1\r\n');
  });

  it('inserts into multi-line flow collections before the closing bracket line', () => {
    expect(editRecipeDocument('values: [\n  one,\n  two\n]\nafter: 1\n', { type: 'append', path: ['values'], value: 'three' })).toBe(
      'values: [\n  one,\n  two, three\n]\nafter: 1\n',
    );
    expect(editRecipeDocument('auth: {\n  mode: none,\n  name: x\n}\nafter: 1\n', { type: 'set', path: ['auth', 'prefix'], value: 'Bearer' })).toBe(
      'auth: {\n  mode: none,\n  name: x, prefix: Bearer\n}\nafter: 1\n',
    );
  });

  it('deletes the last key of a document with no final newline without leaving one', () => {
    expect(editRecipeDocument('a: 1\nb: 2', { type: 'delete', path: ['b'] })).toBe('a: 1');
  });

  it('returns the exact source for a no-op edit', () => {
    const source = '# preserved\ncategory: media  \n';
    expect(editRecipeDocument(source, { type: 'set', path: ['category'], value: 'media' })).toBe(source);
  });

  it('builds a recipe through sequential edits from empty or BOM-only text', () => {
    let recipe = '';
    recipe = editRecipeDocument(recipe, { type: 'set', path: ['version'], value: 1 });
    recipe = editRecipeDocument(recipe, { type: 'set', path: ['category'], value: 'media' });
    recipe = editRecipeDocument(recipe, { type: 'set', path: ['auth', 'mode'], value: 'none' });
    recipe = editRecipeDocument(recipe, { type: 'set', path: ['endpoints'], value: [] });
    recipe = editRecipeDocument(recipe, { type: 'append', path: ['endpoints'], value: { name: 'items' } });
    expect(recipe).toBe('version: 1\ncategory: media\nauth:\n  mode: none\nendpoints:\n  - name: items\n');

    expect(editRecipeDocument('\uFEFF', { type: 'set', path: ['version'], value: 1 })).toBe('\uFEFFversion: 1\n');
  });

  describe('values with flow indicators', () => {
    const placeholder = '/items/{external_id}/rescan';

    it('quotes a path with a placeholder when it is set, edited or added inside a flow mapping', () => {
      const source = 'actions: {rescan: {method: POST, path: /old}}\n';
      const set = editRecipeDocument(source, { type: 'set', path: ['actions', 'rescan', 'path'], value: placeholder });
      expect(set).toBe('actions: {rescan: {method: POST, path: "/items/{external_id}/rescan"}}\n');
      const edited = editRecipeDocument(set, { type: 'set', path: ['actions', 'rescan', 'path'], value: `${placeholder}?x=1` });
      expect(edited).toBe('actions: {rescan: {method: POST, path: "/items/{external_id}/rescan?x=1"}}\n');
      const added = editRecipeDocument('actions: {rescan: {method: POST}}\n', { type: 'set', path: ['actions', 'rescan', 'path'], value: placeholder });
      expect(added).toBe('actions: {rescan: {method: POST, path: "/items/{external_id}/rescan"}}\n');
      const key = editRecipeDocument(source, { type: 'set', path: ['actions', 'rescan', 'label'], value: 'Rescan, now [all]' });
      expect(parseRecipeDocument(key).document?.toJS()).toEqual({ actions: { rescan: { method: 'POST', path: '/old', label: 'Rescan, now [all]' } } });
      const sequence = editRecipeDocument('values: [a, b]\n', { type: 'append', path: ['values'], value: '{x}' });
      expect(parseRecipeDocument(sequence).document?.toJS()).toEqual({ values: ['a', 'b', '{x}'] });
    });

    it('keeps a path with a placeholder plain in a block mapping', () => {
      const source = 'actions:\n  rescan:\n    method: POST\n    path: /old\n';
      const set = editRecipeDocument(source, { type: 'set', path: ['actions', 'rescan', 'path'], value: placeholder });
      expect(set).toBe('actions:\n  rescan:\n    method: POST\n    path: /items/{external_id}/rescan\n');
      const added = editRecipeDocument('actions:\n  rescan:\n    method: POST\n', { type: 'set', path: ['actions', 'rescan', 'path'], value: placeholder });
      expect(added).toBe('actions:\n  rescan:\n    method: POST\n    path: /items/{external_id}/rescan\n');
      expect(parseRecipeDocument(added).document?.toJS()).toEqual({ actions: { rescan: { method: 'POST', path: placeholder } } });
    });

    it('renames a key inside a flow mapping to a name that needs quoting', () => {
      const renamed = editRecipeDocument('x: {a: 1}\n', { type: 'rename', path: ['x', 'a'], to: 'b,c' });
      expect(parseRecipeDocument(renamed).document?.toJS()).toEqual({ x: { 'b,c': 1 } });
    });
  });

  describe('line endings of inserted lines', () => {
    it('uses the ending of the line the new line follows, not the majority of the file', () => {
      const source = 'version: 1\r\nauth:\r\n  mode: none\r\nendpoints:\n  - name: one\n    path: /one\n';
      expect(editRecipeDocument(source, { type: 'set', path: ['auth', 'name'], value: 'X' })).toBe(
        'version: 1\r\nauth:\r\n  mode: none\r\n  name: X\r\nendpoints:\n  - name: one\n    path: /one\n',
      );
      expect(editRecipeDocument(source, { type: 'set', path: ['endpoints', 0, 'method'], value: 'GET' })).toBe(
        'version: 1\r\nauth:\r\n  mode: none\r\nendpoints:\n  - name: one\n    path: /one\n    method: GET\n',
      );
    });

    it('takes the first line ending for an insertion at the very start and LF for text without line breaks', () => {
      expect(editRecipeDocument('\r\n', { type: 'set', path: ['a'], value: 1 })).toContain('a: 1');
      expect(editRecipeDocument('a: 1', { type: 'set', path: ['b'], value: 2 })).toBe('a: 1\nb: 2\n');
      expect(editRecipeDocument('a: 1\r\nb: 2\r\n', { type: 'set', path: ['c'], value: 3 })).toBe('a: 1\r\nb: 2\r\nc: 3\r\n');
    });
  });

  it.each([
    ['indented like the item', 'endpoints:\n  # the main list\n  - name: one\n    path: /one\nafter: 1\n', 'endpoints: []\n  # the main list\nafter: 1\n'],
    ['at the key column', 'endpoints:\n# the main list\n  - name: one\nafter: 1\n', 'endpoints: []\n# the main list\nafter: 1\n'],
    ['with a note on the key line too', 'endpoints: # list\n  # the main list\n  - name: one\nafter: 1\n', 'endpoints: [] # list\n  # the main list\nafter: 1\n'],
    ['with CRLF', 'endpoints:\r\n  # the main list\r\n  - name: one\r\nafter: 1\r\n', 'endpoints: []\r\n  # the main list\r\nafter: 1\r\n'],
    ['without a final newline', 'endpoints:\n  # the main list\n  - name: one', 'endpoints: []\n  # the main list'],
  ])('keeps a comment above the only item, unchanged and still indented, when that item is removed (%s)', (_name, source, expected) => {
    const result = editRecipeDocument(source, { type: 'remove', path: ['endpoints'], index: 0 });
    expect(result).toBe(expected);
    expect(parseRecipeDocument(result).error).toBeUndefined();
    expect(parseRecipeDocument(result).document?.toJS()).toMatchObject({ endpoints: [] });
  });
});
