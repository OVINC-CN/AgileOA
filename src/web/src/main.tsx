import '@/index.css';

import { QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { matchRoutes } from 'react-router';
import { RouterProvider } from 'react-router/dom';

import { router } from '@/app/router';
import { MessageViewport } from '@/components/message-viewport';
import { PageUpdateDetector } from '@/components/page-update-detector';
import { getConfig } from '@/lib/config';
import { queryClient } from '@/lib/query-client';
import { initializeRum } from '@/lib/rum';

const config = getConfig();
document.title = config.appName;
void initializeRum(config, () => {
  const { origin, pathname } = window.location;
  const route = matchRoutes(router.routes, { pathname })?.at(-1)?.route;
  return `${origin}${route && route.path !== '*' ? pathname : '/404'}`;
});

const container = document.getElementById('root');
if (!container) {
  throw new Error('页面缺少根节点');
}

createRoot(container).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <MessageViewport />
      <PageUpdateDetector />
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
