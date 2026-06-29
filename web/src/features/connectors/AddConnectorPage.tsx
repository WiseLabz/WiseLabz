/**
 * Add-connector flow — a dedicated surface (not a row action), per ARCHITECTURE.md.
 * Thin page chrome around the shared <ConnectorForm/>; on success it returns to the
 * services list. The form itself is reused verbatim by the onboarding stepper.
 */
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ConnectorForm } from './ConnectorForm';
import { ArrowRightIcon } from '../../components/icons';

export function AddConnectorPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  return (
    <div className="mx-auto max-w-170 px-6 py-6">
      <button
        onClick={() => navigate('/services')}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('connectors.back')}
      </button>

      <header className="mb-5">
        <h1 className="text-xl font-semibold tracking-tight text-ink">{t('connectors.title')}</h1>
        <p className="text-sm text-ink-muted">{t('connectors.subtitle')}</p>
      </header>

      <ConnectorForm onCreated={() => navigate('/services')} onCancel={() => navigate('/services')} />
    </div>
  );
}
