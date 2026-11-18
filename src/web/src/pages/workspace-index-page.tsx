import { Navigate } from 'react-router';

import { Card } from '@/components/ui/card';
import { useWorkspaceContext } from '@/features/workspace/context';
import {
  getWorkspaceEntryPath,
  getWorkspaceNavigation,
  useWorkspaceSection,
  workspaceURL,
} from '@/features/workspace/navigation';

export function WorkspaceIndexPage() {
  const { actions, workspaceID } = useWorkspaceContext();
  const section = useWorkspaceSection();
  const [first] = getWorkspaceNavigation(actions, section.value);
  if (first) {
    return <Navigate to={workspaceURL(`${section.path}/${first.path}`, workspaceID)} replace />;
  }
  const entryPath = getWorkspaceEntryPath(actions, section.value);
  if (entryPath !== section.path) {
    return <Navigate to={workspaceURL(entryPath, workspaceID)} replace />;
  }
  return <Card className="p-6 text-sm text-muted-foreground">暂无可查看的{section.label}页面</Card>;
}
