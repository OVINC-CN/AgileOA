import { QueryClient, QueryClientProvider, QueryObserver } from '@tanstack/react-query';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { personKey } from '@/features/cpc-person/api';
import { personTimelineKey } from '@/features/cpc-person/timeline-api';
import { workKey } from '@/features/monthly-work/api';
import { notificationKey, notificationUnreadKey } from '@/features/notification/api';
import { isNotificationURL } from '@/features/notification/navigation';
import { NotificationCenter } from '@/features/notification/notification-center';
import { projectKey } from '@/features/project/api';
import { getConfig } from '@/lib/config';

interface Notice {
  id: number;
  title: string;
  content: string;
  url?: string;
  read_at: string | null;
  created_at: string;
}

const readRequest = z.object({ ids: z.array(z.number().int().positive()) });
const trace = '4bf92f3577b34da6a3ce929d0e0e4736';
const workspaceID = 'w7m2k9p4s6n8c3x5';
const otherWorkspaceID = 'r4v8b2m6k9d3p5s7';
const notice = (id: number): Notice => ({
  id,
  title: `工作分配通知 ${String(id)}`,
  content: `你已被分配为虚构工作 ${String(id)} 的参与人。`,
  read_at: null,
  created_at: '2026-11-08T08:00:00Z',
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function CurrentLocation() {
  const location = useLocation();
  return <output aria-label="当前地址">{location.pathname + location.search}</output>;
}

function setup(initial: Notice[] = [notice(1)], entry = '/', studentWorkspaceID?: string) {
  let items = initial;
  const base = studentWorkspaceID
    ? `/v1/workspaces/${studentWorkspaceID}/student-service/notifications`
    : '/v1/notifications';
  const fetchMock = vi.fn((url: string, options: RequestInit) => {
    const address = new URL(url, 'https://fixture.example');
    const unread = () => items.filter((item) => item.read_at === null).length;
    let data: unknown;
    if (address.pathname === `${base}/unread`) {
      data = { unread_count: unread() };
    } else if (address.pathname === `${base}/read`) {
      if (typeof options.body !== 'string') throw new Error('缺少标记已读参数');
      const { ids } = readRequest.parse(JSON.parse(options.body));
      let changed = 0;
      items = items.map((item) => {
        if (!ids.includes(item.id) || item.read_at !== null) return item;
        changed += 1;
        return { ...item, read_at: '2026-11-08T09:00:00Z' };
      });
      data = { changed, unread_count: unread() };
    } else if (address.pathname === base) {
      const page = Number(address.searchParams.get('page'));
      const size = Number(address.searchParams.get('page_size'));
      const scope =
        address.searchParams.get('status') === 'all'
          ? items
          : items.filter((item) => item.read_at === null);
      const ordered = scope.toSorted(
        (left, right) =>
          Number(left.read_at !== null) - Number(right.read_at !== null) ||
          Date.parse(right.created_at) - Date.parse(left.created_at) ||
          right.id - left.id,
      );
      data = {
        items: ordered.slice((page - 1) * size, page * size),
        total: scope.length,
        page,
        page_size: size,
        unread_count: unread(),
      };
    } else if (address.pathname === '/v1/user-info' && studentWorkspaceID) {
      data = {
        id: 1,
        identity_type: 'student',
        workspace_id: studentWorkspaceID,
        workspace_name: '虚构空间',
        username: '0001',
        display_name: '虚构学生',
        email: 'student@example.com',
        is_admin: false,
        password_login_enabled: false,
        password_change_required: false,
      };
    } else if (/^\/v1\/workspaces\/[a-z0-9]{16}$/u.test(address.pathname)) {
      data = {
        workspace: {
          id: address.pathname.slice('/v1/workspaces/'.length),
          name: '虚构工作空间',
          description: '',
          is_enabled: true,
          revision: 2,
          created_at: '2026-11-08T08:00:00Z',
          updated_at: '2026-11-08T09:00:00Z',
        },
        actions: [
          'access_workspace',
          'cpc.person.list',
          'monthly_work.list',
          'cpc.monthly_work.list',
        ],
      };
    } else {
      throw new Error(`意外请求：${url}`);
    }
    return Promise.resolve(Response.json({ trace, error_msg: '', data }));
  });
  vi.stubGlobal('fetch', fetchMock);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 60000, gcTime: 0 } },
  });
  const content = (userID: number) => (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[entry]}>
        <NotificationCenter key={userID} userID={userID} studentWorkspaceID={studentWorkspaceID} />
        <CurrentLocation />
      </MemoryRouter>
    </QueryClientProvider>
  );
  const view = render(content(1));
  return {
    client,
    fetchMock,
    user: userEvent.setup(),
    add: (item: Notice) => {
      items = [item, ...items];
    },
    switchUser: (userID: number, next: Notice[]) => {
      items = next;
      view.rerender(content(userID));
    },
  };
}

