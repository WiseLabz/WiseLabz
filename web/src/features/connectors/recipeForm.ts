import type { ConnectorTypeSchema, FieldError, RecipePreview } from '../../api/model';
import { fieldDefault, isTopLevelField } from './schemaFields';

/** One save or preview outcome, tagged with the recipe text it was produced from. */
export type RecipeFeedbackEntry = { recipe: string; errors: FieldError[]; endpoints?: RecipePreview['endpoints']; seq: number };

/** Latest save and latest preview outcome; each slot is written independently. */
export type RecipeFeedback = { save?: RecipeFeedbackEntry; preview?: RecipeFeedbackEntry };

/** Keeps field locations supplied by the API intact for inline form errors. */
export function locatedErrorsFrom(error: unknown): FieldError[] {
  const details = (error as { response?: { data?: { details?: unknown } } } | undefined)?.response?.data?.details;
  if (!Array.isArray(details)) return [];
  return details.filter(
    (detail): detail is FieldError =>
      !!detail &&
      typeof detail === 'object' &&
      typeof (detail as FieldError).field === 'string' &&
      typeof (detail as FieldError).msg === 'string',
  );
}

/**
 * Picks the feedback to show for the recipe currently in the editor. Entries for that
 * exact text combine (save first, then preview); when none match, the newest entry is
 * returned with its own recipe so the editor can mark it stale.
 */
export function mergeRecipeFeedback(
  feedback: RecipeFeedback | undefined,
  recipe: string,
): { errors: FieldError[]; errorRecipe?: string; endpoints?: RecipePreview['endpoints'] } {
  const entries = [feedback?.save, feedback?.preview].filter((entry): entry is RecipeFeedbackEntry => !!entry);
  if (entries.length === 0) return { errors: [] };
  const current = entries.filter((entry) => entry.recipe === recipe);
  if (current.length === 0) {
    const newest = entries.reduce((latest, entry) => (entry.seq > latest.seq ? entry : latest));
    return { errors: newest.errors, errorRecipe: newest.recipe };
  }
  const seen = new Set<string>();
  const errors: FieldError[] = [];
  for (const entry of current) {
    for (const error of entry.errors) {
      const key = JSON.stringify([error.field, error.msg]);
      if (seen.has(key)) continue;
      seen.add(key);
      errors.push(error);
    }
  }
  const preview = feedback?.preview?.recipe === recipe ? feedback.preview : undefined;
  return { errors, errorRecipe: recipe, endpoints: preview?.endpoints };
}

export function errorsForField(errors: FieldError[], fieldName: string): string | undefined {
  const messages = errors
    .filter((error) => {
      const location = error.field.replace(/^config\./, '');
      return location === fieldName || location.startsWith(`${fieldName}.`) || location.startsWith(`${fieldName}[`);
    })
    .map((error) => error.msg);
  return messages.length ? messages.join(' ') : undefined;
}

export function focusFirstLocatedError(errors: FieldError[], fallbackField?: string): void {
  if (/^(config\.)?recipe(?:[.[]|$)/.test(errors[0]?.field ?? '')) {
    const event = new CustomEvent('connector-recipe-focus', { detail: { field: errors[0].field }, cancelable: true });
    if (!window.dispatchEvent(event)) return;
  }
  const location = errors[0]?.field.replace(/^config\./, '');
  const fieldName = location?.match(/^[^.[\]]+/)?.[0] ?? fallbackField;
  const target = fieldName ? document.getElementById(`connector-field-${fieldName}`) : null;
  (target ?? (fallbackField ? document.getElementById(`connector-field-${fallbackField}`) : null))?.focus();
}

/** Combines unsaved edits with the non-secret values returned for an existing connector. */
export function recipePreviewConfig(
  schema: ConnectorTypeSchema,
  values: Record<string, string | boolean>,
  stored: Record<string, unknown> = {},
): Record<string, unknown> {
  const config: Record<string, unknown> = {};
  for (const field of schema.fields) {
    if (isTopLevelField(field)) continue;
    config[field.name] = values[field.name] ?? stored[field.name] ?? fieldDefault(field);
  }
  return config;
}
