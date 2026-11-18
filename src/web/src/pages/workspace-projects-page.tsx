import { ProjectPage } from '@/features/project/project-page';
import { useWorkspaceContext } from '@/features/workspace/context';

export function WorkspaceProjectsPage() {
  return <ProjectPage {...useWorkspaceContext()} />;
}
