/** Copy on HTTPS or fall back to selection-based copying for HTTP deployments. */
export async function copyText(text: string): Promise<void> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return;
    }
  } catch {
    // Permissions may deny the modern API even when it is available.
  }

  const focused = document.activeElement;
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.readOnly = true;
  textarea.tabIndex = -1;
  textarea.setAttribute('aria-hidden', 'true');
  textarea.style.cssText = 'position:fixed;opacity:0;pointer-events:none';
  // A modal dialog makes nodes outside it inert, so copy inside the open dialog.
  const container = focused?.closest('dialog[open]') ?? document.querySelector('dialog[open]') ?? document.body;
  container.appendChild(textarea);
  try {
    textarea.select();
    if (!document.execCommand('copy')) throw new Error('Clipboard copy failed');
  } finally {
    textarea.remove();
    if (focused instanceof HTMLElement) focused.focus();
  }
}
