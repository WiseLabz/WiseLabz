import { MutationCache, QueryClient } from '@tanstack/react-query';
import i18n from '../i18n';
import { toast } from '../lib/toast';

export const queryClient = new QueryClient({
  // Fallback so a failed mutation never looks like nothing happened. Mutations
  // that define their own onError keep full control of their messaging.
  mutationCache: new MutationCache({
    onError: (_error, _vars, _ctx, mutation) => {
      if (mutation.options.onError) return;
      toast.error(i18n.t('common.actionFailed'));
    },
  }),
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});
