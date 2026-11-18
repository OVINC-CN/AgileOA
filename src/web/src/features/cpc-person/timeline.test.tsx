import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { PersonTimelineDrawer } from '@/features/cpc-person/person-timeline';
import type { PersonTimeline, PersonTimelineNode } from '@/features/cpc-person/timeline-api';
import { PersonTimelineNodeEditor } from '@/features/cpc-person/timeline-node-editor';
import { parsePersonTimelineValues } from '@/features/cpc-person/timeline-values';
import type { TimelineNode } from '@/features/cpc-timeline/model';

const node: TimelineNode = {
  numeric_id: 77,
  id: 'node_first',
  name: '材料核对',
  description: '',
  group_name: '',
  is_baseline: false,
  time_field_id: 'occurred_at',
  fields: [
    { id: 'occurred_at', name: '发生时间', type: 'date', required: true },
    { id: 'count', name: '数量', type: 'number', required: true },
    { id: 'confirmed', name: '已确认', type: 'boolean', required: true },
  ],
  rules: [],
};

afterEach(() => {
  vi.unstubAllGlobals();
});

it('节点完成要求必填值，零和否有效，异常时间仍可保存后提示', () => {
  expect(
    parsePersonTimelineValues(node, { occurred_at: '2028-02-29', count: '0', confirmed: false }),
  ).toEqual({ occurred_at: '2028-02-29', count: 0, confirmed: false });
  expect(() =>
    parsePersonTimelineValues(node, { occurred_at: '2027-02-29', count: 0, confirmed: false }),
  ).toThrow('有效日期');
  expect(() =>
    parsePersonTimelineValues(node, { occurred_at: '2028-02-29', count: '', confirmed: false }),
  ).toThrow('数量');
  expect(() => parsePersonTimelineValues(node, { occurred_at: '2028-02-29', count: 0 })).toThrow(
    '已确认',
  );
});

it.each(['pending', 'completed'] as const)(
  '跳过按钮位于重置左侧，跳过不提交草稿、不校验必填并独立加载（%s）',
  async (status) => {
    const original: PersonTimelineNode = {
      ...node,
      values: { occurred_at: '2026-11-06', count: 0, confirmed: false },
      status,
      status_label: status === 'completed' ? '已完成' : '待处理',
      revision: 3,
      attachments: [],
      template_attachments: [],
      issues: [],
      checks: [],
    };
    const fetchMock = vi.fn().mockResolvedValue(
      Response.json({
        trace: '4bf92f3577b34da6a3ce929d0e0e4736',
        error_msg: '',
        data: {
          person_id: 1,
          person_revision: 1,
          template_id: 2,
          template_name: '虚构时间线',
          template_revision: 2,
          can_update: true,
          status: 'normal',
          nodes: [{ ...original, status: 'skipped', status_label: '已跳过', revision: 4 }],
        },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    let finishRefresh: () => void = () => undefined;
    const refreshed = new Promise<void>((resolve) => {
      finishRefresh = resolve;
    });
    const invalidate = vi.fn(() => refreshed);
    const closed = vi.fn();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <PersonTimelineNodeEditor
          workspaceID="w7m2k9p4s6n8c3x5"
          personID={1}
          original={original}
          current={original}
          nodes={[original]}
          templateRevision={2}
          invalidate={invalidate}
          onClose={closed}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.clear(screen.getByLabelText('数量 *'));
    const skip = screen.getByRole('button', { name: '跳过节点' });
    const reset = screen.getByRole('button', { name: '重置节点' });
    const save = screen.getByRole('button', { name: '保存并完成' });
    expect(skip.nextElementSibling).toBe(reset);
    await user.click(skip);
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledOnce();
    });
    expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
      '/v1/workspaces/w7m2k9p4s6n8c3x5/cpc-people/1/timeline/nodes/node_first/skip',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ revision: 3, template_revision: 2 }),
      }),
    );
    expect(skip).toHaveAttribute('aria-busy', 'true');
    expect(reset).not.toHaveAttribute('aria-busy', 'true');
    expect(save).not.toHaveAttribute('aria-busy', 'true');
    expect(reset).toBeDisabled();
    expect(save).toBeDisabled();
    expect(screen.getByRole('button', { name: '关闭抽屉' })).toBeDisabled();
    await user.keyboard('{Escape}');
    expect(closed).not.toHaveBeenCalled();
    await act(async () => {
      finishRefresh();
      await refreshed;
    });
    expect(closed).toHaveBeenCalledOnce();
    await waitFor(() => {
      expect(skip).not.toHaveAttribute('aria-busy', 'true');
    });
  },
);

