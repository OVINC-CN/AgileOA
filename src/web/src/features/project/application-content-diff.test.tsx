import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { applicationSchema, applicationVersionSchema } from '@/features/project/application-api';
import { ApplicationContentDiff } from '@/features/project/application-content-diff';
import { ApplicationDrawer } from '@/features/project/application-drawer';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const updatedAt = '2026-11-16T00:00:00Z';
const firstKey = '0123456789abcdef0123456789abcdef';
const secondKey = '1123456789abcdef0123456789abcdef';
const thirdKey = '2123456789abcdef0123456789abcdef';
const detail = (id: number, key: string, name: string, description = '现有内容') => ({
  id,
  key,
  name,
  description,
  score: 0,
  attachments: [],
});
const existingDetail = detail(8, firstKey, '志愿服务');
const addedDetail = detail(9, secondKey, '新增成果', '新增内容');
const previousDetail = {
  ...detail(7, firstKey, '志愿服务', '历史内容'),
  detail_code: firstKey,
  score: null,
};
const removedDetail = {
  ...detail(6, thirdKey, '删除成果', '删除内容'),
  detail_code: thirdKey,
  score: 8,
};
const record = applicationSchema.parse({
  id: 5,
  project_id: 1,
  project_name: '优秀学生',
  project_description: '年度评选',
  year: 2026,
  allow_self_apply: true,
  self_apply_deadline: null,
  student_id: 7,
  student_no: '0007',
  student_name: '虚构学生',
  class_name: '一班',
  status: 'pending',
  current_step: 1,
  current_reviewer: '审批人',
  can_edit: true,
  can_approve: true,
  can_reject: true,
  revision: 5,
  created_at: updatedAt,
  updated_at: updatedAt,
  statement: '共同说明\n当前说明',
  rank: null,
  score: 0,
  details: [existingDetail, addedDetail],
  steps: [],
  detail_names: ['志愿服务', '新增成果'],
  history: [],
});
const previous = applicationVersionSchema.parse({
  revision: 3,
  actor_name: '虚构修改人(editor)',
  updated_at: updatedAt,
  statement: '共同说明\n历史说明',
  rank: 1,
  score: null,
  details: [previousDetail, removedDetail],
});
const respond = (data: unknown) =>
  Response.json({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data });

afterEach(() => {
  vi.unstubAllGlobals();
});

it('按 Word 修订在原文内标记增删，区分零和空值，按稳定标识匹配子项', () => {
  render(<ApplicationContentDiff record={record} previous={previous} />);
  const statement = screen.getByLabelText('申报说明差异');
  expect(statement.querySelector('del')).toHaveTextContent('历史');
  expect(statement.querySelector('ins')).toHaveTextContent('当前');
  expect(statement.querySelector('del')?.closest('.content-revision')).not.toBeNull();
  expect(within(statement).getByText('共同说明')).toBeVisible();
  const score = screen.getByLabelText('总分差异');
  expect(score).toHaveTextContent('0.00(原未填)');
  expect(score.querySelector('ins, del')).toBeNull();
  const existing = screen.getByRole('listitem', { name: '子项：志愿服务' });
  expect(within(existing).getByLabelText('子项内容差异').querySelector('del')).toHaveTextContent(
    '历史',
  );
  expect(within(existing).getByLabelText('子项内容差异').querySelector('ins')).toHaveTextContent(
    '现有',
  );
  expect(within(existing).getByLabelText('子项分数差异')).toHaveTextContent('0(原未填)');
});

it.each([
  [20, 10, '-10', 'text-danger-foreground'],
  [18.2, 20, '+1.8', 'text-success-foreground'],
  [0.1, 0.3, '+0.2', 'text-success-foreground'],
  [0.0000001, 0.0000002, '+0.0000001', 'text-success-foreground'],
])('分数 %s 到 %s 单行显示当前分数及十进制差值', (before, after, delta, color) => {
  render(
    <ApplicationContentDiff
      record={{ ...record, score: after, details: [{ ...existingDetail, score: after }] }}
      previous={{ ...previous, score: before, details: [{ ...previousDetail, score: before }] }}
    />,
  );
  for (const field of ['总分差异', '子项分数差异']) {
    const score = screen.getByLabelText(field);
    expect(within(score).getByText(`(${delta})`)).toHaveClass(color);
    expect(score.querySelector('ins, del')).toBeNull();
    expect(score.firstElementChild).toHaveClass('whitespace-nowrap');
  }
});

