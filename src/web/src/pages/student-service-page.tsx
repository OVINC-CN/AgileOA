import { Outlet, useLocation } from 'react-router';

import { SideNavigationLayout } from '@/components/side-navigation-layout';
import { getStudentServiceNavigation } from '@/features/student-service/navigation';
import { useCurrentUser } from '@/features/user/api';

export function StudentServicePage() {
  const { data: user } = useCurrentUser();
  const location = useLocation();
  const workspaceID = user?.workspace_id ?? '';
  return (
    <SideNavigationLayout
      label="学生服务"
      items={getStudentServiceNavigation(workspaceID, location)}
    >
      <Outlet key={workspaceID} context={{ workspaceID }} />
    </SideNavigationLayout>
  );
}
