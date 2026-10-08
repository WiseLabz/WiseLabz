/**
 * Add-connector flow — a dedicated surface (not a row action), per ARCHITECTURE.md.
 * Thin page chrome around the shared <ConnectorForm/>; on success it returns to the
 * services list. The form itself is reused verbatim by the onboarding stepper.
 *
 * Instance admins also get a "Scan network" action that swaps the form for the
 * network scan; when its connect queue ends the page returns to the services list too.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ConnectorForm } from './ConnectorForm';
import { ArrowRightIcon, NetworkIcon } from '../../components/icons';
import { Button } from '../../components/ui/Button';
import { useIsInstanceAdmin } from '../../hooks/useRole';
import { DiscoveryFlow } from '../discovery/DiscoveryFlow';

export function AddConnectorPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isInstanceAdmin = useIsInstanceAdmin();
  const [scanning, setScanning] = useState(false);

  return (
    <div className="mx-auto max-w-170 px-6 py-6">
      <button
        onClick={() => navigate('/services')}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('connectors.back')}
      </button>

      <header className="mb-5 flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-ink">{t('connectors.title')}</h1>
          <p className="text-sm text-ink-muted">{t('connectors.subtitle')}</p>
        </div>
        {isInstanceAdmin && (
          <Button variant="secondary" size="md" onClick={() => setScanning((s) => !s)} aria-pressed={scanning}>
            {scanning ? (
              t('connectors.backToForm')
            ) : (
              <>
                <NetworkIcon size={15} />
                {t('connectors.scanNetwork')}
              </>
            )}
          </Button>
        )}
      </header>

      {isInstanceAdmin && scanning ? (
        <DiscoveryFlow onQueueFinished={() => navigate('/services')} />
      ) : (
        <ConnectorForm onCreated={() => navigate('/services')} onCancel={() => navigate('/services')} />
      )}
    </div>
  );
}
