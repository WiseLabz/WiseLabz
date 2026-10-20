import { afterEach, describe, expect, it, vi } from 'vitest';

const mermaidState = vi.hoisted(() => ({
  config: undefined as unknown,
  initialize: vi.fn(),
  render: vi.fn(),
}));
vi.mock('mermaid', () => ({
  default: {
    initialize: (config: unknown) => {
      mermaidState.config = config;
      mermaidState.initialize(config);
    },
    render: (...args: unknown[]) => mermaidState.render(...args),
  },
}));

import { renderMermaid } from './mermaid';

describe('renderMermaid', () => {
  afterEach(() => vi.clearAllMocks());

  it('serializes interleaved renders under each requested config', async () => {
    mermaidState.render.mockImplementation(async (id: string, source: string) => {
      const artifact = document.createElement('div');
      artifact.id = `d${id}`;
      document.body.append(artifact);
      await new Promise((resolve) => setTimeout(resolve, 0));
      return {
        svg: `<svg data-source="${source}" data-theme="${(mermaidState.config as { theme: string }).theme}" id="${id}"></svg>`,
      };
    });
    const light = { theme: 'default', securityLevel: 'strict' } as const;
    const dark = { theme: 'base', themeVariables: { background: '#111' } } as const;

    const [first, second, third, fourth] = await Promise.all([
      renderMermaid('first', light),
      renderMermaid('second', dark),
      renderMermaid('third', light),
      renderMermaid('fourth', light),
    ]);

    expect(first).toContain('data-theme="default"');
    expect(second).toContain('data-theme="base"');
    expect(third).toContain('data-theme="default"');
    expect(fourth).toContain('data-theme="default"');
    expect(mermaidState.initialize.mock.calls.map(([config]) => config)).toEqual([
      light,
      dark,
      light,
    ]);
    const ids = mermaidState.render.mock.calls.map(([id]) => id);
    expect(new Set(ids).size).toBe(4);
    expect(ids.every((id) => String(id).startsWith('mermaid-render-'))).toBe(true);
    expect(document.querySelector('[id^="dmermaid-render-"]')).toBeNull();
  });
});
