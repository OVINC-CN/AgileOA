import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { expect, it, vi } from 'vitest';

import { AppLayout } from '@/components/app-layout';
import { SystemLayout } from '@/components/system-layout';
import { getStudentServiceNavigation } from '@/features/student-service/navigation';
import { getWorkspaceNavigation, getWorkspacePageItems } from '@/features/workspace/navigation';
import { WorkspaceIndexPage } from '@/pages/workspace-index-page';
import { WorkspacePage } from '@/pages/workspace-page';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const root = '/stu';
const cpcRoot = '/cpc';
const cpcPath = '/cpc/monthly-work';
const workspaceSearch = `?workspace=${workspaceID}`;
const workspaceAPI = `/v1/workspaces/${workspaceID}`;

function renderWorkspace(actions: string[], entry = root, suffix = '') {
  const fetchMock = vi.fn((path: string) =>
    Promise.resolve(
      Response.json({
        trace: '4bf92f3577b34da6a3ce929d0e0e4736',
        error_msg: '',
        data:
          path === '/v1/notifications/unread'
            ? { unread_count: 0 }
            : path === workspaceAPI
              ? {
                  workspace: {
                    id: workspaceID,
                    name: '虚构空间',
                    description: '',
                    is_enabled: true,
                    revision: 1,
                    created_at: '2026-11-03T00:00:00Z',
                    updated_at: '2026-11-03T00:00:00Z',
                  },
                  actions,
                }
              : {
                  id: 1,
                  username: 'fixture.admin',
                  display_name: '虚构管理员',
                  email: '',
                  is_admin: true,
                  password_login_enabled: true,
                  password_change_required: false,
                },
      }),
    ),
  );
  vi.stubGlobal('fetch', fetchMock);
  const router = createMemoryRouter(
    [
      {
        Component: AppLayout,
        children: [
          {
            path: root,
            Component: WorkspacePage,
            children: [
              { index: true, Component: WorkspaceIndexPage },
              { path: 'roster', element: <h1>学生信息页面</h1> },
              { path: 'projects', element: <h1>评奖评优页面</h1> },
              { path: 'graduation', element: <h1>毕业去向页面</h1> },
              { path: 'monthly-work', element: <h1>月度工作页面</h1> },
              { path: 'dashboard', element: <h1>学生数据仪表页面</h1> },
            ],
          },
          {
            path: cpcRoot,
            Component: WorkspacePage,
            children: [
              { index: true, Component: WorkspaceIndexPage },
              { path: 'monthly-work', element: <h1>党建月度工作页面</h1> },
              { path: 'dashboard', element: <h1>数据仪表页面</h1> },
              { path: 'data-management', element: <h1>党支部页面</h1> },
            ],
          },
          {
            path: '/system',
            Component: SystemLayout,
            children: [
              { path: 'users', element: <h1>用户管理页面</h1> },
              { path: 'audit', element: <h1>审计日志页面</h1> },
            ],
          },
        ],
      },
    ],
    { initialEntries: [`${entry}${workspaceSearch}${suffix}`] },
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, fetchMock };
}

it('仅有空间进入和非列表权限时隐藏顶部入口和空侧栏', async () => {
  const { router } = renderWorkspace(['access_workspace', 'project.statistics', 'student.detail']);
  expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument();
  expect(await screen.findByText('暂无可查看的学生工作页面')).toBeVisible();
  expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument();
  expect(screen.queryByRole('navigation', { name: '学生工作导航' })).not.toBeInTheDocument();
  expect(router.state.location.pathname).toBe(root);
  expect(router.state.location.search).toBe(workspaceSearch);
});

it.each([root, cpcRoot, cpcPath])('只有党建列表权限时从 %s 进入党建月度工作', async (entry) => {
  const { router } = renderWorkspace(['access_workspace', 'cpc.monthly_work.list'], entry);
  expect(await screen.findByRole('heading', { name: '党建月度工作页面' })).toBeVisible();
  const top = within(screen.getByRole('navigation', { name: '主导航' }));
  expect(top.queryByRole('link', { name: '学生工作' })).not.toBeInTheDocument();
  expect(top.getByRole('link', { name: '党建工作', current: 'page' })).toHaveAttribute(
    'href',
    `${cpcRoot}${workspaceSearch}`,
  );
  const sidebar = within(screen.getByRole('navigation', { name: '党建工作导航' }));
  expect(sidebar.getByRole('link', { name: '月度工作' })).toHaveAttribute(
    'href',
    `${cpcPath}${workspaceSearch}`,
  );
  expect(router.state.location.pathname).toBe(cpcPath);
});

