import {
  Award,
  BriefcaseBusiness,
  CalendarDays,
  Database,
  Flag,
  GraduationCap,
  LayoutDashboard,
  UsersRound,
} from 'lucide-react';
import { type Location, useMatch, useSearchParams } from 'react-router';

import type { WorkCategory } from '@/features/monthly-work/category';
import { workspaceIDSchema } from '@/features/workspace/api';

export const studentRootPath = '/stu';
export const cpcRootPath = '/cpc';
export const workspaceQueryParam = 'workspace';

export const workspaceSections = [
  { value: 'student', path: studentRootPath, label: '学生工作', icon: GraduationCap },
  { value: 'cpc', path: cpcRootPath, label: '党建工作', icon: Flag },
] as const;

export function useWorkspaceSection() {
  const cpc = useMatch(`${cpcRootPath}/*`);
  return workspaceSections[cpc ? 1 : 0];
}

export function workspaceURL(path: string, workspaceID: string) {
  const params = new URLSearchParams({ [workspaceQueryParam]: workspaceID });
  return `${path}?${params.toString()}`;
}

export function useActiveWorkspaceID() {
  const student = useMatch(`${studentRootPath}/*`);
  const cpc = useMatch(`${cpcRootPath}/*`);
  const [params] = useSearchParams();
  if (!student && !cpc) return '';

  const id = workspaceIDSchema.safeParse(params.get(workspaceQueryParam));
  return id.success ? id.data : '';
}

export const projectTabs = [
  { value: 'projects', label: '奖项管理', listAction: 'project.list' },
  { value: 'participations', label: '获奖学生', listAction: 'project.participation.list' },
  { value: 'applications', label: '填报管理', listAction: 'project.application.list' },
] as const;

export const cpcDataTabs = [
  {
    value: 'party-branches',
    label: '党支部',
    group: 'cpc_party_branch',
    listAction: 'cpc.party_branch.list',
  },
  {
    value: 'timeline-templates',
    label: '时间线模板',
    group: 'cpc_timeline_template',
    listAction: 'cpc.timeline_template.list',
  },
] as const;

export const cpcPeopleTabs = [
  { value: 'people', label: '人员视图' },
  { value: 'timeline', label: '时间线视图' },
] as const;

export const studentDashboardTabs = [
  { value: 'majors', label: '学生专业表' },
  { value: 'accommodation', label: '住宿信息表' },
] as const;

export const cpcDashboardTabs = [{ value: 'branches', label: '支部党员表' }] as const;

export const studentDashboardNavigation = {
  path: 'dashboard',
  label: '数据仪表',
  icon: LayoutDashboard,
  group: 'student_dashboard',
  listActions: ['student.dashboard.list'],
};

const workspaceNavigation = [
  { path: 'roster', label: '学生信息', icon: UsersRound, listActions: ['student.list'] },
  {
    path: 'projects',
    label: '评奖评优',
    icon: Award,
    listActions: projectTabs.map(({ listAction }) => listAction),
  },
  {
    path: 'graduation',
    label: '毕业去向',
    icon: BriefcaseBusiness,
    listActions: ['graduation.list'],
  },
  {
    path: 'monthly-work',
    label: '月度工作',
    icon: CalendarDays,
    listActions: ['monthly_work.list'],
  },
  studentDashboardNavigation,
];

export const cpcDashboardNavigation = {
  path: 'dashboard',
  label: '数据仪表',
  icon: LayoutDashboard,
  group: 'cpc_dashboard',
  listActions: ['cpc.dashboard.list'],
};

const cpcNavigation = [
  { path: 'roster', label: '人员名单', icon: UsersRound, listActions: ['cpc.person.list'] },
  {
    path: 'monthly-work',
    label: '月度工作',
    icon: CalendarDays,
    listActions: ['cpc.monthly_work.list'],
  },
  cpcDashboardNavigation,
  {
    path: 'data-management',
    label: '数据管理',
    icon: Database,
    listActions: cpcDataTabs.map(({ listAction }) => listAction),
  },
];

export function getWorkspaceNavigation(actions: string[], category: WorkCategory = 'student') {
  return (category === 'cpc' ? cpcNavigation : workspaceNavigation).filter(({ listActions }) =>
    listActions.some((action) => actions.includes(action)),
  );
}

type NavigationLocation = Pick<Location, 'pathname' | 'search' | 'hash'>;
interface NavigationTab {
  value: string;
  label: string;
  listAction?: string;
}

// 各级菜单共用视图目录，当前页链接保留空间、定位条件及锚点。
export function getTabMenuItems(
  id: string,
  pathname: string,
  workspaceID: string,
  tabs: readonly NavigationTab[],
  location: NavigationLocation,
) {
  const current = location.pathname === pathname;
  const requestedTab = new URLSearchParams(location.search).get('tab');
  const selected = tabs.find(({ value }) => value === requestedTab) ?? tabs[0];

  return tabs.map(({ value, label }) => {
    const params = new URLSearchParams(current ? location.search : '');
    params.set(workspaceQueryParam, workspaceID);
    params.set('tab', value);

    return {
      id: `${id}-tab-${value}`,
      to: `${pathname}?${params.toString()}${current ? location.hash : ''}`,
      label,
      isActive: current && selected?.value === value,
    };
  });
}

function getPageTabs(path: string, category: WorkCategory): readonly NavigationTab[] {
  if (category === 'student') {
    if (path === 'projects') return projectTabs;
    if (path === 'dashboard') return studentDashboardTabs;
  } else {
    if (path === 'roster') return cpcPeopleTabs;
    if (path === 'dashboard') return cpcDashboardTabs;
    if (path === 'data-management') return cpcDataTabs;
  }
  return [];
}

export function getWorkspacePageItems(
  actions: string[],
  category: WorkCategory,
  workspaceID: string,
  location: NavigationLocation,
) {
  const section = workspaceSections[category === 'cpc' ? 1 : 0];

  return getWorkspaceNavigation(actions, category).map(({ path, label, icon }) => {
    const pathname = `${section.path}/${path}`;
    const tabs = getPageTabs(path, category).filter(
      ({ listAction }) => listAction === undefined || actions.includes(listAction),
    );
    const tabID = category === 'student' && path === 'projects' ? 'project' : `${category}-${path}`;

    return {
      id: `${category}-${path}`,
      to: workspaceURL(pathname, workspaceID),
      label,
      icon,
      isActive: location.pathname === pathname,
      children: getTabMenuItems(tabID, pathname, workspaceID, tabs, location),
    };
  });
}

export function getWorkspaceEntryPath(actions: string[], preferred: WorkCategory) {
  const section = workspaceSections[preferred === 'cpc' ? 1 : 0];
  if (getWorkspaceNavigation(actions, preferred).length > 0) return section.path;
  return (
    workspaceSections.find(({ value }) => getWorkspaceNavigation(actions, value).length > 0)
      ?.path ?? section.path
  );
}
