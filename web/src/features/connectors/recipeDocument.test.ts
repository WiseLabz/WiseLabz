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
});
