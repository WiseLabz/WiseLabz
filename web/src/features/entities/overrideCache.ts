import type { QueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import {
  getGetEntitiesIdQueryOptions,
  getGetEntityOverridesQueryKey,
} from '../../api/generated/search/search';

/** Refresh the overrides list and every cached entity identity after an override change. */
export function invalidateEntityCaches(queryClient: QueryClient) {
  void queryClient.invalidateQueries({ queryKey: getGetEntityOverridesQueryKey() });
  void queryClient.invalidateQueries({
    predicate: (q) => typeof q.queryKey[0] === 'string' && q.queryKey[0].startsWith('/entities/'),
  });
}

/**
 * Probes whether the caller can open an identity. Resolves false only on 404
 * (hidden from this caller); any other outcome counts as visible so the caller
 * still gets a link. Goes through the query cache to prime the next navigation.
 */
export async function isEntityVisible(queryClient: QueryClient, entityId: string) {
  try {
    await queryClient.fetchQuery(getGetEntitiesIdQueryOptions(entityId));
    return true;
  } catch (err) {
    return !(isAxiosError(err) && err.response?.status === 404);
  }
}
