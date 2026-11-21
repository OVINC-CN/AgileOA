import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  createMemoryRouter,
  MemoryRouter,
  Outlet,
  RouterProvider,
  useLocation,
} from 'react-router';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';

import {
  type Application,
  applicationSchema,
  type ApplicationScope,
} from '@/features/project/application-api';
import { ApplicationDrawer } from '@/features/project/application-drawer';
import { ApplicationList } from '@/features/project/application-list';
import { ProjectPage } from '@/features/project/project-page';
import { StudentAwardsPage } from '@/pages/student-awards-page';
import { cosMock, preparedUpload } from '@/test/mock-cos';
import { PageNavigationHarness } from '@/test/page-navigation-harness';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const respond = (data: unknown) =>
  Response.json({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data });
function stubApplicationFetch(
  request: (path: string, options: RequestInit) => Response | Promise<Response> = () => {
    throw new Error('未预期的业务请求');
  },
) {
  const handler = (path: string, options: RequestInit) =>
    path.endsWith('/versions') ? Promise.resolve(respond({ items: [] })) : request(path, options);
  vi.stubGlobal('fetch', handler);
}
beforeEach(() => {
  stubApplicationFetch();
});
const filters = {
  names: [{ value: '优秀学生', label: '优秀学生' }],
  years: [{ value: '2026', label: '2026 年' }],
  education_levels: [],
  majors: [],
  grades: [],
};
const list = { items: [], total: 0, page: 1, page_size: 5 };
const rejectedApplication: Application = {
  id: 1,
  project_id: 1,
  project_name: '优秀学生',
  project_description: '年度评选说明',
  year: 2026,
  allow_self_apply: false,
  self_apply_deadline: null,
  student_id: 7,
  student_no: '0007',
  student_name: '虚构学生',
  class_name: '一班',
  status: 'rejected',
  current_step: 0,
  current_reviewer: '申报人',
  can_edit: true,
  can_approve: true,
  can_reject: false,
  revision: 2,
  created_at: '2026-11-12T00:00:00Z',
  updated_at: '2026-11-12T01:00:00Z',
  statement: '年度申报',
  rank: null,
  score: null,
  details: [],
  steps: [],
  detail_names: [],
  history: [],
};

afterEach(() => {
  vi.unstubAllGlobals();
});

it('编辑中的申报即时应用项目说明和开关，保留草稿及原并发版本', async () => {
  const client = new QueryClient();
  const renderDrawer = (record: Application) => (
    <QueryClientProvider client={client}>
      <ApplicationDrawer workspaceID={workspaceID} scope="mine" record={record} onClose={vi.fn()} />
    </QueryClientProvider>
  );
  const { rerender } = render(renderDrawer(rejectedApplication));
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '重新申报' }));
  await user.type(screen.getByLabelText('申报说明'), '未保存草稿');
  await user.type(screen.getByLabelText('总分（选填）'), '88');
  expect(screen.getByRole('button', { name: '添加子项' })).toBeVisible();

  const updated = {
    ...rejectedApplication,
    project_name: '更新后的奖项',
    project_description: '更新后的奖项说明',
    detail_names: ['志愿服务'],
    score_calculation_enabled: true,
  };
  rerender(renderDrawer(updated));
  expect(screen.getByText('更新后的奖项说明')).toBeVisible();
  expect(screen.getByRole('dialog', { name: '编辑申报' })).toHaveTextContent('更新后的奖项');
  expect(screen.getByRole('button', { name: '添加子项' })).toBeEnabled();
  expect(screen.queryByLabelText('总分（选填）')).not.toBeInTheDocument();
  expect(screen.queryByLabelText('排名（选填）')).not.toBeInTheDocument();
  expect(screen.getByLabelText('申报说明')).toHaveValue('年度申报未保存草稿');

  rerender(renderDrawer({ ...updated, score_calculation_enabled: false }));
  expect(screen.getByLabelText('总分（选填）')).toHaveValue(88);
  const request = vi.fn((_path: string, options: RequestInit) => {
    if (typeof options.body !== 'string') throw new Error('保存请求缺少 JSON 正文');
    const body = JSON.parse(options.body) as unknown;
    expect(body).toMatchObject({ revision: 2, statement: '年度申报未保存草稿', score: 88 });
    return Promise.resolve(respond({ ...updated, revision: 3 }));
  });
  stubApplicationFetch(request);
  await user.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() => {
    expect(request).toHaveBeenCalledOnce();
    expect(screen.getByRole('button', { name: '重新申报' })).toBeEnabled();
  });
});

