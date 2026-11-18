import { DashboardPage } from '@/features/cpc-dashboard/dashboard-page';
import { useWorkspaceContext } from '@/features/workspace/context';

export function WorkspaceCPCDashboardPage() {
  return <DashboardPage {...useWorkspaceContext()} />;
}
