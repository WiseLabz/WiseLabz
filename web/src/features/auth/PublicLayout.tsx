/**
 * Off-shell layout for the auth surfaces (login, OIDC callback). Centered card on
 * the canvas with a faint blueprint grid behind it — no dock, no topbar.
 */
import type { ReactNode } from 'react';

export function PublicLayout({ children }: { children: ReactNode }) {
  return (
    <div className="relative flex min-h-screen w-screen items-center justify-center overflow-hidden bg-canvas px-4">
      <div className="bg-blueprint pointer-events-none absolute inset-0 opacity-40" aria-hidden />
      <div className="relative w-full max-w-sm">{children}</div>
    </div>
  );
}
