import DOMPurify from 'dompurify';
import { renderMermaid } from '../../lib/mermaid';

export async function renderLabBook(
  html: string,
  {
    signal,
    onProgress,
  }: { signal: AbortSignal; onProgress?: (done: number, total: number) => void }
): Promise<string> {
  signal.throwIfAborted();
  const document = new DOMParser().parseFromString(html, 'text/html');
  const blocks = [...document.querySelectorAll('pre > code.language-mermaid')];
  const total = blocks.length;
  onProgress?.(0, total);

  for (const [index, code] of blocks.entries()) {
    signal.throwIfAborted();
    const source = code.textContent ?? '';
    try {
      const svg = await renderMermaid(source, {
        startOnLoad: false,
        theme: 'default',
        securityLevel: 'strict',
        htmlLabels: false,
        flowchart: { htmlLabels: false },
      });
      signal.throwIfAborted();
      const figure = document.createElement('figure');
      figure.className = 'lab-book-diagram';
      figure.innerHTML = sanitizeSvg(svg);
      code.parentElement?.replaceWith(figure);
    } catch (error) {
      if (signal.aborted) throw error;
      // Keep the original pre/code block for the embedded Mermaid fallback.
    }
    onProgress?.(index + 1, total);
    signal.throwIfAborted();
    if (index + 1 < total) {
      await new Promise<void>((resolve) => setTimeout(resolve, 0));
      signal.throwIfAborted();
    }
  }

  signal.throwIfAborted();
  const doctype = document.doctype
    ? `${new XMLSerializer().serializeToString(document.doctype)}\n`
    : '';
  return `${doctype}${document.documentElement.outerHTML}`;
}

function sanitizeSvg(svg: string): string {
  const safeSvg = DOMPurify.sanitize(svg, {
    USE_PROFILES: { svg: true, svgFilters: true },
    FORBID_TAGS: ['script', 'foreignObject'],
  });
  const document = new DOMParser().parseFromString(safeSvg, 'image/svg+xml');
  if (document.documentElement?.localName.toLowerCase() !== 'svg') {
    throw new Error('Mermaid returned invalid SVG');
  }
  for (const element of [...document.querySelectorAll('*')]) {
    if (['script', 'foreignobject'].includes(element.localName.toLowerCase())) {
      element.remove();
      continue;
    }
    for (const attribute of [...element.attributes]) {
      const name = attribute.name.toLowerCase();
      const value = attribute.value.trim();
      if (
        name.startsWith('on') ||
        ((name === 'href' || name === 'xlink:href') && !value.startsWith('#')) ||
        hasExternalUrl(value)
      ) {
        element.removeAttributeNode(attribute);
      }
    }
    if (element.localName.toLowerCase() === 'style' && hasExternalUrl(element.textContent ?? '')) {
      element.remove();
    }
  }
  return document.documentElement?.outerHTML ?? '';
}

function hasExternalUrl(value: string): boolean {
  if (/@import\b/i.test(value)) return true;
  const urls = [...value.matchAll(/url\(([^)]*)\)/gi)];
  if (/url\s*\(/i.test(value) && urls.length === 0) return true;
  return urls.some(([, raw]) => {
    const reference = raw
      .trim()
      .replace(/^(['"])(.*)\1$/, '$2')
      .trim();
    return !reference.startsWith('#');
  });
}
