import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { PermissionsPage } from '@/pages/permissions-page';

const workspaceID = 'w7m2k9p4s6n8c3x5';
const otherWorkspaceID = 'r4v8b2m6k9d3p5s7';
const account = (id: number) => ({
  id,
  username: `fixture.${String(id)}`,
  display_name: `虚构用户${String(id)}`,
  is_enabled: true,
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
});
const catalog = [
  {
    value: 'access_workspace',
    label: '进入工作空间',
    description: '显式授予当前工作空间的进入权限',
    group: { value: 'workspace', label: '工作空间' },
    scope: 'workspace',
    endpoints: [],
  },
];

function respond(data: unknown, status = 200, errorMsg = '') {
  return new Response(
    JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: errorMsg, data }),
    { status },
  );
}

function setup(assignedCount = 1, candidateCount = 1, rejectSave = false) {
  const accounts = Array.from({ length: assignedCount + candidateCount }, (_, i) => account(i + 1));
  const grants = new Map(
    accounts.slice(0, assignedCount).map((user) => [user.id, ['access_workspace']]),
  );
  const calls: URL[] = [];
  const submitted: { userID: number; actions: string[]; revision: number }[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((path: string, options: RequestInit) => {
      const url = new URL(path, 'https://example.test');
      calls.push(url);
      if (url.pathname === '/v1/user-info')
        return Promise.resolve(
          respond({
            ...account(100),
            email: '',
            is_admin: true,
            password_login_enabled: true,
            password_change_required: false,
          }),
        );
      if (url.pathname === '/v1/system/workspaces/enum')
        return Promise.resolve(
          respond([
            { value: workspaceID, label: '虚构空间甲', is_enabled: true },
            { value: otherWorkspaceID, label: '虚构空间乙', is_enabled: true },
          ]),
        );
      if (url.pathname === '/v1/system/access/actions')
        return Promise.resolve(respond({ items: catalog }));
      if (url.pathname.endsWith('/users')) {
        const ungranted = url.searchParams.get('permission_status') === 'ungranted';
        const keyword = url.searchParams.get('keyword') ?? '';
        const page = Number(url.searchParams.get('page'));
        const pageSize = Number(url.searchParams.get('page_size'));
        const items = accounts
          .map((user) => ({
            ...user,
            permission_count: url.pathname.includes(workspaceID)
              ? (grants.get(user.id)?.length ?? 0)
              : 0,
          }))
          .filter(
            (user) =>
              (ungranted ? user.permission_count === 0 : user.permission_count > 0) &&
              `${user.username} ${user.display_name}`.includes(keyword),
          )
          .reverse();
        return Promise.resolve(
          respond({
            items: items.slice((page - 1) * pageSize, page * pageSize),
            total: items.length,
            page,
            page_size: pageSize,
          }),
        );
      }
      const match = /\/users\/(\d+)\/actions$/.exec(url.pathname);
      if (match) {
        const userID = Number(match[1]);
        if (options.method === 'PUT') {
          if (typeof options.body !== 'string') throw new Error('请求体必须为 JSON 字符串');
          const values = z
            .object({ actions: z.array(z.string()), revision: z.number() })
            .parse(JSON.parse(options.body) as unknown);
          submitted.push({ userID, ...values });
          if (rejectSave)
            return Promise.resolve(respond(null, 409, '工作空间或权限已被修改，请刷新后重试'));
          grants.set(userID, values.actions);
          return Promise.resolve(respond({ actions: values.actions, revision: 8 }));
        }
        return Promise.resolve(respond({ actions: grants.get(userID) ?? [], revision: 7 }));
      }
      throw new Error(`未预期的接口：${path}`);
    }),
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: '/system/permissions', Component: PermissionsPage }], {
    initialEntries: [`/system/permissions?workspace=${workspaceID}`],
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { calls, submitted };
}

