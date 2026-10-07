/**
 * Entity picker for connector lifecycle ops (restart/start/stop) and runbook
 * steps (#282): lists the entities (VMs, containers, …) from a connector's
 * latest snapshot, keyed by SnapshotEntity.externalId, plus a "Whole
 * service" option (entityRef '') for connectors that manage a single
 * implicit service or when no specific entity should be targeted.
 */
import { useTranslation } from 'react-i18next';
import {
  useGetConnectorsConnectorIdSnapshots,
  useGetConnectorsConnectorIdSnapshotsSnapshotId,
} from '../../api/generated/connectors/connectors';
import type { SnapshotEntity } from '../../api/model';
import { ChevronDownIcon } from '../icons';

export function EntityPicker({
  connectorId,
  value,
  onChange,
  onEntityChange,
  id,
  label,
  kind,
  placeholder,
  hideWholeService,
  disabled = false,
}: {
  connectorId: string;
  value: string;
  onChange: (entityRef: string) => void;
  onEntityChange?: (entity: SnapshotEntity | undefined) => void;
  id?: string;
  label?: string;
  kind?: string;
  placeholder?: string;
  hideWholeService?: boolean;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const latest = useGetConnectorsConnectorIdSnapshots(
    connectorId,
    { limit: 1 },
    { query: { enabled: !!connectorId } }
  );
  const latestId = latest.data?.[0]?.id ?? '';
  const full = useGetConnectorsConnectorIdSnapshotsSnapshotId(connectorId, latestId, {
    query: { enabled: !!connectorId && !!latestId },
  });
  // When kind is specified (e.g. merge picker), allow fallback to entity.name if externalId
  // is absent, matching backend topology entityRef() resolution. Other callers require externalId.
  const getEntityRef = (e: SnapshotEntity) => (kind ? e.externalId || e.name : e.externalId);
  // In kind mode entities are one member per (connector, kind, ref) to the backend, so
  // same-ref entities collapse to the first, keeping option values/keys unique.
  const seen = new Set<string>();
  const entities = (full.data?.entities ?? []).filter((e) => {
    const ref = getEntityRef(e);
    if (!ref || (kind && e.kind !== kind)) return false;
    if (!kind) return true;
    if (seen.has(ref)) return false;
    seen.add(ref);
    return true;
  });

  const status = (() => {
    if (!connectorId) return null;
    if (latest.isError || full.isError) return t('entityPicker.loadError');
    if (latest.isLoading || full.isLoading) return t('entityPicker.loading');
    if (latest.data && latest.data.length === 0) return t('entityPicker.noSnapshot');
    if (kind && full.data && entities.length === 0) return t('entityPicker.noEntitiesOfKind');
    return null;
  })();

  const pickerLabel = label ?? t('entityPicker.label');

  return (
    <label className="block">
      <span className="mb-1 block text-2xs text-ink-faint">{pickerLabel}</span>
      <div className="relative">
        <select
          id={id}
          aria-label={pickerLabel}
          value={value}
          disabled={disabled}
          onChange={(e) => {
            onChange(e.target.value);
            onEntityChange?.(entities.find((entity) => getEntityRef(entity) === e.target.value));
          }}
          className="h-8 w-full appearance-none rounded-sm border border-line bg-surface pl-2.5 pr-7 text-xs text-ink outline-none focus-visible:border-accent-primary-soft"
        >
          <option value="">
            {placeholder ??
              (hideWholeService ? t('entityPicker.label') : t('entityPicker.wholeService'))}
          </option>
          {entities.map((entity) => {
            const ref = getEntityRef(entity) ?? '';
            return (
              <option key={ref} value={ref}>
                {entity.name}
                {entity.externalId ? ` (${entity.externalId})` : ''}
              </option>
            );
          })}
        </select>
        <ChevronDownIcon
          size={12}
          className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-ink-faint"
        />
      </div>
      {status && <p className="mt-1 text-2xs text-ink-muted">{status}</p>}
    </label>
  );
}
