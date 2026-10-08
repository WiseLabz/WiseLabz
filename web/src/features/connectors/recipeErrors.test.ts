import { describe, expect, it } from 'vitest';
import { resolveRecipeError } from './recipeErrors';

const recipe = `version: 1
category: media
auth: {mode: none}
endpoints:
  - name: first
    entity: {kind: media_item, name: name, external_id: id}
  - name: second
    entity:
      external_id: broken
      attributes:
        service.status:
          path: status
          map: {"1": enabled}
    actions: {restart: {method: POST}}
dependencies:
  - kind: storage
    path: folders
`;

describe('recipe server error locations', () => {
  it('resolves the second endpoint mapping to its field and source span', () => {
    const target = resolveRecipeError(recipe, 'config.recipe.endpoints[1].entity.external_id');
    expect(target).toMatchObject({ path: ['endpoints', 1, 'entity', 'external_id'], id: 'recipe-field-endpoints-1-entity-external_id', line: 9 });
    expect(recipe.slice(target!.from, target!.to)).toBe('broken');
  });
  it('handles dotted attribute names and nested value maps', () => {
    expect(resolveRecipeError(recipe, 'recipe.endpoints[1].entity.attributes.service.status.map')?.path)
      .toEqual(['endpoints', 1, 'entity', 'attributes', 'service.status', 'map']);
  });
  it('routes missing known fields to the field with its parent source range', () => {
    expect(resolveRecipeError(recipe, 'recipe.endpoints[1].entity.name')).toMatchObject({ id: 'recipe-field-endpoints-1-entity-name' });
  });
  it('resolves dependency fields and keeps unknown locations in the list', () => {
    expect(resolveRecipeError(recipe, 'recipe.dependencies[0].path')?.line).toBe(17);
    expect(resolveRecipeError(recipe, 'recipe.endpoints[1].actions.restart.method')).toBeUndefined();
    expect(resolveRecipeError(recipe, 'url')).toBeUndefined();
    expect(resolveRecipeError('category: [', 'recipe.category')).toBeUndefined();
  });
});
