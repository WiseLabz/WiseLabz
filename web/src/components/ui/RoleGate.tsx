/**
 * Renders its children only for users who may mutate (operator); everyone else
 * sees `fallback` (or nothing). This is a UI affordance only — the server still
 * enforces the real boundary on every mutating endpoint (ARCHITECTURE.md).
 */
import type { ReactNode } from 'react';
import { useCanMutate } from '../../hooks/useRole';

interface RoleGateProps {
  children: ReactNode;
  fallback?: ReactNode;
}

export function RoleGate({ children, fallback }: RoleGateProps) {
  const canMutate = useCanMutate();
  return <>{canMutate ? children : (fallback ?? null)}</>;
}
