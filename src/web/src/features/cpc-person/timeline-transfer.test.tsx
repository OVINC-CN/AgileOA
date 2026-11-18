import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';

import { PersonTimelineDrawer } from '@/features/cpc-person/person-timeline';
import type { TimelineTransferPreview } from '@/features/cpc-person/timeline-transfer-api';
import { TimelineTransferDrawer } from '@/features/cpc-person/timeline-transfer-drawer';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const preview: TimelineTransferPreview = {
  source_template_id: 1,
  source_template_name: '来源模板',
  preview_version: 'a'.repeat(64),
  discarded_fields: [{ node_name: '原节点', field_name: '备注', value: false }],
  timeline: {
    person_id: 7,
    person_revision: 3,
    template_id: 2,
    template_name: '目标模板',
    template_revision: 4,
    can_update: true,
    status: 'normal',
    nodes: [
      {
        id: 'node_target',
        name: '目标节点',
        description: '',
        group_name: '',
        is_baseline: false,
        time_field_id: 'date',
        fields: [{ id: 'date', name: '日期', type: 'date', required: true }],
        rules: [],
        values: { date: '2026-11-08' },
        status: 'completed',
        status_label: '已完成',
        revision: 8,
        issues: [],
        checks: [],
        template_attachments: [],
        attachments: [
          {
            id: 8,
            person_id: 7,
            category: 'cpc_person_timeline_node',
            node_id: 'node_target',
            filename: '原人员附件.pdf',
            size: 100,
            state: 'ready',
            revision: 3,
            created_at: '2026-11-08T00:00:00Z',
            updated_at: '2026-11-08T00:00:00Z',
          },
        ],
      },
    ],
  },
};

