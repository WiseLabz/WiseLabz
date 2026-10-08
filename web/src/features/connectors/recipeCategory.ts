import { ConnectorCategory } from '../../api/model';
import type { Connector } from '../../api/model';
import { isMap } from 'yaml';
import { parseRecipeDocument } from './recipeDocument';

const categories = Object.values(ConnectorCategory);

// Kept apart from recipeForm.ts so the YAML parser loads only with the lazy category display.
/** Reads the root category through the same YAML document model as the recipe form. */
export function readRecipeCategory(recipe: string): Connector['category'] | undefined {
  const parsed = parseRecipeDocument(recipe);
  if (parsed.error || !parsed.document || !isMap(parsed.document.contents)) return undefined;
  const category = parsed.document.get('category');
  return typeof category === 'string' ? categories.find((value) => value === category) : undefined;
}
