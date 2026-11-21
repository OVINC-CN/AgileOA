import { useQuery } from '@tanstack/react-query';
import { Building, GraduationCap, Settings2 } from 'lucide-react';
import { useState } from 'react';
import { Link, NavLink, Outlet, useLocation } from 'react-router';

import { MobileNavigation } from '@/components/mobile-navigation';
import { NavigationMotion } from '@/components/navigation-motion';
import { Tooltip } from '@/components/ui/tooltip';
import { systemNavigation } from '@/features/management/navigation';
import { getStudentServiceNavigation } from '@/features/student-service/navigation';
import { useCurrentUser } from '@/features/user/api';
import { studentServicePath, studentServiceURL } from '@/features/user/navigation';
import { UserMenu } from '@/features/user/user-menu';
import { workspaceContextOptions } from '@/features/workspace/api';
import {
  getWorkspaceNavigation,
  getWorkspacePageItems,
  useActiveWorkspaceID,
  useWorkspaceSection,
  workspaceSections,
  workspaceURL,
} from '@/features/workspace/navigation';
import { WorkspaceSwitcher } from '@/features/workspace/workspace-switcher';
import { getConfig } from '@/lib/config';

const headerNavigationItemClass =
  'inline-flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-normal whitespace-nowrap text-foreground-secondary transition-colors hover:bg-muted focus-visible:outline-2 focus-visible:outline-ring aria-[current=page]:font-medium aria-[current=page]:text-accent-foreground aria-[current=page]:hover:bg-accent motion-reduce:transition-none';

export function AppLayout() {
  const config = getConfig();
  const { data: user } = useCurrentUser();
  const workspaceID = useActiveWorkspaceID();
  const section = useWorkspaceSection();
  const location = useLocation();
  const [switching, setSwitching] = useState(false);
  const [userBusy, setUserBusy] = useState(false);
  const student = user?.identity_type === 'student';
  const inSystemManagement =
    location.pathname === '/system' || location.pathname.startsWith('/system/');
  // 订阅当前空间的权限，由工作空间页面负责请求。
  const workspace = useQuery({ ...workspaceContextOptions(workspaceID), enabled: false });
  const sections =
    !student && !inSystemManagement && workspace.isSuccess
      ? workspaceSections.filter(
          ({ value }) => getWorkspaceNavigation(workspace.data.actions, value).length > 0,
        )
      : [];
  const menuItems = student
    ? [
        {
          id: 'student-service',
          to: studentServiceURL(user.workspace_id),
          label: '学生服务',
          icon: GraduationCap,
          isActive: location.pathname.startsWith(`${studentServicePath}/`),
          children: getStudentServiceNavigation(user.workspace_id, location),
        },
      ]
    : [
        ...sections.map(({ value, path, label, icon }) => ({
          id: value,
          to: workspaceURL(path, workspaceID),
          label,
          icon,
          isActive: location.pathname === path || location.pathname.startsWith(`${path}/`),
          children: getWorkspacePageItems(
            workspace.data?.actions ?? [],
            value,
            workspaceID,
            location,
          ),
        })),
        ...(user?.is_admin && inSystemManagement
          ? [
              {
                id: 'system',
                to: '/system',
                label: '系统管理',
                icon: Settings2,
                isActive: location.pathname.startsWith('/system/'),
                children: systemNavigation.map((item) => ({
                  ...item,
                  isActive: location.pathname === item.to,
                })),
              },
            ]
          : []),
      ];
  return (
    <div className="flex min-h-svh flex-col">
      <a
        href="#main-content"
        className="sr-only fixed top-2 left-2 z-50 rounded-md bg-primary p-3 text-primary-foreground focus:not-sr-only"
      >
        跳转到主要内容
      </a>
      <header className="sticky top-0 z-40 border-b border-border-subtle bg-background/85 backdrop-blur-xl">
        <div className="mx-auto flex min-h-14 max-w-[1520px] items-center gap-2 px-4 py-2 lg:grid lg:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] lg:gap-6 lg:px-8">
          <div className="flex min-w-0 flex-1 items-center gap-2 lg:gap-3">
            <NavLink
              to="/"
              aria-label={`${config.appName} 首页`}
              className="inline-flex h-9 shrink-0 items-center rounded-lg focus-visible:outline-2 focus-visible:outline-ring"
            >
              <img src="/agile.svg" alt={config.appName} className="h-4 w-auto lg:h-[18px]" />
            </NavLink>
            {user && !user.password_change_required ? (
              student ? (
                <span className="flex h-9 max-w-40 min-w-0 items-center gap-2 rounded-lg bg-muted/70 px-2 text-sm text-foreground sm:max-w-56 lg:px-3">
                  <Building className="size-4 shrink-0" aria-hidden="true" />
                  <Tooltip content={user.workspace_name} onlyOverflow>
                    <span className="max-w-[6em] min-w-0 truncate sm:max-w-none">
                      {user.workspace_name}
                    </span>
                  </Tooltip>
                </span>
              ) : (
                <WorkspaceSwitcher
                  key={user.id}
                  workspaceName={workspace.data?.workspace.name}
                  onBusyChange={setSwitching}
                  disabled={userBusy}
                />
              )
            ) : null}
            {user && !user.password_change_required && menuItems.length > 0 ? (
              <MobileNavigation
                key={`${user.identity_type}:${String(user.id)}:${location.key}`}
                items={menuItems}
                workspaceName={
                  student ? user.workspace_name : (workspace.data?.workspace.name ?? config.appName)
                }
                disabled={switching || userBusy}
              />
            ) : null}
          </div>
          {student ? (
            <NavigationMotion
              as="nav"
              value="student-service"
              aria-label="主导航"
              className="hidden shrink-0 items-center gap-1 lg:col-start-2 lg:flex"
            >
              <NavLink
                to={studentServiceURL(user.workspace_id)}
                aria-current="page"
                className={headerNavigationItemClass}
              >
                <GraduationCap className="size-4" aria-hidden="true" />
                学生服务
              </NavLink>
            </NavigationMotion>
          ) : sections.length > 0 ? (
            <NavigationMotion
              as="nav"
              value={section.value}
              aria-label="主导航"
              className="hidden shrink-0 items-center gap-1 lg:col-start-2 lg:flex"
            >
              {sections.map(({ value, path, label, icon: Icon }) => (
                <Link
                  key={value}
                  to={workspaceURL(path, workspaceID)}
                  aria-current={section.value === value ? 'page' : undefined}
                  className={headerNavigationItemClass}
                >
                  <Icon className="size-4" aria-hidden="true" />
                  {label}
                </Link>
              ))}
            </NavigationMotion>
          ) : null}
          <div className="flex shrink-0 items-center justify-end lg:col-start-3 lg:min-w-0">
            <UserMenu disabled={switching} onBusyChange={setUserBusy} />
          </div>
        </div>
      </header>
      <main
        id="main-content"
        className="mx-auto flex w-full max-w-[1520px] flex-1 flex-col px-4 py-6 lg:px-8"
      >
        <div className="flex min-w-0 flex-1 flex-col">
          <Outlet />
        </div>
      </main>
      <footer className="border-t border-border-subtle bg-background/80 px-4 py-4 text-center text-xs text-muted-foreground">
        © {config.copyrightYear} {config.copyrightOwner}®
      </footer>
    </div>
  );
}