it('已跳过节点显示状态和数量，可通过节点状态单独筛选', async () => {
  const skipped: PersonTimelineNode = {
    ...node,
    values: {},
    status: 'skipped',
    status_label: '已跳过',
    revision: 1,
    attachments: [],
    template_attachments: [],
    issues: [],
    checks: [],
  };
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      Response.json({
        trace: '4bf92f3577b34da6a3ce929d0e0e4736',
        error_msg: '',
        data: {
          person_id: 1,
          person_revision: 1,
          template_id: 2,
          template_name: '虚构时间线',
          template_revision: 1,
          can_update: false,
          status: 'normal',
          nodes: [
            skipped,
            {
              ...skipped,
              id: 'node_pending',
              name: '待填写材料',
              status: 'pending',
              status_label: '待处理',
            },
          ],
        },
      }),
    ),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <PersonTimelineDrawer
        workspaceID="w7m2k9p4s6n8c3x5"
        person={{ id: 1, student_no: '0001', name: '虚构人员' }}
        canUpdate={false}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(await screen.findByText('已跳过')).toBeVisible();
  expect(screen.getByText('已完成 0 / 2 · 已跳过 1')).toBeVisible();
  await userEvent.click(screen.getByRole('combobox', { name: '节点状态' }));
  await userEvent.click(screen.getByRole('option', { name: '已跳过' }));
  expect(screen.getByRole('heading', { name: '1 · 材料核对' })).toBeVisible();
  expect(screen.queryByRole('heading', { name: '2 · 待填写材料' })).not.toBeInTheDocument();
});

it.each(['2026-02-28', undefined])(
  '引用日期只读并显示当前值，保存不提交引用或校验其必填要求（%s）',
  async (date) => {
    const original: PersonTimelineNode = {
      ...node,
      fields: node.fields.map((field) =>
        field.id === 'occurred_at' ? { ...field, student_field: 'adulthood_date' } : field,
      ),
      values: { occurred_at: '2000-01-01', count: 0, confirmed: false },
      status: 'pending',
      status_label: '待处理',
      revision: 1,
      attachments: [],
      template_attachments: [],
      issues: [],
      checks: [],
    };
    const current = {
      ...original,
      values: { count: 0, confirmed: false, ...(date ? { occurred_at: date } : {}) },
    };
    expect(parsePersonTimelineValues(original, current.values)).toEqual({
      count: 0,
      confirmed: false,
    });
    const fetchMock = vi.fn().mockResolvedValue(
      Response.json({
        trace: '4bf92f3577b34da6a3ce929d0e0e4736',
        error_msg: '',
        data: {
          person_id: 1,
          person_revision: 1,
          template_id: 2,
          template_name: '虚构时间线',
          template_revision: 1,
          can_update: true,
          status: 'normal',
          nodes: [{ ...current, status: 'completed', status_label: '已完成' }],
        },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    const closed = vi.fn();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <PersonTimelineNodeEditor
          workspaceID="w7m2k9p4s6n8c3x5"
          personID={1}
          original={original}
          current={current}
          nodes={[original]}
          templateRevision={1}
          invalidate={() => Promise.resolve()}
          onClose={closed}
        />
      </QueryClientProvider>,
    );
    expect(screen.getByLabelText('发生时间')).toHaveValue(date ?? '-');
    expect(screen.getByLabelText('发生时间')).toHaveAttribute('readonly');
    expect(screen.getByLabelText('发生时间')).toBeDisabled();
    await userEvent.click(screen.getByRole('button', { name: '保存并完成' }));
    await waitFor(() => {
      expect(closed).toHaveBeenCalledOnce();
    });
    expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
      '/v1/workspaces/w7m2k9p4s6n8c3x5/cpc-people/1/timeline/nodes/node_first',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          values: { count: 0, confirmed: false },
          revision: 1,
          template_revision: 1,
        }),
      }),
    );
  },
);

