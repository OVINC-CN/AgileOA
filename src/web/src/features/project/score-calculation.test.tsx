import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, expect, it, vi } from 'vitest';

import type { Project } from '@/features/project/api';
import { ProjectDetail } from '@/features/project/project-detail';
import {
  cleanScoreCalculation,
  emptyScoreCalculation,
  type ScoreCalculation,
  scoreCalculationSchema,
  scoreReference,
} from '@/features/project/score-calculation';
import { ScoreCalculationEditor } from '@/features/project/score-calculation-editor';
import { ScoreCalculationPreview } from '@/features/project/score-calculation-preview';

afterEach(() => {
  vi.unstubAllGlobals();
});

function Editor() {
  const [value, setValue] = useState<ScoreCalculation>(emptyScoreCalculation());
  return (
    <ScoreCalculationEditor
      workspaceID="w7m2k9p4s6n8c3x5"
      value={value}
      names={['子项A', '子项B']}
      onChange={setValue}
    />
  );
}

it('项目详情用条件树和公式代码块展示配置，不显示编辑控件或发起试算', async () => {
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  const formula = 'MIN([子项A], 10) +\n  MAX([子项B], 0)';
  const project: Project = {
    id: 1,
    revision: 1,
    name: '虚构年度项目',
    year: 2026,
    description: '',
    detail_names: ['子项A', '子项B'],
    attachment_ids: [],
    allow_self_apply: false,
    self_apply_deadline: null,
    approval_steps: [],
    created_at: '2026-11-03T00:00:00Z',
    updated_at: '2026-11-03T00:00:00Z',
    score_calculation: {
      enabled: true,
      ranking_dimensions: [],
      rules: [
        {
          condition: {
            type: 'group',
            logic: 'and',
            children: [
              { type: 'condition', field: 'grade', operator: 'in', values: ['2026'] },
              { type: 'condition', field: 'education_level', operator: 'in', values: ['博士'] },
              {
                type: 'group',
                logic: 'or',
                children: [
                  { type: 'condition', field: 'major', operator: 'in', values: ['软件工程'] },
                  { type: 'condition', field: 'class_name', operator: 'in', values: ['01班'] },
                ],
              },
            ],
          },
          formula,
        },
      ],
    },
  };
  render(
    <ProjectDetail
      workspaceID="w7m2k9p4s6n8c3x5"
      project={project}
      canReadAttachments={false}
      onClose={vi.fn()}
    />,
  );
  await userEvent.setup().click(screen.getByRole('tab', { name: '分数计算' }));
  const detail = within(screen.getByRole('tabpanel', { name: '分数计算' }));
  const tree = within(detail.getByRole('list', { name: '第 1 组条件' }));
  expect(tree.getByText('全部满足（AND）')).toBeVisible();
  expect(tree.getByText('任一满足（OR）')).toBeVisible();
  for (const text of ['年级', '2026', '学历', '博士', '专业', '软件工程', '班级', '01班']) {
    expect(tree.getByText(text)).toBeVisible();
  }
  expect(detail.getByText('开启')).toBeVisible();
  expect(detail.getByText('全奖项统一排名')).toBeVisible();
  const code = detail.getByText(/MIN\(\[子项A\], 10\)/u, { selector: 'code' });
  expect(code.textContent).toBe(formula);
  expect(code.parentElement?.tagName).toBe('PRE');
  for (const role of ['checkbox', 'combobox', 'textbox', 'button']) {
    expect(detail.queryByRole(role)).not.toBeInTheDocument();
  }
  expect(fetch).not.toHaveBeenCalled();
});

it.each([
  { score: 0, display: '0.00', field: 'major', label: '专业', value: '软件工程' },
  {
    score: 46.480000000000004,
    display: '46.48',
    field: 'education_level',
    label: '学历',
    value: '硕士',
  },
] as const)(
  '自动试算未保存配置，将 $score 展示为两位小数，修改输入后隐藏旧结果',
  async ({ score, display, field, label, value }) => {
    const bodies: { path: string; body: unknown }[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string, options: RequestInit) => {
        bodies.push({
          path,
          body: JSON.parse(typeof options.body === 'string' ? options.body : '') as unknown,
        });
        return Promise.resolve(
          Response.json({
            trace: '4bf92f3577b34da6a3ce929d0e0e4736',
            error_msg: '',
            data: { matched_rule: 1, score },
          }),
        );
      }),
    );
    const calculation: ScoreCalculation = {
      enabled: true,
      ranking_dimensions: [],
      rules: [
        {
          condition: { type: 'condition', field, operator: 'in', values: [value] },
          formula: '[子项A]*0.6',
        },
      ],
    };
    render(
      <ScoreCalculationPreview
        workspaceID="w7m2k9p4s6n8c3x5"
        calculation={calculation}
        names={['子项A', '子项B']}
        disabled={false}
      />,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: '分数试算' }));
    await user.type(screen.getByRole('textbox', { name: `试算${label}` }), value);
    await user.type(screen.getByRole('spinbutton', { name: '试算子项分数：子项A' }), '0');
    expect(screen.queryByRole('button', { name: '试算' })).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('status', { name: '试算总分' })).toHaveTextContent(display),
    );
    expect(bodies).toEqual([
      {
        path: '/v1/workspaces/w7m2k9p4s6n8c3x5/projects/score-preview',
        body: {
          score_calculation: calculation,
          detail_names: ['子项A', '子项B'],
          dimensions: {
            grade: '',
            education_level: field === 'education_level' ? value : '',
            major: field === 'major' ? value : '',
            class_name: '',
          },
          details: [
            { name: '子项A', description: '', score: 0 },
            { name: '子项B', description: '', score: null },
          ],
        },
      },
    ]);
    await user.clear(screen.getByRole('textbox', { name: `试算${label}` }));
    expect(screen.getByRole('status', { name: '试算总分' })).toHaveTextContent('-');
  },
);