it('分数不变不显示差值，清空分数显示历史值，零与空不混淆', () => {
  render(
    <ApplicationContentDiff
      record={{ ...record, score: 0, details: [{ ...existingDetail, score: null }] }}
      previous={{ ...previous, score: 0, details: [{ ...previousDetail, score: 0 }] }}
    />,
  );
  expect(screen.getByLabelText('总分差异')).toHaveTextContent(/^0\.00$/u);
  const childScore = screen.getByLabelText('子项分数差异');
  expect(childScore).toHaveTextContent('-(原 0)');
  expect(within(childScore).getByText('(原 0)')).toHaveClass('text-danger-foreground');
});

it('完整保留新增、删除子项的内容、分数与附件名', () => {
  const before = {
    ...previous,
    details: previous.details.map((item) => ({
      ...item,
      attachments: [{ filename: '历史材料.pdf' }],
    })),
  };
  render(<ApplicationContentDiff record={record} previous={before} />);
  const added = screen.getByRole('listitem', { name: '新增子项：新增成果' });
  const removed = screen.getByRole('listitem', { name: '删除子项：删除成果' });
  expect(added).toHaveClass('bg-success');
  expect(removed).toHaveClass('bg-danger');
  expect(within(added).getByText('新增内容').tagName).toBe('INS');
  expect(within(removed).getByText('删除内容').tagName).toBe('DEL');
  expect(within(removed).getByText('8').tagName).toBe('DEL');
  expect(within(removed).getByText('历史材料.pdf').tagName).toBe('DEL');
  expect(removed.querySelector('a')).toBeNull();
});

it('附件只对比名称及同名数量，忽略编号、状态、大小和顺序', () => {
  const attachments = (filenames: string[]) =>
    filenames.map((filename, index) => ({
      id: index + 100,
      application_id: 5,
      detail_id: 8,
      category: 'project_application_detail' as const,
      filename,
      size: 99,
      state: 'deleting' as const,
      revision: 3,
      created_at: updatedAt,
      updated_at: updatedAt,
    }));
  const after = {
    ...record,
    details: [{ ...existingDetail, attachments: attachments(['保留.pdf', '新增.pdf']) }],
  };
  const before = {
    ...previous,
    details: [
      { ...previousDetail, attachments: attachments(['删除.pdf', '保留.pdf', '保留.pdf']) },
    ],
  };
  render(<ApplicationContentDiff record={after} previous={before} />);
  const files = screen.getByLabelText('附件名称差异');
  expect(files.querySelectorAll('del')).toHaveLength(2);
  expect(files.querySelector('ins')).toHaveTextContent('新增.pdf');
  expect(files).not.toHaveTextContent('待删除');
  expect(
    within(files)
      .getAllByText('保留.pdf')
      .filter((node) => node.tagName === 'DEL'),
  ).toHaveLength(1);
});

it('删除全部子项仍显示完整历史，同名替换不混淆子项', () => {
  const { rerender } = render(
    <ApplicationContentDiff record={{ ...record, details: [] }} previous={previous} />,
  );
  expect(screen.getAllByRole('listitem', { name: /删除子项/u })).toHaveLength(2);
  rerender(
    <ApplicationContentDiff
      record={{ ...record, details: [{ ...addedDetail, name: '删除成果' }] }}
      previous={{ ...previous, details: [removedDetail] }}
    />,
  );
  expect(screen.getByRole('listitem', { name: '新增子项：删除成果' })).toBeVisible();
  expect(screen.getByRole('listitem', { name: '删除子项：删除成果' })).toBeVisible();
});

