/**
 * Boots the full mock layer: MSW REST worker + WS mock socket. Called from
 * main.tsx behind USE_MOCKS, before the app renders, so the first queries/sockets
 * already hit mocks. Dynamically imported so none of this ships to a prod bundle
 * with mocks disabled.
 */
export async function enableMocks(): Promise<void> {
  const { worker } = await import('./browser');
  await worker.start({
    // Real assets, the Vite client, etc. should pass through untouched.
    onUnhandledRequest: 'bypass',
    quiet: false,
  });

  const { installMockWebSocket } = await import('./ws/install');
  installMockWebSocket();

  console.info(
    '[mocks] MSW REST + WS mock active. Fire events from the console: window.__wsMock.emit(window.__wsMock.env("alert.created", { alertId: "x", serviceId: "s", severity: "critical", title: "Test" }))',
  );
}