it('两类工作为并列顶级入口，切换时保留当前空间', async () => {
  const { router } = renderWorkspace([
    'access_workspace',
    'monthly_work.list',
    'cpc.monthly_work.list',
  ]);
  await screen.findByRole('heading', { name: '月度工作页面' });
  const top = within(screen.getByRole('navigation', { name: '主导航' }));
  expect(top.getAllByRole('link').map((link) => link.textContent)).toEqual([
    '学生工作',
    '党建工作',
  ]);
  const user = userEvent.setup();
  await user.click(top.getByRole('link', { name: '党建工作' }));
  expect(await screen.findByRole('heading', { name: '党建月度工作页面' })).toBeVisible();
  expect(top.getAllByRole('link', { current: 'page' })).toHaveLength(1);
  expect(top.getByRole('link', { name: '党建工作', current: 'page' })).toBeVisible();
  expect(screen.queryByRole('navigation', { name: '学生工作导航' })).not.toBeInTheDocument();
  expect(router.state.location.search).toBe(workspaceSearch);
  expect(router.state.location.pathname).toBe(cpcPath);
  await user.click(top.getByRole('link', { name: '学生工作' }));
  expect(await screen.findByRole('heading', { name: '月度工作页面' })).toBeVisible();
  expect(top.getAllByRole('link', { current: 'page' })).toHaveLength(1);
  expect(top.getByRole('link', { name: '学生工作', current: 'page' })).toBeVisible();
});

it('党支部列表权限显示党建数据管理入口并默认进入该页面', async () => {
  const { router } = renderWorkspace(['access_workspace', 'cpc.party_branch.list'], cpcRoot);
  expect(await screen.findByRole('heading', { name: '党支部页面' })).toBeVisible();
  const top = within(screen.getByRole('navigation', { name: '主导航' }));
  expect(top.getByRole('link', { name: '党建工作' })).toBeVisible();
  const sidebar = within(screen.getByRole('navigation', { name: '党建工作导航' }));
  expect(sidebar.getAllByRole('link').map((link) => link.textContent)).toEqual([
    '数据管理',
    '党支部',
  ]);
  expect(router.state.location.pathname).toBe('/cpc/data-management');
});

it('数据管理在月度工作之后且紧邻侧栏折叠按钮上方', async () => {
  renderWorkspace(
    ['access_workspace', 'cpc.monthly_work.list', 'cpc.dashboard.list', 'cpc.party_branch.list'],
    cpcRoot,
  );
  await screen.findByRole('heading', { name: '党建月度工作页面' });
  const sidebar = screen.getByRole('navigation', { name: '党建工作导航' });
  const data = within(sidebar).getByRole('link', { name: '数据管理' });
  expect(
    within(sidebar)
      .getAllByRole('link')
      .map((link) => link.textContent),
  ).toEqual(['月度工作', '数据仪表', '数据管理']);
  expect(data.parentElement?.nextElementSibling).toBe(
    within(sidebar).getByRole('button', { name: '折叠侧栏' }),
  );
});

it('数据仪表使用独立列表权限，仅有下载权限时不显示导航', async () => {
  const { router } = renderWorkspace(['access_workspace', 'cpc.dashboard.list'], cpcRoot);
  expect(await screen.findByRole('heading', { name: '数据仪表页面' })).toBeVisible();
  expect(router.state.location.pathname).toBe('/cpc/dashboard');
  const sidebar = within(screen.getByRole('navigation', { name: '党建工作导航' }));
  expect(sidebar.getAllByRole('link').map((link) => link.textContent)).toEqual([
    '数据仪表',
    '支部党员表',
  ]);
});

it.each([
  { entry: root, label: '学生工作' },
  { entry: cpcRoot, label: '党建工作' },
])('只有党建非列表权限时从 $entry 进入显示空状态', async ({ entry, label }) => {
  const { router } = renderWorkspace(
    [
      'access_workspace',
      'cpc.monthly_work.create',
      'cpc.party_branch.create',
      'cpc.dashboard.export',
    ],
    entry,
  );
  expect(await screen.findByText(`暂无可查看的${label}页面`)).toBeVisible();
  expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument();
  expect(screen.queryByRole('navigation', { name: '学生工作导航' })).not.toBeInTheDocument();
  expect(screen.queryByRole('navigation', { name: '党建工作导航' })).not.toBeInTheDocument();
  expect(router.state.location.pathname).toBe(entry);
});

