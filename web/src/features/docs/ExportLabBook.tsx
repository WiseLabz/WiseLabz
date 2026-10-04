import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AXIOS_INSTANCE } from '../../api/axios-instance';
import { Button } from '../../components/ui/Button';
import { downloadBlob, filenameFromContentDisposition } from '../../lib/download';
import { toast } from '../../lib/toast';

export function ExportLabBook() {
  const { t } = useTranslation();
  const [format, setFormat] = useState('html');
  const [pending, setPending] = useState(false);
  const download = async () => {
    setPending(true);
    try {
      const response = await AXIOS_INSTANCE.get('/docs/export', {
        params: { format },
        responseType: 'blob',
      });
      downloadBlob(
        response.data,
        filenameFromContentDisposition(
          response.headers['content-disposition'],
          `lab-book.${format}`
        )
      );
    } catch {
      toast.error(t('docs.export.error'));
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="flex gap-2">
      <select
        aria-label={t('docs.export.format')}
        value={format}
        disabled={pending}
        onChange={(event) => setFormat(event.target.value)}
        className="rounded-sm border border-line bg-surface px-2 text-sm text-ink"
      >
        <option value="html">{t('docs.export.html')}</option>
        <option value="md.zip">{t('docs.export.markdown')}</option>
      </select>
      <Button size="sm" variant="secondary" disabled={pending} onClick={() => void download()}>
        {t(pending ? 'docs.export.pending' : 'docs.export.action')}
      </Button>
    </div>
  );
}
