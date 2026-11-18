import { useQuery } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { Outlet, useLocation } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { SideNavigationLayout } from '@/components/side-navigation-layout';
import { Button } from '@/components/ui/button';
import { workspaceContextOptions } from '@/features/workspace/api';
import type { WorkspaceContext } from '@/features/workspace/context';
import {
  getWorkspacePageItems,
  useActiveWorkspaceID,
  useWorkspaceSection,
} from '@/features/workspace/navigation';
import { useApiAction, useInitialLoading } from '@/lib/api-action';

export function WorkspacePage() {
  const workspaceID = useActiveWorkspaceID();
  const section = useWorkspaceSection();
  const location = useLocation();
  const context = useQuery(workspaceContextOptions(workspaceID));
  const refresh = useApiAction();
  const initial = useInitialLoading(context.isPending);
  if (!workspaceID) {
    return <ErrorMessage>工作空间地址无效。</ErrorMessage>;
  }
  if (context.error || !context.data) {
    return (
      <section className="space-y-4">
        <div className="flex justify-end">
          <Button
            variant="outline"
            loading={refresh.pending || initial}
            onClick={() => {
              void refresh.run('refresh', () => context.refetch());
            }}
          >
            <RefreshCw aria-hidden="true" />
            刷新
          </Button>
        </div>
        {context.error ? <ErrorMessage>{context.error.message}</ErrorMessage> : null}
      </section>
    );
  }
  const value: WorkspaceContext = {
    workspaceID,
    workspaceName: context.data.workspace.name,
    actions: context.data.actions,
    canReviewApplications: context.data.can_review_project_applications,
    refresh: async () => {
      const result = await context.refetch();
      if (result.error) {
        throw result.error;
      }
    },
  };
  const items = getWorkspacePageItems(value.actions, section.value, workspaceID, location);
  const outlet = (
    <Outlet
      key={`${workspaceID}:${section.value}:${value.actions.join(',')}:${String(value.canReviewApplications)}`}
      context={value}
    />
  );
  if (items.length === 0) return outlet;

  return (
    <SideNavigationLayout label={section.label} items={items}>
      {outlet}
    </SideNavigationLayout>
  );
}
