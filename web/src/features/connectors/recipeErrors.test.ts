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

  describe('action locations', () => {
    const withActions = `version: 1
category: other
auth: {mode: none}
endpoints:
  - name: items
    entity:
      kind: item
      name: name
      external_id: id
      actions:
        rescan:
          method: GET
          path: /items/{external_id}
          body: {reason: "x {attr.node}"}
actions:
  restart-now: {method: POST, path: "/restart/{external_id}"}
`;
    it.each([
      ['recipe.endpoints[0].entity.actions.rescan', ['endpoints', 0, 'entity', 'actions', 'rescan']],
      ['recipe.endpoints[0].entity.actions.rescan.method', ['endpoints', 0, 'entity', 'actions', 'rescan', 'method']],
      ['recipe.endpoints[0].entity.actions.rescan.path', ['endpoints', 0, 'entity', 'actions', 'rescan', 'path']],
      ['recipe.endpoints[0].entity.actions', ['endpoints', 0, 'entity', 'actions']],
      ['recipe.actions.restart-now.path', ['actions', 'restart-now', 'path']],
      ['recipe.actions.restart-now', ['actions', 'restart-now']],
    ])('resolves %s to its row', (field, path) => {
      expect(resolveRecipeError(withActions, field)?.path).toEqual(path);
    });
    it('reports an error below a body as the body row but marks the nested line', () => {
      const target = resolveRecipeError(withActions, 'recipe.endpoints[0].entity.actions.rescan.body.reason');
      expect(target?.path).toEqual(['endpoints', 0, 'entity', 'actions', 'rescan', 'body']);
      expect(withActions.slice(target!.from, target!.to)).toContain('x {attr.node}');
    });
    it('keeps a location below an unknown action key in the list', () => {
      expect(resolveRecipeError(withActions, 'recipe.actions.restart-now.surprise')).toBeUndefined();
    });
  });
});
