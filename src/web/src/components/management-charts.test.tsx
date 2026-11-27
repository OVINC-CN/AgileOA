import { render, screen } from '@testing-library/react';
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