it('刷新申报后展示重置后的首级流程及重置历史', async () => {
  const client = new QueryClient();
  const renderDrawer = (record: Application) => (
    <QueryClientProvider client={client}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="manage"
        record={record}
        onClose={vi.fn()}
      />
    </QueryClientProvider>
  );
  const { rerender } = render(renderDrawer(rejectedApplication));
  const updated = applicationSchema.parse({
    ...rejectedApplication,
    status: 'pending',
    current_step: 1,
    current_reviewer: '新的审批人',
    revision: 3,
    can_edit: false,
    can_approve: false,
    steps: [
      {
        kind: 'user',
        sign_mode: 'all',
        user_ids: [9],
        reviewer_label: '新的审批人',
        position: 1,
        ready: true,
        reviewers: [{ identity_type: 'user', id: 9, label: '新的审批人', approved: false }],
      },
    ],
    history: [
      {
        id: 2,
        step: 0,
        operation: 'reset',
        actor_type: 'user',
        actor_id: 9,
        actor_name: '奖项编辑人',
        comment: '奖项审批流已更新，未完成申报的审批状态已重置',
        created_at: '2026-11-16T00:00:00Z',
      },
    ],
  });
  rerender(renderDrawer(updated));
  const user = userEvent.setup();
  await user.click(screen.getByRole('tab', { name: '审批流程' }));
  expect(screen.getByRole('list', { name: '审批节点' })).toHaveTextContent('待审批');
  expect(screen.getByRole('list', { name: '审批节点' })).toHaveTextContent('已通过 0 / 1');
  await user.click(screen.getByRole('tab', { name: '流转记录' }));
  expect(screen.getByText('重置审批流')).toBeVisible();
  expect(screen.getByText('奖项编辑人')).toBeVisible();
});

it('编辑过程中关闭本级编辑开关会禁用表单及保存，保留未保存内容', async () => {
  const record = { ...rejectedApplication, status: 'pending' as const, current_step: 1 };
  const client = new QueryClient();
  const renderDrawer = (current: Application) => (
    <QueryClientProvider client={client}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="manage"
        record={current}
        onClose={vi.fn()}
      />
    </QueryClientProvider>
  );
  const { rerender } = render(renderDrawer(record));
  const user = userEvent.setup();
  const request = vi.fn();
  stubApplicationFetch(request);
  await user.click(screen.getByRole('button', { name: '编辑' }));
  await user.type(screen.getByLabelText('申报说明'), '未保存内容');
  rerender(renderDrawer({ ...record, can_edit: false, revision: record.revision + 1 }));
  expect(screen.getByLabelText('申报说明')).toHaveValue('年度申报未保存内容');
  expect(screen.getByLabelText('申报说明')).toBeDisabled();
  expect(screen.getByRole('button', { name: '保存' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: '保存' }));
  expect(request).not.toHaveBeenCalled();
});

