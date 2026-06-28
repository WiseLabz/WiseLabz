/** Tiny className joiner — drops falsy values, joins with spaces. */
export type ClassValue = string | number | false | null | undefined;

export function cn(...parts: ClassValue[]): string {
  let out = '';
  for (const p of parts) {
    if (!p && p !== 0) continue;
    out += (out ? ' ' : '') + p;
  }
  return out;
}
