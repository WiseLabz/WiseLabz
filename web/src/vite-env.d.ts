/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Force-enable/disable the mock layer (MSW + WS). Default: on in dev. */
  readonly VITE_USE_MOCKS?: 'true' | 'false';
}
