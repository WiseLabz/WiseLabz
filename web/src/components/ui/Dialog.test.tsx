import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Dialog } from './Dialog';

describe('Dialog', () => {
  it('is already open when it is first inserted into the DOM (#622)', async () => {
    const { rerender } = render(
      <Dialog open={false} onClose={() => {}}>
        body
      </Dialog>
    );
    let openOnInsert: boolean | undefined;
    const observer = new MutationObserver(() => {
      const dlg = document.querySelector('dialog');
      if (dlg && openOnInsert === undefined) openOnInsert = dlg.open;
    });
    observer.observe(document.body, { childList: true, subtree: true });
    // Like the app: the rAF-driven mount happens outside act(), so React
    // commits and flushes passive effects in separate scheduler tasks.
    const g = globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean };
    g.IS_REACT_ACT_ENVIRONMENT = false;
    try {
      rerender(
        <Dialog open onClose={() => {}}>
          body
        </Dialog>
      );
      await waitFor(() => expect(document.querySelector('dialog')).not.toBeNull());
      await new Promise((resolve) => setTimeout(resolve, 20));
    } finally {
      g.IS_REACT_ACT_ENVIRONMENT = true;
      observer.disconnect();
    }
    expect(openOnInsert).toBe(true);
    expect(screen.getByText('body')).toBeVisible();
  });
});