it('党建入口在只有学生列表权限时进入学生信息', async () => {
  const { router } = renderWorkspace(['access_workspace', 'student.list'], cpcRoot);
  expect(await screen.findByRole('heading', { name: '学生信息页面' })).toBeVisible();
  expect(router.state.location.pathname).toBe(`${root}/roster`);
  expect(router.state.location.search).toBe(workspaceSearch);
});

it.each([
  { actions: ['project.list'], links: ['评奖评优', '奖项管理'], first: 'projects' },
  { actions: ['project.participation.list'], links: ['评奖评优', '获奖学生'], first: 'projects' },
  { actions: ['graduation.list'], links: ['毕业去向'], first: 'graduation' },
  { actions: ['monthly_work.list'], links: ['月度工作'], first: 'monthly-work' },
  {
    actions: ['student.dashboard.list'],
    links: ['数据仪表', '学生专业表', '住宿信息表'],
    first: 'dashboard',
  },
  {
    actions: ['monthly_work.list', 'student.dashboard.list'],
    links: ['月度工作', '数据仪表'],
    first: 'monthly-work',
  },
  {
    actions: ['monthly_work.list', 'student.list'],
    links: ['学生信息', '月度工作'],
    first: 'roster',
  },
])('列表权限 $actions 决定两层导航和默认落页', async ({ actions, links, first }) => {
  const { router, fetchMock } = renderWorkspace(['access_workspace', ...actions]);
  expect(await screen.findByRole('heading')).toBeVisible();
  const top = within(screen.getByRole('navigation', { name: '主导航' }));
  expect(top.getByRole('link', { name: '学生工作' })).toHaveAttribute(
    'href',
    `${root}${workspaceSearch}`,
  );
  const sidebar = within(screen.getByRole('navigation', { name: '学生工作导航' }));
  expect(sidebar.getAllByRole('link').map((link) => link.textContent)).toEqual(links);
  expect(router.state.location.pathname).toBe(`${root}/${first}`);
  expect(router.state.location.search).toBe(workspaceSearch);
  expect(sidebar.getByRole('link', { current: 'page' })).toHaveAttribute(
    'href',
    `${root}/${first}${workspaceSearch}${
      first === 'projects'
        ? `&tab=${actions.includes('project.list') ? 'projects' : 'participations'}`
        : first === 'dashboard'
          ? '&tab=majors'
          : ''
    }`,
  );
  expect(fetchMock.mock.calls.filter(([path]) => path === workspaceAPI)).toHaveLength(1);
});

it('仅有学生仪表导出权限时隐藏学生工作导航', async () => {
  renderWorkspace(['access_workspace', 'student.dashboard.export']);
  expect(await screen.findByText('暂无可查看的学生工作页面')).toBeVisible();
  expect(screen.queryByRole('navigation', { name: '学生工作导航' })).not.toBeInTheDocument();
  expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument();
});

it('评奖评优入口仅由列表权限决定，移除填报查看权限后入口隐藏', () => {
  expect(
    getWorkspaceNavigation(['project.application.list']).some((item) => item.path === 'projects'),
  ).toBe(true);
  expect(getWorkspaceNavigation([]).some((item) => item.path === 'projects')).toBe(false);
});

it('评奖评优左侧子菜单按顺序展示，保留参数与锚点并支持历史导航', async () => {
  const { router } = renderWorkspace(
    ['access_workspace', 'project.list', 'project.participation.list', 'project.application.list'],
    '/stu/projects',
    '&tab=projects&application_id=12#details',
  );
  await screen.findByRole('heading', { name: '评奖评优页面' });
  const submenu = within(screen.getByRole('group', { name: '评奖评优子菜单' }));
  expect(submenu.getAllByRole('link').map((link) => link.textContent)).toEqual([
    '奖项管理',
    '获奖学生',
    '填报管理',
  ]);
  const participations = submenu.getByRole('link', { name: '获奖学生' });
  expect(participations).toHaveAttribute(
    'href',
    `/stu/projects${workspaceSearch}&tab=participations&application_id=12#details`,
  );
  const user = userEvent.setup();
  await user.click(participations);
  expect(submenu.getByRole('link', { name: '获奖学生', current: 'page' })).toBeVisible();
  expect(router.state.location.hash).toBe('#details');
  await act(() => router.navigate(-1));
  expect(submenu.getByRole('link', { name: '奖项管理', current: 'page' })).toBeVisible();
});