beforeEach(() => {
  window.localStorage.clear();
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

describe('工作空间权限列表与独立授权入口', () => {
  it('只查询已授权用户，新增保存后进入列表，撤销全部权限后移出列表', async () => {
    const { calls, submitted } = setup();
    await screen.findByText('虚构用户1(fixture.1)');
    expect(screen.queryByText('虚构用户2(fixture.2)')).not.toBeInTheDocument();
    expect(
      calls.find((url) => url.pathname.endsWith('/users'))?.searchParams.get('permission_status'),
    ).toBe('granted');

    await userEvent.click(screen.getByRole('button', { name: '添加授权' }));
    let dialog = await screen.findByRole('dialog', { name: '添加授权' });
    expect(await within(dialog).findByText('虚构用户2(fixture.2)')).toBeInTheDocument();
    expect(within(dialog).queryByText('虚构用户1(fixture.1)')).not.toBeInTheDocument();
    expect(submitted).toHaveLength(0);
    await userEvent.click(within(dialog).getByRole('button', { name: '选择用户' }));
    dialog = await screen.findByRole('dialog', { name: '添加操作权限' });
    expect(within(dialog).getByRole('button', { name: '保存权限' })).toBeDisabled();
    await userEvent.click(within(dialog).getByRole('checkbox', { name: '进入工作空间' }));
    await userEvent.click(within(dialog).getByRole('button', { name: '保存权限' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(screen.getByText('虚构用户2(fixture.2)')).toBeInTheDocument();
    expect(submitted).toEqual([{ userID: 2, actions: ['access_workspace'], revision: 7 }]);

    const row = screen.getByText('虚构用户2(fixture.2)').closest('tr');
    if (!row) throw new Error('未找到新增用户行');
    await userEvent.click(within(row).getByRole('button', { name: '配置权限' }));
    dialog = await screen.findByRole('dialog', { name: '配置操作权限' });
    await userEvent.click(within(dialog).getByRole('button', { name: '取消全选' }));
    await userEvent.click(within(dialog).getByRole('button', { name: '保存权限' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByText('虚构用户2(fixture.2)')).not.toBeInTheDocument();
    });
    expect(submitted[1]).toEqual({ userID: 2, actions: [], revision: 7 });
    await userEvent.click(screen.getByRole('button', { name: '添加授权' }));
    dialog = await screen.findByRole('dialog', { name: '添加授权' });
    expect(await within(dialog).findByText('虚构用户2(fixture.2)')).toBeInTheDocument();
  });

  it('候选可分页及搜索，取消选择或取消配置均不创建授权', async () => {
    const { calls, submitted } = setup(1, 6);
    await userEvent.click(await screen.findByRole('button', { name: '添加授权' }));
    let dialog = await screen.findByRole('dialog', { name: '添加授权' });
    await within(dialog).findByText('虚构用户7(fixture.7)');
    await userEvent.click(within(dialog).getByRole('button', { name: '下一页' }));
    await within(dialog).findByText('虚构用户2(fixture.2)');
    await userEvent.type(within(dialog).getByRole('textbox', { name: '用户信息' }), 'fixture.6');
    await userEvent.click(within(dialog).getByRole('button', { name: '搜索' }));
    await within(dialog).findByText('虚构用户6(fixture.6)');
    expect(within(dialog).queryByText('虚构用户2(fixture.2)')).not.toBeInTheDocument();
    const search = calls.find((url) => url.searchParams.get('keyword') === 'fixture.6');
    expect(search?.searchParams.get('page')).toBe('1');
    expect(search?.searchParams.get('permission_status')).toBe('ungranted');
    await userEvent.click(within(dialog).getByRole('button', { name: '取消' }));

    await userEvent.click(screen.getByRole('button', { name: '添加授权' }));
    dialog = await screen.findByRole('dialog', { name: '添加授权' });
    await within(dialog).findByText('虚构用户7(fixture.7)');
    const candidate = within(dialog).getByText('虚构用户7(fixture.7)').closest('tr');
    if (!candidate) throw new Error('未找到未授权用户行');
    await userEvent.click(within(candidate).getByRole('button', { name: '选择用户' }));
    dialog = await screen.findByRole('dialog', { name: '添加操作权限' });
    await userEvent.click(within(dialog).getByRole('checkbox', { name: '进入工作空间' }));
    await userEvent.click(within(dialog).getByRole('button', { name: '取消' }));
    expect(submitted).toHaveLength(0);
    expect(screen.queryByText('虚构用户7(fixture.7)')).not.toBeInTheDocument();
  });

  it('新增保存冲突时保留草稿且不加入权限列表', async () => {
    const { submitted } = setup(1, 1, true);
    await userEvent.click(await screen.findByRole('button', { name: '添加授权' }));
    let dialog = await screen.findByRole('dialog', { name: '添加授权' });
    await userEvent.click(await within(dialog).findByRole('button', { name: '选择用户' }));
    dialog = await screen.findByRole('dialog', { name: '添加操作权限' });
    const permission = within(dialog).getByRole('checkbox', { name: '进入工作空间' });
    await userEvent.click(permission);
    await userEvent.click(within(dialog).getByRole('button', { name: '保存权限' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('请刷新后重试');
    expect(permission).toBeChecked();
    expect(submitted).toHaveLength(1);
    await userEvent.click(within(dialog).getByRole('button', { name: '取消' }));
    expect(screen.queryByText('虚构用户2(fixture.2)')).not.toBeInTheDocument();
  });

  it('切换空间后分别查询已授权列表与候选，不复用上个空间的账号归类', async () => {
    const { calls } = setup();
    await screen.findByText('虚构用户1(fixture.1)');
    await userEvent.click(screen.getByRole('combobox', { name: '工作空间' }));
    await userEvent.click(await screen.findByRole('option', { name: '虚构空间乙' }));
    await screen.findByText('没有符合条件的已授权用户');
    expect(screen.queryByText('虚构用户1(fixture.1)')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: '添加授权' }));
    const dialog = await screen.findByRole('dialog', { name: '添加授权' });
    expect(await within(dialog).findByText('虚构用户1(fixture.1)')).toBeInTheDocument();
    expect(within(dialog).getByText('虚构用户2(fixture.2)')).toBeInTheDocument();
    expect(
      calls.some(
        (url) =>
          url.pathname.includes(otherWorkspaceID) &&
          url.searchParams.get('permission_status') === 'ungranted',
      ),
    ).toBe(true);
  });

  it('撤销最后一页的全部权限后回退有效页码', async () => {
    setup(6, 0);
    await screen.findByText('虚构用户6(fixture.6)');
    await userEvent.click(screen.getByRole('button', { name: '下一页' }));
    await screen.findByText('虚构用户1(fixture.1)');
    await userEvent.click(screen.getByRole('button', { name: '配置权限' }));
    const dialog = await screen.findByRole('dialog', { name: '配置操作权限' });
    await userEvent.click(within(dialog).getByRole('button', { name: '取消全选' }));
    await userEvent.click(within(dialog).getByRole('button', { name: '保存权限' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
    expect(await screen.findByText('虚构用户6(fixture.6)')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled();
    expect(screen.getByText('共 5 条记录')).toBeInTheDocument();
  });
});
