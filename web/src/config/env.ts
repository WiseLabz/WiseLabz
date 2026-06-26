/**
 * Single source for build-time flags.
 *
 * USE_MOCKS gates BOTH the MSW REST layer and the WS mock emitter (FRONTEND_PLAN
 * §6 / WS_CONTRACT.md). Default: on in dev, off in prod. Override explicitly with
 * VITE_USE_MOCKS=true|false (e.g. to run dev against a real local backend).
 */
export const USE_MOCKS =
  import.meta.env.VITE_USE_MOCKS === 'true' ||
  (import.meta.env.DEV && import.meta.env.VITE_USE_MOCKS !== 'false');
