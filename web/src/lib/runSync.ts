/**
 * Trigger a sync the production way — a REST call (`POST /sync` globally, or
 * `POST /connectors/{id}/sync` per-connector); progress then streams over /ws.
 * In dev (MSW + the mock WS) we additionally drive the scripted progress frames
 * so the dashboard sweep + activity feed react exactly as they will in prod.
 * Against a real backend `triggerMockSync` is a no-op (no `window.__wsMock`).
 */
import { postSync, postConnectorsConnectorIdSync } from '../api/generated/connectors/connectors';
import { triggerMockSync } from '../ws/triggerSync';

export async function runSync(connectorId: string | null = null): Promise<void> {
  try {
    if (connectorId) await postConnectorsConnectorIdSync(connectorId);
    else await postSync();
  } finally {
    triggerMockSync(connectorId);
  }
}
