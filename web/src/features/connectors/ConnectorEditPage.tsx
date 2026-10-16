/** Edit an existing connector (`/connectors/:id/edit`). */
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useGetConnectorsConnectorId } from '../../api/generated/connectors/connectors';
import { Panel } from '../../components/ui/Panel';
import { ErrorState, SkeletonRows } from '../../components/ui/states';
import { ArrowRightIcon } from '../../components/icons';
import { useConnectorRole } from '../../hooks/useRole';
import { ConnectorEditForm } from './ConnectorEditForm';
import { ConnectorPermissionsTab } from './ConnectorPermissionsTab';

export function ConnectorEditPage() {
  const { t } = useTranslation();
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const connector = useGetConnectorsConnectorId(id);
  const canEdit = useConnectorRole(id) === 'operator';

  if (connector.isLoading) {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="p-6">
          <SkeletonRows rows={5} />
        </Panel>
      </div>
    );
  }
  if (connector.isError || !connector.data) {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="min-h-[30vh]">
          <ErrorState description={t('services.detail.notFound')} onRetry={() => connector.refetch()} />
        </Panel>
      </div>
    );
  }

  const c = connector.data;
  if (c.managedBy === 'config' || c.managedBy === 'config-orphaned') {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="min-h-[30vh]">
          <ErrorState
            title={t('connectors.managed.editTitle')}
            description={t(
              c.managedBy === 'config' ? 'connectors.managed.editDesc' : 'connectors.managed.orphanedEditDesc',
            )}
          />
        </Panel>
      </div>
    );
  }
  if (!canEdit) {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="min-h-[30vh]">
          <ErrorState title={t('connectors.edit.noAccessTitle')} description={t('connectors.edit.noAccessDesc')} />
        </Panel>
      </div>
    );
  }

  const navigateToService = () => navigate(`/services/${id}`);
  return (
    <div className="mx-auto max-w-170 px-6 py-6">
      <button
        onClick={navigateToService}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('connectors.edit.back')}
      </button>

      <header className="mb-5">
        <h1 className="text-xl font-semibold tracking-tight text-ink">{t('connectors.edit.title', { name: c.name })}</h1>
        <p className="text-sm text-ink-muted">{t('connectors.edit.subtitle')}</p>
      </header>

      <ConnectorEditForm key={id} connectorId={id} onSaved={navigateToService} onCancel={navigateToService} />
      <ConnectorPermissionsTab connectorId={id} />
    </div>
  );
}
