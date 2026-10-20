import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AXIOS_INSTANCE } from '../../api/axios-instance';
import { Button } from '../../components/ui/Button';
import { downloadBlob, filenameFromContentDisposition } from '../../lib/download';
import { toast } from '../../lib/toast';
import { renderLabBook } from './labBook';

function printBook(html: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const frame = document.createElement('iframe');
    frame.setAttribute('sandbox', 'allow-same-origin allow-modals');
    frame.setAttribute('aria-hidden', 'true');
    frame.style.cssText = 'position:fixed;left:-10000px;top:0;width:0;height:0;border:0';
    const remove = () => {
      window.clearTimeout(timeout);
      frame.remove();
    };
    const timeout = window.setTimeout(() => {
      remove();
      reject(new Error('Print frame timed out'));
    }, 60_000);
    frame.onload = () => {
      try {
        const target = frame.contentWindow;
        if (!target) throw new Error('Print frame unavailable');
        target.addEventListener('afterprint', remove, { once: true });
        target.print();
        resolve();
      } catch (error) {
        remove();
        reject(error);
      }
    };
    frame.srcdoc = html;
    document.body.append(frame);
  });
}

export function ExportLabBook() {
  const { t } = useTranslation();
  const [format, setFormat] = useState('html');
  const [pending, setPending] = useState(false);
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);

  const cancel = () => {
    request.current?.abort();
    request.current = null;
    setPending(false);
    setProgress(null);
  };
  const download = async () => {
    const controller = new AbortController();
    request.current = controller;
    setPending(true);
    setProgress(null);
    const serverFormat = format === 'pdf' ? 'html' : format;
    try {
      const response = await AXIOS_INSTANCE.get('/docs/export', {
        params: { format: serverFormat },
        responseType: 'blob',
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;
      let data = response.data as Blob;
      if (serverFormat === 'html') {
        const original = await data.text();
        let html = original;
        try {
          html = await renderLabBook(original, {
            signal: controller.signal,
            onProgress: (done, total) => {
              if (!controller.signal.aborted) setProgress({ done, total });
            },
          });
        } catch {
          // Keep the server book available when pre-rendering fails.
        }
        if (controller.signal.aborted) return;
        if (format === 'pdf') {
          await printBook(html);
          return;
        }
        data = new Blob([html], { type: 'text/html;charset=utf-8' });
      }
      if (controller.signal.aborted) return;
      downloadBlob(
        data,
        filenameFromContentDisposition(
          response.headers['content-disposition'],
          `lab-book.${serverFormat}`
        )
      );
    } catch {
      if (!controller.signal.aborted) toast.error(t('docs.export.error'));
    } finally {
      if (request.current === controller) {
        request.current = null;
        setPending(false);
        setProgress(null);
      }
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2">
      <select
        aria-label={t('docs.export.format')}
        value={format}
        disabled={pending}
        onChange={(event) => setFormat(event.target.value)}
        className="rounded-sm border border-line bg-surface px-2 text-sm text-ink"
      >
        <option value="html">{t('docs.export.html')}</option>
        <option value="md.zip">{t('docs.export.markdown')}</option>
        <option value="pdf">{t('docs.export.pdf')}</option>
      </select>
      <Button size="sm" variant="secondary" disabled={pending} onClick={() => void download()}>
        {t(pending ? 'docs.export.pending' : 'docs.export.action')}
      </Button>
      {pending && (
        <>
          {progress && <span role="status">{t('docs.export.progress', progress)}</span>}
          <Button size="sm" variant="secondary" onClick={cancel}>
            {t('docs.export.cancel')}
          </Button>
        </>
      )}
    </div>
  );
}
