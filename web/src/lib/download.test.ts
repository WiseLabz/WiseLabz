import { afterEach, describe, expect, it, vi } from 'vitest';
import { filenameFromContentDisposition, downloadBlob } from './download';

describe('filenameFromContentDisposition', () => {
  it('extracts filename from standard Content-Disposition header', () => {
    const header = 'attachment; filename="export.json"';
    expect(filenameFromContentDisposition(header, 'default.txt')).toBe('export.json');
  });

  it('extracts filename without quotes', () => {
    const header = 'attachment; filename=backup.zip';
    expect(filenameFromContentDisposition(header, 'default.txt')).toBe('backup.zip');
  });

  it('handles filenames with special characters', () => {
    const header = 'attachment; filename="audit-2026-10-01.csv"';
    expect(filenameFromContentDisposition(header, 'default.txt')).toBe('audit-2026-10-01.csv');
  });

  it('uses fallback for undefined header', () => {
    expect(filenameFromContentDisposition(undefined, 'fallback.dat')).toBe('fallback.dat');
  });

  it('uses fallback when no filename in header', () => {
    const header = 'attachment; something=else';
    expect(filenameFromContentDisposition(header, 'fallback.dat')).toBe('fallback.dat');
  });

  it('uses fallback for empty header', () => {
    expect(filenameFromContentDisposition('', 'fallback.dat')).toBe('fallback.dat');
  });
});

describe('downloadBlob', () => {
  afterEach(() => {
    vi.clearAllMocks();
    document.body.replaceChildren();
  });

  it('creates and triggers download of a blob', () => {
    const blob = new Blob(['test content'], { type: 'text/plain' });
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:url');
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});

    const clickSpy = vi.fn();
    vi.spyOn(document.body, 'appendChild').mockImplementation((el: Node) => {
      if (el instanceof HTMLAnchorElement) {
        el.click = clickSpy;
      }
      return el;
    });

    downloadBlob(blob, 'test.txt');

    expect(clickSpy).toHaveBeenCalled();
  });

  it('sets correct href and download attributes', () => {
    const blob = new Blob(['content']);
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock-url');
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});

    let capturedAnchor: HTMLAnchorElement | null = null;
    vi.spyOn(document.body, 'appendChild').mockImplementation((el: Node) => {
      if (el instanceof HTMLAnchorElement) {
        capturedAnchor = el;
        el.click = vi.fn();
      }
      return el;
    });

    downloadBlob(blob, 'download.csv');

    expect(capturedAnchor).not.toBeNull();
    expect(capturedAnchor!.href).toBe('blob:mock-url');
    expect(capturedAnchor!.download).toBe('download.csv');
  });

  it('removes the anchor element from DOM after download', () => {
    const blob = new Blob(['test']);
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:url');
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});

    downloadBlob(blob, 'file.txt');

    expect(document.body.querySelector('a')).toBeNull();
  });
});
