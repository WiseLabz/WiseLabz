/**
 * The whole scan-to-connectors flow in one place: the scan panel, and the
 * connect queue it hands its selection to. Onboarding and the add-connector page
 * both embed it and only decide what happens with the connectors it creates.
 */
import { useState } from 'react';
import type { Connector, DiscoveryCandidate } from '../../api/model';
import { useDiscovery } from '../../store/discovery';
import { ConnectQueue } from './ConnectQueue';
import { DiscoveryPanel } from './DiscoveryPanel';

export function DiscoveryFlow({
  onConnectorCreated,
  onQueueFinished,
}: {
  /** A connector was created from a candidate (also when the queue is later stopped). */
  onConnectorCreated?: (created: Connector) => void;
  /** Every selected candidate was saved or skipped. Not called when the admin stops. */
  onQueueFinished?: () => void;
}) {
  const [queue, setQueue] = useState<DiscoveryCandidate[] | null>(null);
  const markConnected = useDiscovery((s) => s.markConnected);

  if (queue) {
    return (
      <ConnectQueue
        candidates={queue}
        onCreated={(created, candidate) => {
          // Shown as already connected as soon as it exists, not after the next scan read.
          markConnected([{ candidate, connectorId: created.id }]);
          onConnectorCreated?.(created);
        }}
        onFinished={() => {
          setQueue(null);
          onQueueFinished?.();
        }}
        onStop={() => setQueue(null)}
      />
    );
  }
  return <DiscoveryPanel onConnect={setQueue} />;
}