it.each([
  ['manage', 'approve'],
  ['manage', 'reject'],
  ['student-reviews', 'approve'],
  ['student-reviews', 'reject'],
] as const)('只读审批级别隐藏编辑入口并允许处理：%s/%s', async (scope, operation) => {
  const record = applicationSchema.parse({
    ...rejectedApplication,
    status: 'pending',
    current_step: 1,
    can_edit: false,
    can_approve: true,
    can_reject: true,
    steps: [
      {
        kind: 'applicant',
        reviewer_label: '申报人',
        allow_edit: false,
        position: 1,
        ready: true,
        reviewers: [],
      },
    ],
  });
  const request = vi.fn((path: string, options: RequestInit) => {
    expect(path).toContain(`/1/${operation}`);
    expect(JSON.parse(typeof options.body === 'string' ? options.body : '')).toMatchObject({
      statement: record.statement,
      details: record.details,
      revision: record.revision,
      comment: '已核对',
    });
    return Promise.resolve(respond({ ...record, revision: record.revision + 1 }));
  });
  stubApplicationFetch(request);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope={scope}
        record={record}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  expect(screen.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '上传附件' })).not.toBeInTheDocument();
  await user.click(screen.getByRole('tab', { name: '审批流程' }));
  expect(screen.getByText('内容只读')).toBeVisible();
  const label = operation === 'approve' ? '通过' : '驳回';
  await user.click(screen.getByRole('button', { name: label }));
  const dialog = screen.getByRole('dialog', { name: `${label}申报` });
  await user.type(
    within(dialog).getByLabelText(operation === 'approve' ? '审批说明（选填）' : '驳回意见'),
    '已核对',
  );
  await user.click(within(dialog).getByRole('button', { name: label }));
  await waitFor(() => {
    expect(request).toHaveBeenCalledOnce();
    expect(screen.queryByRole('dialog', { name: `${label}申报` })).not.toBeInTheDocument();
  });
});

it.each<ApplicationScope>(['mine', 'manage', 'student-reviews'])(
  '强制动态计算时 %s 编辑隐藏总分和排名输入并忽略旧的手填值',
  async (scope) => {
    const record: Application = {
      ...rejectedApplication,
      score_calculation_enabled: true,
      score: 46.480000000000004,
      rank: 2,
    };
    const submitted: Record<string, unknown>[] = [];
    stubApplicationFetch(
      vi.fn((_path: string, options: RequestInit) => {
        submitted.push(
          JSON.parse(typeof options.body === 'string' ? options.body : '') as Record<
            string,
            unknown
          >,
        );
        return Promise.resolve(respond({ ...record, revision: record.revision + 1 }));
      }),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ApplicationDrawer
          workspaceID={workspaceID}
          scope={scope}
          record={record}
          onClose={vi.fn()}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: scope === 'mine' ? '重新申报' : '编辑' }));
    expect(screen.queryByRole('spinbutton', { name: '总分（选填）' })).not.toBeInTheDocument();
    expect(screen.queryByRole('spinbutton', { name: '排名（选填）' })).not.toBeInTheDocument();
    expect(screen.getByText('保存、提交时按项目公式计算。')).toBeVisible();
    expect(screen.getByText('46.48')).toBeVisible();
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(submitted).toHaveLength(1);
    });
    expect(submitted[0]).toMatchObject({ score: null, rank: null });
  },
);

function ApplicationLocation() {
  const location = useLocation();
  return <output aria-label="申报地址">{location.pathname + location.search}</output>;
}