function response(data: unknown, status = 200, message = '') {
  return new Response(JSON.stringify({ trace: '1'.repeat(32), error_msg: message, data }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function requestPath(input: RequestInfo | URL) {
  return typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const choices = {
  source_template_id: 1,
  source_template_name: '来源模板',
  targets: [{ value: '2', label: '目标模板' }],
};

function setup(status = 200, abnormal = false, reviewing = false) {
  const onClose = vi.fn();
  const currentPreview: TimelineTransferPreview = {
    ...preview,
    timeline: { ...preview.timeline, status: abnormal ? 'abnormal' : 'normal' },
  };
  const fetch = vi.fn<typeof globalThis.fetch>((input) => {
    const path = requestPath(input);
    if (path.endsWith('/transfer-options'))
      return Promise.resolve(
        response({
          source_template_id: 1,
          source_template_name: '来源模板',
          targets: [{ value: '2', label: '目标模板' }],
        }),
      );
    if (path.endsWith('/transfer-preview')) return Promise.resolve(response(currentPreview));
    if (path.endsWith('/transfer-review'))
      return Promise.resolve(response(currentPreview.timeline));
    if (path.endsWith('/transfer'))
      return Promise.resolve(
        status === 200
          ? response({ ...preview.timeline, status: 'transferring', transfer_target_id: 2 })
          : response(null, 409, '时间线或转移配置已发生变化，请重新预览后确认'),
      );
    throw new Error(`未预期请求：${path}`);
  });
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <TimelineTransferDrawer
        workspaceID={workspaceID}
        person={{ id: 7, name: '虚构人员', student_no: '000007' }}
        transfer={
          reviewing
            ? { mode: 'review', preview: currentPreview }
            : { mode: 'request', options: choices }
        }
        onClose={onClose}
      />
    </QueryClientProvider>,
  );
  return { fetch, onClose };
}

async function showPreview() {
  const user = userEvent.setup();
  await user.click(await screen.findByRole('combobox', { name: '转移目标模板' }));
  await user.click(screen.getByRole('option', { name: '目标模板' }));
  await user.click(screen.getByRole('button', { name: '预览转移' }));
  await screen.findByRole('heading', { name: '转移后预览 · 目标模板' });
  return user;
}

it('先预览字段、附件及未映射值，发起时只提交目标模板和预览版本', async () => {
  const { fetch, onClose } = setup();
  expect(screen.queryByRole('button', { name: '发起转移' })).not.toBeInTheDocument();
  const user = await showPreview();
  expect(screen.getByText('2026-11-08')).toBeVisible();
  expect(screen.getByText('原人员附件.pdf')).toBeVisible();
  expect(screen.getByText('否')).toBeVisible();
  expect(screen.queryByRole('link')).not.toBeInTheDocument();
  expect(fetch.mock.calls.some(([path]) => requestPath(path).endsWith('/transfer'))).toBe(false);
  expect(screen.queryByRole('button', { name: '通过' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '发起转移' }));
  await waitFor(() => {
    expect(onClose).toHaveBeenCalledOnce();
  });
  const confirmation = fetch.mock.calls.find(([path]) => requestPath(path).endsWith('/transfer'));
  expect(confirmation).toBeDefined();
  // 请求体只使用服务端预览，不提交浏览器计算的字段或附件。
  const body = confirmation?.[1]?.body;
  if (typeof body !== 'string') throw new Error('缺少确认请求体');
  expect(JSON.parse(body)).toEqual({
    target_template_id: 2,
    preview_version: preview.preview_version,
  });
});

it('过期提交后保留当前预览，重新预览后才能再次提交', async () => {
  const { onClose } = setup(409);
  const user = await showPreview();
  await user.click(screen.getByRole('button', { name: '发起转移' }));
  await waitFor(() => {
    expect(screen.getByRole('button', { name: '发起转移' })).toBeDisabled();
  });
  expect(onClose).not.toHaveBeenCalled();
  expect(screen.getByRole('heading', { name: '转移后预览 · 目标模板' })).toBeVisible();
  await user.click(screen.getByRole('button', { name: '重新预览' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '发起转移' })).toBeEnabled());
});

it.each([false, true])('目标存在时间异常时仍可发起或审核转移：%s', async (reviewing) => {
  const { fetch, onClose } = setup(200, true, reviewing);
  const user = reviewing ? userEvent.setup() : await showPreview();
  expect(screen.getByText('时间线状态：异常')).toBeVisible();
  const button = screen.getByRole('button', { name: reviewing ? '通过' : '发起转移' });
  expect(button).toBeEnabled();
  await user.click(button);
  await waitFor(() => {
    expect(onClose).toHaveBeenCalledOnce();
  });
  const confirmation = fetch.mock.calls.find(([input]) =>
    requestPath(input).endsWith(reviewing ? '/transfer-review' : '/transfer'),
  );
  expect(confirmation?.[1]?.method).toBe('POST');
  const body = confirmation?.[1]?.body;
  if (typeof body !== 'string') throw new Error('缺少转移请求体');
  expect(JSON.parse(body)).toEqual({
    target_template_id: 2,
    preview_version: preview.preview_version,
  });
});

it('审核使用独立完成接口，待审核目标不能重新选择', async () => {
  const { fetch, onClose } = setup(200, false, true);
  const user = userEvent.setup();
  expect(screen.queryByRole('combobox', { name: '转移目标模板' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '发起转移' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: '通过' }));
  await waitFor(() => {
    expect(onClose).toHaveBeenCalledOnce();
  });
  expect(fetch.mock.calls.some(([input]) => requestPath(input).endsWith('/transfer'))).toBe(false);
  const request = fetch.mock.calls.find(([input]) =>
    requestPath(input).endsWith('/transfer-review'),
  );
  expect(request?.[1]?.method).toBe('POST');
  const body = request?.[1]?.body;
  if (typeof body !== 'string') throw new Error('缺少审核请求体');
  expect(JSON.parse(body)).toEqual({
    target_template_id: 2,
    preview_version: preview.preview_version,
  });
});

it('入口校验失败时保留当前人员时间线，不打开空白转移抽屉', async () => {
  const fetch = vi.fn<typeof globalThis.fetch>((input) =>
    Promise.resolve(
      requestPath(input).endsWith('/transfer-options')
        ? response(null, 400, '当前时间线存在异常时间节点，请先处理后再转移')
        : response({ ...preview.timeline, has_transfer_targets: true }),
    ),
  );
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <PersonTimelineDrawer
        workspaceID={workspaceID}
        person={{ id: 7, name: '虚构人员', student_no: '000007' }}
        canUpdate
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', { name: '转移时间线' }));
  await waitFor(() => {
    expect(
      fetch.mock.calls.some(([input]) => requestPath(input).endsWith('/transfer-options')),
    ).toBe(true);
    expect(screen.getByRole('button', { name: '转移时间线' })).toBeEnabled();
  });
  expect(screen.getByRole('heading', { name: '目标模板' })).toBeVisible();
  expect(screen.queryByRole('heading', { name: '转移人员时间线' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '重新校验' })).not.toBeInTheDocument();
});

it.each([false, true])('转移中禁止编辑节点，仅按审核权限显示完成入口：%s', async (reviewing) => {
  vi.stubGlobal(
    'fetch',
    vi.fn<typeof globalThis.fetch>(() =>
      Promise.resolve(
        response({
          ...preview.timeline,
          has_transfer_targets: true,
          status: 'transferring',
          can_update: false,
          can_cancel_transfer: true,
          can_review_transfer: reviewing,
          transfer_target_id: 2,
          transfer_target_name: '目标模板',
        }),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <PersonTimelineDrawer
        workspaceID={workspaceID}
        person={{ id: 7, name: '虚构人员', student_no: '000007' }}
        canUpdate
        canReview={reviewing}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  await screen.findByText('时间线状态：转移中');
  expect(screen.getByRole('button', { name: '取消' })).toBeVisible();
  expect(screen.queryByRole('button', { name: '转移时间线' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '编辑节点 目标节点' })).not.toBeInTheDocument();
  if (reviewing) {
    expect(screen.getByRole('button', { name: '审核' })).toBeVisible();
    expect(screen.getByRole('button', { name: '驳回' })).toBeVisible();
  } else {
    expect(screen.queryByRole('button', { name: '审核' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '驳回' })).not.toBeInTheDocument();
  }
});

it.each([
  { configured: false, editing: true, visible: false },
  { configured: true, editing: true, visible: true },
  { configured: true, editing: false, visible: false },
])('时间线转移入口按目标配置和编辑权限显示：%j', async ({ configured, editing, visible }) => {
  vi.stubGlobal(
    'fetch',
    vi.fn<typeof globalThis.fetch>(() =>
      Promise.resolve(
        response({
          ...preview.timeline,
          has_transfer_targets: configured,
        }),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <PersonTimelineDrawer
        workspaceID={workspaceID}
        person={{ id: 7, name: '虚构人员', student_no: '000007' }}
        canUpdate={editing}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  await screen.findByRole('heading', { name: '目标模板' });
  if (visible) expect(screen.getByRole('button', { name: '转移时间线' })).toBeVisible();
  else expect(screen.queryByRole('button', { name: '转移时间线' })).not.toBeInTheDocument();
});
