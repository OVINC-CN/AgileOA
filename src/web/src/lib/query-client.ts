import { QueryClient } from '@tanstack/react-query';

import { getConfig } from '@/lib/config';

const config = getConfig();

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: config.queryStaleTimeMs,
      retry: config.queryRetryCount,
      refetchOnWindowFocus: false,
    },
    mutations: { retry: false },
  },
});
