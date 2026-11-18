import { useQuery } from '@tanstack/react-query';
import { Building, GraduationCap, Settings2 } from 'lucide-react';
import { useState } from 'react';
import { Link, NavLink, Outlet, useLocation } from 'react-router';

import { MobileNavigation } from '@/components/mobile-navigation';
import { NavigationMotion } from '@/components/navigation-motion';
import { Separator } from '@/components/ui/separator';
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
import { cn } from '@/lib/utils';

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
        <div
          className={cn(
            'mx-auto flex min-h-14 max-w-[1520px] items-center gap-1',
            'px-4 py-1 lg:px-8',
          )}
        >
          <div className="-ml-3 flex min-w-0 flex-1 items-center gap-1 lg:flex-initial">
            <NavLink
              to="/"
              aria-label={`${config.appName} 首页`}
              className="shrink-0 rounded-lg px-3 py-1.5"
            >
              <img src="/agile.svg" alt={config.appName} className="h-[18px] w-auto" />
            </NavLink>
            {user && !user.password_change_required ? (
              <>
                <Separator
                  orientation="vertical"
                  className="bg-border/80 data-[orientation=vertical]:h-4"
                />
                {student ? (
                  <span className="flex max-w-40 min-w-0 items-center gap-2 px-3 text-sm text-muted-foreground sm:max-w-56">
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
                )}
              </>
            ) : null}
            {user && !user.password_change_required && menuItems.length > 0 ? (
              <>
                <Separator
                  orientation="vertical"
                  className="bg-border/80 data-[orientation=vertical]:h-4 lg:hidden"
                />
                <MobileNavigation
                  key={`${user.identity_type}:${String(user.id)}:${location.key}`}
                  items={menuItems}
                  workspaceName={
                    student
                      ? user.workspace_name
                      : (workspace.data?.workspace.name ?? config.appName)
                  }
                  disabled={switching || userBusy}
                />
              </>
            ) : null}
          </div>
          {student ? (
            <>
              <Separator
                orientation="vertical"
                className="hidden bg-border/80 data-[orientation=vertical]:h-4 lg:block"
              />
              <NavigationMotion
                as="nav"
                value="student-service"
                aria-label="主导航"
                className="hidden gap-1 lg:flex"
              >
                <NavLink
                  to={studentServiceURL(user.workspace_id)}
                  aria-current="page"
                  className="inline-flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-medium whitespace-nowrap text-accent-foreground hover:bg-accent"
                >
                  <GraduationCap className="size-4" aria-hidden="true" />
                  学生服务
                </NavLink>
              </NavigationMotion>
            </>
          ) : sections.length > 0 ? (
            <>
              <Separator
                orientation="vertical"
                className="hidden bg-border/80 data-[orientation=vertical]:h-4 lg:block"
              />
              <NavigationMotion
                as="nav"
                value={section.value}
                aria-label="主导航"
                className="hidden gap-1 lg:flex"
              >
                {sections.map(({ value, path, label, icon: Icon }) => (
                  <Link
                    key={value}
                    to={workspaceURL(path, workspaceID)}
                    aria-current={section.value === value ? 'page' : undefined}
                    className={cn(
                      'inline-flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-medium whitespace-nowrap',
                      'transition-colors hover:bg-accent focus-visible:outline-2',
                      'focus-visible:outline-ring motion-reduce:transition-none',
                      section.value === value ? 'text-accent-foreground' : 'text-muted-foreground',
                    )}
                  >
                    <Icon className="size-4" aria-hidden="true" />
                    {label}
                  </Link>
                ))}
              </NavigationMotion>
            </>
          ) : null}
          <UserMenu disabled={switching} onBusyChange={setUserBusy} />
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
