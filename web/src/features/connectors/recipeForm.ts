import { ConnectorCategory } from '../../api/model';
import type { Connector, ConnectorTypeSchema, FieldError } from '../../api/model';
import { fieldDefault, isTopLevelField } from './schemaFields';

const categories = Object.values(ConnectorCategory);

/** Reads the recipe's root category scalar without trying to parse all of YAML. */
export function readRecipeCategory(recipe: string): Connector['category'] | undefined {
  const source = recipe.replace(/^\uFEFF/, '').trim();
  const lines = source.split(/\r?\n/);
  const firstContent = lines.findIndex((line) =>
    line.trim() && !line.trimStart().startsWith('#') && !['---', '...'].includes(line.trim()),
  );
  const flowSource = firstContent < 0 ? '' : lines.slice(firstContent).join('\n').trimStart();
  if (flowSource.startsWith('{')) {
    const flowValue = rootFlowValue(flowSource, 'category');
    if (flowValue !== undefined) return categoryValue(flowValue);
  }

  const rootIndent = lines
    .filter((line) => line.trim() && !line.trimStart().startsWith('#') && !['---', '...'].includes(line.trim()))
    .reduce<number | undefined>((indent, line) => {
      const current = line.length - line.trimStart().length;
      return indent === undefined || current < indent ? current : indent;
    }, undefined);

  for (const line of lines) {
    if (!line.trim() || line.trimStart().startsWith('#')) continue;
    if (line.trim() === '---' || line.trim() === '...') continue;
    const indent = line.length - line.trimStart().length;
    if (indent !== rootIndent) continue;
    const match = /^\s*(?:category|'category'|"category")\s*:\s*(.*?)\s*$/.exec(line);
    if (!match) continue;
    return categoryValue(stripComment(match[1]));
  }
  return undefined;
}

function rootFlowValue(source: string, wantedKey: string): string | undefined {
  let depth = 0;
  let quote: "'" | '"' | undefined;
  let escaped = false;
  let segmentStart = 1;
  for (let index = 1; index < source.length; index += 1) {
    const character = source[index];
    if (quote === '"' && character === '\\' && !escaped) {
      escaped = true;
      continue;
    }
    if (quote === "'" && character === "'" && source[index + 1] === "'") {
      index += 1;
      continue;
    }
    if (quote && character === quote && !escaped) quote = undefined;
    else if (!quote && (character === "'" || character === '"')) quote = character;
    else if (!quote && (character === '{' || character === '[')) depth += 1;
    else if (!quote && (character === '}' || character === ']')) {
      if (character === '}' && depth === 0) {
        const found = flowSegmentValue(source.slice(segmentStart, index), wantedKey);
        if (found !== undefined) return found;
        break;
      }
      depth -= 1;
    } else if (!quote && character === ',' && depth === 0) {
      const found = flowSegmentValue(source.slice(segmentStart, index), wantedKey);
      if (found !== undefined) return found;
      segmentStart = index + 1;
    }
    escaped = false;
  }
  return undefined;
}

function flowSegmentValue(segment: string, wantedKey: string): string | undefined {
  const key = wantedKey.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = new RegExp(`^\\s*(?:${key}|'${key}'|"${key}")\\s*:\\s*([\\s\\S]*?)\\s*$`).exec(segment);
  return match?.[1];
}

function categoryValue(raw: string): Connector['category'] | undefined {
  let value = stripComment(raw).trim();
  if (value.startsWith("'") && value.endsWith("'")) value = value.slice(1, -1).replace(/''/g, "'");
  else if (value.startsWith('"') && value.endsWith('"')) value = value.slice(1, -1);
  return categories.find((category) => category === value);
}

function stripComment(value: string): string {
  let quote: "'" | '"' | undefined;
  let escaped = false;
  for (let index = 0; index < value.length; index += 1) {
    const character = value[index];
    if (quote === '"' && character === '\\' && !escaped) {
      escaped = true;
      continue;
    }
    if (quote === "'" && character === "'" && value[index + 1] === "'") {
      index += 1;
      continue;
    }
    if (quote && character === quote && !escaped) quote = undefined;
    else if (!quote && (character === "'" || character === '"')) quote = character;
    else if (!quote && character === '#' && (index === 0 || /\s/.test(value[index - 1]))) return value.slice(0, index);
    escaped = false;
  }
  return value;
}

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
