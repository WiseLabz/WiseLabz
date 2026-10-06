import { useParams, Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { RunDetail } from './RunDetail';

export function RunPage() {
  const { runId = '' } = useParams();
  const { t } = useTranslation();
  return (
    <div className="mx-auto max-w-3xl space-y-4 p-4">
      <h1 className="font-mono text-xl text-ink">{t('runbooks.runs.detailTitle')}</h1>
      <RunDetail key={runId} runId={runId} showTitle={false} />
      <Link to="/" className="inline-block font-mono text-xs text-ink-muted hover:text-ink">
        {t('runbooks.runs.backToDashboard')}
      </Link>
    </div>
  );
}