it('审批中默认读取关键历史版本，取消勾选恢复当前内容，审批仍提交当前版本', async () => {
  const user = userEvent.setup();
  const request = vi.fn((path: string, options: RequestInit) => {
    if (path.includes('/approve')) {
      if (typeof options.body !== 'string') throw new Error('缺少审批正文');
      expect(JSON.parse(options.body)).toMatchObject({
        revision: 5,
        statement: record.statement,
      });
      return Promise.resolve(respond({ ...record, status: 'approved', revision: 6 }));
    }
    expect(path).toContain(`/v1/workspaces/${workspaceID}/project-applications/5/versions`);
    expect(path).not.toContain('?');
    return Promise.resolve(respond({ items: [previous, { ...previous, revision: 1 }, previous] }));
  });
  vi.stubGlobal('fetch', request);
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="manage"
        record={record}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole('checkbox', { name: '对比历史版本' })).toBeChecked();
  expect(await screen.findByLabelText('申报内容差异')).toBeVisible();
  expect(request).toHaveBeenCalledOnce();
  expect(screen.queryByRole('button', { name: '加载更早版本' })).toBeNull();
  await user.click(screen.getByRole('combobox', { name: '历史版本' }));
  expect(screen.getAllByRole('option', { name: /版本 3/u })).toHaveLength(1);
  await user.click(screen.getByRole('option', { name: /版本 1/u }));
  expect(screen.getByLabelText('申报内容差异')).toHaveTextContent('历史版本 1 → 当前版本 5');
  await user.click(screen.getByRole('checkbox', { name: '对比历史版本' }));
  expect(screen.queryByLabelText('申报内容差异')).toBeNull();
  expect(screen.getByText('共同说明 当前说明', { exact: false })).toBeVisible();
  await user.click(screen.getByRole('checkbox', { name: '对比历史版本' }));
  await user.click(screen.getByRole('button', { name: '通过' }));
  const decision = screen.getByRole('dialog', { name: '通过申报' });
  await user.click(within(decision).getByRole('button', { name: '通过' }));
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: '通过申报' })).toBeNull();
  });
});

it('没有审计快照时保留当前内容，读取失败后隐藏历史正文并允许重试', async () => {
  const request = vi.fn().mockResolvedValueOnce(respond({ items: [] }));
  vi.stubGlobal('fetch', request);
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="student-reviews"
        record={record}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole('checkbox', { name: '对比历史版本' })).toBeChecked();
  expect(await screen.findByText('暂无可对比的历史版本')).toBeVisible();
  request.mockResolvedValueOnce(
    Response.json(
      { trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '没有操作权限', data: null },
      { status: 403 },
    ),
  );
  await client.invalidateQueries();
  expect(await screen.findByText('没有操作权限')).toBeVisible();
  expect(screen.queryByLabelText('申报内容差异')).toBeNull();
  expect(screen.getByRole('button', { name: '重试读取历史版本' })).toBeVisible();
});

it.each(['draft', 'rejected', 'approved'] as const)(
  '$0 状态默认不勾选对比，不请求历史',
  (status) => {
    const request = vi.fn();
    vi.stubGlobal('fetch', request);
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ApplicationDrawer
          workspaceID={workspaceID}
          scope="mine"
          record={{ ...record, status, can_edit: false, can_approve: false, can_reject: false }}
          onClose={vi.fn()}
        />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('checkbox', { name: '对比历史版本' })).not.toBeChecked();
    expect(request).not.toHaveBeenCalled();
  },
);

