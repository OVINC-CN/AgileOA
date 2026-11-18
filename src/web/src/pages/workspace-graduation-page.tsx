import { GraduationPage } from '@/features/graduation/graduation-page';
import { useWorkspaceContext } from '@/features/workspace/context';

export function WorkspaceGraduationPage() {
  return <GraduationPage {...useWorkspaceContext()} />;
}
