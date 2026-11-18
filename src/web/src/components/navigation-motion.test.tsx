import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { expect, it } from 'vitest';

import { NavigationTabs } from '@/components/navigation-motion';
import { useNavigationTabs } from '@/lib/navigation-tabs';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const tabs = [
  { value: 'people', label: '人员视图' },
  { value: 'timeline', label: '时间线视图' },
];

function TabPage({ options }: { options: typeof tabs }) {
  const { selected, items } = useNavigationTabs(options);
  return (
    <>
      <NavigationTabs id="test" label="页面视图" value={selected?.value} items={items} />
      {selected ? (
        <div
          role="tabpanel"
          id={`test-panel-${selected.value}`}
          aria-labelledby={`test-tab-${selected.value}`}
        >
          {selected.label}内容
        </div>
      ) : null}
    </>
  );
}

function renderTabs(entry: string, options = tabs) {
  const router = createMemoryRouter(
    [
      { path: '/origin', element: <p>来源页面</p> },
      { path: '/page', element: <TabPage options={options} /> },
    ],
    { initialEntries: ['/origin', entry] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

it('直达非默认 Tab，切换及前进后退保留空间、重复查询参数和锚点', async () => {
  const user = userEvent.setup();
  const search = `?workspace=${workspaceID}&person_id=12&stage=one&stage=two&tab=timeline`;
  const router = renderTabs(`/page${search}#details`);
  expect(screen.getByRole('tabpanel', { name: '时间线视图' })).toBeVisible();
  expect(screen.getByRole('tab', { name: '人员视图' })).toHaveAttribute(
    'href',
    `/page${search.replace('tab=timeline', 'tab=people')}#details`,
  );

  await user.click(screen.getByRole('tab', { name: '人员视图' }));
  expect(screen.getByRole('tabpanel', { name: '人员视图' })).toBeVisible();
  expect(router.state.location.search).toBe(search.replace('tab=timeline', 'tab=people'));
  expect(router.state.location.hash).toBe('#details');

  await act(() => router.navigate(-1));
  expect(screen.getByRole('tabpanel', { name: '时间线视图' })).toBeVisible();
  await act(() => router.navigate(1));
  expect(screen.getByRole('tabpanel', { name: '人员视图' })).toBeVisible();
});

it.each(['', '&tab=invalid', '&tab=timeline'])(
  '缺省、无效或无权限的 Tab %s 回退到唯一可访问项且替换历史记录',
  async (query) => {
    const router = renderTabs(`/page?workspace=${workspaceID}${query}`, [
      { value: 'people', label: '人员视图' },
    ]);
    const expected = `?workspace=${workspaceID}&tab=people`;
    await waitFor(() => {
      expect(router.state.location.search).toBe(expected);
    });
    expect(screen.getByRole('tab', { name: '人员视图', selected: true })).toHaveAttribute(
      'href',
      `/page${expected}`,
    );
    expect(screen.queryByRole('tab', { name: '时间线视图' })).not.toBeInTheDocument();
    await act(() => router.navigate(-1));
    expect(screen.getByText('来源页面')).toBeVisible();
  },
);

it('没有可访问项时隐藏 Tab 且不修改 URL', () => {
  const search = `?workspace=${workspaceID}&tab=timeline`;
  const router = renderTabs(`/page${search}`, []);
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(screen.queryByRole('tabpanel')).not.toBeInTheDocument();
  expect(router.state.location.search).toBe(search);
});

it('方向键和首尾键同步焦点与 URL，修饰键保留浏览器快捷键', async () => {
  const user = userEvent.setup();
  const router = renderTabs(`/page?workspace=${workspaceID}&tab=people`);
  screen.getByRole('tab', { name: '人员视图' }).focus();
  await user.keyboard('{ArrowRight}');
  expect(screen.getByRole('tab', { name: '时间线视图', selected: true })).toHaveFocus();
  expect(new URLSearchParams(router.state.location.search).get('tab')).toBe('timeline');
  await user.keyboard('{Home}');
  expect(screen.getByRole('tab', { name: '人员视图', selected: true })).toHaveFocus();
  await user.keyboard('{End}');
  expect(screen.getByRole('tab', { name: '时间线视图', selected: true })).toHaveFocus();
  await user.keyboard('{Alt>}{ArrowLeft}{/Alt}');
  expect(screen.getByRole('tab', { name: '时间线视图', selected: true })).toHaveFocus();
  screen.getByRole('tab', { name: '人员视图' }).focus();
  await user.keyboard(' ');
  expect(screen.getByRole('tabpanel', { name: '人员视图' })).toBeVisible();
});