it('关键版本显示修改人和截止分隔线，搜索、展开及键盘选择均保留正确边界', async () => {
  const user = userEvent.setup();
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      respond({
        items: [
          {
            ...previous,
            revision: 4,
            actor_name: '虚构审批人(reviewer)',
            updated_at: '2026-11-16T01:00:00Z',
          },
          {
            ...previous,
            revision: 3,
            actor_name: '虚构学生(0007)',
            updated_at: '2026-11-16T00:00:00Z',
          },
          { ...previous, revision: 1, actor_name: '', updated_at: '2026-11-15T23:00:00Z' },
        ],
      }),
    ),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="manage"
        record={{ ...record, self_apply_deadline: '2026-11-16T00:00:00Z' }}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole('checkbox', { name: '对比历史版本' })).toBeChecked();
  await user.click(await screen.findByRole('combobox', { name: '历史版本' }));
  const order = () =>
    [
      ...screen
        .getByRole('listbox', { name: '历史版本' })
        .querySelectorAll('[role="option"], [role="separator"]'),
    ].map((node) => node.textContent);
  expect(order()).toEqual([
    '版本 4 · 26/11/16 09:00 · 虚构审批人(reviewer)',
    '版本 3 · 26/11/16 08:00 · 虚构学生(0007)',
    '申报截止：26/11/16 08:00',
    '版本 1 · 26/11/16 07:00 · -',
  ]);
  await user.click(screen.getByRole('button', { name: '展开历史版本选择器' }));
  expect(screen.getAllByRole('separator', { name: '申报截止：26/11/16 08:00' })).toHaveLength(1);
  await user.type(screen.getByRole('searchbox', { name: '搜索历史版本' }), '版本 1');
  expect(order()).toEqual(['申报截止：26/11/16 08:00', expect.stringContaining('版本 1')]);
  const scrollIntoView = vi.fn();
  screen.getByRole('option', { name: /版本 1/u }).scrollIntoView = scrollIntoView;
  await user.keyboard('{ArrowDown}{Enter}');
  expect(scrollIntoView).toHaveBeenCalledExactlyOnceWith({ block: 'nearest' });
  expect(screen.getByLabelText('申报内容差异')).toHaveTextContent('历史版本 1 → 当前版本 5');
});

it.each([
  {
    name: '选中截止前最后一版',
    deadline: '2026-11-16T00:00:00Z',
    times: ['2026-11-16T01:00:00Z', '2026-11-15T23:30:00Z', '2026-11-15T23:00:00Z'],
    expected: 3,
  },
  {
    name: '截止前无版本时选中截止后首版，包含截止时刻',
    deadline: '2026-11-16T00:00:00Z',
    times: ['2026-11-16T01:00:00Z', '2026-11-16T00:30:00Z', '2026-11-16T00:00:00Z'],
    expected: 1,
  },
  {
    name: '全部在截止前时选中最后一版',
    deadline: '2026-11-16T00:00:00Z',
    times: ['2026-11-15T23:59:00Z', '2026-11-15T23:30:00Z', '2026-11-15T23:00:00Z'],
    expected: 4,
  },
  {
    name: '未设置截止时间时选中最近版本',
    deadline: null,
    times: ['2026-11-16T01:00:00Z', '2026-11-15T23:30:00Z', '2026-11-15T23:00:00Z'],
    expected: 4,
  },
])('$name，手动选择后再次勾选恢复默认', async ({ deadline, times, expected }) => {
  const user = userEvent.setup();
  const items = [4, 3, 1].map((revision, index) => ({
    ...previous,
    revision,
    updated_at: times[index],
  }));
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(respond({ items }))),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="manage"
        record={{ ...record, self_apply_deadline: deadline }}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole('checkbox', { name: '对比历史版本' })).toBeChecked();
  expect(await screen.findByLabelText('申报内容差异')).toHaveTextContent(
    `历史版本 ${String(expected)} → 当前版本 5`,
  );
  await user.click(screen.getByRole('combobox', { name: '历史版本' }));
  const manualRevision = expected === 4 ? 1 : 4;
  await user.click(
    screen.getByRole('option', { name: new RegExp(`版本 ${String(manualRevision)}`, 'u') }),
  );
  expect(screen.getByLabelText('申报内容差异')).toHaveTextContent(
    `历史版本 ${String(manualRevision)} → 当前版本 5`,
  );
  await user.click(screen.getByRole('checkbox', { name: '对比历史版本' }));
  await user.click(screen.getByRole('checkbox', { name: '对比历史版本' }));
  await waitFor(() => {
    expect(screen.getByLabelText('申报内容差异')).toHaveTextContent(
      `历史版本 ${String(expected)} → 当前版本 5`,
    );
  });
});
