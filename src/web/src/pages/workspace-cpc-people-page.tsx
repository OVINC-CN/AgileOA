import { PersonPage } from '@/features/cpc-person/person-page';
import { useWorkspaceContext } from '@/features/workspace/context';

export function WorkspaceCPCPeoplePage() {
  return <PersonPage {...useWorkspaceContext()} />;
}
