import type { MermaidConfig } from 'mermaid';

let queue: Promise<unknown> = Promise.resolve();
let initializedConfig: string | undefined;
let nextRenderId = 0;

/** Render Mermaid diagrams one at a time because initialize() changes global state. */
export function renderMermaid(source: string, config: MermaidConfig = {}): Promise<string> {
  const render = queue.then(async () => {
    const { default: mermaid } = await import('mermaid');
    const configKey = JSON.stringify(config);
    if (configKey !== initializedConfig) {
      mermaid.initialize(config);
      initializedConfig = configKey;
    }

    const id = `mermaid-render-${++nextRenderId}`;
    try {
      return (await mermaid.render(id, source)).svg;
    } finally {
      // Mermaid can leave its temporary render container behind on errors.
      document.getElementById(id)?.remove();
      document.getElementById(`d${id}`)?.remove();
    }
  });
  queue = render.catch(() => undefined);
  return render;
}
