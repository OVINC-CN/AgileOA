import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LayoutDashboard, UsersRound } from 'lucide-react';
import { createMemoryRouter, RouterProvider, useLocation } from 'react-router';
import { expect, it } from 'vitest';

import { type NavigationItem, SideNavigationLayout } from '@/components/side-navigation-layout';

const workspaceQuery = 'workspace=w7m2k9p4s6n8c3x5&year=2026';

function Example() {
  const location = useLocation();
  const currentTab = new URLSearchParams(location.search).get('tab');
  const items: NavigationItem[] = [
    { to: '/stu/roster', label: '学生信息', icon: UsersRound },
    {
      to: `/stu/dashboard?${workspaceQuery}`,
      label: '数据仪表',
      icon: LayoutDashboard,
      children: [
        {
          id: 'majors',
          label: '学生专业表',
          to: `/stu/dashboard?${workspaceQuery}&tab=majors#results`,
          isActive: location.pathname === '/stu/dashboard' && currentTab === 'majors',
        },
        {
          id: 'accommodation',
          label: '住宿信息表',
          to: `/stu/dashboard?${workspaceQuery}&tab=accommodation#results`,
          isActive: location.pathname === '/stu/dashboard' && currentTab === 'accommodation',
        },
      ],
    },
  ];
  return (
    <SideNavigationLayout label="学生工作" items={items}>
      <p>当前页面</p>
    </SideNavigationLayout>
  );
}

function renderNavigation() {
  const router = createMemoryRouter([{ path: '*', Component: Example }], {
    initialEntries: [`/stu/dashboard?${workspaceQuery}&tab=accommodation#results`],
  });
  render(<RouterProvider router={router} />);
  return router;
}

it('收起后只保留主菜单入口，完整子菜单就地展开，Escape 返回图标按钮', async () => {
  renderNavigation();
  const user = userEvent.setup();
  const aside = screen.getByRole('complementary', { name: '学生工作' });
  const navigation = within(aside).getByRole('navigation', { name: '学生工作导航' });
  const collapse = within(aside).getByRole('button', { name: '折叠侧栏' });
  expect(navigation).not.toContainElement(collapse);
  expect(within(navigation).getByRole('link', { name: '住宿信息表' })).toHaveAttribute(
    'aria-current',
    'page',
  );

  await user.click(collapse);
  expect(within(navigation).queryByRole('link', { name: '住宿信息表' })).not.toBeInTheDocument();
  expect(within(navigation).queryByText('学生')).not.toBeInTheDocument();
  expect(within(navigation).queryByText('住宿')).not.toBeInTheDocument();
  const trigger = within(navigation).getByRole('button', { name: '数据仪表' });
  expect(trigger).toHaveAttribute('aria-current', 'page');
  await user.click(trigger);
  const menu = within(screen.getByRole('dialog', { name: '数据仪表子菜单' }));
  expect(menu.getByRole('link', { name: '学生专业表' })).toHaveTextContent('学生专业表');
  expect(menu.getByRole('link', { name: '住宿信息表' })).toHaveAttribute('aria-current', 'page');
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();

  await user.click(within(aside).getByRole('button', { name: '展开侧栏' }));
  expect(within(navigation).getByRole('link', { name: '住宿信息表' })).toBeVisible();
});

it('收起后的子菜单保留完整链接，普通选择关闭浮层并保持侧栏收起', async () => {
  const router = renderNavigation();
  const user = userEvent.setup();
  const aside = screen.getByRole('complementary', { name: '学生工作' });
  await user.click(within(aside).getByRole('button', { name: '折叠侧栏' }));
  await user.click(within(aside).getByRole('button', { name: '数据仪表' }));
  const menu = within(screen.getByRole('dialog', { name: '数据仪表子菜单' }));
  const link = menu.getByRole('link', { name: '学生专业表' });
  expect(link).toHaveAttribute('href', `/stu/dashboard?${workspaceQuery}&tab=majors#results`);
  await user.click(link);
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(router.state.location.pathname).toBe('/stu/dashboard');
  expect(router.state.location.search).toBe(`?${workspaceQuery}&tab=majors`);
  expect(router.state.location.hash).toBe('#results');
  expect(within(aside).getByRole('button', { name: '展开侧栏' })).toBeVisible();
});
