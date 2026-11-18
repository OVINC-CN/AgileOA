import type { ReactNode } from 'react';
import { useLocation } from 'react-router';

import { SideNavigationLayout } from '@/components/side-navigation-layout';
import { getStudentServiceNavigation } from '@/features/student-service/navigation';
import { getWorkspacePageItems, useWorkspaceSection } from '@/features/workspace/navigation';

export function PageNavigationHarness({
  workspaceID,
  actions,
  children,
  studentService = false,
}: {
  workspaceID: string;
  actions: string[];
  children: ReactNode;
  studentService?: boolean;
}) {
  const location = useLocation();
  const section = useWorkspaceSection();
  const items = studentService
    ? getStudentServiceNavigation(workspaceID, location)
    : getWorkspacePageItems(actions, section.value, workspaceID, location);

  return (
    <SideNavigationLayout label={studentService ? '学生服务' : section.label} items={items}>
      {children}
    </SideNavigationLayout>
  );
}
