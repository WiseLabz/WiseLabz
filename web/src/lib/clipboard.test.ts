import { afterEach, describe, expect, it, vi } from 'vitest';
import { copyText } from './clipboard';

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  document.body.replaceChildren();
});

describe('copyText', () => {
  it('uses the modern API when available', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });
    await copyText('token');
    expect(writeText).toHaveBeenCalledWith('token');
    expect(document.querySelector('textarea')).toBeNull();
  });

  it.each([false, true])('falls back when clipboard is missing or denied (denied=%s)', async (denied) => {
    vi.stubGlobal('navigator', denied ? { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('Denied')) } } : {});
    const dialog = document.createElement('dialog');
    dialog.open = true;
    const button = document.createElement('button');
    dialog.appendChild(button);
    document.body.appendChild(dialog);
    button.focus();
    const execCommand = vi.fn(() => {
      const textarea = dialog.querySelector('textarea')!;
      expect(textarea.value).toBe('one\ntwo');
      expect(textarea.selectionStart).toBe(0);
      expect(textarea.selectionEnd).toBe(7);
      return true;
    });
    Object.defineProperty(document, 'execCommand', { value: execCommand, configurable: true });
    await copyText('one\ntwo');
    expect(execCommand).toHaveBeenCalledWith('copy');
    expect(dialog.querySelector('textarea')).toBeNull();
    expect(document.activeElement).toBe(button);
  });

  it.each(['false', 'throw', 'missing'])('rejects and removes temporary text when fallback fails (%s)', async (failure) => {
    vi.stubGlobal('navigator', {});
    const execCommand = failure === 'missing' ? undefined : vi.fn(() => {
      if (failure === 'throw') throw new Error('Denied');
      return false;
    });
    Object.defineProperty(document, 'execCommand', { value: execCommand, configurable: true });
    await expect(copyText('secret')).rejects.toThrow();
    expect(document.querySelector('textarea')).toBeNull();
  });
});
