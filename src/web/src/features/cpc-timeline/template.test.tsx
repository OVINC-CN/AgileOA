import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { createMemoryRouter, Outlet, RouterProvider } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import type { TimelineInfo } from '@/features/cpc-timeline/api';
import { newNode, type TimelineNode } from '@/features/cpc-timeline/model';
import { NodeEditor } from '@/features/cpc-timeline/node-editor';
import { NodeList } from '@/features/cpc-timeline/node-list';
import { TimelinePage } from '@/features/cpc-timeline/template-page';
import { TemplatePreview } from '@/features/cpc-timeline/template-preview';
import { WorkspaceCPCDataPage } from '@/pages/workspace-cpc-data-page';
import { PageNavigationHarness } from '@/test/page-navigation-harness';

const workspaceID = 'w7m2k9p4s6n8c3x5';

function FieldSourceHarness() {
  const [node, setNode] = useState(newNode);
  return <NodeEditor node={node} nodes={[node]} onChange={setNode} />;
}

it('节点可新建分组、选择已有分组及取消分组', async () => {
  const user = userEvent.setup();
  const existing = { ...newNode(), group_name: '已有阶段' };
  function GroupHarness() {
    const [node, setNode] = useState(newNode);
    return <NodeEditor node={node} nodes={[node, existing]} onChange={setNode} />;
  }
  render(<GroupHarness />);
  const group = screen.getByRole('combobox', { name: '节点分组' });
  expect(group).toHaveTextContent('不分组');
  await user.click(group);
  await user.click(screen.getByRole('option', { name: '已有阶段' }));
  expect(group).toHaveTextContent('已有阶段');
  await user.click(group);
  await user.type(screen.getByRole('searchbox', { name: '搜索节点分组' }), '新增阶段');
  await user.click(screen.getByRole('option', { name: '新建“新增阶段”' }));
  expect(group).toHaveTextContent('新增阶段');
  await user.click(group);
  await user.click(screen.getByRole('option', { name: '不分组' }));
  expect(group).toHaveTextContent('不分组');
});

it('预览用分割线标题展示连续分组，保留未分组节点及全局顺序', () => {
  const nodes = [
    { ...newNode(), name: '申请材料', group_name: '申请阶段' },
    { ...newNode(), name: '核对材料', group_name: '申请阶段' },
    { ...newNode(), name: '独立节点' },
    { ...newNode(), name: '后续材料', group_name: '申请阶段' },
  ];
  render(
    <QueryClientProvider client={new QueryClient()}>
      <TemplatePreview nodes={nodes} workspaceID={workspaceID} template={null} actions={[]} />
    </QueryClientProvider>,
  );
  const timeline = within(screen.getByRole('list', { name: '时间线预览' }));
  expect(timeline.getAllByRole('heading', { level: 3 }).map((title) => title.textContent)).toEqual([
    '申请材料',
    '核对材料',
    '独立节点',
    '后续材料',
  ]);
  const headings = timeline.getAllByRole('heading', { level: 2, name: '申请阶段' });
  expect(headings).toHaveLength(2);
  expect(headings[0]?.previousElementSibling).toHaveAttribute('data-slot', 'separator');
  expect(headings[0]?.nextElementSibling).toHaveAttribute('data-slot', 'separator');
  expect(timeline.queryByRole('button')).not.toBeInTheDocument();
  expect(timeline.getByRole('heading', { name: '独立节点' })).toBeVisible();
  expect(timeline.getByRole('heading', { name: '后续材料' }).closest('li')).toHaveTextContent('4');
});

it('发生时间可切换手动填写、出生日期和成年日期，引用时提示只读', async () => {
  const user = userEvent.setup();
  render(<FieldSourceHarness />);
  const source = screen.getByRole('combobox', { name: '发生时间数据来源' });
  expect(source).toHaveTextContent('手动填写');
  await user.click(source);
  await user.click(screen.getByRole('option', { name: '出生日期' }));
  expect(source).toHaveTextContent('出生日期');
  expect(screen.getByText(/内容来自学生档案，请在学生信息中修改/u)).toBeVisible();
  await user.click(source);
  await user.click(screen.getByRole('option', { name: '成年日期（出生日期加 18 年）' }));
  expect(source).toHaveTextContent('成年日期（出生日期加 18 年）');
  await user.click(source);
  await user.click(screen.getByRole('option', { name: '手动填写' }));
  expect(screen.queryByText(/内容来自学生档案，请在学生信息中修改/u)).not.toBeInTheDocument();
});

