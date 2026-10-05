import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@testing-library/react';

const { initialize, renderDiagram } = vi.hoisted(() => ({
  initialize: vi.fn(),
  renderDiagram: vi.fn(async () => ({ svg: '<svg></svg>' })),
}));
vi.mock('mermaid', () => ({ default: { initialize, render: renderDiagram } }));

import { Mermaid } from './Mermaid';

describe('Mermaid theme colors', () => {
  afterEach(() => vi.restoreAllMocks());

  it('never hands mermaid an oklch() color, which its parser rejects', async () => {
    // Browsers keep oklch() in computed styles; paint-and-read converts it.
    vi.spyOn(window, 'getComputedStyle').mockReturnValue({
      color: 'oklch(0.133 0.006 85)',
      getPropertyValue: () => '',
    } as unknown as CSSStyleDeclaration);
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
      clearRect: () => undefined,
      fillRect: () => undefined,
      getImageData: () => ({ data: [12, 13, 14, 255] }),
    } as unknown as CanvasRenderingContext2D);

    render(<Mermaid chart={'graph LR\n a --> b'} />);
    await waitFor(() => expect(initialize).toHaveBeenCalled());
    const vars = initialize.mock.calls[initialize.mock.calls.length - 1]![0]
      .themeVariables as Record<string, string>;
    const colors = Object.entries(vars).filter(([key]) => key !== 'fontFamily');
    expect(colors.length).toBeGreaterThan(0);
    for (const [, value] of colors) expect(value).toBe('rgb(12, 13, 14)');
  });
});
