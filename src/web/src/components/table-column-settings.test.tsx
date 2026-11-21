import type { ColumnDef } from '@tanstack/react-table';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { ManagementTable } from '@/components/management-table';

const columns: ColumnDef<{ name: string; email: string; major: string }>[] = [
  { accessorKey: 'name', header: '姓名' },
  { accessorKey: 'email', header: '邮箱' },
  { accessorKey: 'major', header: '专业' },
];

afterEach(() => {
  localStorage.clear();
  vi.unstubAllGlobals();
});

it('字段浮层即时更新表头、数据及空态跨度，恢复全部不触发请求或分页', async () => {
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  const onPage = vi.fn(() => Promise.resolve());
  const onPageSize = vi.fn(() => Promise.resolve());
  const props = {
    tableID: 'fixture-settings',
    columns,
    total: 1,
    page: 1,
    pageSize: 5,
    loading: false,
    onPage,
    onPageSize,
    emptyMessage: '暂无记录',
  };
  const view = render(
    <ManagementTable
      {...props}
      data={[{ name: '张同学', email: 'a@example.com', major: '数学' }]}
    />,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '列显示' }));
  const panel = within(screen.getByRole('dialog', { name: '列显示' }));
  expect(panel.getByRole('searchbox', { name: '搜索列名称' })).toHaveFocus();
  await user.click(panel.getByRole('checkbox', { name: '邮箱' }));
  await user.click(panel.getByRole('checkbox', { name: '姓名' }));
  expect(panel.getByRole('checkbox', { name: '专业' })).toBeDisabled();
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('dialog', { name: '列显示' })).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: '列显示' })).toHaveFocus();

  const table = within(screen.getByRole('table'));
  expect(table.getAllByRole('columnheader').map((header) => header.textContent)).toEqual(['专业']);
  expect(table.queryByText('a@example.com')).not.toBeInTheDocument();
  expect(table.queryByText('张同学')).not.toBeInTheDocument();
  expect(table.getByRole('cell', { name: '数学' })).toBeVisible();
  expect(localStorage.getItem('table-hidden-columns:fixture-settings')).toBe('["email","name"]');

  view.rerender(<ManagementTable {...props} data={[]} />);
  expect(table.getByRole('cell', { name: '暂无记录' })).toHaveAttribute('colspan', '1');
  await user.click(screen.getByRole('button', { name: '列显示' }));
  await user.click(screen.getByRole('button', { name: '恢复全部' }));
  await user.keyboard('{Escape}');
  expect(table.getAllByRole('columnheader')).toHaveLength(3);
  expect(table.getByRole('cell', { name: '暂无记录' })).toHaveAttribute('colspan', '3');
  expect(localStorage.getItem('table-hidden-columns:fixture-settings')).toBeNull();
  expect(fetch).not.toHaveBeenCalled();
  expect(onPage).not.toHaveBeenCalled();
  expect(onPageSize).not.toHaveBeenCalled();
});

it('搜索字段只过滤选项，保留显示设置，恢复全部覆盖未匹配字段', async () => {
  render(
    <ManagementTable
      tableID="fixture-search-settings"
      columns={columns}
      data={[{ name: '张同学', email: 'a@example.com', major: '数学' }]}
      total={1}
      page={1}
      pageSize={5}
      loading={false}
      onPage={() => Promise.resolve()}
      onPageSize={() => Promise.resolve()}
    />,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '列显示' }));
  const panel = within(screen.getByRole('dialog', { name: '列显示' }));
  expect(panel.getByRole('searchbox', { name: '搜索列名称' })).toHaveFocus();
  await user.click(panel.getByRole('checkbox', { name: '邮箱' }));
  await user.type(panel.getByRole('searchbox', { name: '搜索列名称' }), '专业');
  expect(panel.getAllByRole('checkbox')).toHaveLength(1);
  expect(
    screen.queryByRole('columnheader', { name: '邮箱', hidden: true }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('columnheader', { name: '姓名', hidden: true })).toBeVisible();
  await user.click(panel.getByRole('button', { name: '恢复全部' }));
  expect(screen.getByRole('columnheader', { name: '邮箱', hidden: true })).toBeVisible();
  await user.clear(panel.getByRole('searchbox', { name: '搜索列名称' }));
  expect(panel.getByRole('checkbox', { name: '邮箱' })).toBeChecked();
});
