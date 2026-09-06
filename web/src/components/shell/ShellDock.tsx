/**
 * Bottom-dock shell (soft-dark, the locked navigation per PRODUCT.md): slim
 * Topbar up top (brand + search + sync + account), full-width content, floating
 * nav dock at the bottom. Content is supplied by AppShell.
 */
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Topbar } from './Topbar';
import { Dock } from './Dock';

export function ShellDock({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <div className="flex h-dvh w-full flex-col overflow-hidden bg-canvas text-ink">
      <a
        href="#main-content"
        className="sr-only focus-visible:not-sr-only focus-visible:fixed focus-visible:left-3 focus-visible:top-3 focus-visible:z-(--z-toast) focus-visible:rounded-md focus-visible:bg-accent-primary focus-visible:px-3 focus-visible:py-2 focus-visible:text-sm focus-visible:font-medium focus-visible:text-accent-primary-ink"
      >
        {t('a11y.skipToContent')}
      </a>
      <Topbar />
      <main id="main-content" tabIndex={-1} className="flex-1 overflow-y-auto pb-28">
        {children}
      </main>
      <Dock />
    </div>
  );
}
