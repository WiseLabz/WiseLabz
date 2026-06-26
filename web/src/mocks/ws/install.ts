/**
 * Swaps the global WebSocket for MockWebSocket and exposes a small devtools
 * handle (`window.__wsMock.emit(envelope)`) for manually firing events from the
 * console. Idempotent.
 */
import type { WsEvent } from '../../types/ws';
import { env } from './timeline';
import { MockWebSocket } from './MockWebSocket';

interface WsMockHandle {
  /** Broadcast a raw envelope to all open mock sockets. */
  emit: (event: WsEvent) => void;
  /** Envelope builder (auto-fills ts + id), e.g. __wsMock.env('alert.created', {...}). */
  env: typeof env;
}

declare global {
  interface Window {
    __wsMock?: WsMockHandle;
  }
}

let installed = false;

export function installMockWebSocket(): void {
  if (installed || typeof window === 'undefined') return;
  installed = true;

  window.WebSocket = MockWebSocket as unknown as typeof WebSocket;
  window.__wsMock = {
    emit: (event) => MockWebSocket.broadcast(event),
    env,
  };
}
