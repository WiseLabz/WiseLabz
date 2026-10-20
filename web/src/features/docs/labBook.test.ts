import { afterEach, describe, expect, it, vi } from 'vitest';

const { renderMermaid } = vi.hoisted(() => ({ renderMermaid: vi.fn() }));
vi.mock('../../lib/mermaid', () => ({ renderMermaid }));

import { renderLabBook } from './labBook';

afterEach(() => vi.clearAllMocks());

describe('renderLabBook', () => {
  it('replaces Mermaid blocks, reports progress, and preserves doctype, CSP, and script', async () => {
    renderMermaid.mockResolvedValue(
      '<svg xmlns="http://www.w3.org/2000/svg"><text>diagram</text></svg>'
    );
    const progress = vi.fn();
    const html =
      '<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src \'none\'"></head><body><pre><code class="language-mermaid">graph LR\na --&gt; b</code></pre><script id="first">if (a < b) boot()</script><script id="mermaid-bootstrap">boot()</script></body></html>';

    const result = await renderLabBook(html, {
      signal: new AbortController().signal,
      onProgress: progress,
    });

    expect(result).toMatch(/^<!DOCTYPE html>/i);
    expect(result).toContain('http-equiv="Content-Security-Policy"');
    expect(result).toContain('<script id="first">if (a < b) boot()</script>');
    expect(result).toContain('<script id="mermaid-bootstrap">boot()</script>');
    expect(result).toContain('<figure class="lab-book-diagram"><svg');
    expect(result).not.toContain('language-mermaid');
    expect(renderMermaid).toHaveBeenCalledWith('graph LR\na --> b', {
      startOnLoad: false,
      theme: 'default',
      securityLevel: 'strict',
      htmlLabels: false,
      flowchart: { htmlLabels: false },
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
    expect(progress.mock.calls).toEqual([
      [0, 1],
      [1, 1],
    ]);
  });

  it('strips scripts, handlers, foreign objects, and external href references', async () => {
    renderMermaid.mockResolvedValue(
      '<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(1)</script><foreignObject><div>unsafe</div></foreignObject><a href="https://evil.test/x" xlink:href="//evil.test/y" xmlns:xlink="http://www.w3.org/1999/xlink" onclick="alert(1)"><text>safe</text></a><image href="#local"/><text filter="url( \'#local\' )" fill="url(https://evil.test/paint)">ok</text><style>.safe { filter: url( \'#local\' ) }</style></svg>'
    );
    const result = await renderLabBook(
      '<html><body><pre><code class="language-mermaid">x</code></pre></body></html>',
      { signal: new AbortController().signal }
    );

    expect(result).not.toMatch(
      /<script|foreignObject|onload|onclick|https:\/\/evil|xlink:href|href="\/\//i
    );
    expect(result).toContain('href="#local"');
    expect(result).toContain('filter="url( \'#local\' )"');
    expect(result).toContain("<style>.safe { filter: url( '#local' ) }</style>");
    expect(result).toContain('>ok</text>');
  });

  it('prevents source directives from changing the light theme or enabling HTML labels', async () => {
    renderMermaid.mockImplementationOnce(async (source, config) => {
      const { default: mermaid } = await import('mermaid');
      mermaid.initialize(config);
      await mermaid.parse(source);
      const applied = mermaid.mermaidAPI.getConfig();
      expect(applied.theme).toBe('default');
      expect(applied.htmlLabels).toBe(false);
      expect(applied.flowchart?.htmlLabels).toBe(false);
      expect(applied.themeVariables?.background).not.toBe('#000');
      return '<svg xmlns="http://www.w3.org/2000/svg"><text>safe</text></svg>';
    });
    const source =
      '%%{init: {"theme":"dark","themeVariables":{"background":"#000"},"htmlLabels":true,"flowchart":{"htmlLabels":true}}}%%\ngraph LR\nA-->B';
    const result = await renderLabBook(
      `<pre><code class="language-mermaid">${source}</code></pre>`,
      { signal: new AbortController().signal }
    );
    expect(result).toContain('lab-book-diagram');
    expect(renderMermaid).toHaveBeenCalledOnce();
  });

  it('removes CSS imports that do not use url()', async () => {
    renderMermaid.mockResolvedValue(
      '<svg xmlns="http://www.w3.org/2000/svg"><style>@import "https://evil.test/style.css";</style><text>safe</text></svg>'
    );
    const result = await renderLabBook('<pre><code class="language-mermaid">x</code></pre>', {
      signal: new AbortController().signal,
    });
    expect(result).not.toContain('@import');
    expect(result).not.toContain('evil.test');
    expect(result).toContain('<text>safe</text>');
  });

  it('keeps source for a diagram that fails and continues with later diagrams', async () => {
    renderMermaid
      .mockRejectedValueOnce(new Error('invalid diagram'))
      .mockResolvedValueOnce('<svg><text>ok</text></svg>');
    const progress = vi.fn();
    const result = await renderLabBook(
      '<html><body><pre><code class="language-mermaid">bad</code></pre><pre><code class="language-mermaid">good</code></pre></body></html>',
      { signal: new AbortController().signal, onProgress: progress }
    );

    expect(result).toContain('<pre><code class="language-mermaid">bad</code></pre>');
    expect(result).toContain(
      '<figure class="lab-book-diagram"><svg><text>ok</text></svg></figure>'
    );
    expect(progress.mock.calls).toEqual([
      [0, 2],
      [1, 2],
      [2, 2],
    ]);
  });

  it('honours cancellation before work and after an in-flight render', async () => {
    const alreadyAborted = new AbortController();
    alreadyAborted.abort();
    await expect(
      renderLabBook('<html></html>', { signal: alreadyAborted.signal })
    ).rejects.toMatchObject({ name: 'AbortError' });
    expect(renderMermaid).not.toHaveBeenCalled();

    let finish!: (value: string) => void;
    renderMermaid.mockReturnValueOnce(
      new Promise<string>((resolve) => {
        finish = resolve;
      })
    );
    const controller = new AbortController();
    const pending = renderLabBook(
      '<html><body><pre><code class="language-mermaid">diagram</code></pre></body></html>',
      { signal: controller.signal }
    );
    await vi.waitFor(() => expect(renderMermaid).toHaveBeenCalled());
    controller.abort();
    finish('<svg></svg>');
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  });

  it('keeps Mermaid source when sanitized output has no SVG root', async () => {
    renderMermaid.mockResolvedValue('<script>no svg</script>');
    const result = await renderLabBook(
      '<html><body><pre><code class="language-mermaid">source</code></pre></body></html>',
      { signal: new AbortController().signal }
    );
    expect(result).toContain('<pre><code class="language-mermaid">source</code></pre>');
    expect(result).not.toContain('<figure class="lab-book-diagram">');
  });
});
