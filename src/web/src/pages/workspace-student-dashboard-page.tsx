import { DashboardPage } from '@/features/student-dashboard/dashboard-page';
import { useWorkspaceContext } from '@/features/workspace/context';

export function WorkspaceStudentDashboardPage() {
  return <DashboardPage {...useWorkspaceContext()} />;
}
