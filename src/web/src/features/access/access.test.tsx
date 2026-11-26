import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { ActionEditor } from '@/features/access/action-editor';
import { WorkspaceSwitcher } from '@/features/workspace/workspace-switcher';

const firstID = 'w7m2k9p4s6n8c3x5';
const secondID = 'r4v8b2m6k9d3p5s7';
const first = {
  id: firstID,
  name: '虚构空间甲',
  description: '',
  is_enabled: true,
  revision: 7,
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};
const second = { ...first, id: secondID, name: '虚构空间乙' };
const account = {
  id: 2,
  username: 'fixture.user',
  display_name: '虚构用户',
  is_enabled: true,
  permission_count: 1,
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};
const directory = [
  {
    value: 'monthly_work.list',
    label: '查看学生月度工作',
    group: { value: 'monthly_work', label: '学生工作 · 月度工作' },
  },
  {
    value: 'cpc.monthly_work.list',
    label: '查看党建月度工作',
    group: { value: 'cpc_monthly_work', label: '党建工作 · 月度工作' },
  },
  {
    value: 'graduation.list',
    label: '查看毕业去向',
    group: { value: 'graduation', label: '毕业去向' },
  },
  {
    value: 'project.list',
    label: '查看评优项目',
    group: { value: 'project', label: '评奖评优' },
  },
  {
    value: 'project.participation.list',
    label: '查看项目参与记录',
    group: { value: 'project', label: '评奖评优' },
  },
  {
    value: 'access_workspace',
    label: '访问工作空间',
    group: { value: 'workspace', label: '工作空间' },
  },
  {
    value: 'student.list',
    label: '查看学生明细表',
    group: { value: 'student', label: '学生信息' },
  },
  {
    value: 'student.create',
    label: '添加学生',
    group: { value: 'student', label: '学生信息' },
  },
  {
    value: 'user.create',
    label: '添加系统账号',
    group: { value: 'user', label: '用户管理' },
  },
].map((action) => ({
  ...action,
  scope: action.value === 'user.create' ? 'system' : 'workspace',
  description: '当前工作空间内的独立操作',
  endpoints: [],
}));

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe = vi.fn();
      unobserve = vi.fn();
      disconnect = vi.fn();
    },
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(data: unknown, status = 200, errorMsg = '') {
  return new Response(
    JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: errorMsg, data }),
    { status },
  );
}

function requestBody(options: RequestInit): unknown {
  if (typeof options.body !== 'string') throw new Error('请求体必须为 JSON 字符串');
  return JSON.parse(options.body) as unknown;
}

