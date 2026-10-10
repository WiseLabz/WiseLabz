import { isAxiosError } from 'axios';
import type { useTranslation } from 'react-i18next';

type RunT = ReturnType<typeof useTranslation>['t'];

/** Maps a run API failure to a localised message by HTTP status. */
export function runErrorMessage(error: unknown, t: RunT, fallbackKey: string): string {
  const status = isAxiosError(error) ? error.response?.status : undefined;
  if (isNoEligibleApprover(error)) return t('runbooks.runs.noApproverAvailable');
  if (status === 403) return t('runbooks.runs.error.forbidden');
  if (status === 409) return t('runbooks.runs.error.conflict');
  if (status === 503) return t('runbooks.runs.error.unavailable');
  return t(fallbackKey);
}

export function isConflict(error: unknown): boolean {
  return isAxiosError(error) && error.response?.status === 409;
}

/** The start was refused because no other operator can approve the request. */
export function isNoEligibleApprover(error: unknown): boolean {
  if (!isAxiosError(error) || error.response?.status !== 409) return false;
  return (error.response.data as { code?: unknown } | undefined)?.code === 'no_eligible_approver';
}