it('时间线展示基准和异常标记，支持组合筛选基准、状态和异常且保留整体进度', async () => {
  const user = userEvent.setup();
  const data: PersonTimeline = {
    person_id: 1,
    person_revision: 1,
    template_id: 2,
    template_name: '虚构时间线',
    template_revision: 1,
    can_update: false,
    status: 'abnormal',
    nodes: [
      {
        ...node,
        is_baseline: true,
        group_name: '申请阶段',
        rules: [
          {
            id: 'range',
            reference_node_id: 'node_reference',
            min_offset: { years: 0, months: 0, days: 1, workdays: 0 },
            max_offset: { years: 0, months: 0, days: 3, workdays: 0 },
          },
          {
            id: 'missing_range',
            reference_node_id: 'node_pending',
            min_offset: null,
            max_offset: { years: 0, months: 0, days: -10, workdays: 0 },
          },
        ],
        values: { occurred_at: '2026-11-06', count: 0, confirmed: false },
        status: 'completed',
        status_label: '已完成',
        revision: 1,
        attachments: [],
        template_attachments: [],
        checks: [
          {
            rule_id: 'range',
            reference_node_name: '提交申请',
            reference_date: '2026-11-06',
            actual_date: '2026-11-06',
            min_date: '2026-11-07',
            max_date: '2026-11-09',
            message: '请按预期范围核对发生时间',
            result: 'abnormal',
            result_label: '时间异常',
          },
        ],
        issues: [
          {
            rule_id: 'range',
            reference_node_name: '提交申请',
            actual_date: '2026-11-06',
            min_date: '2026-11-07',
            max_date: '2026-11-09',
            message: '请按预期范围核对发生时间',
          },
        ],
      },
      {
        ...node,
        id: 'node_pending',
        name: '待填写材料',
        group_name: '申请阶段',
        rules: [
          {
            id: 'pending_range',
            reference_node_id: 'node_reference',
            min_offset: { years: 0, months: 0, days: 25, workdays: 0 },
            max_offset: null,
          },
        ],
        values: {},
        status: 'pending',
        status_label: '待处理',
        revision: 1,
        attachments: [],
        template_attachments: [],
        issues: [],
        checks: [
          {
            rule_id: 'pending_range',
            reference_node_name: '提交申请',
            reference_date: '2026-11-06',
            actual_date: '',
            min_date: '2026-12-01',
            max_date: '',
            message: '请按预期范围核对发生时间',
            result: 'pending',
            result_label: '待校验',
          },
        ],
      },
      {
        ...node,
        id: 'node_reference',
        name: '提交申请',
        group_name: '后续阶段',
        values: { occurred_at: '2026-11-06', count: 0, confirmed: false },
        status: 'pending',
        status_label: '待处理',
        revision: 1,
        attachments: [],
        template_attachments: [],
        issues: [],
        checks: [],
      },
    ],
  };
  const fetchMock = vi
    .fn()
    .mockResolvedValue(
      new Response(
        JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data }),
      ),
    );
  vi.stubGlobal('fetch', fetchMock);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <PersonTimelineDrawer
        workspaceID="w7m2k9p4s6n8c3x5"
        person={{ id: 1, student_no: '0001', name: '虚构学生' }}
        canUpdate={false}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(await screen.findByText('时间范围异常')).toBeVisible();
  const rules = within(screen.getByRole('region', { name: '材料核对校验规则' }));
  expect(rules.getByText('材料核对应该在2026-11-07至2026-11-09。')).toBeVisible();
  expect(rules.getByText('材料核对应该在待填写材料的10天前。')).toBeVisible();
  expect(rules.getByText('最晚时间')).toBeVisible();
  expect(rules.getByText('待校验')).toBeVisible();
  const pendingRules = within(screen.getByRole('region', { name: '待填写材料校验规则' }));
  expect(pendingRules.getByText('待填写材料应该在2026-12-01。')).toBeVisible();
  expect(pendingRules.getByText('最早时间')).toBeVisible();
  expect(pendingRules.getByText('待校验')).toBeVisible();
  expect(screen.queryByText('编辑节点')).not.toBeInTheDocument();
  expect(screen.getByText('基准节点')).toBeVisible();

  // 分组标题与节点同时展示，保留原有编号、完成进度和其他分组。
  expect(screen.getByRole('heading', { level: 2, name: '申请阶段' })).toBeVisible();
  expect(screen.getByRole('heading', { level: 2, name: '后续阶段' })).toBeVisible();
  expect(screen.getByRole('heading', { name: '1 · 材料核对' })).toBeVisible();
  expect(screen.getByRole('heading', { name: '3 · 提交申请' })).toBeVisible();
  expect(screen.getByText('已完成 1 / 3')).toBeVisible();

  // 基准标记与完成、异常状态独立，筛选后整体进度不变。
  const baseline = screen.getByRole('checkbox', { name: '仅看基准节点' });
  expect(baseline).not.toBeChecked();
  await user.click(baseline);
  expect(screen.getByRole('heading', { name: '1 · 材料核对' })).toBeVisible();
  expect(screen.getByRole('heading', { level: 2, name: '申请阶段' })).toBeVisible();
  expect(screen.queryByRole('heading', { level: 2, name: '后续阶段' })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '2 · 待填写材料' })).not.toBeInTheDocument();
  expect(screen.getByText('已完成 1 / 3')).toBeVisible();
  expect(screen.getByText('材料核对应该在2026-11-07至2026-11-09。')).toBeVisible();
  await user.click(baseline);

  // 待校验不属于异常，两个筛选条件取交集。
  const status = screen.getByRole('combobox', { name: '节点状态' });
  const abnormal = screen.getByRole('checkbox', { name: '仅看异常节点' });
  await user.click(status);
  await user.click(screen.getByRole('option', { name: '待处理' }));
  expect(screen.getByRole('heading', { name: '2 · 待填写材料' })).toBeVisible();
  expect(screen.queryByRole('heading', { name: '1 · 材料核对' })).not.toBeInTheDocument();
  expect(screen.getByText('已完成 1 / 3')).toBeVisible();
  await user.click(baseline);
  expect(screen.getByText('没有符合筛选条件的节点')).toBeVisible();
  await user.click(abnormal);
  expect(screen.getByText('没有符合筛选条件的节点')).toBeVisible();
  expect(screen.getByText('时间线状态：异常')).toBeVisible();
  await user.click(status);
  await user.click(screen.getByRole('option', { name: '已完成' }));
  expect(screen.getByRole('heading', { name: '1 · 材料核对' })).toBeVisible();
  expect(screen.queryByRole('heading', { name: '2 · 待填写材料' })).not.toBeInTheDocument();
  await user.click(status);
  await user.click(screen.getByRole('option', { name: '全部状态' }));
  expect(screen.queryByRole('heading', { name: '2 · 待填写材料' })).not.toBeInTheDocument();
  await user.click(abnormal);
  expect(screen.queryByRole('heading', { name: '2 · 待填写材料' })).not.toBeInTheDocument();
  await user.click(baseline);
  expect(screen.getByRole('heading', { name: '2 · 待填写材料' })).toBeVisible();
  expect(fetchMock).toHaveBeenCalledOnce();
});