describe('直接用户操作工作空间授权', () => {
  it('逐项勾选操作，不推导其他权限或提供系统管理权限', async () => {
    const save = vi.fn();
    const submitted: { actions: string[]; revision: number }[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string, options: RequestInit) => {
        if (url.endsWith('/actions') && options.method === 'PUT') {
          const values = z
            .object({ actions: z.array(z.string()), revision: z.number() })
            .parse(requestBody(options));
          submitted.push(values);
          return Promise.resolve(respond({ ...values, revision: 8 }));
        }
        if (url === '/v1/system/access/actions')
          return Promise.resolve(respond({ items: directory }));
        return Promise.resolve(respond({ actions: ['student.create'], revision: 7 }));
      }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <ActionEditor
          workspaceID={firstID}
          workspaceName={first.name}
          user={account}
          onClose={vi.fn()}
          onSaved={save}
        />
      </QueryClientProvider>,
    );
    await userEvent.click(await screen.findByRole('button', { name: '学生信息' }));
    const reader = screen.getByRole('checkbox', { name: '查看学生明细表' });
    expect(reader).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: '添加学生' })).toBeChecked();
    expect(screen.queryByRole('checkbox', { name: /添加系统账号/ })).not.toBeInTheDocument();
    await userEvent.click(reader);
    await userEvent.click(screen.getByRole('button', { name: '保存权限' }));
    await waitFor(() => {
      expect(save).toHaveBeenCalledOnce();
    });
    expect(submitted).toEqual([{ actions: ['student.create', 'student.list'], revision: 7 }]);
  });

  it('按导航层级分组，折叠后保留选择且全选互不影响，保存冲突时不报告成功', async () => {
    const save = vi.fn();
    const submitted: unknown[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string, options: RequestInit) => {
        if (options.method === 'PUT') {
          submitted.push(requestBody(options));
          return Promise.resolve(respond(null, 409, '工作空间或权限已被修改，请刷新后重试'));
        }
        return Promise.resolve(
          respond(
            url === '/v1/system/access/actions'
              ? { items: directory }
              : { actions: ['student.create'], revision: 7 },
          ),
        );
      }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <ActionEditor
          workspaceID={firstID}
          workspaceName={first.name}
          user={account}
          onClose={vi.fn()}
          onSaved={save}
        />
      </QueryClientProvider>,
    );
    const studentToggle = await screen.findByRole('button', { name: '学生信息' });
    expect(studentToggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('checkbox', { name: '查看学生明细表' })).not.toBeInTheDocument();
    const studentSection = screen.getByRole('group', { name: '学生工作' });
    expect(
      within(studentSection)
        .getAllByRole('heading', { level: 3 })
        .map((item) => item.textContent),
    ).toEqual(['学生信息1/2', '评奖评优0/2', '毕业去向0/1', '月度工作0/1']);

    await userEvent.click(studentToggle);
    const creator = screen.getByRole('checkbox', { name: '添加学生' });
    const reader = screen.getByRole('checkbox', { name: '查看学生明细表' });
    const workspaceAccess = screen.getByRole('checkbox', { name: /访问工作空间/ });
    const selectGroup = screen.getByRole('button', { name: '全选学生工作 · 学生信息权限' });
    const selectAll = screen.getByRole('button', { name: '全选' });
    const clearAll = screen.getByRole('button', { name: '取消全选' });

    await userEvent.click(selectGroup);
    expect(creator).toBeChecked();
    expect(reader).toBeChecked();
    expect(workspaceAccess).not.toBeChecked();
    expect(selectGroup).toBeEnabled();
    expect(selectGroup).toHaveAccessibleName('取消全选学生工作 · 学生信息权限');

    await userEvent.click(workspaceAccess);
    await userEvent.click(selectGroup);
    expect(creator).not.toBeChecked();
    expect(reader).not.toBeChecked();
    expect(workspaceAccess).toBeChecked();
    expect(selectGroup).toHaveAccessibleName('全选学生工作 · 学生信息权限');
    await userEvent.click(selectGroup);
    await userEvent.click(workspaceAccess);

    await userEvent.click(studentToggle);
    await userEvent.click(screen.getByRole('button', { name: '全选学生工作权限' }));
    const clearStudent = screen.getByRole('button', { name: '取消全选学生工作权限' });
    expect(clearStudent).toBeEnabled();
    await userEvent.click(clearStudent);
    expect(screen.getByRole('button', { name: '全选学生工作权限' })).toBeEnabled();
    await userEvent.click(screen.getByRole('button', { name: '全选学生工作权限' }));
    expect(screen.getByRole('group', { name: '学生工作' })).toBe(studentSection);
    await userEvent.click(screen.getByRole('button', { name: '评奖评优' }));
    const projectSection = screen.getByRole('group', { name: '评奖评优' });
    expect(
      within(projectSection)
        .getAllByRole('heading', { level: 4 })
        .map((item) => item.textContent),
    ).toEqual(['奖项管理1/1', '获奖学生1/1']);
    expect(screen.getByRole('checkbox', { name: '查看项目参与记录' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: '查看评优项目' })).toBeChecked();
    const cpcSection = screen.getByRole('group', { name: '党建工作' });
    await userEvent.click(within(cpcSection).getByRole('button', { name: '月度工作' }));
    const cpcReader = screen.getByRole('checkbox', { name: '查看党建月度工作' });
    expect(cpcReader).not.toBeChecked();
    await userEvent.click(studentToggle);
    expect(screen.getByRole('checkbox', { name: '添加学生' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: '查看学生明细表' })).toBeChecked();

    await userEvent.click(selectAll);
    expect(workspaceAccess).toBeChecked();
    expect(cpcReader).toBeChecked();
    expect(selectAll).toBeDisabled();
    expect(screen.queryByRole('checkbox', { name: /添加系统账号/ })).not.toBeInTheDocument();
    expect(submitted).toHaveLength(0);

    await userEvent.click(clearAll);
    for (const checkbox of screen.getAllByRole('checkbox')) expect(checkbox).not.toBeChecked();
    expect(clearAll).toBeDisabled();
    await userEvent.click(screen.getByRole('button', { name: '保存权限' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('请刷新后重试');
    expect(submitted).toEqual([{ actions: [], revision: 7 }]);
    expect(save).not.toHaveBeenCalled();
  });

  it.each([
    { entry: '/stu/roster', actions: ['student.list'], destination: '/stu' },
    { entry: '/stu/roster', actions: ['cpc.monthly_work.list'], destination: '/cpc' },
    { entry: '/cpc/monthly-work', actions: ['cpc.monthly_work.list'], destination: '/cpc' },
    { entry: '/cpc/monthly-work', actions: ['student.list'], destination: '/stu' },
    { entry: '/cpc/monthly-work', actions: [], destination: '/cpc' },
  ])(
    '从 $entry 切换空间后按列表权限进入 $destination 并清理旧缓存',
    async ({ entry, actions, destination }) => {
      const calls: string[] = [];
      const items = [
        first,
        ...Array.from({ length: 4 }, (_, index) => ({
          ...first,
          id: `w${String(index).padStart(15, '0')}`,
          name: `虚构空间 ${String(index + 1)}`,
        })),
        second,
      ].map(({ id, name, description }) => ({ id, name, description }));
      vi.stubGlobal(
        'fetch',
        vi.fn((url: string) => {
          calls.push(url);
          if (url === '/v1/workspaces') return Promise.resolve(respond({ items }));
          return Promise.resolve(
            respond({
              workspace: url.endsWith(secondID) ? second : first,
              actions: ['access_workspace', ...actions],
            }),
          );
        }),
      );
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      client.setQueryData(['workspace', firstID, 'records'], { private: '虚构旧空间数据' });
      const router = createMemoryRouter(
        ['/stu', '/stu/roster', '/cpc', '/cpc/monthly-work'].map((path) => ({
          path,
          Component: WorkspaceSwitcher,
        })),
        { initialEntries: [`${entry}?workspace=${firstID}`] },
      );
      render(
        <QueryClientProvider client={client}>
          <RouterProvider router={router} />
        </QueryClientProvider>,
      );
      expect(calls).toHaveLength(0);
      await userEvent.click(screen.getByRole('button', { name: '选择工作空间' }));
      const target = await screen.findByRole('menuitemradio', { name: second.name });
      expect(screen.getAllByRole('menuitemradio')).toHaveLength(items.length);
      expect(screen.queryByText('加载更多')).not.toBeInTheDocument();
      await userEvent.click(target);
      await waitFor(() => {
        expect(router.state.location.pathname).toBe(destination);
        expect(router.state.location.search).toBe(`?workspace=${secondID}`);
      });
      expect(client.getQueryData(['workspace', firstID, 'records'])).toBeUndefined();
      expect(calls).toEqual(['/v1/workspaces', `/v1/workspaces/${secondID}`]);
    },
  );
});
