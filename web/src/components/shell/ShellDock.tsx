/**
 * Bottom-dock shell (soft-dark, the locked navigation per PRODUCT.md): slim
 * Topbar up top (brand + search + sync + account), full-width content, floating
 * nav dock at the bottom. Content is supplied by AppShell.
 */
import type { ReactNode } from 'react';
import { Topbar } from './Topbar';
import { Dock } from './Dock';

export function ShellDock({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-screen w-screen flex-col overflow-hidden bg-[var(--color-canvas)] text-[var(--color-ink)]">
      <Topbar />
      <main className="flex-1 overflow-y-auto pb-28">{children}</main>
      <Dock />
    </div>
  );
}
