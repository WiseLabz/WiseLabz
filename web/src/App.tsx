import { useEffect } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createBrowserRouter, Navigate } from 'react-router-dom';
import { queryClient } from './app/queryClient';
import { MotionProvider } from './components/MotionProvider';
import { WebSocketProvider } from './ws/WebSocketProvider';
import { AppShell } from './components/shell/AppShell';
import { useAuth } from './store/auth';
import { PublicLayout } from './features/auth/PublicLayout';
import { LoginPage } from './features/auth/LoginPage';
import { AuthCallbackPage } from './features/auth/AuthCallbackPage';
import { RequireAuth, RequireOnboarded, ForbiddenPage, Splash } from './features/auth/guards';
import { OnboardingPage } from './features/onboarding/OnboardingPage';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ServicesPage } from './features/services/ServicesPage';
import { AddConnectorPage } from './features/connectors/AddConnectorPage';
import { DocsPage } from './features/docs/DocsPage';
import { ChangesPage } from './features/changes/ChangesPage';
import { ChangeDetailPage } from './features/changes/ChangeDetailPage';
import { AlertsPage } from './features/alerts/AlertsPage';
import { SettingsPage } from './features/settings/SettingsPage';
import { EmptyState } from './components/ui/states';
import { Button } from './components/ui/Button';

function NotFound() {
  return (
    <div className="flex h-full items-center justify-center">
      <EmptyState
        title="Page not found"
        description="That route doesn't exist."
        action={
          <a href="/dashboard">
            <Button variant="secondary" size="sm">
              Back to dashboard
            </Button>
          </a>
        }
      />
    </div>
  );
}

const router = createBrowserRouter([
  { path: '/login', element: <PublicLayout><LoginPage /></PublicLayout> },
  { path: '/auth/callback', element: <AuthCallbackPage /> },
  { path: '/forbidden', element: <ForbiddenPage /> },
  {
    // Onboarding sits under auth only (NOT RequireOnboarded) so it never loops.
    path: '/onboarding',
    element: (
      <RequireAuth>
        <OnboardingPage />
      </RequireAuth>
    ),
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <RequireOnboarded>
          <AppShell />
        </RequireOnboarded>
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: 'dashboard', element: <DashboardPage /> },
      { path: 'services', element: <ServicesPage /> },
      { path: 'services/new', element: <AddConnectorPage /> },
      { path: 'docs', element: <DocsPage /> },
      { path: 'docs/:docId', element: <DocsPage /> },
      { path: 'changes', element: <ChangesPage /> },
      { path: 'changes/:changeId', element: <ChangeDetailPage /> },
      { path: 'alerts', element: <AlertsPage /> },
      { path: 'settings', element: <SettingsPage /> },
      { path: '*', element: <NotFound /> },
    ],
  },
]);

function Root() {
  const status = useAuth((s) => s.status);
  const bootstrap = useAuth((s) => s.bootstrap);
  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);
  if (status === 'unknown') return <Splash />;
  return <RouterProvider router={router} />;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <WebSocketProvider>
        <MotionProvider>
          <Root />
        </MotionProvider>
      </WebSocketProvider>
    </QueryClientProvider>
  );
}
