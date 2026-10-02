import { FormData as NodeFormData } from 'undici';
import { File as NodeFile } from 'node:buffer';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { waitFor } from '@testing-library/react';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { AXIOS_INSTANCE } from '../../api/axios-instance';
import { attachmentUpload } from './attachmentUpload';

const server = setupServer();
let view: EditorView;
beforeAll(() => {
  vi.stubGlobal('FormData', NodeFormData);
  AXIOS_INSTANCE.defaults.adapter = 'fetch';
  AXIOS_INSTANCE.defaults.baseURL = 'http://localhost/api';
  server.listen({ onUnhandledRequest: 'error' });
});
afterEach(() => {
  view?.destroy();
  document.body.replaceChildren();
  server.resetHandlers();
});
afterAll(() => server.close());

function editor(enabled = true) {
  const complete = vi.fn();
  const error = vi.fn();
  const host = document.createElement('div');
  document.body.append(host);
  view = new EditorView({
    parent: host,
    state: EditorState.create({
      extensions: [attachmentUpload('doc-id', { enabled: () => enabled, complete, error })],
    }),
  });
  return { complete, error };
}

function transfer(kind: 'drop' | 'paste') {
  const file = new NodeFile(['image data'], 'photo.png', { type: 'image/png' });
  const event = new Event(kind, { bubbles: true, cancelable: true });
  Object.defineProperty(event, kind === 'drop' ? 'dataTransfer' : 'clipboardData', {
    value: { files: [file], getData: () => '' },
  });
  // jsdom has no layout; a null position uses the current selection.
  vi.spyOn(view, 'posAtCoords').mockReturnValue(null);
  view.contentDOM.dispatchEvent(event);
  return event;
}

describe('attachment drop/paste upload', () => {
  for (const kind of ['drop', 'paste'] as const)
    it(`replaces a ${kind} placeholder after multipart upload`, async () => {
      let release!: () => void;
      const gate = new Promise<void>((resolve) => {
        release = resolve;
      });
      server.use(
        http.post('*/docs/doc-id/attachments', async ({ request }) => {
          const form = await request.formData();
          expect((form.get('file') as File).name).toBe('photo.png');
          await gate;
          return HttpResponse.json(
            { id: 'attachment-id', filename: 'photo.png', contentType: 'image/png' },
            { status: 201 }
          );
        })
      );
      const { complete } = editor();
      const event = transfer(kind);
      expect(event.defaultPrevented).toBe(true);
      expect(view.state.doc.toString()).toContain('![Uploading…');
      // Edits before the placeholder must not make replacement use a stale offset.
      view.dispatch({ changes: { from: 0, insert: 'before ' } });
      release();
      await waitFor(() => expect(complete).toHaveBeenCalledOnce());
      expect(view.state.doc.toString()).toBe('before ![photo.png](attachment:attachment-id)');
    });

  it('preserves deletion of an in-flight placeholder', async () => {
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.post('*/docs/doc-id/attachments', async () => {
        await gate;
        return HttpResponse.json({ id: 'a', filename: 'photo.png', contentType: 'image/png' });
      })
    );
    const { complete } = editor();
    transfer('paste');
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: 'my text' } });
    release();
    await waitFor(() => expect(complete).toHaveBeenCalledOnce());
    expect(view.state.doc.toString()).toBe('my text');
  });

  it('removes a failed upload placeholder and reports the failure', async () => {
    server.use(
      http.post('*/docs/doc-id/attachments', () => HttpResponse.json({}, { status: 415 }))
    );
    const { error } = editor();
    transfer('paste');
    await waitFor(() => expect(error).toHaveBeenCalledOnce());
    expect(view.state.doc.toString()).toBe('');
  });

  it('does not upload when editing is denied', () => {
    editor(false);
    transfer('paste');
    expect(view.state.doc.toString()).toBe('');
  });
});