it.each<{ scope: ApplicationScope; page: string; tab: string; resource: string }>([
  { scope: 'manage', page: '/stu/projects', tab: 'applications', resource: 'project-applications' },
  {
    scope: 'mine',
    page: '/student-service/projects',
    tab: 'mine',
    resource: 'student-service/project-applications',
  },
  {
    scope: 'student-reviews',
    page: '/student-service/projects',
    tab: 'reviews',
    resource: 'student-service/project-reviews',
  },
])(
  '通知链接在空列表中直接打开正确范围的申报详情：$scope',
  async ({ scope, page, tab, resource }) => {
    const requests: string[] = [];
    stubApplicationFetch(
      vi.fn((path: string) => {
        requests.push(path);
        return Promise.resolve(
          respond(
            path.endsWith('/7')
              ? {
                  ...rejectedApplication,
                  id: 7,
                  status: 'approved',
                  can_edit: false,
                  can_approve: false,
                }
              : path.includes('/filter-options')
                ? filters
                : list,
          ),
        );
      }),
    );
    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <MemoryRouter
          initialEntries={[`${page}?workspace=${workspaceID}&tab=${tab}&application_id=7`]}
        >
          <ApplicationList
            workspaceID={workspaceID}
            scope={scope}
            breadcrumb={null}
            panelID="application-panel"
            panelLabel="申报列表"
          />
          <ApplicationLocation />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByRole('dialog', { name: '申报详情' });
    await screen.findByText('年度申报');
    expect(requests).toContain(`/v1/workspaces/${workspaceID}/${resource}/7`);
    await userEvent.setup().click(screen.getByRole('button', { name: '关闭' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: '申报详情' })).not.toBeInTheDocument();
    });
    expect(screen.getByLabelText('申报地址')).toHaveTextContent(
      `${page}?workspace=${workspaceID}&tab=${tab}`,
    );
  },
);

it('学生草稿展示当前截止时间和自动提交说明，系统流转记录通过契约校验', () => {
  const record = applicationSchema.parse({
    ...rejectedApplication,
    status: 'draft',
    self_apply_deadline: '2026-12-31T15:59:00Z',
    history: [
      {
        id: 1,
        step: 0,
        operation: 'submit',
        actor_type: 'system',
        actor_id: 0,
        actor_name: '系统',
        comment: '截止时间已过，系统自动提交草稿申报',
        created_at: '2026-12-31T16:00:00Z',
      },
    ],
  });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer workspaceID={workspaceID} scope="mine" record={record} onClose={vi.fn()} />
    </QueryClientProvider>,
  );
  const notice = screen.getByRole('note', { name: '自动提交说明' });
  expect(notice).toHaveTextContent('自行申报截止时间');
  expect(notice).toHaveTextContent('截止时间后 1 分钟自动提交并进入审批');
  expect(notice).toHaveTextContent('未保存的修改不会自动提交');
});

it('组织节点审批人直接进入填报管理，刷新收进表格，筛选复用相邻 Tab', async () => {
  const requests: string[] = [];
  stubApplicationFetch(
    vi.fn((path: string) => {
      requests.push(path);
      return Promise.resolve(respond(path.includes('/filter-options') ? filters : list));
    }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[`/stu/projects?workspace=${workspaceID}`]}>
        <ProjectPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={['project.application.list']}
          canReviewApplications
          refresh={() => Promise.resolve()}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  await screen.findByText('暂无符合条件的申报单');
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
    '学生工作/评奖评优/填报管理',
  );
  expect(screen.getByRole('combobox', { name: '审批状态' })).toHaveTextContent('待我审批');
  expect(requests.some((path) => path.includes('status=pending_mine'))).toBe(true);
  expect(screen.queryByRole('tab', { name: '奖项管理' })).not.toBeInTheDocument();
  const header = screen.getByRole('navigation', { name: '面包屑' }).closest('header');
  if (!header) throw new Error('页面缺少顶部操作区');
  expect(within(header).queryByRole('button', { name: '刷新' })).not.toBeInTheDocument();
  const tableCard = screen.getByRole('table').closest<HTMLElement>('[data-slot="card"]');
  if (!tableCard) throw new Error('页面缺少表格操作区');
  expect(within(tableCard).getByRole('button', { name: '刷新' })).toBeVisible();
  expect(screen.getByPlaceholderText('学号或姓名')).toBeVisible();
  expect(screen.getByRole('combobox', { name: '项目名称' })).toHaveTextContent('全部项目');
  expect(screen.getByRole('combobox', { name: '归属年份' })).toHaveTextContent('全部年份');
  const user = userEvent.setup();
  await user.click(screen.getByRole('combobox', { name: '项目名称' }));
  await user.click(await screen.findByRole('option', { name: '优秀学生' }));
  await waitFor(() => {
    expect(
      requests.some((path) => decodeURIComponent(path).includes('project_name=优秀学生')),
    ).toBe(true);
  });
  expect(requests.every((path) => path.includes('/project-applications'))).toBe(true);
});

