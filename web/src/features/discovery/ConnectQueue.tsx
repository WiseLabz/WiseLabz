/**
 * Walks the selected discovery work in order: each product candidate, its
 * selected add-ons, then the optional certificate-monitoring step. The forms
 * stay the standard connector create/edit forms; skip advances and stop returns
 * control to the result list.
 */
import { useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import type { Connector, DiscoveryAddOn, DiscoveryCandidate, DiscoveryTLSTarget } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { ErrorState, SkeletonRows } from '../../components/ui/states';
import { Panel } from '../../components/ui/Panel';
import { ConnectorEditForm } from '../connectors/ConnectorEditForm';
import { ConnectorForm } from '../connectors/ConnectorForm';
import { addOnKey, candidateKey, endpointLabel } from './candidates';

type QueueItem =
  | { kind: 'candidate'; candidate: DiscoveryCandidate }
  | { kind: 'addOn'; candidate: DiscoveryCandidate; addOn: DiscoveryAddOn }
  | { kind: 'tls'; targets: DiscoveryTLSTarget[] };

const TLS_TARGET_LIMIT = 100;

function queueItems(
  candidates: DiscoveryCandidate[],
  selectedAddOns: { candidate: DiscoveryCandidate; addOn: DiscoveryAddOn }[] | undefined,
  tlsTargets: DiscoveryTLSTarget[] | undefined,
): QueueItem[] {
  const explicitlySelected = selectedAddOns === undefined
    ? undefined
    : new Set(selectedAddOns.map(({ candidate, addOn }) => addOnKey(candidate, addOn)));
  const items: QueueItem[] = [];
  const queuedAddOns = new Set<string>();

  for (const candidate of candidates) {
    items.push({ kind: 'candidate', candidate });
    for (const addOn of candidate.addOns ?? []) {
      if (addOn.connectorId) continue;
      if (explicitlySelected && !explicitlySelected.has(addOnKey(candidate, addOn))) continue;
      queuedAddOns.add(addOnKey(candidate, addOn));
      items.push({ kind: 'addOn', candidate, addOn });
    }
  }

  // Add-ons can be selected independently of their product candidate. Preserve
  // those selections even when the parent candidate was not queued.
  for (const { candidate, addOn } of selectedAddOns ?? []) {
    const key = addOnKey(candidate, addOn);
    if (addOn.connectorId || queuedAddOns.has(key)) continue;
    queuedAddOns.add(key);
    items.push({ kind: 'addOn', candidate, addOn });
  }

  if (tlsTargets?.length) items.push({ kind: 'tls', targets: tlsTargets });
  return items;
}

function targetIdentity(line: string): string {
  const value = line.trim();
  const bracketed = value.match(/^\[([^\]]+)\]:(\d+)$/);
  const splitAt = bracketed ? -1 : value.lastIndexOf(':');
  const host = bracketed?.[1] ?? (splitAt > 0 ? value.slice(0, splitAt) : value);
  const port = bracketed?.[2] ?? (splitAt > 0 ? value.slice(splitAt + 1) : '');
  let normalizedHost = host.toLowerCase();
  if (host.includes(':')) {
    try {
      // WHATWG URL parsing canonicalizes IPv6 compression and hex casing.
      normalizedHost = new URL(`http://[${host}]/`).hostname.replace(/^\[|\]$/g, '').toLowerCase();
    } catch {
      // Keep the normalized text for malformed existing entries; the API will
      // report those on save, while valid scan targets still compare exactly.
    }
  }
  return port ? `${normalizedHost}|${Number(port)}` : value.toLowerCase();
}

function targetPlan(existingRaw: string, targets: DiscoveryTLSTarget[]) {
  const lines: string[] = [];
  const seen = new Set<string>();
  for (const raw of existingRaw.split('\n')) {
    const line = raw.trim();
    if (!line) continue;
    const key = targetIdentity(line);
    if (seen.has(key)) continue;
    seen.add(key);
    lines.push(line);
  }

  const notAdded: DiscoveryTLSTarget[] = [];
  for (const target of targets) {
    const line = endpointLabel(target.address, target.port);
    const key = targetIdentity(line);
    if (seen.has(key)) continue;
    seen.add(key);
    if (lines.length >= TLS_TARGET_LIMIT) {
      notAdded.push(target);
      continue;
    }
    lines.push(line);
  }

  return { targets: lines.join('\n'), notAdded };
}

