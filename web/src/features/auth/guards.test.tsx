import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { RequireAuth, RequireInstanceAdmin, RequireOnboarded } from './guards';
import { useAuth } from '../../store/auth';

let isInstanceAdmin: boolean;
let meLoading = false;

vi.mock('../../api/generated/me/me', () => ({
  useGetMe: () => ({ isLoading: meLoading }),
}));

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({ data: [], isLoading: false }),
}));

vi.mock('../../hooks/useRole', () => ({
  useIsInstanceAdmin: () => isInstanceAdmin,
}));

function Location() {
  const location = useLocation();
  return <output>{location.pathname}</output>;
}

function renderAuthGuard() {
  return render(
    <MemoryRouter initialEntries={['/docs/guide']}>
      <Routes>
        <Route path="/docs/guide" element={<RequireAuth><p>protected</p></RequireAuth>} />
        <Route path="/login" element={<Location />} />
      </Routes>
    </MemoryRouter>,
  );
}

function renderRoleGuard() {
  return render(
    <MemoryRouter initialEntries={['/settings/users']}>
      <Routes>
        <Route path="/settings/users" element={<RequireInstanceAdmin><p>operator controls</p></RequireInstanceAdmin>} />
        <Route path="/forbidden" element={<Location />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('route guards', () => {
  afterEach(cleanup);

  beforeEach(() => {
    isInstanceAdmin = false;
    meLoading = false;
    useAuth.setState({ status: 'unknown', user: null });
  });

  it('keeps the splash up until the session resolves, then sends anonymous users to login', () => {
    const { rerender } = renderAuthGuard();

    expect(screen.queryByText('protected')).not.toBeInTheDocument();
    expect(screen.queryByText('/login')).not.toBeInTheDocument();

    useAuth.setState({ status: 'anonymous', user: null });
    rerender(
      <MemoryRouter initialEntries={['/docs/guide']}>
        <Routes>
          <Route path="/docs/guide" element={<RequireAuth><p>protected</p></RequireAuth>} />
          <Route path="/login" element={<Location />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText('/login')).toBeInTheDocument();
  });

  it('blocks non-admins from instance-admin routes while allowing admins through', () => {
    isInstanceAdmin = false;
    const { unmount } = renderRoleGuard();
    expect(screen.getByText('/forbidden')).toBeInTheDocument();

    unmount();
    isInstanceAdmin = true;
    renderRoleGuard();

    expect(screen.getByText('operator controls')).toBeInTheDocument();
  });

  it('waits for the administrator role before deciding access', () => {
    meLoading = true;
    const { unmount } = renderRoleGuard();
    expect(screen.queryByText('/forbidden')).not.toBeInTheDocument();
    expect(screen.queryByText('operator controls')).not.toBeInTheDocument();
    unmount();
    meLoading = false;
    isInstanceAdmin = true;
    renderRoleGuard();
    expect(screen.getByText('operator controls')).toBeInTheDocument();
  });

  it('allows lab notes without service grants while preserving onboarding elsewhere', () => {
    for (const path of ['/docs', '/docs/handbook', '/dashboard']) {
      const view = render(
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path={path} element={<RequireOnboarded><p>lab notes</p></RequireOnboarded>} />
            <Route path="/onboarding" element={<Location />} />
          </Routes>
        </MemoryRouter>,
      );
      expect(screen.queryByText('lab notes') !== null).toBe(path.startsWith('/docs'));
      expect(screen.queryByText('/onboarding') !== null).toBe(path === '/dashboard');
      view.unmount();
    }
  });
});