it('学生两个子 Tab 使用相同页面布局，只有我的申报提供新建入口', async () => {
  stubApplicationFetch(
    vi.fn((path: string) =>
      Promise.resolve(respond(path.includes('/filter-options') ? filters : list)),
    ),
  );
  const router = createMemoryRouter(
    [
      {
        element: (
          <PageNavigationHarness workspaceID={workspaceID} actions={[]} studentService>
            <Outlet context={{ workspaceID }} />
          </PageNavigationHarness>
        ),
        children: [{ path: '/student-service/projects', Component: StudentAwardsPage }],
      },
    ],
    { initialEntries: [`/student-service/projects?workspace=${workspaceID}`] },
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByText('暂无符合条件的申报单');
  expect(screen.getByRole('button', { name: '新建申报' })).toBeVisible();
  expect(screen.getByPlaceholderText('学号或姓名')).toBeVisible();
  const header = screen.getByRole('navigation', { name: '面包屑' }).closest('header');
  if (!header) throw new Error('页面缺少顶部操作区');
  expect(within(header).queryByRole('button', { name: '刷新' })).not.toBeInTheDocument();
  const tableCard = screen.getByRole('table').closest<HTMLElement>('[data-slot="card"]');
  if (!tableCard) throw new Error('页面缺少表格操作区');
  expect(within(tableCard).getByRole('button', { name: '刷新' })).toBeVisible();
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole('link', { name: '我的审核' }));
  expect(screen.queryByRole('button', { name: '新建申报' })).not.toBeInTheDocument();
  expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
    '学生服务/评奖评优/我的审核',
  );
  expect(new URLSearchParams(router.state.location.search).get('tab')).toBe('reviews');
  expect(screen.getByRole('combobox', { name: '审批状态' })).toHaveTextContent('待我审批');
});

it('驳回详情仅保留重新申报入口，进入编辑不请求接口且切换 Tab 保留草稿', async () => {
  const request = vi.fn();
  stubApplicationFetch(request);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="mine"
        record={rejectedApplication}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole('button', { name: '重新申报' })).toBeEnabled();
  expect(screen.queryByRole('button', { name: '关闭' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '重新提交' })).not.toBeInTheDocument();
  expect(screen.queryByRole('textbox', { name: '申报说明' })).not.toBeInTheDocument();
  const user = userEvent.setup();
  await user.click(screen.getByRole('tab', { name: '流转记录' }));
  await user.click(screen.getByRole('button', { name: '重新申报' }));
  expect(screen.getByRole('dialog', { name: '编辑申报' })).toBeVisible();
  expect(screen.getByRole('button', { name: '保存' })).toBeVisible();
  expect(screen.getByRole('button', { name: '提交申报' })).toBeVisible();
  expect(screen.queryByRole('button', { name: '重新申报' })).not.toBeInTheDocument();
  await user.type(screen.getByLabelText('申报说明'), '补充说明');
  await user.click(screen.getByRole('tab', { name: '审批流程' }));
  expect(screen.getByRole('list', { name: '审批节点' })).toBeVisible();
  expect(screen.getByLabelText('申报说明')).not.toBeVisible();
  await user.click(screen.getByRole('tab', { name: '流转记录' }));
  expect(screen.getByText('暂无流转记录')).toBeVisible();
  await user.click(screen.getByRole('tab', { name: '主要信息' }));
  expect(screen.getByLabelText('申报说明')).toHaveValue('年度申报补充说明');
  await user.click(screen.getByRole('button', { name: '取消' }));
  expect(screen.getByRole('button', { name: '重新申报' })).toBeEnabled();
  await user.click(screen.getByRole('button', { name: '重新申报' }));
  expect(screen.getByLabelText('申报说明')).toHaveValue('年度申报');
  expect(request).not.toHaveBeenCalled();
});

