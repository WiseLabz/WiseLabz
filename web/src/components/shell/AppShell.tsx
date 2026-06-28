/**
 * Authenticated app frame. Navigation is the locked bottom dock (PRODUCT.md);
 * the routed content (error boundary + Suspense + the active page) and the
 * command palette (⌘K) live here.
 */
import { Suspense } from 'react';
import { Outlet } from 'react-router-dom';
import { ErrorBoundary } from 'react-error-boundary';
import { ShellDock } from './ShellDock';
import { CommandPalette } from '../command/CommandPalette';
import { ErrorState, SkeletonRows } from '../ui/states';

function Content() {
  return (
    <ErrorBoundary
      fallbackRender={({ resetErrorBoundary }) => (
        <ErrorState
          title="This page hit an error"
          description="The view failed to render. Retry, or head back to the dashboard."
          onRetry={resetErrorBoundary}
        />
      )}
    >
      <Suspense fallback={<SkeletonRows rows={6} className="m-6 max-w-2xl" />}>
        <Outlet />
      </Suspense>
    </ErrorBoundary>
  );
}

export function AppShell() {
  return (
    <>
      <ShellDock>
        <Content />
      </ShellDock>
      <CommandPalette />
    </>
  );
}
