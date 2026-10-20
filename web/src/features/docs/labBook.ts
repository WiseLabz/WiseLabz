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
        // Source directives must not override the export's appearance or HTML-label policy.
        secure: [
          'secure',
          'securityLevel',
          'startOnLoad',
          'maxTextSize',
          'suppressErrorRendering',
          'maxEdges',
          'theme',
          'themeVariables',
          'themeCSS',
          'htmlLabels',
        ],
      });
      signal.throwIfAborted();
      const figure = document.createElement('figure');
      figure.className = 'lab-book-diagram';
      figure.append(document.importNode(sanitizeSvg(svg), true));
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

function sanitizeSvg(svg: string): DocumentFragment {
  const fragment = DOMPurify.sanitize(svg, {
    USE_PROFILES: { svg: true, svgFilters: true },
    FORBID_TAGS: ['script', 'foreignObject'],
    RETURN_DOM_FRAGMENT: true,
  });
  if (fragment.firstElementChild?.localName.toLowerCase() !== 'svg') {
    throw new Error('Mermaid returned invalid SVG');
  }
  for (const element of [...fragment.querySelectorAll('*')]) {
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
  return fragment;
}

function decodeCssEscapes(value: string): string {
  return value
    .replace(/\\([0-9a-fA-F]{1,6})\s?/g, (_, hex: string) => {
      const codePoint = Number.parseInt(hex, 16);
      return codePoint > 0 && codePoint <= 0x10ffff ? String.fromCodePoint(codePoint) : '\ufffd';
    })
    .replace(/\\([\s\S])/g, '$1');
}

function hasExternalUrl(raw: string): boolean {
  const value = decodeCssEscapes(raw);
  if (/@import\b/i.test(value)) return true;
  if (/\b(?:image-set|-webkit-image-set|src)\s*\(/i.test(value)) return true;
  const urls = [...value.matchAll(/url\(([^)]*)\)/gi)];
  if (/url\s*\(/i.test(value) && urls.length === 0) return true;
  return urls.some(([, reference]) => {
    const target = reference
      .trim()
      .replace(/^(['"])(.*)\1$/, '$2')
      .trim();
    return !target.startsWith('#');
  });
}