it('模板详情直接展示基准标记和附件列表，附件保持只读', async () => {
  const reference = { ...newNode(), name: '提交申请', is_baseline: true, group_name: '申请阶段' };
  const current = { ...newNode(), name: '材料核对', group_name: '申请阶段' };
  current.rules = [
    {
      id: 'rule_range',
      reference_node_id: reference.id,
      min_offset: { years: 0, months: 0, days: -10, workdays: 0 },
      max_offset: { years: 0, months: 0, days: 20, workdays: 0 },
    },
  ];
  const record: TimelineInfo = {
    id: 7,
    name: '虚构模板',
    description: '',
    node_count: 2,
    revision: 1,
    created_at: '2026-11-06T00:00:00Z',
    updated_at: '2026-11-06T00:00:00Z',
    definition: { nodes: [reference, current] },
  };
  const fetch = vi.fn((path: string) => {
    const attachments = path.includes(`/nodes/${reference.id}/`)
      ? [
          {
            id: 11,
            timeline_template_id: record.id,
            node_id: reference.id,
            category: 'cpc_timeline_node',
            filename: '虚构材料.pdf',
            size: 1024,
            state: 'ready',
            revision: 1,
            created_at: record.created_at,
            updated_at: record.created_at,
          },
        ]
      : [];
    const list = path.includes('/nodes/')
      ? { items: attachments, total: attachments.length, page: 1, page_size: 5 }
      : { items: [record], total: 1, page: 1, page_size: 5 };
    return Promise.resolve(
      Response.json({
        trace: '4bf92f3577b34da6a3ce929d0e0e4736',
        error_msg: '',
        data: path.endsWith('/7') ? record : list,
      }),
    );
  });
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <TimelinePage
        workspaceID={workspaceID}
        workspaceName="虚构空间"
        refresh={() => Promise.resolve()}
        actions={[
          'cpc.timeline_template.list',
          'cpc.timeline_template.detail',
          'cpc.timeline_template.create',
          'cpc.timeline_template.delete',
          'cpc.timeline_template.attachment.read',
          'cpc.timeline_template.attachment.upload',
          'cpc.timeline_template.attachment.delete',
        ]}
      />
    </QueryClientProvider>,
  );
  const table = within(screen.getByRole('table'));
  const name = await table.findByRole('button', { name: record.name });
  const [firstHeader] = table.getAllByRole('columnheader');
  expect(firstHeader).toHaveTextContent('模板名称');
  expect(firstHeader).toHaveClass('table-pinned', 'left-0');
  expect(name.closest('td')).toHaveClass('table-pinned', 'left-0');
  await userEvent.click(name);
  const detail = within(await screen.findByRole('dialog', { name: '时间线模板详情' }));
  expect(detail.getByRole('list', { name: '时间线预览' })).toBeVisible();
  expect(detail.getByText('基准节点')).toBeVisible();
  expect(detail.getByRole('region', { name: '材料核对的时间校验规则' })).toHaveTextContent(
    '材料核对应该在提交申请的10天前至20天后。',
  );
  expect(detail.queryByRole('button', { name: /节点.*附件/u })).not.toBeInTheDocument();
  expect(detail.queryByLabelText('上传附件')).not.toBeInTheDocument();
  expect(await detail.findByText('虚构材料.pdf')).toBeVisible();
  expect(detail.getByRole('button', { name: '虚构材料.pdf' })).toBeVisible();
  expect(detail.queryByRole('table')).not.toBeInTheDocument();
  expect(detail.getByRole('heading', { level: 2, name: '申请阶段' })).toBeVisible();
  expect(detail.getByRole('heading', { name: '提交申请' })).toBeVisible();
  await userEvent.click(detail.getByRole('button', { name: '关闭抽屉' }));
  expect(table.queryByRole('button', { name: '克隆' })).not.toBeInTheDocument();
  expect(table.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
  await userEvent.click(table.getByRole('button', { name: `更多操作 ${record.name}` }));
  expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['克隆', '删除']);
  await userEvent.click(screen.getByRole('menuitem', { name: '删除' }));
  expect(screen.getByRole('menuitem', { name: '确认' })).toHaveAccessibleDescription(
    `确认删除 ${record.name} ?`,
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function NodeHarness({ initial }: { initial: TimelineNode[] }) {
  const [nodes, setNodes] = useState(initial);
  const [selected, setSelected] = useState(initial[0]?.id ?? '');
  return (
    <NodeList
      nodes={nodes}
      onChange={setNodes}
      selectedID={selected}
      onSelect={setSelected}
      disabled={false}
    />
  );
}

it('编辑列表展示连续分组，键盘排序更新分组并保留节点引用', async () => {
  const first = { ...newNode(), name: '前置材料', group_name: '申请阶段' };
  const second = { ...newNode(), name: '参考材料' };
  const third = { ...newNode(), name: '后续材料', group_name: '申请阶段' };
  first.rules = [
    {
      id: 'rule_first',
      reference_node_id: second.id,
      min_offset: null,
      max_offset: { years: 0, months: 1, days: 0, workdays: 0 },
    },
  ];
  render(<NodeHarness initial={[first, second, third]} />);
  const user = userEvent.setup();
  const list = within(screen.getByRole('list', { name: '时间线节点排序' }));
  expect(list.getAllByRole('heading', { name: '申请阶段' })).toHaveLength(2);
  expect(list.getByRole('separator')).toBeVisible();
  expect(list.getByRole('button', { name: '1. 前置材料' })).toBeVisible();

  list.getByRole('button', { name: '拖动节点 1，方向键调整顺序' }).focus();
  await user.keyboard('{ArrowDown}');
  expect(list.getByRole('heading', { name: '申请阶段' })).toBeVisible();
  expect(list.queryByRole('separator')).not.toBeInTheDocument();
  expect(list.getAllByRole('listitem')[0]).toHaveTextContent('参考材料');
  expect(list.getAllByRole('listitem')[1]).toHaveTextContent('前置材料');
  await user.click(list.getByRole('button', { name: '删除节点 1' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('请先处理引用');
  expect(list.getAllByRole('listitem')).toHaveLength(3);
});

function renderDataPage(actions: string[], tab = '') {
  const fetch = vi.fn(() =>
    Promise.resolve(
      Response.json({
        trace: '4bf92f3577b34da6a3ce929d0e0e4736',
        error_msg: '',
        data: { items: [], total: 0, page: 1, page_size: 5 },
      }),
    ),
  );
  vi.stubGlobal('fetch', fetch);
  const router = createMemoryRouter(
    [
      {
        element: (
          <PageNavigationHarness workspaceID={workspaceID} actions={actions}>
            <Outlet
              context={{
                workspaceID,
                workspaceName: '虚构空间',
                actions,
                refresh: () => Promise.resolve(),
              }}
            />
          </PageNavigationHarness>
        ),
        children: [{ path: '/cpc/data-management', Component: WorkspaceCPCDataPage }],
      },
    ],
    {
      initialEntries: [`/cpc/data-management?workspace=${workspaceID}${tab ? `&tab=${tab}` : ''}`],
    },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, fetch };
}

it('仅有模板列表权限时默认显示模板，不读取党支部；非列表权限不显示操作', async () => {
  const { fetch } = renderDataPage(
    ['cpc.timeline_template.list', 'cpc.party_branch.create'],
    'party-branches',
  );
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(screen.getByRole('link', { name: '时间线模板', current: 'page' })).toBeVisible();
  expect(screen.getByRole('link', { name: '时间线模板' })).toHaveAttribute(
    'href',
    `/cpc/data-management?workspace=${workspaceID}&tab=timeline-templates`,
  );
  expect(screen.queryByRole('link', { name: '党支部' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '新增模板' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '克隆' })).not.toBeInTheDocument();
  expect(await screen.findByText('暂无符合条件的时间线模板')).toBeVisible();
  expect(fetch).toHaveBeenCalledOnce();
  expect(fetch.mock.calls[0]).toEqual(
    expect.arrayContaining([
      expect.stringContaining(`/v1/workspaces/${workspaceID}/cpc-timeline-templates?`),
    ]),
  );
});

function cloneRecord(): TimelineInfo {
  return {
    id: 7,
    name: '虚构克隆模板',
    description: '完整复制节点',
    node_count: 1,
    revision: 3,
    created_at: '2026-11-09T00:00:00Z',
    updated_at: '2026-11-09T00:00:00Z',
    definition: { nodes: [newNode()] },
    transfers: [],
  };
}

function cloneResponse(data: unknown, status = 200, errorMessage = '') {
  return Response.json(
    { trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: errorMessage, data },
    { status },
  );
}

function renderClonePage(fetch: (path: string, init?: RequestInit) => Promise<Response>) {
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <TimelinePage
        workspaceID={workspaceID}
        workspaceName="虚构空间"
        refresh={() => Promise.resolve()}
        actions={['cpc.timeline_template.list', 'cpc.timeline_template.create']}
      />
    </QueryClientProvider>,
  );
}

it('仅有列表和新建权限即可直接克隆，保留原列表并等待刷新后退出加载', async () => {
  const record = cloneRecord();
  const copied = { ...record, id: 8, name: `${record.name} (副本)`, revision: 1 };
  let finishClone: (response: Response) => void = () => {
    throw new Error('尚未提交克隆');
  };
  let items = [record];
  const fetch = vi.fn((path: string) => {
    if (path.endsWith('/7/clone')) {
      return new Promise<Response>((resolve) => {
        finishClone = resolve;
      });
    }
    return Promise.resolve(cloneResponse({ items, total: items.length, page: 1, page_size: 5 }));
  });
  renderClonePage(fetch);
  const more = { name: `更多操作 ${record.name}` };
  expect(screen.queryByRole('button', { name: '详情' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: record.name })).not.toBeInTheDocument();
  await userEvent.click(await screen.findByRole('button', more));
  expect(screen.queryByRole('menuitem', { name: '删除' })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole('menuitem', { name: '克隆' }));
  await waitFor(() => {
    const button = screen.getByRole('button', more);
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
  });
  expect(screen.getByText(record.name)).toBeVisible();
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(fetch).toHaveBeenCalledWith(
    `/v1/workspaces/${workspaceID}/cpc-timeline-templates/7/clone`,
    expect.objectContaining({ method: 'POST', body: JSON.stringify({ revision: 3 }) }),
  );
  items = [copied, record];
  finishClone(cloneResponse(copied));
  expect(await screen.findByText(copied.name)).toBeVisible();
  await waitFor(() => {
    expect(
      screen
        .getAllByRole('button', { name: /^更多操作 /u })
        .every((item) => !item.hasAttribute('disabled')),
    ).toBe(true);
  });
  expect(fetch.mock.calls.filter(([path]) => path.endsWith('/clone'))).toHaveLength(1);
  expect(fetch.mock.calls.every(([path]) => path.endsWith('/7/clone') || path.includes('?'))).toBe(
    true,
  );
});

it('克隆重名时显示服务端错误，不重试其他副本名称或修改原列表', async () => {
  const record = cloneRecord();
  const fetch = vi.fn((path: string) =>
    Promise.resolve(
      path.endsWith('/7/clone')
        ? cloneResponse(null, 409, '当前空间已存在同名时间线模板')
        : cloneResponse({ items: [record], total: 1, page: 1, page_size: 5 }),
    ),
  );
  renderClonePage(fetch);
  const more = { name: `更多操作 ${record.name}` };
  await userEvent.click(await screen.findByRole('button', more));
  await userEvent.click(screen.getByRole('menuitem', { name: '克隆' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('当前空间已存在同名时间线模板');
  await waitFor(() => {
    expect(screen.getByRole('button', more)).toBeEnabled();
  });
  expect(screen.getByText(record.name)).toBeVisible();
  expect(screen.queryByText(`${record.name} (副本)`)).not.toBeInTheDocument();
  expect(fetch.mock.calls.filter(([path]) => path.endsWith('/clone'))).toHaveLength(1);
  expect(fetch).toHaveBeenCalledTimes(2);
});

it('导航子菜单切换保留工作空间并只展示当前授权页面', async () => {
  const { router } = renderDataPage(['cpc.timeline_template.list', 'cpc.party_branch.list']);
  expect(screen.getByRole('link', { name: '党支部', current: 'page' })).toBeVisible();
  await userEvent.click(screen.getByRole('link', { name: '时间线模板' }));
  await waitFor(() => {
    expect(screen.getByRole('link', { name: '时间线模板', current: 'page' })).toBeVisible();
  });
  expect(new URLSearchParams(router.state.location.search).get('workspace')).toBe(workspaceID);
  expect(new URLSearchParams(router.state.location.search).get('tab')).toBe('timeline-templates');
  expect(router.state.location.pathname).toBe('/cpc/data-management');
});
