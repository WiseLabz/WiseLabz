/**
 * The whole scan-to-connectors flow in one place: the scan panel, and the
 * connect queue it hands its selection to. Onboarding and the add-connector page
 * both embed it and only decide what happens with the connectors it creates.
 */
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { getGetDiscoveryScanQueryKey } from '../../api/generated/discovery/discovery';
import type { Connector, DiscoveryAddOn, DiscoveryCandidate, DiscoveryTLSTarget } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { ConnectQueue } from './ConnectQueue';
import { DiscoveryPanel } from './DiscoveryPanel';

export function DiscoveryFlow({
  onConnectorCreated,
  onQueueFinished,
  onQueueActiveChange,
}: {
  /** A connector was created or saved through the queue (also when it is later stopped). */
  onConnectorCreated?: (created: Connector) => void;
  /** Every selected queue item was saved or skipped. Not called when the admin stops. */
  onQueueFinished?: () => void;
  /** The queue (and its connector form) opened or closed. */
  onQueueActiveChange?: (active: boolean) => void;
}) {
  const [queue, setQueue] = useState<{
    candidates: DiscoveryCandidate[];
    tlsTargets?: DiscoveryTLSTarget[];
    addOns?: { candidate: DiscoveryCandidate; addOn: DiscoveryAddOn }[];
  } | null>(null);
  const markConnected = useDiscovery((s) => s.markConnected);
  const queryClient = useQueryClient();

  const openQueue = (
    selected: DiscoveryCandidate[],
    selectedTLSTargets?: DiscoveryTLSTarget[],
    selectedAddOns?: { candidate: DiscoveryCandidate; addOn: DiscoveryAddOn }[],
  ) => {
    // Keep undefined distinct from []: omitted preserves direct v1 queue defaults;
    // [] means the panel explicitly deselected every add-on.
    setQueue({ candidates: selected, tlsTargets: selectedTLSTargets, addOns: selectedAddOns });
    onQueueActiveChange?.(true);
  };
  const closeQueue = () => {
    setQueue(null);
    onQueueActiveChange?.(false);
  };
  const refreshScanCache = () => {
    // The panel is unmounted while the queue is open and hydrates from this cache
    // when it returns. Keep it in step with the store, then re-read the server view.
    const scan = useDiscovery.getState().scan;
    if (scan) queryClient.setQueryData(getGetDiscoveryScanQueryKey(), { scan });
    void queryClient.invalidateQueries({ queryKey: getGetDiscoveryScanQueryKey() });
  };

  if (queue) {
    return (
      <ConnectQueue
        candidates={queue.candidates}
        tlsTargets={queue.tlsTargets}
        addOns={queue.addOns}
        onCreated={(created, candidate) => {
          // Shown as already connected as soon as it exists, not after the next scan read.
          markConnected([{ candidate, connectorId: created.id }]);
          // The panel is unmounted meanwhile and hydrates from the query cache on its
          // return: keep the cache in step with the store, then refetch the server's view.
          refreshScanCache();
          onConnectorCreated?.(created);
        }}
        onAddOnCreated={(created, _candidate, addOn) => {
          markConnected([{ addOn, connectorId: created.id }]);
          refreshScanCache();
          onConnectorCreated?.(created);
        }}
        onTLSProbeSaved={(saved, monitoredTargets) => {
          if (monitoredTargets.length > 0) {
            markConnected([{ tlsTargets: monitoredTargets, connectorId: saved.id }]);
          }
          refreshScanCache();
          onConnectorCreated?.(saved);
        }}
        onFinished={() => {
          closeQueue();
          onQueueFinished?.();
        }}
        onStop={closeQueue}
      />
    );
  }
  return <DiscoveryPanel onConnect={openQueue} />;
}