describe('个人通知', () => {
  it('学生复用通知抽屉，已读与详情跳转只使用绑定空间的学生接口', async () => {
    const url = `/student-service/projects?workspace=${workspaceID}&tab=reviews&application_id=7`;
    const { user, fetchMock, client } = setup([{ ...notice(1), url }], '/', workspaceID);
    const recipient = { workspaceID, studentID: 1 };
    await user.click(await screen.findByRole('button', { name: '通知（1 条未读）' }));
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    expect(within(drawer).getByText(notice(1).content)).toBeVisible();
    await user.click(within(drawer).getByRole('link', { name: `查看详情：${notice(1).title}` }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: '通知' })).not.toBeInTheDocument();
    });
    expect(screen.getByLabelText('当前地址')).toHaveTextContent(url);
    expect(client.getQueryData(notificationUnreadKey(recipient))).toEqual({ unread_count: 0 });
    expect(client.getQueryData(notificationUnreadKey(1))).toBeUndefined();
    const paths = fetchMock.mock.calls.map(
      ([path]) => new URL(path, 'https://fixture.example').pathname,
    );
    expect(paths).toContain(`/v1/workspaces/${workspaceID}/student-service/notifications/read`);
    expect(paths).toContain('/v1/user-info');
    expect(paths).not.toContain('/v1/notifications/unread');
    expect(paths).not.toContain(`/v1/workspaces/${workspaceID}`);
    expect(notificationKey(recipient)).not.toEqual(
      notificationKey({ workspaceID: otherWorkspaceID, studentID: 1 }),
    );
  });

  it.each([
    {
      url: `/stu/monthly-work?workspace=${workspaceID}&work_id=7&year=2026&month=11`,
      scope: workKey(workspaceID, 'student'),
      unrelated: workKey(workspaceID, 'cpc'),
    },
    {
      url: `/cpc/monthly-work?workspace=${workspaceID}&work_id=7&year=2026&month=11`,
      scope: workKey(workspaceID, 'cpc'),
      unrelated: workKey(workspaceID, 'student'),
    },
    {
      url: `/cpc/roster?workspace=${workspaceID}`,
      scope: personKey(workspaceID),
      unrelated: workKey(workspaceID, 'student'),
    },
    {
      url: `/cpc/roster?workspace=${workspaceID}&person_id=7&timeline_mode=review`,
      scope: personKey(workspaceID),
      unrelated: workKey(workspaceID, 'student'),
    },
    {
      url: `/stu/projects?workspace=${workspaceID}&tab=applications&application_id=7`,
      scope: projectKey(workspaceID),
      unrelated: workKey(workspaceID, 'student'),
    },
  ])(
    '同页通知强制刷新当前业务，保留其他空间及类别缓存：$url',
    async ({ url, scope, unrelated }) => {
      const { user, client } = setup([{ ...notice(1), url }], url);
      const currentKey = [...scope, 'list'];
      let revision = 0;
      const fetchCurrent = vi.fn(() => Promise.resolve(++revision));
      const options = {
        queryKey: currentKey,
        queryFn: fetchCurrent,
        staleTime: 60000,
        gcTime: Infinity,
      };
      await client.query(options);
      const observer = new QueryObserver(client, options);
      const unsubscribe = observer.subscribe(vi.fn());
      const foreignKey = [...personKey(otherWorkspaceID), 'list'];
      const unrelatedKey = [...unrelated, 'list'];
      for (const key of [foreignKey, unrelatedKey]) {
        client.setQueryDefaults(key, { gcTime: Infinity });
        client.setQueryData(key, '保留的缓存');
      }
      try {
        expect(client.getQueryData(currentKey)).toBe(1);
        await user.click(await screen.findByRole('button', { name: '通知（1 条未读）' }));
        const drawer = await screen.findByRole('dialog', { name: '通知' });
        await user.click(
          within(drawer).getByRole('link', { name: `查看详情：${notice(1).title}` }),
        );
        await waitFor(() => {
          expect(screen.queryByRole('dialog', { name: '通知' })).not.toBeInTheDocument();
        });
        expect(screen.getByLabelText('当前地址')).toHaveTextContent(url);
        expect(fetchCurrent).toHaveBeenCalledTimes(2);
        expect(client.getQueryData(currentKey)).toBe(2);
        expect(client.getQueryData(foreignKey)).toBe('保留的缓存');
        expect(client.getQueryData(unrelatedKey)).toBe('保留的缓存');
      } finally {
        unsubscribe();
      }
    },
  );

  it('同一人员详情通知清理未打开的时间线缓存，已读消息也重新检查目标空间', async () => {
    const url = `/cpc/roster?workspace=${workspaceID}&person_id=7&timeline_mode=review`;
    const { user, fetchMock, client } = setup(
      [{ ...notice(1), url, read_at: '2026-11-08T09:00:00Z' }],
      url,
    );
    const timelineKey = [...personTimelineKey(workspaceID, 7), 'review'];
    client.setQueryDefaults(timelineKey, { gcTime: Infinity, staleTime: Infinity });
    client.setQueryData(timelineKey, { revision: 1, status: 'transferring' });
    await user.click(await screen.findByRole('button', { name: '通知' }));
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    await user.click(within(drawer).getByRole('button', { name: '全部通知' }));
    await user.click(
      await within(drawer).findByRole('link', { name: `查看详情：${notice(1).title}` }),
    );
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: '通知' })).not.toBeInTheDocument();
    });
    expect(client.getQueryData(timelineKey)).toBeUndefined();
    expect(fetchMock.mock.calls.some(([path]) => path === `/v1/workspaces/${workspaceID}`)).toBe(
      true,
    );
    expect(fetchMock.mock.calls.some(([, options]) => options.method === 'POST')).toBe(false);
  });

  it('当前业务刷新失败时保留通知抽屉，避免带着旧数据打开目标页面', async () => {
    const url = `/cpc/roster?workspace=${workspaceID}&person_id=7`;
    const { user, client } = setup([{ ...notice(1), url }]);
    const options = {
      queryKey: [...personKey(workspaceID), 'list'],
      queryFn: vi
        .fn<() => Promise<number>>()
        .mockResolvedValueOnce(1)
        .mockRejectedValue(new Error('目标数据暂不可用')),
      staleTime: 60000,
      gcTime: Infinity,
    };
    await client.query(options);
    const observer = new QueryObserver(client, options);
    const unsubscribe = observer.subscribe(vi.fn());
    try {
      await user.click(await screen.findByRole('button', { name: '通知（1 条未读）' }));
      const drawer = await screen.findByRole('dialog', { name: '通知' });
      await user.click(within(drawer).getByRole('link', { name: `查看详情：${notice(1).title}` }));
      await waitFor(() => {
        expect(options.queryFn).toHaveBeenCalledTimes(2);
        expect(within(drawer).getByRole('button', { name: '关闭抽屉' })).toBeEnabled();
      });
      expect(drawer).toBeVisible();
      expect(screen.getByLabelText('当前地址')).toHaveTextContent(/^\/$/u);
    } finally {
      unsubscribe();
    }
  });

  it.each([
    '/stu/monthly-work?workspace=w7m2k9p4s6n8c3x5&work_id=7&year=2025&month=12',
    '/cpc/monthly-work?workspace=w7m2k9p4s6n8c3x5&work_id=7&year=2026&month=11',
    '/cpc/roster?workspace=w7m2k9p4s6n8c3x5',
    '/cpc/roster?workspace=w7m2k9p4s6n8c3x5&person_id=7&timeline_mode=review',
    '/cpc/roster?workspace=w7m2k9p4s6n8c3x5&person_id=7',
  ])('点击通知链接先标记该条已读，再关闭抽屉并跳转：%s', async (url) => {
    const { user, fetchMock, client } = setup([{ ...notice(1), url }]);
    await user.click(await screen.findByRole('button', { name: '通知（1 条未读）' }));
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    const link = within(drawer).getByRole('link', { name: `查看详情：${notice(1).title}` });
    expect(link).toHaveAttribute('href', url);
    await user.click(link);
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: '通知' })).not.toBeInTheDocument();
      expect(screen.getByLabelText('当前地址')).toHaveTextContent(url);
    });
    const posts = fetchMock.mock.calls.filter(([, options]) => options.method === 'POST');
    expect(posts).toHaveLength(1);
    expect(posts[0]?.[1].body).toBe(JSON.stringify({ ids: [1] }));
    expect(client.getQueryData(notificationUnreadKey(1))).toEqual({ unread_count: 0 });
  });

  it('已读通知查看详情直接跳转，不重复提交已读请求', async () => {
    const url = '/cpc/roster?workspace=w7m2k9p4s6n8c3x5';
    const { user, fetchMock } = setup([{ ...notice(1), url, read_at: '2026-11-08T09:00:00Z' }]);
    await user.click(await screen.findByRole('button', { name: '通知' }));
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    await user.click(within(drawer).getByRole('button', { name: '全部通知' }));
    await user.click(
      await within(drawer).findByRole('link', { name: `查看详情：${notice(1).title}` }),
    );
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: '通知' })).not.toBeInTheDocument();
      expect(screen.getByLabelText('当前地址')).toHaveTextContent(url);
    });
    expect(fetchMock.mock.calls.some(([, options]) => options.method === 'POST')).toBe(false);
  });

  it('查看详情标记失败时保留通知抽屉，不跳转或清除未读角标', async () => {
    const { user, fetchMock, client } = setup([
      { ...notice(1), url: '/cpc/roster?workspace=w7m2k9p4s6n8c3x5' },
    ]);
    await user.click(await screen.findByRole('button', { name: '通知（1 条未读）' }));
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    fetchMock.mockImplementationOnce(() =>
      Promise.resolve(
        Response.json({ trace, error_msg: '标记已读失败', data: null }, { status: 503 }),
      ),
    );
    await user.click(within(drawer).getByRole('link', { name: `查看详情：${notice(1).title}` }));
    await waitFor(() => {
      expect(within(drawer).getByRole('button', { name: '关闭抽屉' })).toBeEnabled();
    });
    expect(drawer).toBeVisible();
    expect(screen.getByLabelText('当前地址')).toHaveTextContent(/^\/$/u);
    expect(client.getQueryData(notificationUnreadKey(1))).toEqual({ unread_count: 1 });
  });

  it.each(['', undefined])('没有链接的历史通知保持纯文本展示：%s', async (url) => {
    const item = notice(1);
    if (url !== undefined) item.url = url;
    const { user } = setup([item]);
    await user.click(await screen.findByRole('button', { name: '通知（1 条未读）' }));
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    expect(within(drawer).getByText(item.content)).toBeVisible();
    expect(within(drawer).queryByRole('link')).not.toBeInTheDocument();
  });

  it.each([
    'https://example.com/cpc/roster',
    '//example.com/cpc/roster',
    'javascript:alert(1)',
    '/\\example.com',
    '/%2fexample.com',
    '/%5cexample.com',
    '/cpc/%0aroster',
    '/cpc/%ZZ',
    ' /cpc/roster',
  ])('拒绝外站、协议和畸形通知链接：%s', (url) => {
    expect(isNotificationURL(url)).toBe(false);
  });

  it('仅显示图标与未读角标，抽屉展示正文，标记已读后更新列表与角标', async () => {
    const { user, fetchMock } = setup();
    const bell = await screen.findByRole('button', { name: '通知（1 条未读）' });
    expect(bell).toHaveTextContent('1');
    expect(bell).not.toHaveTextContent('通知');
    await user.click(bell);
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    expect(within(drawer).getByRole('button', { name: '未读通知' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(within(drawer).getByRole('heading', { name: notice(1).title })).toBeVisible();
    expect(within(drawer).getByText(notice(1).content)).toBeVisible();
    expect(fetchMock.mock.calls.some(([, options]) => options.method === 'POST')).toBe(false);
    await user.click(within(drawer).getByRole('button', { name: `标为已读：${notice(1).title}` }));
    expect(await within(drawer).findByText('暂无未读通知')).toBeVisible();
    await waitFor(() => {
      expect(within(drawer).getByRole('button', { name: '全部通知' })).toBeEnabled();
    });
    await user.click(within(drawer).getByRole('button', { name: '全部通知' }));
    expect(await within(drawer).findByRole('heading', { name: notice(1).title })).toBeVisible();
    expect(within(drawer).queryByText('已读', { exact: true })).not.toBeInTheDocument();
    await user.click(within(drawer).getByRole('button', { name: '关闭抽屉' }));
    expect(await screen.findByRole('button', { name: '通知' })).not.toHaveTextContent(/\d/);
    const request = fetchMock.mock.calls.find(([url]) => url === '/v1/notifications/read');
    expect(request?.[1].body).toBe(JSON.stringify({ ids: [1] }));
    expect(new Headers(request?.[1].headers).get('X-CSRF-Protection')).toBe('1');
  });

  it('本页标记只提交当前页的未读编号，翻页不会自动标记其他通知', async () => {
    const { user, fetchMock } = setup(Array.from({ length: 7 }, (_, index) => notice(index + 1)));
    await user.click(await screen.findByRole('button', { name: '通知（7 条未读）' }));
    const drawer = await screen.findByRole('dialog', { name: '通知' });
    await user.click(within(drawer).getByRole('button', { name: '本页标为已读' }));
    await waitFor(() => {
      expect(within(drawer).getByText('共 2 条未读通知')).toBeVisible();
      expect(within(drawer).getByRole('button', { name: '全部通知' })).toBeEnabled();
    });
    expect(within(drawer).getByRole('button', { name: '下一页' })).toBeDisabled();
    await user.click(within(drawer).getByRole('button', { name: '全部通知' }));
    await waitFor(() => {
      expect(within(drawer).getByRole('button', { name: '下一页' })).toBeEnabled();
    });
    await user.click(within(drawer).getByRole('button', { name: '下一页' }));
    expect(await within(drawer).findByRole('heading', { name: notice(4).title })).toBeVisible();
    const posts = fetchMock.mock.calls.filter(([, options]) => options.method === 'POST');
    expect(posts).toHaveLength(1);
    expect(posts[0]?.[1].body).toBe(JSON.stringify({ ids: [7, 6, 5, 4, 3] }));
  });

  it('按配置自动轮询未读数量，无需刷新页面即可出现新通知角标', async () => {
    vi.useFakeTimers();
    const { add, client } = setup([]);
    await vi.waitFor(() => {
      expect(client.getQueryData(notificationUnreadKey(1))).toEqual({ unread_count: 0 });
    });
    add(notice(2));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(getConfig().notificationPollIntervalMs);
    });
    await vi.waitFor(() => {
      expect(screen.getByRole('button', { name: '通知（1 条未读）' })).toBeVisible();
    });
  });

  it('抽屉打开时自动查询新通知并展示正文，保留已有消息', async () => {
    vi.useFakeTimers();
    const { add } = setup();
    await vi.waitFor(() => {
      expect(screen.getByRole('button', { name: '通知（1 条未读）' })).toBeVisible();
    });
    // 同步点击避免异步用户事件包装器等待虚拟时钟下未推进的零延时定时器。
    fireEvent.click(screen.getByRole('button', { name: '通知（1 条未读）' }));
    await vi.waitFor(() => {
      expect(screen.getByRole('dialog', { name: '通知' })).toBeVisible();
    });
    const drawer = screen.getByRole('dialog', { name: '通知' });
    add(notice(2));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(getConfig().notificationPollIntervalMs);
    });
    await vi.waitFor(() => {
      expect(within(drawer).getByText(notice(2).content)).toBeVisible();
      expect(within(drawer).getByText(notice(1).content)).toBeVisible();
      expect(within(drawer).getByText('共 2 条未读通知')).toBeVisible();
    });
  });

  it('账号切换时关闭旧抽屉，查询缓存按账号隔离', async () => {
    const { user, switchUser, client } = setup();
    await user.click(await screen.findByRole('button', { name: '通知（1 条未读）' }));
    await screen.findByRole('dialog', { name: '通知' });
    switchUser(2, []);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByText(notice(1).content)).not.toBeInTheDocument();
    await waitFor(() => {
      expect(client.getQueryData(notificationUnreadKey(2))).toEqual({ unread_count: 0 });
    });
    expect(notificationKey(1)).not.toEqual(notificationKey(2));
  });

  it('未读数量超过 99 时使用紧凑角标', async () => {
    setup(Array.from({ length: 100 }, (_, index) => notice(index + 1)));
    expect(await screen.findByRole('button', { name: '通知（100 条未读）' })).toHaveTextContent(
      '99+',
    );
  });
});
