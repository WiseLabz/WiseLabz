export function formatRunbookValue(value: unknown, jsonEncoded = false): string {
  let decoded = value;
  if (jsonEncoded && typeof decoded === 'string') {
    try {
      decoded = JSON.parse(decoded) as unknown;
    } catch {
      // Keep older or malformed stored values visible as-is.
    }
  }
  if (decoded === undefined || decoded === null || decoded === '') return '—';
  return typeof decoded === 'string' ? decoded : String(decoded);
}