it('统一菜单按权限展示三级入口，切换后关闭并保留当前页参数', async () => {
  const { router } = renderWorkspace(
    ['access_workspace', 'project.list', 'project.participation.list', 'cpc.monthly_work.list'],
    '/stu/projects',
    '&tab=projects&application_id=12#details',
  );
  await screen.findByRole('heading', { name: '评奖评优页面' });
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '打开导航菜单' }));
  const menu = within(screen.getByRole('dialog', { name: '导航菜单' }));
  expect(
    within(menu.getByRole('navigation', { name: '全部导航' }))
      .getAllByRole('heading', { level: 2 })
      .map((heading) => heading.textContent),
  ).toEqual(['学生工作', '党建工作']);
  expect(menu.queryByRole('link', { name: '系统管理' })).not.toBeInTheDocument();
  expect(menu.getByRole('link', { name: '奖项管理', current: 'page' })).toBeVisible();
  expect(menu.queryByRole('link', { name: '填报管理' })).not.toBeInTheDocument();
  expect(menu.queryByRole('link', { name: '学生信息' })).not.toBeInTheDocument();
  expect(menu.getByRole('link', { name: '月度工作' })).toHaveAttribute(
    'href',
    `/cpc/monthly-work${workspaceSearch}`,
  );
  await user.click(menu.getByRole('link', { name: '获奖学生' }));
  expect(screen.queryByRole('dialog', { name: '导航菜单' })).not.toBeInTheDocument();
  expect(router.state.location.search).toBe(
    `${workspaceSearch}&tab=participations&application_id=12`,
  );
  expect(router.state.location.hash).toBe('#details');
});

it('完整子菜单复用页面目录，撤权回退与学生绑定空间保持一致', () => {
  const location = {
    pathname: '/cpc/data-management',
    search: `${workspaceSearch}&tab=party-branches&person_id=12&stage=one&stage=two`,
    hash: '#details',
  };
  const cpc = getWorkspacePageItems(
    ['cpc.person.list', 'cpc.timeline_template.list', 'cpc.dashboard.list'],
    'cpc',
    workspaceID,
    location,
  );
  expect(cpc.find(({ id }) => id === 'cpc-roster')?.children.map(({ label }) => label)).toEqual([
    '人员视图',
    '时间线视图',
  ]);
  const data = cpc.find(({ id }) => id === 'cpc-data-management');
  expect(data?.children.map(({ label }) => label)).toEqual(['时间线模板']);
  expect(data?.children[0]?.isActive).toBe(true);
  expect(data?.children[0]?.to).toBe(
    `/cpc/data-management${location.search.replace('tab=party-branches', 'tab=timeline-templates')}#details`,
  );
  const student = getStudentServiceNavigation(workspaceID, {
    pathname: '/student-service/projects',
    search: `${workspaceSearch}&tab=reviews&application_id=12`,
    hash: '#details',
  });
  expect(student.map(({ label }) => label)).toEqual(['个人信息', '评奖评优']);
  expect(student[1]?.children?.map(({ label }) => label)).toEqual(['我的申报', '我的审核']);
  expect(student[1]?.children?.[1]?.isActive).toBe(true);
  expect(student[1]?.children?.[0]?.to).toBe(
    `/student-service/projects${workspaceSearch}&tab=mine&application_id=12#details`,
  );
});

it('系统页面使用独立菜单，系统目录不与工作空间业务混排', async () => {
  const { router } = renderWorkspace(['access_workspace', 'student.list'], '/stu/roster');
  await screen.findByRole('heading', { name: '学生信息页面' });
  await act(() => router.navigate(`/system/users${workspaceSearch}`));
  await screen.findByRole('heading', { name: '用户管理页面' });
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '打开导航菜单' }));
  const menu = within(screen.getByRole('dialog', { name: '导航菜单' }));
  expect(
    within(menu.getByRole('navigation', { name: '全部导航' }))
      .getAllByRole('heading', { level: 2 })
      .map((heading) => heading.textContent),
  ).toEqual(['系统管理']);
  expect(menu.getByRole('link', { name: '用户管理', current: 'page' })).toBeVisible();
  expect(menu.queryByRole('link', { name: '学生工作' })).not.toBeInTheDocument();
  await user.click(menu.getByRole('link', { name: '审计日志' }));
  expect(router.state.location.pathname).toBe('/system/audit');
  expect(screen.queryByRole('dialog', { name: '导航菜单' })).not.toBeInTheDocument();
});
