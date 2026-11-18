import { StudentPage } from '@/features/student/student-page';
import { useWorkspaceContext } from '@/features/workspace/context';

export function WorkspaceStudentsPage() {
  const workspace = useWorkspaceContext();
  return (
    <StudentPage
      workspaceID={workspace.workspaceID}
      workspaceName={workspace.workspaceName}
      actions={workspace.actions}
      onRefresh={workspace.refresh}
    />
  );
}
