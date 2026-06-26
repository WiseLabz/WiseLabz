/**
 * A drop-in fake `WebSocket` that replays the scripted timeline (timeline.ts).
 *
 * Installed over `window.WebSocket` when USE_MOCKS is on, so a future
 * `useWebSocket` that just does `new WebSocket('/ws')` transparently receives
 * WS_CONTRACT.md events with no awareness of the mock. Supports both the
 * `onmessage = fn` and `addEventListener('message', fn)` styles.
 *
 * Per the contract the client sends nothing, so `send()` is a no-op.
 */
import type { WsEnvelope } from '../../types/ws';
import { startTimeline } from './timeline';

type Listenerish = ((ev: Event) => void) | null;

export class MockWebSocket extends EventTarget {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;

  readonly CONNECTING = MockWebSocket.CONNECTING;
  readonly OPEN = MockWebSocket.OPEN;
  readonly CLOSING = MockWebSocket.CLOSING;
  readonly CLOSED = MockWebSocket.CLOSED;

  /** Every live mock socket, so manual emits can broadcast to all of them. */
  private static readonly instances = new Set<MockWebSocket>();

  url: string;
  readyState: number = MockWebSocket.CONNECTING;
  protocol = '';
  extensions = '';
  bufferedAmount = 0;
  binaryType: BinaryType = 'blob';

  onopen: Listenerish = null;
  onmessage: Listenerish = null;
  onclose: Listenerish = null;
  onerror: Listenerish = null;

  private stopTimeline?: () => void;

  constructor(url: string | URL, _protocols?: string | string[]) {
    super();
    this.url = url.toString();
    // Open on the next tick to mimic async connection establishment.
    queueMicrotask(() => this.handleOpen());
  }

  private handleOpen(): void {
    if (this.readyState !== MockWebSocket.CONNECTING) return;
    this.readyState = MockWebSocket.OPEN;
    MockWebSocket.instances.add(this);
    const ev = new Event('open');
    this.onopen?.(ev);
    this.dispatchEvent(ev);
    this.stopTimeline = startTimeline((envelope) => this.emit(envelope));
  }

  /** Deliver one envelope to this socket's consumers. */
  emit(envelope: WsEnvelope): void {
    if (this.readyState !== MockWebSocket.OPEN) return;
    const ev = new MessageEvent('message', { data: JSON.stringify(envelope) });
    this.onmessage?.(ev);
    this.dispatchEvent(ev);
  }

  send(_data?: unknown): void {
    // Client → server frames are out of contract scope; ignore.
  }

  close(code = 1000, reason = ''): void {
    if (this.readyState === MockWebSocket.CLOSED) return;
    this.readyState = MockWebSocket.CLOSED;
    this.stopTimeline?.();
    MockWebSocket.instances.delete(this);
    const ev = new CloseEvent('close', { code, reason, wasClean: true });
    this.onclose?.(ev);
    this.dispatchEvent(ev);
  }

  /** Push an envelope to every open mock socket — handy from the devtools console. */
  static broadcast(envelope: WsEnvelope): void {
    MockWebSocket.instances.forEach((s) => s.emit(envelope));
  }
}
