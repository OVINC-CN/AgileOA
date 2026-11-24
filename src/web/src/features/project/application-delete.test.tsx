import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import {
  applicationDetailOptions,
  type ApplicationSummary,
} from '@/features/project/application-api';
import { ApplicationList } from '@/features/project/application-list';
import { ProjectPage } from '@/features/project/project-page';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const trace = '4bf92f3577b34da6a3ce929d0e0e4736';
const respond = (data: unknown) => Response.json({ trace, error_msg: '', data });
const filters = { names: [], years: [], education_levels: [], majors: [], grades: [] };
const record: ApplicationSummary = {
  id: 7,
  project_id: 1,
  project_name: '优秀学生',
  year: 2026,
  allow_self_apply: false,
  student_id: 7,
  student_no: '0007',
  student_name: '虚构学生',
  class_name: '一班',
  status: 'approved',
  current_step: 1,
  current_reviewer: '',
  can_edit: false,
  can_approve: false,
  can_reject: false,
  revision: 3,
  created_at: '2026-11-12T00:00:00Z',
  updated_at: '2026-11-12T01:00:00Z',
};
const list = (items = [record], total = items.length, page = 1) => ({
  items,
  total,
  page,
  page_size: 5,
});
const moreActions = /^更多操作 /u;
const confirmation = { name: '确认' };

async function selectDelete(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: moreActions }));
  await user.click(screen.getByRole('menuitem', { name: '删除' }));
}