it.each(['', '材料已核对，可以通过'])('通过时说明可选，并随操作提交：%s', async (comment) => {
  const record: Application = {
    ...rejectedApplication,
    status: 'pending',
    current_step: 1,
    current_reviewer: '虚构审批人',
    can_approve: true,
    can_reject: true,
  };
  const requests: { path: string; body: unknown }[] = [];
  stubApplicationFetch(
    vi.fn((path: string, options: RequestInit) => {
      if (typeof options.body !== 'string') throw new Error('审批请求缺少 JSON 正文');
      requests.push({ path, body: JSON.parse(options.body) as unknown });
      return Promise.resolve(
        respond({
          ...record,
          status: 'approved',
          can_edit: false,
          can_approve: false,
          can_reject: false,
          revision: 3,
        }),
      );
    }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope="manage"
        record={record}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '通过' }));
  const decision = screen.getByRole('dialog', { name: '通过申报' });
  const input = within(decision).getByLabelText('审批说明（选填）');
  expect(input).not.toBeRequired();
  if (comment) await user.type(input, comment);
  await user.click(within(decision).getByRole('button', { name: '通过' }));
  await waitFor(() => {
    expect(requests).toHaveLength(1);
    expect(requests[0]?.path).toContain(`/project-applications/${String(record.id)}/approve`);
    expect(requests[0]?.body).toMatchObject({ comment, revision: record.revision });
  });
});

it.each([
  { status: 'draft', scope: 'mine' },
  { status: 'rejected', scope: 'mine' },
  { status: 'pending', scope: 'manage' },
] as const)(
  '可编辑的申报统一支持新增、更换、修改和移除子项：$status',
  async ({ status, scope }) => {
    const key = '0123456789abcdef0123456789abcdef';
    const record: Application = {
      ...rejectedApplication,
      status,
      current_step: scope === 'manage' ? 1 : 0,
      detail_names: ['志愿服务', '社会实践'],
      details: [{ id: 1, key, name: '志愿服务', description: '', score: null, attachments: [] }],
    };
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ApplicationDrawer
          workspaceID={workspaceID}
          scope={scope}
          record={record}
          onClose={vi.fn()}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(
      screen.getByRole('button', {
        name: scope === 'mine' && status === 'rejected' ? '重新申报' : '编辑',
      }),
    );
    const add = screen.getByRole('button', { name: '添加子项' });
    const name = screen.getByRole('combobox', { name: '子项名称' });
    expect(add).toBeEnabled();
    expect(name).toBeEnabled();
    await user.click(name);
    await user.click(screen.getByRole('option', { name: '社会实践' }));
    expect(name).toHaveTextContent('社会实践');
    await user.click(add);
    expect(screen.getAllByRole('combobox', { name: '子项名称' })).toHaveLength(2);
    await user.click(screen.getByRole('button', { name: '移除子项 2' }));
    const description = screen.getByLabelText('子项说明');
    await user.type(description, '补充说明');
    expect(description).toHaveValue('补充说明');
    await user.click(screen.getByRole('button', { name: '移除子项 1' }));
    expect(screen.getByText('暂无评分子项')).toBeVisible();
  },
);

it('申报上传仅允许图片和 PDF，选择其他类型时不保存申报或发起上传', async () => {
  const request = vi.fn();
  stubApplicationFetch(request);
  const record: Application = {
    ...rejectedApplication,
    detail_names: ['志愿服务'],
    details: [
      {
        id: 1,
        key: '0123456789abcdef0123456789abcdef',
        name: '志愿服务',
        description: '',
        score: null,
        attachments: [],
      },
    ],
  };
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer workspaceID={workspaceID} scope="mine" record={record} onClose={vi.fn()} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup({ applyAccept: false });
  await user.click(screen.getByRole('button', { name: '重新申报' }));
  const input = screen.getByLabelText('上传附件');
  expect(new Set(input.getAttribute('accept')?.split(','))).toEqual(
    new Set(['image/*', 'application/pdf', '.pdf']),
  );
  await user.upload(input, new File(['fixture'], '材料.txt', { type: 'text/plain' }));
  expect(await screen.findByText('文件类型不在允许上传的范围内')).toBeVisible();
  expect(request).not.toHaveBeenCalled();
});

