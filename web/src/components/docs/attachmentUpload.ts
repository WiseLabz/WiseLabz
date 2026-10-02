import { EditorView } from '@codemirror/view';
import { postDocsDocIdAttachments } from '../../api/generated/docs/docs';
import type { DocAttachment } from '../../api/model';

// Refresh both mounted viewers and editor metadata before fifteen-minute URL expiry.
export const attachmentQueryOptions = { staleTime: 5 * 60_000, refetchInterval: 10 * 60_000 };

export function attachmentMarkdown(a: Pick<DocAttachment, 'id' | 'filename' | 'contentType'>) {
  const name = a.filename.replace(/[\\[\]\r\n]/g, ' ');
  return `${a.contentType.startsWith('image/') ? '!' : ''}[${name}](attachment:${a.id})`;
}

export function attachmentUpload(
  docId: string,
  options: {
    enabled: () => boolean;
    complete: () => void;
    error: () => void;
  }
) {
  const upload = (files: File[], view: EditorView) => {
    for (const file of files) {
      const placeholder = `![Uploading… ${crypto.randomUUID()}]()`;
      const selection = view.state.selection.main;
      view.dispatch({ changes: { from: selection.from, to: selection.to, insert: placeholder } });
      const replace = (insert: string) => {
        if (!view.dom.isConnected) return;
        const from = view.state.doc.toString().indexOf(placeholder);
        // Deleted or edited placeholders stay deleted/edited when uploads complete.
        if (from >= 0) view.dispatch({ changes: { from, to: from + placeholder.length, insert } });
      };
      void postDocsDocIdAttachments(docId, { file })
        .then((a) => {
          replace(attachmentMarkdown(a));
          options.complete();
        })
        .catch(() => {
          replace('');
          options.error();
        });
    }
  };
  return EditorView.domEventHandlers({
    drop: (event, view) => {
      const files = Array.from(event.dataTransfer?.files ?? []);
      if (!files.length || !options.enabled()) return false;
      event.preventDefault();
      const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
      if (pos !== null) view.dispatch({ selection: { anchor: pos } });
      upload(files, view);
      return true;
    },
    paste: (event, view) => {
      const files = Array.from(event.clipboardData?.files ?? []);
      if (!files.length || !options.enabled()) return false;
      event.preventDefault();
      upload(files, view);
      return true;
    },
  });
}