function renderApplications(actions = ['project.application.list', 'project.application.delete']) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const detailKey = applicationDetailOptions(workspaceID, 'manage', record.id).queryKey;
  client.setQueryData(detailKey, {
    ...record,
    project_description: '年度评选说明',
    statement: '旧的详情缓存',
    rank: null,
    score: null,
    details: [],
    steps: [],
    detail_names: [],
    history: [],
    self_apply_deadline: null,
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/stu/projects?workspace=${workspaceID}&tab=applications`]}>
        <ProjectPage
          workspaceID={workspaceID}
          workspaceName="虚构空间"
          actions={actions}
          refresh={() => Promise.resolve()}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { client, detailKey };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

it('查看权限和当前节点资格不自动提供删除填报入口', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) =>
      Promise.resolve(
        respond(
          path.includes('/filter-options')
            ? filters
            : list([{ ...record, status: 'pending', can_edit: true, can_approve: true }]),
        ),
      ),
    ),
  );
  renderApplications(['project.application.list']);
  await screen.findByRole('button', { name: '处理' });
  expect(screen.queryByRole('button', { name: moreActions })).not.toBeInTheDocument();
});

it.each(['pending', 'rejected', 'approved'] as const)(
  '独立删除权限可删除 %s 填报，不要求当前节点资格',
  async (status) => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) =>
        Promise.resolve(
          respond(path.includes('/filter-options') ? filters : list([{ ...record, status }])),
        ),
      ),
    );
    renderApplications();
    await screen.findByRole('button', { name: '详情' });
    expect(screen.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: moreActions }));
    expect(screen.getByRole('menuitem', { name: '删除' })).toBeVisible();
  },
);

it.each(['mine', 'student-reviews'] as const)('学生 %s 不提供删除填报入口', async (scope) => {
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string) =>
      Promise.resolve(respond(path.includes('/filter-options') ? filters : list())),
    ),
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <ApplicationList
          workspaceID={workspaceID}
          scope={scope}
          canDelete
          breadcrumb={null}
          panelID="student-applications"
          panelLabel="学生申报"
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  await screen.findByRole('button', { name: '详情' });
  expect(screen.queryByRole('button', { name: moreActions })).not.toBeInTheDocument();
});

it('删除须确认，取消不请求；提交携带版本与 CSRF，加载期间防重复并更新列表和详情缓存', async () => {
  const requests: { path: string; options: RequestInit }[] = [];
  let deleted = false;
  let completeDelete: ((value: Response) => void) | undefined;
  let completeRefresh: ((value: Response) => void) | undefined;
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string, options: RequestInit) => {
      requests.push({ path, options });
      if (options.method === 'DELETE')
        return new Promise<Response>((resolve) => {
          completeDelete = resolve;
        });
      if (path.includes('/filter-options')) return Promise.resolve(respond(filters));
      if (deleted)
        return new Promise<Response>((resolve) => {
          completeRefresh = resolve;
        });
      return Promise.resolve(respond(list()));
    }),
  );
  const { client, detailKey } = renderApplications();
  const user = userEvent.setup();
  const more = await screen.findByRole('button', { name: moreActions });
  await user.click(more);
  const remove = screen.getByRole('menuitem', { name: '删除' });
  await user.dblClick(remove);
  expect(screen.getByRole('menuitem', confirmation)).toBe(remove);
  expect(remove).toHaveAccessibleDescription('确认删除 优秀学生 · 2026 · 虚构学生(0007) ?');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(requests.filter(({ options }) => options.method === 'DELETE')).toHaveLength(0);
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('menuitem', confirmation)).not.toBeInTheDocument();
  await waitFor(() => expect(more).toHaveFocus());

  await user.keyboard('{Enter}');
  await user.keyboard('{Enter}');
  const confirm = screen.getByRole('menuitem', confirmation);
  expect(confirm).toHaveFocus();
  await user.dblClick(confirm);
  expect(confirm).toHaveAttribute('aria-busy', 'true');
  expect(confirm).toHaveAttribute('aria-disabled', 'true');
  await user.keyboard('{Escape}');
  expect(screen.getByRole('menuitem', confirmation)).toBeVisible();
  const deletes = requests.filter(({ options }) => options.method === 'DELETE');
  expect(deletes).toHaveLength(1);
  expect(deletes[0]?.path).toBe(`/v1/workspaces/${workspaceID}/project-applications/7?revision=3`);
  expect(new Headers(deletes[0]?.options.headers).get('X-CSRF-Protection')).toBe('1');

  deleted = true;
  await act(() => {
    completeDelete?.(respond(null));
    return Promise.resolve();
  });
  await waitFor(() => {
    expect(completeRefresh).toBeDefined();
  });
  expect(confirm).toHaveAttribute('aria-busy', 'true');
  expect(client.getQueryData(detailKey)).toBeUndefined();
  await act(() => {
    completeRefresh?.(respond(list([])));
    return Promise.resolve();
  });
  await screen.findByText('暂无符合条件的申报单');
  expect(screen.queryByRole('menuitem', confirmation)).not.toBeInTheDocument();
  expect(requests.filter(({ options }) => options.method === 'DELETE')).toHaveLength(1);
});

it('版本冲突保留记录和确认入口，通过顶部消息提示并允许关闭', async () => {
  const message = '记录已更新，请刷新后重试';
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string, options: RequestInit) =>
      Promise.resolve(
        options.method === 'DELETE'
          ? Response.json({ trace, error_msg: message, data: null }, { status: 409 })
          : respond(path.includes('/filter-options') ? filters : list()),
      ),
    ),
  );
  renderApplications();
  const user = userEvent.setup();
  await selectDelete(user);
  const confirm = screen.getByRole('menuitem', confirmation);
  await user.click(confirm);
  expect(await screen.findByRole('alert')).toHaveTextContent(message);
  expect(screen.getAllByText(message)).toHaveLength(1);
  expect(confirm).not.toHaveAttribute('aria-disabled', 'true');
  await waitFor(() => {
    expect(confirm).not.toHaveAttribute('aria-busy', 'true');
  });
  await user.keyboard('{Escape}');
  expect(screen.queryByRole('menuitem', confirmation)).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: moreActions })).toBeVisible();
});

it('删除末页唯一记录后回到上一页并保留已提交筛选', async () => {
  let deleted = false;
  const requests: URL[] = [];
  const firstPage = Array.from({ length: 5 }, (_, index) => ({ ...record, id: index + 1 }));
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string, options: RequestInit) => {
      const url = new URL(path, 'https://example.com');
      requests.push(url);
      if (options.method === 'DELETE') {
        deleted = true;
        return Promise.resolve(respond(null));
      }
      if (path.includes('/filter-options')) return Promise.resolve(respond(filters));
      const page = Number(url.searchParams.get('page') ?? 1);
      return Promise.resolve(
        respond(list(page === 1 ? firstPage : deleted ? [] : [record], deleted ? 5 : 6, page)),
      );
    }),
  );
  renderApplications();
  const user = userEvent.setup();
  await screen.findByText('共 6 条记录');
  await user.type(screen.getByPlaceholderText('学号或姓名'), '虚构学生');
  await user.click(screen.getByRole('button', { name: '搜索' }));
  await user.click(screen.getByRole('button', { name: '下一页' }));
  await screen.findByText('第 2 / 2 页');
  await selectDelete(user);
  await user.click(screen.getByRole('menuitem', confirmation));
  await screen.findByText('第 1 / 1 页');
  await waitFor(() => {
    expect(screen.queryByRole('menuitem', confirmation)).not.toBeInTheDocument();
  });
  expect(screen.getAllByRole('button', { name: moreActions })).toHaveLength(5);
  expect(requests.at(-1)?.searchParams.get('page')).toBe('1');
  expect(requests.at(-1)?.searchParams.get('student')).toBe('虚构学生');
  expect(requests.at(-1)?.searchParams.getAll('status')).toEqual(['pending_mine']);
});
