/**
 * Phase 8 restructured Settings into a nested hub (SettingsLayout + sub-pages).
 * This module is kept only as a back-compat alias so the prior
 * `{ path: 'settings', element: <SettingsPage /> }` route in App.tsx keeps
 * compiling until the nested `/settings/*` route tree (see Phase 8 hand-off) is
 * spliced in. New code should import from `./SettingsLayout` and the sub-pages.
 */
export { SettingsLayout as SettingsPage } from './SettingsLayout';
