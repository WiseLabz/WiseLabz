import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { parseRecipeDocument } from './recipeDocument';
import { unknownRecipeKeys } from './recipeFormat';

const repositoryRoot = resolve(process.cwd(), '..');
const recipesDirectory = `${repositoryRoot}/docs/connectors/recipes`;
const recipeFormat = readFileSync(`${repositoryRoot}/docs/connectors/RECIPE_FORMAT.md`, 'utf8');

describe('recipe format table', () => {
  it('recognizes the shipped and documented complete recipes', () => {
    const shipped = readdirSync(recipesDirectory)
      .filter((name) => name.endsWith('.yaml'))
      .map((name) => readFileSync(`${recipesDirectory}/${name}`, 'utf8'));
    const examples = [...recipeFormat.matchAll(/<!-- pagination-recipes-start -->([\s\S]*?)<!-- pagination-recipes-end -->/g)]
      .flatMap(([, section]) => [...section.matchAll(/```yaml\n([\s\S]*?)```/g)].map(([, example]) => example));

    for (const source of [...shipped, ...examples]) {
      expect(unknownRecipeKeys(parseRecipeDocument(source).document)).toEqual([]);
    }
  });

  it('reports unknown keys at the root, endpoint, entity and attribute mappings', () => {
    const source = `version: 1
category: media
auth: {mode: none}
custom_root: true
endpoints:
  - name: items
    path: /items
    method: GET
    items: items
    custom_endpoint: true
    entity:
      kind: media_item
      name: name
      external_id: id
      custom_entity: true
      attributes:
        status:
          path: status
          type: string
          custom_attribute: true
    actions:
      - type: custom_action
`;
    const unknown = unknownRecipeKeys(parseRecipeDocument(source).document);

    expect(unknown.map(({ path, keys }) => ({ path, keys }))).toEqual([
      { path: [], keys: ['custom_root'] },
      { path: ['endpoints', 0], keys: ['custom_endpoint', 'actions'] },
      { path: ['endpoints', 0, 'entity'], keys: ['custom_entity'] },
      { path: ['endpoints', 0, 'entity', 'attributes', 'status'], keys: ['custom_attribute'] },
    ]);
  });
});
