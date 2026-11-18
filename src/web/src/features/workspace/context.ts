import { useOutletContext } from 'react-router';
export interface WorkspaceContext {
  workspaceID: string;
  workspaceName: string;
  actions: string[];
  canReviewApplications?: boolean;
  refresh: () => Promise<unknown>;
}
export function useWorkspaceContext() {
  return useOutletContext<WorkspaceContext>();
}