it.each([false, true])(
  '附件上传前确认保存全部修改，取消不请求，上传失败保留已保存内容：%s',
  async (failed) => {
    const record: Application = {
      ...rejectedApplication,
      detail_names: ['志愿服务'],
      details: [
        {
          id: 1,
          key: '0123456789abcdef0123456789abcdef',
          name: '志愿服务',
          description: '',
          score: null,
          attachments: [],
        },
      ],
    };
    const saved = { ...record, statement: '年度申报补充内容', revision: 3 };
    const requests: { path: string; options: RequestInit }[] = [];
    stubApplicationFetch((path, options) => {
      requests.push({ path, options });
      if (options.method === 'PUT') return respond(saved);
      if (path.endsWith('/complete')) return respond({ ...saved, revision: 4 });
      if (path.endsWith('/attachments')) return respond(preparedUpload());
      return respond(saved);
    });
    cosMock.putObject.mockReset();
    if (failed) cosMock.putObject.mockRejectedValue(new Error('fixture failure'));
    else cosMock.putObject.mockResolvedValue({ statusCode: 200 });
    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <ApplicationDrawer
          workspaceID={workspaceID}
          scope="mine"
          record={record}
          onClose={vi.fn()}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: '重新申报' }));
    await user.type(screen.getByLabelText('申报说明'), '补充内容');
    const openUpload = () => user.click(screen.getByRole('button', { name: '保存内容并上传' }));
    await openUpload();
    let confirmation = within(screen.getByRole('dialog', { name: '保存内容并上传附件' }));
    expect(
      confirmation.getByText('保存成功后，取消编辑或上传失败均不会撤销本次保存。'),
    ).toBeVisible();
    expect(requests).toHaveLength(0);
    await user.click(confirmation.getByRole('button', { name: '取消' }));
    expect(requests).toHaveLength(0);
    expect(screen.getByLabelText('申报说明')).toHaveValue(saved.statement);
    await openUpload();
    confirmation = within(screen.getByRole('dialog', { name: '保存内容并上传附件' }));
    await user.upload(
      confirmation.getByLabelText('选择文件并保存上传'),
      new File(['fixture'], '材料.pdf', { type: 'application/pdf' }),
    );
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: '保存内容并上传附件' })).not.toBeInTheDocument(),
    );
    expect(requests[0]?.options.method).toBe('PUT');
    expect(
      JSON.parse(typeof requests[0]?.options.body === 'string' ? requests[0].options.body : ''),
    ).toMatchObject({
      statement: saved.statement,
      revision: record.revision,
      details: [{ name: '志愿服务' }],
    });
    expect(requests[1]?.path).toMatch(/\/attachments$/u);
    expect(
      JSON.parse(typeof requests[1]?.options.body === 'string' ? requests[1].options.body : ''),
    ).toMatchObject({
      application_revision: saved.revision,
    });
    if (failed) {
      expect(
        await screen.findByText('申报内容已保存，附件上传未完成。文件上传失败，请重试'),
      ).toBeVisible();
      await user.click(
        within(screen.getByRole('dialog', { name: '编辑申报' })).getByRole('button', {
          name: '取消',
        }),
      );
      expect(screen.getByRole('dialog', { name: '申报详情' })).toHaveTextContent(saved.statement);
    } else {
      expect(requests[2]?.path).toMatch(/\/attachments\/complete$/u);
      await user.upload(
        screen.getByLabelText('上传附件'),
        new File(['fixture'], '第二份材料.pdf', { type: 'application/pdf' }),
      );
      await waitFor(() => {
        expect(cosMock.putObject).toHaveBeenCalledTimes(2);
      });
      expect(requests.filter(({ options }) => options.method === 'PUT')).toHaveLength(1);
    }
    cosMock.putObject.mockResolvedValue({ statusCode: 200 });
  },
);

