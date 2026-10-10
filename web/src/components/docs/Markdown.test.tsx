import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Markdown } from './Markdown';

describe('Markdown', () => {
  // jsdom has no layout engine, so SVGElement.getBBox() (used by mermaid to
  // measure node/edge label text) doesn't exist. Real browsers implement it;
  // this stub just lets a diagram finish laying out in a jsdom test.
  beforeAll(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- jsdom's SVGElement has no getBBox at all
    (SVGElement.prototype as any).getBBox = () => ({ x: 0, y: 0, width: 0, height: 0 });
  });

  afterEach(cleanup);

  it('renders a normal fenced code block as <pre><code>', () => {
    render(<Markdown source={'```json\n{"a":1}\n```'} />);
    expect(screen.getByText('{"a":1}').closest('pre')).toBeInTheDocument();
  });

  it('renders a ```mermaid fenced block via the Mermaid component instead of a plain code block', async () => {
    const { container } = render(<Markdown source={'```mermaid\ngraph LR\n  a --> b\n```'} />);

    // The Mermaid component owns its own container div and never falls back
    // to a <pre> wrapper for a mermaid block.
    expect(container.querySelector('pre')).not.toBeInTheDocument();

    // Mermaid loads its renderer lazily; a cold import plus SVG layout can
    // exceed waitFor's one-second default under coverage.
    await waitFor(
      () => {
        expect(container.querySelector('svg')).toBeInTheDocument();
      },
      { timeout: 4000 }
    );
  });

  it('hides sync ownership markers but keeps the generated content', () => {
    const source = [
      '<!-- wl:gen key="head" h="3fa9c0d1e2b4" -->',
      '# Node one',
      '<!-- /wl:gen -->',
      '',
      'Human notes',
      '<!-- wl:gen key="snap.status" h="aaaaaaaaaaaa" -->',
      'healthy',
      '<!-- /wl:gen -->',
      '',
    ].join('\n');
    const { container } = render(<Markdown source={source} />);
    expect(screen.getByRole('heading', { name: 'Node one' })).toBeInTheDocument();
    expect(screen.getByText('healthy')).toBeInTheDocument();
    expect(screen.getByText('Human notes')).toBeInTheDocument();
    expect(container.textContent).not.toContain('wl:gen');
  });

  it('uses router navigation for internal doc and entity links', () => {
    render(
      <MemoryRouter>
        <Markdown source={'[Doc](/docs/doc-1) [Entity](/entities/entity-1)'} />
      </MemoryRouter>
    );
    expect(screen.getByRole('link', { name: 'Doc' }).getAttribute('href')).toBe('/docs/doc-1');
    expect(screen.getByRole('link', { name: 'Entity' }).getAttribute('href')).toBe(
      '/entities/entity-1'
    );
  });

  it('keeps internal links as ordinary anchors in shared documentation', () => {
    render(<Markdown source="[Doc](/docs/doc-1)" internalLinks={false} />);
    expect(screen.getByRole('link', { name: 'Doc' }).tagName).toBe('A');
  });
});