it.each(['completed', 'pending'] as const)(
  '重置节点确认后清空全部内容，不校验未保存表单并独立加载（%s）',
  async (status) => {
    const user = userEvent.setup();
    const original: PersonTimelineNode = {
      ...node,
      values: { occurred_at: '2026-11-06', count: 0, confirmed: false },
      status,
      status_label: status === 'completed' ? '已完成' : '待处理',
      revision: 3,
      attachments: [],
      template_attachments: [],
      issues: [],
      checks: [],
    };
    const data: PersonTimeline = {
      person_id: 1,
      person_revision: 1,
      template_id: 2,
      template_name: '虚构时间线',
      template_revision: 2,
      can_update: true,
      status: 'normal',
      nodes: [{ ...original, values: {}, status: 'pending', status_label: '待处理', revision: 4 }],
    };
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data }),
        ),
      );
    vi.stubGlobal('fetch', fetchMock);
    let finishRefresh: () => void = () => {
      throw new Error('尚未开始刷新');
    };
    const refreshed = new Promise<void>((resolve) => {
      finishRefresh = resolve;
    });
    const invalidate = vi.fn(() => refreshed);
    const closed = vi.fn();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <PersonTimelineNodeEditor
          workspaceID="w7m2k9p4s6n8c3x5"
          personID={1}
          original={original}
          current={original}
          nodes={[original]}
          templateRevision={2}
          invalidate={invalidate}
          onClose={closed}
        />
      </QueryClientProvider>,
    );
    await user.clear(screen.getByLabelText('数量 *'));
    const reset = screen.getByRole('button', { name: '重置节点' });
    const save = screen.getByRole('button', { name: '保存并完成' });
    expect(reset.nextElementSibling).toBe(save);
    expect(reset).toBeEnabled();
    await user.click(reset);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(
      screen.getByText('重置后将清空此节点的全部填写内容和人员附件，并恢复为待处理，无法恢复。'),
    ).toBeVisible();
    await user.click(screen.getByRole('button', { name: '取消' }));
    expect(fetchMock).not.toHaveBeenCalled();
    await user.click(reset);
    const confirmReset = screen.getByRole('button', { name: '确认重置' });
    await user.click(confirmReset);
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledOnce();
    });
    expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
      '/v1/workspaces/w7m2k9p4s6n8c3x5/cpc-people/1/timeline/nodes/node_first/reset',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ revision: 3, template_revision: 2 }),
      }),
    );
    expect(reset).toHaveAttribute('aria-busy', 'true');
    expect(confirmReset).toHaveAttribute('aria-busy', 'true');
    expect(save).not.toHaveAttribute('aria-busy', 'true');
    expect(save).toBeDisabled();
    expect(screen.getByRole('button', { name: '关闭抽屉' })).toBeDisabled();
    await user.keyboard('{Escape}');
    expect(closed).not.toHaveBeenCalled();
    await act(async () => {
      finishRefresh();
      await refreshed;
    });
    expect(closed).toHaveBeenCalledOnce();
    await waitFor(() => {
      expect(reset).not.toHaveAttribute('aria-busy', 'true');
    });
  },
);
