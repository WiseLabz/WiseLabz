import type { SchemaField } from '../../api/model';

/**
 * The backend schema emits `kind: "toggle"` and the key `verify_tls`
 * (backend/internal/connector/registry.go), while the generated client types
 * still name the kind `boolean` and the request field `verifyTls`. These
 * helpers accept both so the forms follow what the server really sends.
 */

/** True for switch-style fields. */
export function isToggleField(f: SchemaField): boolean {
  return (f.kind as string) === 'toggle' || f.kind === 'boolean';
}

/** True for fields holding a credential: masked and write-only. */
export function isSecretField(f: SchemaField): boolean {
  return f.kind === 'password' || f.kind === 'secret' || !!f.secret;
}

/** True for the field that maps to the top-level `verifyTls`, not to `config`. */
export function isVerifyTlsField(f: SchemaField): boolean {
  return f.name === 'verify_tls' || f.name === 'verifyTls';
}

/** True for fields sent at the top level of the request, not inside `config`. */
export function isTopLevelField(f: SchemaField): boolean {
  return f.name === 'url' || isVerifyTlsField(f);
}

/** Initial value of a field on the create form. */
export function fieldDefault(f: SchemaField): string | boolean {
  const d = (f as { default?: string }).default;
  if (isToggleField(f)) return d === undefined ? false : d !== 'false';
  return d ?? '';
}
