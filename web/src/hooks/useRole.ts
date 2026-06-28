/**
 * Role helpers. The role comes from the authenticated user (`/me`). The UI uses
 * this only to hide controls the user can't action — the server enforces the real
 * boundary on every mutating endpoint (ARCHITECTURE.md). v1 has two roles:
 * `viewer` (read) and `operator` (mutate).
 */
import { useGetMe } from '../api/generated/me/me';
import type { Role } from '../api/model';

export function useRole(): Role | undefined {
  const { data } = useGetMe();
  return data?.role;
}

/** True when the current user may perform mutating manager actions. */
export function useCanMutate(): boolean {
  return useRole() === 'operator';
}
