/** Shared blob-download helpers for file exports (audit log, backups, …). */

/** Reads a filename from a Content-Disposition header, falling back to a default. */
export function filenameFromContentDisposition(
  header: string | undefined,
  fallback: string
): string {
  const match = header?.match(/filename="?([^"；;]+)"?/);
  return match?.[1] ?? fallback;
}

/** Triggers a browser download of a blob via a throwaway anchor element. */
export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