function targetsStillListed(saved: Connector, targets: DiscoveryTLSTarget[]) {
  const savedLines = new Set(
    String(saved.config?.targets ?? '')
      .split('\n')
      .map(targetIdentity)
      .filter(Boolean),
  );
  return targets.filter((target) => savedLines.has(targetIdentity(endpointLabel(target.address, target.port))));
}

export function ConnectQueue({
  candidates,
  addOns,
  tlsTargets,
  onCreated,
  onAddOnCreated,
  onTLSProbeSaved,
  onFinished,
  onStop,
}: {
  candidates: DiscoveryCandidate[];
  /** Omitted means the v1 queue should include every unconnected candidate add-on. */
  addOns?: { candidate: DiscoveryCandidate; addOn: DiscoveryAddOn }[];
  tlsTargets?: DiscoveryTLSTarget[];
  /** A connector was created for a candidate. */
  onCreated: (created: Connector, candidate: DiscoveryCandidate) => void;
  onAddOnCreated?: (created: Connector, candidate: DiscoveryCandidate, addOn: DiscoveryAddOn) => void;
  onTLSProbeSaved?: (saved: Connector, monitoredTargets: DiscoveryTLSTarget[]) => void;
  /** Every queue item was saved or skipped. */
  onFinished: () => void;
  /** The admin left the queue early. */
  onStop: () => void;
}) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  const [selectedProbeId, setSelectedProbeId] = useState('');
  const currentIndex = useRef(0);
  const stopped = useRef(false);
  const connectorsQuery = useGetConnectors();
  const { data: connectors } = connectorsQuery;
  const items = useMemo(() => queueItems(candidates, addOns, tlsTargets), [candidates, addOns, tlsTargets]);
  const item = items[index];
  if (!item) return null;

  const advanceFrom = (sourceIndex: number) => {
    // A form save can resolve after Skip or Stop. Its save callback still needs
    // to report the connector, but it must not move this queue a second time.
    if (stopped.current || currentIndex.current !== sourceIndex) return;
    currentIndex.current = sourceIndex + 1;
    if (sourceIndex + 1 >= items.length) onFinished();
    else setIndex(sourceIndex + 1);
  };
  const advance = () => advanceFrom(index);
  const stop = () => {
    stopped.current = true;
    onStop();
  };

  const candidatePosition = items.slice(0, index + 1).filter((entry) => entry.kind === 'candidate').length;
  const totalCandidates = candidates.length;
  const position = items.length === totalCandidates
    ? item.kind === 'candidate'
      ? t('discovery.queue.position', { index: candidatePosition, total: totalCandidates })
      : null
    : t('discovery.queue.stepPosition', { index: index + 1, total: items.length });
  const controls = (
    <div className="flex items-center gap-2">
      <Button variant="secondary" size="sm" onClick={advance}>
        {t('discovery.queue.skip')}
      </Button>
      <Button variant="ghost" size="sm" onClick={stop}>
        {t('discovery.queue.stop')}
      </Button>
    </div>
  );

  if (item.kind === 'candidate' || item.kind === 'addOn') {
    const candidate = item.candidate;
    const addOn = item.kind === 'addOn' ? item.addOn : undefined;
    const name = addOn
      ? addOn.type === 'dnsresolver' ? t('discovery.dnsResolverAddon') : addOn.type
      : candidate.name;
    return (
      <section aria-labelledby="connect-queue-title" className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 id="connect-queue-title" className="text-sm font-semibold text-ink">
              {t('discovery.queue.heading', {
                name,
                address: endpointLabel(candidate.address, candidate.port),
              })}
            </h2>
            {position && (
              <p className="text-2xs text-ink-faint" aria-live="polite">
                {position}
              </p>
            )}
          </div>
          {controls}
        </div>

        {addOn ? (
          <ConnectorForm
            key={addOnKey(candidate, addOn)}
            initialType={addOn.type}
            initialValues={{
              url: addOn.url,
              name: `${name} (${candidate.address})`,
            }}
            onCreated={(created) => {
              onAddOnCreated?.(created, candidate, addOn);
              advance();
            }}
          />
        ) : (
          <ConnectorForm
            key={candidateKey(candidate)}
            initialType={candidate.type}
            initialValues={{
              [candidate.urlField]: candidate.url,
              name: `${candidate.name} (${candidate.address})`,
            }}
            onCreated={(created) => {
              onCreated(created, candidate);
              advance();
            }}
          />
        )}
      </section>
    );
  }

  const probes = (connectors ?? []).filter(
    (connector) => connector.type === 'tlsprobe' && connector.managedBy !== 'config' && connector.managedBy !== 'config-orphaned',
  );
  if (connectorsQuery.isLoading) {
    return (
      <section aria-labelledby="connect-queue-title" className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 id="connect-queue-title" className="text-sm font-semibold text-ink">
              {t('discovery.monitorCertificates', { count: item.targets.length })}
            </h2>
            {position && <p className="text-2xs text-ink-faint" aria-live="polite">{position}</p>}
          </div>
          {controls}
        </div>
        <Panel className="p-6"><SkeletonRows rows={3} /></Panel>
      </section>
    );
  }
  if (connectorsQuery.isError) {
    return (
      <section aria-labelledby="connect-queue-title" className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 id="connect-queue-title" className="text-sm font-semibold text-ink">
              {t('discovery.monitorCertificates', { count: item.targets.length })}
            </h2>
            {position && <p className="text-2xs text-ink-faint" aria-live="polite">{position}</p>}
          </div>
          {controls}
        </div>
        <ErrorState
          description={t('discovery.queue.connectorsLoadError')}
          onRetry={() => void connectorsQuery.refetch()}
        />
      </section>
    );
  }
  // The list endpoint is newest-first, so the first probe is the default.
  const selectedProbe = probes.find((probe) => probe.id === selectedProbeId) ?? probes[0];
  const plan = targetPlan(String(selectedProbe?.config?.targets ?? ''), item.targets);
  return (
    <section aria-labelledby="connect-queue-title" className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 id="connect-queue-title" className="text-sm font-semibold text-ink">
            {t('discovery.monitorCertificates', { count: item.targets.length })}
          </h2>
          {position && (
            <p className="text-2xs text-ink-faint" aria-live="polite">
              {position}
            </p>
          )}
        </div>
        {controls}
      </div>

      {probes.length > 1 && (
        <label className="block text-xs text-ink-muted">
          {t('discovery.chooseProbe')}
          <select
            aria-label={t('discovery.chooseProbe')}
            value={selectedProbe?.id ?? ''}
            onChange={(event) => setSelectedProbeId(event.target.value)}
            className="mt-1 block h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink"
          >
            {probes.map((probe) => (
              <option key={probe.id} value={probe.id}>{probe.name}</option>
            ))}
          </select>
        </label>
      )}

      {plan.notAdded.length > 0 && (
        <div className="rounded-sm border border-line-soft px-3 py-2 text-xs text-ink-muted">
          <p className="font-medium">{t('discovery.notAdded')}</p>
          <ul className="mt-1 space-y-1">
            {plan.notAdded.map((target) => (
              <li key={`${target.address}|${target.port}`} className="font-mono">
                {endpointLabel(target.address, target.port)}
              </li>
            ))}
          </ul>
        </div>
      )}

      {selectedProbe ? (
        <ConnectorEditForm
          key={selectedProbe.id}
          connectorId={selectedProbe.id}
          overrides={{ targets: plan.targets }}
          onCancel={advance}
          onSaved={(saved) => {
            onTLSProbeSaved?.(saved, targetsStillListed(saved, item.targets));
            advance();
          }}
        />
      ) : (
        <ConnectorForm
          key="tlsprobe-create"
          initialType="tlsprobe"
          initialValues={{
            name: t('discovery.tlsProbe'),
            targets: plan.targets,
          }}
          onCreated={(created) => {
            onTLSProbeSaved?.(created, targetsStillListed(created, item.targets));
            advance();
          }}
        />
      )}
    </section>
  );
}
