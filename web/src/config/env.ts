/**
 * Single source for build-time flags.
 *
 * USE_MOCKS gates BOTH the MSW REST layer and the WS mock emitter.
 * Default: on in dev, off in prod. Override explicitly with
 * VITE_USE_MOCKS=true|false (e.g. to run dev against a real local backend).
 */
export const USE_MOCKS =
  import.meta.env.VITE_USE_MOCKS === 'true' ||
  (import.meta.env.DEV && import.meta.env.VITE_USE_MOCKS !== 'false');