it.each(['draft', 'rejected'] as const)('关闭申报后，已有 $0 单据仍可提交', async (status) => {
  const record = { ...rejectedApplication, status };
  const requests: { path: string; body: unknown }[] = [];
  stubApplicationFetch(
    vi.fn((path: string, options: RequestInit) => {
      if (typeof options.body !== 'string') throw new Error('提交请求缺少 JSON 正文');
      requests.push({ path, body: JSON.parse(options.body) as unknown });
      return Promise.resolve(
        respond({
          ...record,
          status: 'pending',
          current_step: 1,
          can_edit: false,
          can_approve: false,
          can_reject: false,
          revision: 3,
        }),
      );
    }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer workspaceID={workspaceID} scope="mine" record={record} onClose={vi.fn()} />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  if (status === 'rejected') {
    await user.click(screen.getByRole('button', { name: '重新申报' }));
    expect(requests).toHaveLength(0);
    await user.type(screen.getByLabelText('申报说明'), '补充说明');
  }
  await user.click(screen.getByRole('button', { name: '提交申报' }));
  await waitFor(() => {
    expect(requests).toHaveLength(1);
    expect(requests[0]?.path).toContain(`/project-applications/${String(record.id)}/submit`);
    expect(requests[0]?.body).toMatchObject({
      statement: status === 'rejected' ? '年度申报补充说明' : record.statement,
      revision: record.revision,
    });
    expect(screen.getByRole('dialog', { name: '申报详情' })).toBeVisible();
    expect(screen.queryByRole('button', { name: '提交申报' })).not.toBeInTheDocument();
  });
});

it.each(['manage', 'student-reviews'] as const)('审批驳回直接退回申报人：%s', async (scope) => {
  const record: Application = {
    ...rejectedApplication,
    status: 'pending',
    current_step: 3,
    can_reject: true,
  };
  const requests: string[] = [];
  stubApplicationFetch(
    vi.fn((path: string) => {
      requests.push(path);
      return Promise.resolve(
        respond({
          ...record,
          status: 'rejected',
          current_step: 0,
          current_reviewer: '申报人',
          can_edit: false,
          can_approve: false,
          can_reject: false,
          revision: 3,
        }),
      );
    }),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ApplicationDrawer
        workspaceID={workspaceID}
        scope={scope}
        record={record}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: '驳回' }));
  const dialog = screen.getByRole('dialog', { name: '驳回申报' });
  expect(
    within(dialog).getByText('直接退回申报人，修改并重新提交后从第一级重新审批'),
  ).toBeVisible();
  await user.type(within(dialog).getByLabelText('驳回意见'), '请补充材料');
  await user.click(within(dialog).getByRole('button', { name: '驳回' }));
  await waitFor(() => {
    expect(requests).toHaveLength(1);
    expect(requests[0]).toContain(`/${String(record.id)}/reject`);
    expect(screen.queryByRole('dialog', { name: '驳回申报' })).not.toBeInTheDocument();
  });
});

it('没有查看填报权限时，即使是审批人也隐藏 Tab 且不请求申报接口', () => {
  const fetch = vi.fn();
  vi.stubGlobal('fetch', fetch);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[`/stu/projects?workspace=${workspaceID}&tab=applications`]}>
        <ProjectPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={[]}
          canReviewApplications
          refresh={() => Promise.resolve()}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  expect(screen.queryByRole('tab', { name: '填报管理' })).not.toBeInTheDocument();
  expect(fetch).not.toHaveBeenCalled();
});