it('学历条件沿用准入规则录入 in 多值和 OR 子组，公式独立填写', async () => {
  render(<Editor />);
  const user = userEvent.setup();
  await user.click(screen.getByRole('checkbox', { name: '启用总分计算' }));
  await user.click(screen.getByRole('button', { name: '添加一组' }));
  expect(screen.getByText('in', { exact: true })).toBeVisible();
  expect(screen.queryByRole('combobox', { name: '条件关系' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('combobox', { name: '条件字段' }));
  await user.click(screen.getByRole('option', { name: '学历' }));
  await user.type(screen.getByRole('textbox', { name: '学历候选值' }), '硕士、博士、硕士{Enter}');
  const rule = within(screen.getByRole('region', { name: '分数计算第 1 组' }));
  expect(rule.getAllByText('硕士', { exact: true })).toHaveLength(1);
  expect(rule.getByText('博士', { exact: true })).toBeVisible();
  await user.click(rule.getByRole('textbox', { name: '公式' }));
  await user.paste('MAX([子项A] * 0.6, 10)');
  expect(rule.getByRole('textbox', { name: '公式' })).toHaveValue('MAX([子项A] * 0.6, 10)');
  await user.click(rule.getByRole('button', { name: '删除条件' }));
  await user.click(rule.getByRole('button', { name: '添加条件组' }));
  await user.click(rule.getByRole('button', { name: '添加子组' }));
  const logics = rule.getAllByRole('combobox', { name: '组合逻辑' });
  const nestedLogic = logics[1];
  if (!nestedLogic) throw new Error('缺少嵌套条件组');
  await user.click(nestedLogic);
  await user.click(screen.getByRole('option', { name: '任一满足（OR）' }));
  expect(rule.getByRole('textbox', { name: '公式' })).toHaveValue('MAX([子项A] * 0.6, 10)');
});

it('验证条件必填、固定 in、规则层数及纯文本引用转义', () => {
  expect(
    scoreCalculationSchema.safeParse({
      enabled: true,
      ranking_dimensions: [],
      rules: [
        {
          condition: { type: 'condition', field: 'major', operator: 'eq', values: ['甲'] },
          formula: '1',
        },
      ],
    }).success,
  ).toBe(false);
  expect(() => cleanScoreCalculation({ ...emptyScoreCalculation(), enabled: true })).toThrow(
    '至少配置',
  );
  expect(() =>
    cleanScoreCalculation({
      enabled: true,
      ranking_dimensions: [],
      rules: [
        {
          condition: { type: 'condition', field: 'grade', operator: 'in', values: [] },
          formula: '1',
        },
      ],
    }),
  ).toThrow('候选值');
  expect(scoreReference('含]括号\\名称')).toBe('[含\\]括号\\\\名称]');
});

it('学历条件保存与解析沿用学生档案的 32 字符长度限制', () => {
  const calculation = (value: string): ScoreCalculation => ({
    enabled: true,
    ranking_dimensions: [],
    rules: [
      {
        condition: { type: 'condition', field: 'education_level', operator: 'in', values: [value] },
        formula: '1',
      },
    ],
  });
  const valid = cleanScoreCalculation(calculation('学'.repeat(32)));
  expect(scoreCalculationSchema.safeParse(valid).success).toBe(true);
  expect(() => cleanScoreCalculation(calculation('学'.repeat(33)))).toThrow('候选值');
});

it('开启总分计算后可组合排名维度，关闭后清空且隐藏配置', async () => {
  render(<Editor />);
  const user = userEvent.setup();
  expect(screen.queryByRole('group', { name: '排名维度' })).not.toBeInTheDocument();
  expect(screen.queryByText('只允许动态计算总分')).not.toBeInTheDocument();
  const enabled = screen.getByRole('checkbox', { name: '启用总分计算' });
  await user.click(enabled);
  const dimensions = within(screen.getByRole('group', { name: '排名维度' }));
  for (const label of ['年级', '专业', '班级']) {
    await user.click(dimensions.getByRole('checkbox', { name: label }));
    expect(dimensions.getByRole('checkbox', { name: label })).toBeChecked();
  }
  await user.click(dimensions.getByRole('checkbox', { name: '专业' }));
  expect(dimensions.getByRole('checkbox', { name: '年级' })).toBeChecked();
  expect(dimensions.getByRole('checkbox', { name: '专业' })).not.toBeChecked();
  expect(dimensions.getByRole('checkbox', { name: '班级' })).toBeChecked();
  await user.click(enabled);
  expect(screen.queryByRole('group', { name: '排名维度' })).not.toBeInTheDocument();
  await user.click(enabled);
  for (const label of ['年级', '专业', '班级']) {
    expect(screen.getByRole('checkbox', { name: label })).not.toBeChecked();
  }
});

it('排名维度按固定目录保存，拒绝未启用、重复或未知维度', () => {
  const base: ScoreCalculation = {
    enabled: true,
    ranking_dimensions: ['class_name', 'grade', 'major'],
    rules: [
      {
        condition: { type: 'condition', field: 'grade', operator: 'in', values: ['2026'] },
        formula: '1',
      },
    ],
  };
  expect(cleanScoreCalculation(base)?.ranking_dimensions).toEqual(['grade', 'major', 'class_name']);
  expect(() => cleanScoreCalculation({ ...base, enabled: false })).toThrow('先启用');
  expect(() => cleanScoreCalculation({ ...base, ranking_dimensions: ['grade', 'grade'] })).toThrow(
    '重复',
  );
  expect(
    scoreCalculationSchema.safeParse({ ...base, ranking_dimensions: ['education_level'] }).success,
  ).toBe(false);
});
