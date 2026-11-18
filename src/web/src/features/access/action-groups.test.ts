import { describe, expect, it } from 'vitest';

import { getActionGroups } from '@/features/access/action-groups';
import type { Action } from '@/features/access/api';

const branchActions: Action[] = ['list', 'create', 'update', 'delete', 'import', 'export'].map(
  (value) => ({
    value: `cpc.party_branch.${value}`,
    label: `党支部${value}`,
    description: '当前空间内独立授权',
    group: { value: 'cpc_party_branch', label: '党建工作 · 党支部' },
    scope: 'workspace',
    endpoints: [],
  }),
);
const monthlyWork: Action = {
  value: 'cpc.monthly_work.list',
  label: '查看党建月度工作',
  description: '当前空间内独立授权',
  group: { value: 'cpc_monthly_work', label: '党建工作 · 月度工作' },
  scope: 'workspace',
  endpoints: [],
};

it.each(['list', 'export'])('学生仪表 %s 权限归入学生工作，且不混入党建仪表', (operation) => {
  const studentWork: Action = {
    ...monthlyWork,
    value: 'monthly_work.list',
    group: { value: 'monthly_work', label: '[学生工作] 月度工作' },
  };
  const dashboard: Action = {
    ...monthlyWork,
    value: `student.dashboard.${operation}`,
    group: { value: 'student_dashboard', label: '[学生工作] 数据仪表' },
  };
  const cpcDashboard: Action = {
    ...monthlyWork,
    value: 'cpc.dashboard.export',
    group: { value: 'cpc_dashboard', label: '[党建工作] 数据仪表' },
  };
  const groups = getActionGroups([dashboard, cpcDashboard, studentWork]);
  expect(groups.map(({ label }) => label)).toEqual(['学生工作', '党建工作']);
  expect(groups[0]?.children?.map(({ label }) => label)).toEqual(['月度工作', '数据仪表']);
  expect(groups[0]?.children?.[1]?.actions).toEqual([dashboard]);
  expect(groups[1]?.children?.[0]?.actions).toEqual([cpcDashboard]);
});

describe('党建数据管理权限分组', () => {
  it.each(['list', 'export'])('仪表 %s 权限始终位于数据管理上方', (operation) => {
    const dashboard: Action = {
      ...monthlyWork,
      value: `cpc.dashboard.${operation}`,
      group: { value: 'cpc_dashboard', label: '[党建工作] 数据仪表' },
    };
    const groups = getActionGroups([...branchActions, dashboard, monthlyWork]);
    expect(groups.map(({ label }) => label)).toEqual(['党建工作']);
    expect(groups[0]?.children?.map(({ label }) => label)).toEqual([
      '月度工作',
      '数据仪表',
      '数据管理',
    ]);
    expect(groups[0]?.children?.[1]?.actions).toEqual([dashboard]);
  });

  it('模板权限按导航顺序位于党支部之后，不因缺少列表权限散落到外层', () => {
    const template: Action = {
      ...monthlyWork,
      value: 'cpc.timeline_template.update',
      group: { value: 'cpc_timeline_template', label: '[党建工作] 时间线模板' },
    };
    const groups = getActionGroups([...branchActions, template]);
    const data = groups[0]?.children?.find((item) => item.value === 'cpc_data');
    expect(data?.children?.map((item) => item.label)).toEqual(['党支部', '时间线模板']);
    expect(data?.children?.[1]?.actions).toEqual([template]);
  });
  it.each([
    { actions: branchActions },
    { actions: branchActions.filter(({ value }) => !value.endsWith('.list')) },
  ])('党支部操作始终位于党建工作、数据管理的子层级', ({ actions }) => {
    const groups = getActionGroups([...actions, monthlyWork]);
    expect(groups.map(({ label }) => label)).toEqual(['党建工作']);
    const cpc = groups[0];
    expect(cpc?.children?.map(({ label }) => label)).toEqual(['月度工作', '数据管理']);
    const data = cpc?.children?.[1];
    expect(data?.children?.map(({ label }) => label)).toEqual(['党支部']);
    expect(data?.children?.[0]?.actions).toEqual(actions);
    expect(cpc?.actions).toHaveLength(actions.length + 1);
  });
});
