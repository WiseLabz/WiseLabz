import { describe, expect, it } from 'vitest';
import { mergeRecipeFeedback, type RecipeFeedback } from './recipeForm';

const endpoints = [{ name: 'items', items: 1, count: 1, skipped: 0, samples: [], dependencies: [] }];
const saveError = { field: 'config.recipe.endpoints[0].entity.external_id', msg: 'mapping must select an ID' };
const previewError = { field: 'config.recipe.endpoints[0].path', msg: 'path is required' };

describe('mergeRecipeFeedback', () => {
  it('returns no errors when nothing was recorded', () => {
    expect(mergeRecipeFeedback(undefined, 'a')).toEqual({ errors: [] });
    expect(mergeRecipeFeedback({}, 'a')).toEqual({ errors: [] });
  });

  it('returns the save errors for the current recipe without endpoints', () => {
    const feedback: RecipeFeedback = { save: { recipe: 'a', errors: [saveError], seq: 1 } };
    expect(mergeRecipeFeedback(feedback, 'a')).toEqual({ errors: [saveError], errorRecipe: 'a', endpoints: undefined });
  });

  it('keeps save errors when a later preview for the same recipe succeeds', () => {
    const feedback: RecipeFeedback = {
      save: { recipe: 'a', errors: [saveError], seq: 1 },
      preview: { recipe: 'a', errors: [], endpoints, seq: 2 },
    };
    expect(mergeRecipeFeedback(feedback, 'a')).toEqual({ errors: [saveError], errorRecipe: 'a', endpoints });
  });

  it('concatenates save then preview errors and de-duplicates by field and message', () => {
    const duplicateOfSave = { ...saveError };
    const feedback: RecipeFeedback = {
      save: { recipe: 'a', errors: [saveError], seq: 1 },
      preview: { recipe: 'a', errors: [previewError, duplicateOfSave], seq: 2 },
    };
    expect(mergeRecipeFeedback(feedback, 'a').errors).toEqual([saveError, previewError]);
  });

  it('keeps errors that share a field but differ in message', () => {
    const other = { field: saveError.field, msg: 'different message' };
    const feedback: RecipeFeedback = {
      save: { recipe: 'a', errors: [saveError], seq: 1 },
      preview: { recipe: 'a', errors: [other], seq: 2 },
    };
    expect(mergeRecipeFeedback(feedback, 'a').errors).toEqual([saveError, other]);
  });

  it('takes endpoints only from a preview of the current recipe', () => {
    const stalePreview: RecipeFeedback = { preview: { recipe: 'old', errors: [], endpoints, seq: 1 } };
    const merged = mergeRecipeFeedback(stalePreview, 'a');
    expect(merged.endpoints).toBeUndefined();
    expect(merged.errorRecipe).toBe('old');
  });

  it('falls back to the newest stale entry when nothing matches the current recipe', () => {
    const feedback: RecipeFeedback = {
      save: { recipe: 'old', errors: [saveError], seq: 1 },
      preview: { recipe: 'older', errors: [previewError], seq: 2 },
    };
    expect(mergeRecipeFeedback(feedback, 'a')).toEqual({ errors: [previewError], errorRecipe: 'older' });

    const newestIsSave: RecipeFeedback = {
      save: { recipe: 'old', errors: [saveError], seq: 3 },
      preview: { recipe: 'older', errors: [previewError], seq: 2 },
    };
    expect(mergeRecipeFeedback(newestIsSave, 'a')).toEqual({ errors: [saveError], errorRecipe: 'old' });
  });

  it('ignores stale entries once a current entry exists', () => {
    const feedback: RecipeFeedback = {
      save: { recipe: 'old', errors: [saveError], seq: 3 },
      preview: { recipe: 'a', errors: [], endpoints, seq: 1 },
    };
    expect(mergeRecipeFeedback(feedback, 'a')).toEqual({ errors: [], errorRecipe: 'a', endpoints });
  });
});
