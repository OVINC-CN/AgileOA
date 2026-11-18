import { useOutletContext } from 'react-router';

export function useStudentServiceContext() {
  return useOutletContext<{ workspaceID: string }>();
}
