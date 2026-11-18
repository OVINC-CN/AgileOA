import { useLocation, useSearchParams } from 'react-router';

import { MonthlyWorkPage } from '@/features/monthly-work/monthly-work-page';
import { readWorkTarget } from '@/features/monthly-work/navigation';
import { useWorkspaceContext } from '@/features/workspace/context';
import { useWorkspaceSection } from '@/features/workspace/navigation';

export function WorkspaceMonthlyWorkPage() {
  const section = useWorkspaceSection();
  const location = useLocation();
  const [params] = useSearchParams();
  return (
    <MonthlyWorkPage
      key={location.key}
      {...useWorkspaceContext()}
      category={section.value}
      target={readWorkTarget(params)}
    />
  );
}
