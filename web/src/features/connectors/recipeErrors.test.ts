import { describe, expect, it } from 'vitest';
import { recipeFieldId, resolveRecipeError } from './recipeErrors';

describe('recipeFieldId', () => {
  it('returns distinct IDs for an action named restart-path vs the path field of an action named restart', () => {
    const actionHyphen = recipeFieldId(['actions', 'restart-path']);
    const actionNested = recipeFieldId(['actions', 'restart', 'path']);
    expect(actionHyphen).toBe('recipe-field-actions-restart--path');
    expect(actionNested).toBe('recipe-field-actions-restart-path');
    expect(actionHyphen).not.toBe(actionNested);
  });

  it('returns distinct IDs for an action restart-path with field path', () => {
    const actionHyphen = recipeFieldId(['actions', 'restart-path']);
    const actionNested = recipeFieldId(['actions', 'restart', 'path']);
    const actionHyphenWithPath = recipeFieldId(['actions', 'restart-path', 'path']);
    expect(actionHyphenWithPath).toBe('recipe-field-actions-restart--path-path');
    expect(actionHyphenWithPath).not.toBe(actionHyphen);
    expect(actionHyphenWithPath).not.toBe(actionNested);
  });

  it('returns distinct IDs for a header x-y vs a header x with nested field y', () => {
    const headerHyphen = recipeFieldId(['endpoints', 0, 'headers', 'x-y']);
    const headerNested = recipeFieldId(['endpoints', 0, 'headers', 'x', 'y']);
    expect(headerHyphen).toBe('recipe-field-endpoints-0-headers-x--y');
    expect(headerNested).toBe('recipe-field-endpoints-0-headers-x-y');
    expect(headerHyphen).not.toBe(headerNested);
  });

  it('returns distinct IDs for a header x-key vs a header x key field', () => {
    const headerKeyHyphen = recipeFieldId(['endpoints', 0, 'headers', 'x-key']);
    const headerKeyNested = recipeFieldId(['endpoints', 0, 'headers', 'x', 'key']);
    expect(headerKeyHyphen).toBe('recipe-field-endpoints-0-headers-x--key');
    expect(headerKeyNested).toBe('recipe-field-endpoints-0-headers-x-key');
    expect(headerKeyHyphen).not.toBe(headerKeyNested);
  });

  it('returns distinct IDs for query parameter keys with hyphens', () => {
    const queryHyphen = recipeFieldId(['endpoints', 0, 'query', 'api-key']);
    const queryNested = recipeFieldId(['endpoints', 0, 'query', 'api', 'key']);
    expect(queryHyphen).toBe('recipe-field-endpoints-0-query-api--key');
    expect(queryNested).toBe('recipe-field-endpoints-0-query-api-key');
    expect(queryHyphen).not.toBe(queryNested);

    const actionQueryHyphen = recipeFieldId(['actions', 'restart', 'query', 'dry-run']);
    const actionQueryNested = recipeFieldId(['actions', 'restart', 'query', 'dry', 'run']);
    expect(actionQueryHyphen).toBe('recipe-field-actions-restart-query-dry--run');
    expect(actionQueryNested).toBe('recipe-field-actions-restart-query-dry-run');
    expect(actionQueryHyphen).not.toBe(actionQueryNested);

    const multiHyphenQuery = recipeFieldId(['endpoints', 0, 'query', 'x-auth-token']);
    const splitQuery = recipeFieldId(['endpoints', 0, 'query', 'x-auth', 'token']);
    expect(multiHyphenQuery).toBe('recipe-field-endpoints-0-query-x--auth--token');
    expect(splitQuery).toBe('recipe-field-endpoints-0-query-x--auth-token');
    expect(multiHyphenQuery).not.toBe(splitQuery);
  });
});

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
    it('returns the expected escaped id for error locations with hyphenated action names', () => {
      const targetPath = resolveRecipeError(withActions, 'recipe.actions.restart-now.path');
      expect(targetPath).toMatchObject({
        path: ['actions', 'restart-now', 'path'],
        id: 'recipe-field-actions-restart--now-path',
      });

      const targetAction = resolveRecipeError(withActions, 'recipe.actions.restart-now');
      expect(targetAction).toMatchObject({
        path: ['actions', 'restart-now'],
        id: 'recipe-field-actions-restart--now',
      });
    });
  });

  describe('header and query locations with hyphens', () => {
    const withHeadersAndQuery = `version: 1
category: other
auth: {mode: none}
endpoints:
  - name: items
    headers:
      x-api-key: secret
    query:
      page-size: 50
`;

    it('resolves hyphenated header and query locations with escaped ids', () => {
      const headerTarget = resolveRecipeError(withHeadersAndQuery, 'recipe.endpoints[0].headers.x-api-key');
      expect(headerTarget).toMatchObject({
        path: ['endpoints', 0, 'headers', 'x-api-key'],
        id: 'recipe-field-endpoints-0-headers-x--api--key',
      });

      const queryTarget = resolveRecipeError(withHeadersAndQuery, 'recipe.endpoints[0].query.page-size');
      expect(queryTarget).toMatchObject({
        path: ['endpoints', 0, 'query', 'page-size'],
        id: 'recipe-field-endpoints-0-query-page--size',
      });
    });
  });
});

