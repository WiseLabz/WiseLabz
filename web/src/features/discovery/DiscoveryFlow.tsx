/**
 * The whole scan-to-connectors flow in one place: the scan panel, and the
 * connect queue it hands its selection to. Onboarding and the add-connector page
 * both embed it and only decide what happens with the connectors it creates.
 */
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { getGetDiscoveryScanQueryKey } from '../../api/generated/discovery/discovery';
import type { Connector, DiscoveryCandidate } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { ConnectQueue } from './ConnectQueue';
import { DiscoveryPanel } from './DiscoveryPanel';

export function DiscoveryFlow({
  onConnectorCreated,
  onQueueFinished,
  onQueueActiveChange,
}: {
  /** A connector was created from a candidate (also when the queue is later stopped). */
  onConnectorCreated?: (created: Connector) => void;
  /** Every selected candidate was saved or skipped. Not called when the admin stops. */
  onQueueFinished?: () => void;
  /** The queue (and its connector form) opened or closed. */
  onQueueActiveChange?: (active: boolean) => void;
}) {
  const [queue, setQueue] = useState<DiscoveryCandidate[] | null>(null);
  const markConnected = useDiscovery((s) => s.markConnected);
  const queryClient = useQueryClient();

  const openQueue = (selected: DiscoveryCandidate[]) => {
    setQueue(selected);
    onQueueActiveChange?.(true);
  };
  const closeQueue = () => {
    setQueue(null);
    onQueueActiveChange?.(false);
  };

  if (queue) {
    return (
      <ConnectQueue
        candidates={queue}
        onCreated={(created, candidate) => {
          // Shown as already connected as soon as it exists, not after the next scan read.
          markConnected([{ candidate, connectorId: created.id }]);
          // The panel is unmounted meanwhile and hydrates from the query cache on its
          // return: keep the cache in step with the store, then refetch the server's view.
          const scan = useDiscovery.getState().scan;
          if (scan) queryClient.setQueryData(getGetDiscoveryScanQueryKey(), { scan });
          void queryClient.invalidateQueries({ queryKey: getGetDiscoveryScanQueryKey() });
          onConnectorCreated?.(created);
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
