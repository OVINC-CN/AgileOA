import { Navigate, Outlet } from 'react-router';

import { SideNavigationLayout } from '@/components/side-navigation-layout';
import { systemNavigation } from '@/features/management/navigation';
import { useCurrentUser } from '@/features/user/api';

export function SystemLayout() {
  const { data: user } = useCurrentUser();
  if (!user?.is_admin) {
    return <Navigate to="/" replace />;
  }
  return (
    <SideNavigationLayout label="系统管理" items={systemNavigation}>
      <Outlet />
    </SideNavigationLayout>
  );
}
