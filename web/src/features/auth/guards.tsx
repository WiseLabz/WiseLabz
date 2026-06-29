/**
 * Route guards + the auth splash. RequireAuth gates the protected tree on an
 * authenticated session; RequireRole gates operator-only surfaces; RequireOnboarded
 * routes a fresh install (zero connectors) into onboarding. The server enforces the
 * real boundary — these guards are navigation, not security.
 */
import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../../store/auth';
import { useRole } from '../../hooks/useRole';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import type { Role } from '../../api/model';
import { EmptyState, SkeletonRows } from '../../components/ui/states';
import { Button } from '../../components/ui/Button';

/** Centered brand splash shown while the session resolves. */
export function Splash() {
  return (
    <div className="flex h-screen w-screen items-center justify-center bg-canvas">
      <div className="flex h-11 w-11 items-center justify-center rounded-lg bg-signal shadow-(--shadow-raised)">
        <span className="font-mono text-lg font-bold text-signal-ink">W</span>
      </div>
    </div>
  );
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const status = useAuth((s) => s.status);
  const location = useLocation();
  if (status === 'unknown') return <Splash />;
  if (status === 'anonymous') {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }
  return <>{children}</>;
}

export function RequireRole({ role, children }: { role: Role; children: ReactNode }) {
  const current = useRole();
  // viewer < operator. Only operator clears an operator gate.
  if (role === 'operator' && current !== 'operator') {
    return <Navigate to="/forbidden" replace />;
  }
  return <>{children}</>;
}

export function RequireOnboarded({ children }: { children: ReactNode }) {
  const { data, isLoading } = useGetConnectors();
  if (isLoading) return <SkeletonRows rows={6} className="m-6 max-w-2xl" />;
  if (Array.isArray(data) && data.length === 0) {
    return <Navigate to="/onboarding" replace />;
  }
  return <>{children}</>;
}

export function ForbiddenPage() {
  const { t } = useTranslation();
  return (
    <div className="flex h-screen w-screen items-center justify-center bg-canvas p-6">
      <EmptyState
        title={t('auth.forbiddenTitle')}
        description={t('auth.forbiddenDesc')}
        action={
          <a href="/dashboard">
            <Button variant="secondary" size="sm">
              {t('nav.dashboard')}
            </Button>
          </a>
        }
      />
    </div>
  );
}
