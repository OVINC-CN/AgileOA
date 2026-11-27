import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it } from 'vitest';

import { AuditCharts, UserCharts } from '@/components/management-charts';

it.each([
  { Chart: UserCharts, title: '账号与登录趋势' },
  { Chart: AuditCharts, title: '操作结果趋势' },
])('$title不提供提示字段设置', ({ Chart, title }) => {
  render(<Chart data={undefined} loading={false} />);

  expect(screen.getByRole('figure', { name: title })).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '提示字段' })).not.toBeInTheDocument();
});

it('用户管理使用可切换的趋势图例', async () => {
  render(<UserCharts data={undefined} loading={false} />);
  const legend = within(screen.getByRole('group', { name: '趋势图例' }));
  const logins = legend.getByRole('button', { name: '成功登录' });
  const created = legend.getByRole('button', { name: '新增账号' });
  expect(logins).toHaveAttribute('aria-pressed', 'true');
  const user = userEvent.setup();
  await user.click(logins);
  expect(logins).toHaveAttribute('aria-pressed', 'false');
  expect(created).toHaveAttribute('aria-pressed', 'true');
});
