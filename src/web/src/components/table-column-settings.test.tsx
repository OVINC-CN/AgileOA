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

it('字段抽屉即时更新表头、数据及空态跨度，恢复全部不触发请求或分页', async () => {
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
  await user.click(screen.getByRole('button', { name: '显示字段' }));
  const drawer = within(screen.getByRole('dialog', { name: '显示字段' }));
  await user.click(drawer.getByRole('checkbox', { name: '邮箱' }));
  await user.click(drawer.getByRole('checkbox', { name: '姓名' }));
  expect(drawer.getByRole('checkbox', { name: '专业' })).toBeDisabled();
  await user.click(drawer.getByRole('button', { name: '完成' }));

  const table = within(screen.getByRole('table'));
  expect(table.getAllByRole('columnheader').map((header) => header.textContent)).toEqual(['专业']);
  expect(table.queryByText('a@example.com')).not.toBeInTheDocument();
  expect(table.queryByText('张同学')).not.toBeInTheDocument();
  expect(table.getByRole('cell', { name: '数学' })).toBeVisible();
  expect(localStorage.getItem('table-hidden-columns:fixture-settings')).toBe('["email","name"]');

  view.rerender(<ManagementTable {...props} data={[]} />);
  expect(table.getByRole('cell', { name: '暂无记录' })).toHaveAttribute('colspan', '1');
  await user.click(screen.getByRole('button', { name: '显示字段' }));
  await user.click(screen.getByRole('button', { name: '恢复全部' }));
  await user.click(screen.getByRole('button', { name: '完成' }));
  expect(table.getAllByRole('columnheader')).toHaveLength(3);
  expect(table.getByRole('cell', { name: '暂无记录' })).toHaveAttribute('colspan', '3');
  expect(localStorage.getItem('table-hidden-columns:fixture-settings')).toBeNull();
  expect(fetch).not.toHaveBeenCalled();
  expect(onPage).not.toHaveBeenCalled();
  expect(onPageSize).not.toHaveBeenCalled();
});
