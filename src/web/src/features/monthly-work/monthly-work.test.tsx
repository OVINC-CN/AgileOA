import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  isExternalURL,
  saveWork,
  uploadWorkAttachment,
  workAttachmentDownload,
  type WorkInput,
  type WorkRecord,
} from '@/features/monthly-work/api';
import { workAction } from '@/features/monthly-work/category';
import { MonthlyWorkPage } from '@/features/monthly-work/monthly-work-page';
import { readWorkTarget } from '@/features/monthly-work/navigation';
import { groupWorkRecords } from '@/features/monthly-work/timeline';
import { WorkEditor } from '@/features/monthly-work/work-editor';
import { currentUserKey, userSchema } from '@/features/user/api';
import { currentCalendarDate } from '@/lib/calendar';
import { preparedUpload } from '@/test/mock-cos';

type WorkRequest = WorkInput & { revision: number };

const first = 'w7m2k9p4s6n8c3x5';
const second = 'r4v8b2m6k9d3p5s7';
const record: WorkRecord = {
  id: 1,
  date: '2026-11-03',
  work_item: '虚构工作项目',
  owner_id: 2,
  owner_name: '虚构负责人',
  owner_username: 'fictional.owner',
  participant_ids: [3, 4],
  participants: [
    { id: 3, name: '虚构参与人甲', username: 'fictional.first' },
    { id: 4, name: '虚构参与人乙', username: 'fictional.second' },
  ],
  description: '第一行说明\n第二行说明',
  attachment_ids: [],
  attachments: [],
  links: [
    { name: '工作材料', url: 'https://example.com/work' },
    { name: '', url: 'https://example.com/notes' },
  ],
  revision: 1,
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};
const createClient = () =>
  new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
function listResponse(path: string, items: WorkRecord[] = [record]) {
  return path.endsWith('/filter-options')
    ? respond({
        years: [
          { value: '2025', label: '2025 年' },
          { value: '2026', label: '2026 年' },
        ],
      })
    : respond({ items, total: items.length });
}
function respond(data: unknown) {
  return new Response(
    JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data }),
  );
}
afterEach(() => {
  vi.unstubAllGlobals();
});

describe('月度工作', () => {
  it.each(['student', 'cpc'] as const)('%s 从通知按历史年月定位工作', async (category) => {
    const fetch = vi.fn((path: string) => Promise.resolve(listResponse(path)));
    vi.stubGlobal('fetch', fetch);
    const target = readWorkTarget(new URLSearchParams('work_id=1&year=2025&month=03'));
    render(
      <QueryClientProvider client={createClient()}>
        <MonthlyWorkPage
          workspaceID={first}
          category={category}
          workspaceName="虚构空间"
          actions={[workAction(category, 'list')]}
          refresh={() => Promise.resolve()}
          target={target}
        />
      </QueryClientProvider>,
    );
    await screen.findByRole('heading', { name: record.work_item });
    const resource = category === 'cpc' ? 'cpc-monthly-work' : 'monthly-work';
    const request = fetch.mock.calls.find(([path]) =>
      path.startsWith(`/v1/workspaces/${first}/${resource}?`),
    );
    const params = new URL(request?.[0] ?? '', 'https://example.com').searchParams;
    expect(params.getAll('year')).toEqual(['2025']);
    expect(params.getAll('month')).toEqual(['3']);
    expect(params.has('scope')).toBe(false);
  });

  it.each([
    'work_id=0&year=2025&month=03',
    'work_id=9007199254740992&year=2025&month=03',
    'work_id=1&year=1899&month=03',
    'work_id=1&year=2025&month=13',
    'work_id=1&year=2025',
  ])('忽略无效的通知工作定位参数：%s', (query) => {
    expect(readWorkTarget(new URLSearchParams(query))).toBeUndefined();
  });

  it('按工作日期倒序按月分组，同日按编号倒序，保持自然日不做时区转换', () => {
    const groups = groupWorkRecords([
      { ...record, id: 2, date: '2026-10-31' },
      { ...record, id: 4, date: '2026-11-02' },
      record,
      { ...record, id: 3 },
    ]);
    expect(groups.map((group) => group.month)).toEqual(['2026-11', '2026-10']);
    expect(groups[0]?.items.map((item) => item.id)).toEqual([3, 1, 4]);
    expect(groups[1]?.items[0]?.date).toBe('2026-10-31');
  });

  it('没有列表权限时不请求或展示工作数据', () => {
    const fetch = vi.fn();
    vi.stubGlobal('fetch', fetch);
    render(
      <QueryClientProvider client={createClient()}>
        <MonthlyWorkPage
          workspaceID={first}
          workspaceName="虚构空间"
          actions={['monthly_work.create']}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    expect(screen.queryByRole('region', { name: '月度工作时间轴' })).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each(['student', 'cpc'] as const)(
    '%s 展示负责人、参与人和链接，并按权限隐藏操作',
    async (category) => {
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string) => Promise.resolve(listResponse(path))),
      );
      render(
        <QueryClientProvider client={createClient()}>
          <MonthlyWorkPage
            workspaceID={first}
            category={category}
            workspaceName="虚构空间"
            actions={[workAction(category, 'list')]}
            refresh={() => Promise.resolve()}
          />
        </QueryClientProvider>,
      );
      expect(await screen.findByText('虚构工作项目')).toBeInTheDocument();
      const work = within(screen.getByRole('article', { name: record.work_item }));
      expect(work.getByText('负责人：')).toBeInTheDocument();
      expect(work.getByText('虚构负责人(fictional.owner)')).toBeInTheDocument();
      expect(screen.getByText('虚构参与人甲(fictional.first)')).toBeInTheDocument();
      expect(screen.getByText('虚构参与人乙(fictional.second)')).toBeInTheDocument();
      expect(screen.getByRole('link', { name: /工作材料/u })).toHaveAttribute(
        'href',
        record.links[0]?.url,
      );
      expect(screen.getByRole('link', { name: /https:\/\/example.com\/notes/u })).toHaveAttribute(
        'rel',
        'noopener noreferrer',
      );
      expect(screen.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: '管理附件' })).not.toBeInTheDocument();
    },
  );

  it('没有参与人时统一显示短横线', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) =>
        Promise.resolve(listResponse(path, [{ ...record, participant_ids: [], participants: [] }])),
      ),
    );
    render(
      <QueryClientProvider client={createClient()}>
        <MonthlyWorkPage
          workspaceID={first}
          workspaceName="虚构空间"
          actions={['monthly_work.list']}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    await screen.findByRole('article', { name: record.work_item });
    expect(screen.getByText('参与人：').nextElementSibling).toHaveTextContent('-');
  });

  it('详细说明为空时隐藏区域，不显示占位文本', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) =>
        Promise.resolve(listResponse(path, [{ ...record, description: '' }])),
      ),
    );
    render(
      <QueryClientProvider client={createClient()}>
        <MonthlyWorkPage
          workspaceID={first}
          workspaceName="虚构空间"
          actions={['monthly_work.list']}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    expect(await screen.findByText(record.work_item)).toBeInTheDocument();
    expect(screen.queryByText('暂无详细说明')).not.toBeInTheDocument();
    expect(screen.getByRole('region', { name: '月度工作时间轴' }).querySelector('p')).toBeNull();
  });

  it.each(['student', 'cpc'] as const)(
    '%s 年份和月份变化立即请求，重置恢复默认且没有分页',
    async (category) => {
      const paths: string[] = [];
      const items = Array.from({ length: 12 }, (_, index) => ({
        ...record,
        id: index + 1,
        work_item: `虚构工作 ${String(index + 1)}`,
      }));
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string) => {
          paths.push(path);
          return Promise.resolve(listResponse(path, items));
        }),
      );
      render(
        <QueryClientProvider client={createClient()}>
          <MonthlyWorkPage
            workspaceID={first}
            category={category}
            workspaceName="虚构空间"
            actions={[workAction(category, 'list')]}
            refresh={() => Promise.resolve()}
          />
        </QueryClientProvider>,
      );
      const timeline = await screen.findByRole('region', { name: '月度工作时间轴' });
      await waitFor(() => {
        expect(within(timeline).getAllByRole('heading', { level: 3 })).toHaveLength(12);
      });
      expect(timeline.querySelectorAll('span[aria-hidden="true"]')).toHaveLength(1);
      const resource = category === 'cpc' ? 'cpc-monthly-work' : 'monthly-work';
      const initial = paths.find((path) => path.includes(`/${resource}?`));
      const initialParameters = new URL(initial ?? '', 'https://example.com').searchParams;
      expect(initialParameters.has('scope')).toBe(false);
      expect(screen.queryByRole('combobox', { name: '工作范围' })).not.toBeInTheDocument();
      expect(initialParameters.get('year')).toBe(currentCalendarDate().slice(0, 4));
      expect(initialParameters.has('month')).toBe(false);
      expect(initialParameters.has('page')).toBe(false);
      expect(initialParameters.has('page_size')).toBe(false);
      expect(screen.queryByRole('button', { name: '下一页' })).not.toBeInTheDocument();
      expect(screen.queryByRole('combobox', { name: '每页条数' })).not.toBeInTheDocument();
      const user = userEvent.setup();
      await user.click(screen.getByRole('combobox', { name: '年份' }));
      expect(screen.getByRole('option', { name: '全部年份' })).toBeVisible();
      await user.click(await screen.findByRole('option', { name: '2025 年' }));
      await user.click(screen.getByRole('combobox', { name: '月份' }));
      await user.click(screen.getByRole('option', { name: '11 月' }));
      await user.click(screen.getByRole('option', { name: '12 月' }));
      await waitFor(() => {
        expect(
          paths.some((path) => {
            const parameters = new URL(path, 'https://example.com').searchParams;
            return (
              !parameters.has('scope') &&
              parameters.getAll('year').includes(currentCalendarDate().slice(0, 4)) &&
              parameters.getAll('year').includes('2025') &&
              parameters.getAll('month').join(',') === '11,12'
            );
          }),
        ).toBe(true);
      });
      await user.keyboard('{Escape}');
      await waitFor(() => {
        expect(screen.getByRole('button', { name: '重置' })).toBeEnabled();
      });
      await user.click(screen.getByRole('button', { name: '重置' }));
      await waitFor(() => {
        const latest = paths.filter((path) => path.includes(`/${resource}?`)).at(-1);
        const parameters = new URL(latest ?? '', 'https://example.com').searchParams;
        expect(parameters.has('scope')).toBe(false);
        expect(parameters.get('year')).toBe(currentCalendarDate().slice(0, 4));
        expect(parameters.has('month')).toBe(false);
      });
    },
  );

  it.each([true, false])('链接和附件逐个展示，文件名受读取权限保护：%s', async (canRead) => {
    const item = {
      ...record,
      attachment_ids: [7, 8],
      attachments: [
        { id: 7, filename: '虚构工作材料.pdf', size: 12 },
        { id: 8, filename: '虚构会议记录.docx', size: 34 },
      ],
    };
    const paths: string[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) => {
        paths.push(path);
        return Promise.resolve(listResponse(path, [item]));
      }),
    );
    render(
      <QueryClientProvider client={createClient()}>
        <MonthlyWorkPage
          workspaceID={first}
          workspaceName="虚构空间"
          actions={['monthly_work.list', ...(canRead ? ['monthly_work.attachment.read'] : [])]}
          refresh={() => Promise.resolve()}
        />
      </QueryClientProvider>,
    );
    await screen.findByText(record.work_item);
    const resources = screen.getByRole('list', { name: '链接和附件' });
    expect(within(resources).getAllByRole('link')).toHaveLength(2);
    if (canRead) {
      expect(screen.getByRole('button', { name: '虚构工作材料.pdf' })).toBeVisible();
    } else {
      expect(screen.queryByText('虚构工作材料.pdf')).not.toBeInTheDocument();
    }
    expect(paths.some((path) => path.includes('/attachments/'))).toBe(false);
  });

  it.each(['student', 'cpc'] as const)(
    '%s 负责人转交后，仍参与工作时只能查看附件',
    async (category) => {
      const client = createClient();
      client.setQueryData(
        currentUserKey,
        userSchema.parse({
          id: record.owner_id,
          username: record.owner_username,
          display_name: record.owner_name,
          email: '',
          is_admin: true,
          password_login_enabled: true,
          password_change_required: false,
        }),
      );
      const transferred = {
        ...record,
        owner_id: 3,
        owner_name: '虚构参与人甲',
        owner_username: 'fictional.first',
        participant_ids: [2, 3, 4],
        participants: [
          { id: 2, name: record.owner_name, username: record.owner_username },
          ...record.participants,
        ],
        revision: 2,
      };
      let item = record;
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string, options: RequestInit) => {
          if (path.includes('/attachments/')) {
            return Promise.resolve(
              respond({
                items: [
                  {
                    id: 7,
                    filename: '虚构附件.pdf',
                    monthly_work_id: record.id,
                    category: category === 'cpc' ? 'cpc_monthly_work' : 'monthly_work',
                    state: 'ready',
                    size: 1,
                    revision: 1,
                    created_at: record.created_at,
                    updated_at: record.updated_at,
                  },
                ],
                total: 1,
                page: 1,
                page_size: 20,
              }),
            );
          }
          if (path.endsWith('/owners')) {
            return Promise.resolve(
              respond([
                { value: 2, label: '虚构负责人(fictional.owner)' },
                { value: 3, label: '虚构参与人甲(fictional.first)' },
              ]),
            );
          }
          if (options.method === 'PUT') {
            item = transferred;
            return Promise.resolve(respond(item));
          }
          return Promise.resolve(listResponse(path, [item]));
        }),
      );
      render(
        <QueryClientProvider client={client}>
          <MonthlyWorkPage
            workspaceID={first}
            category={category}
            workspaceName="虚构空间"
            actions={[
              'list',
              'update',
              'delete',
              'attachment.read',
              'attachment.upload',
              'attachment.delete',
            ].map((operation) => workAction(category, operation))}
            refresh={() => Promise.resolve()}
          />
        </QueryClientProvider>,
      );
      const user = userEvent.setup();
      expect(await screen.findByRole('button', { name: '管理附件' })).toBeVisible();
      expect(screen.getByRole('button', { name: '删除' })).toBeVisible();
      await user.click(screen.getByRole('button', { name: '编辑' }));
      await user.click(screen.getByRole('combobox', { name: '负责人' }));
      await user.click(
        await screen.findByRole('option', { name: '虚构参与人甲(fictional.first)' }),
      );
      await user.click(screen.getByRole('combobox', { name: '参与人' }));
      await user.click(screen.getByRole('option', { name: '虚构负责人(fictional.owner)' }));
      await user.keyboard('{Escape}');
      await user.click(screen.getByRole('button', { name: '保存' }));
      const readAttachments = await screen.findByRole('button', { name: '查看附件' });
      const work = within(screen.getByRole('article', { name: record.work_item }));
      expect(work.queryByRole('button', { name: '编辑' })).not.toBeInTheDocument();
      expect(work.queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
      await user.click(readAttachments);
      const drawer = await screen.findByRole('dialog', { name: '月度工作附件' });
      expect(await within(drawer).findByRole('button', { name: '虚构附件.pdf' })).toBeVisible();
      expect(within(drawer).queryByRole('button', { name: '上传' })).not.toBeInTheDocument();
      expect(within(drawer).queryByRole('button', { name: '删除' })).not.toBeInTheDocument();
    },
  );

  it.each(['student', 'cpc'] as const)(
    '%s 负责人和多选参与人提交数字编号，多条链接保持顺序',
    async (category) => {
      const bodies: WorkRequest[] = [];
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string, options: RequestInit) => {
          if (path.endsWith('/owners'))
            return Promise.resolve(
              respond([
                { value: 2, label: '虚构负责人(fictional.owner)' },
                { value: 3, label: '虚构参与人甲(fictional.first)' },
                { value: 4, label: '虚构参与人乙(fictional.second)' },
              ]),
            );
          const input = JSON.parse(
            typeof options.body === 'string' ? options.body : '',
          ) as WorkRequest;
          bodies.push(input);
          // 新增请求的版本为零，响应返回服务端保存后的初始版本。
          return Promise.resolve(respond({ ...record, ...input, revision: 1 }));
        }),
      );
      const close = vi.fn();
      render(
        <QueryClientProvider client={createClient()}>
          <WorkEditor
            workspaceID={first}
            category={category}
            record={null}
            actions={[workAction(category, 'list'), workAction(category, 'create')]}
            onClose={close}
          />
        </QueryClientProvider>,
      );
      const user = userEvent.setup();
      await user.type(screen.getByRole('textbox', { name: '工作项目' }), '虚构新增工作');
      await user.click(screen.getByRole('combobox', { name: '负责人' }));
      await user.click(await screen.findByRole('option', { name: '虚构负责人(fictional.owner)' }));
      await user.click(screen.getByRole('combobox', { name: '参与人' }));
      await user.click(screen.getByRole('option', { name: '虚构参与人甲(fictional.first)' }));
      await user.click(screen.getByRole('option', { name: '虚构参与人乙(fictional.second)' }));
      await user.keyboard('{Escape}');
      for (const [index, link] of record.links.entries()) {
        await user.click(screen.getByRole('button', { name: '添加链接' }));
        const fields = screen.getAllByLabelText('链接地址');
        const field = fields[index];
        if (!field) throw new Error('链接输入未创建');
        await user.type(field, link.url);
      }
      await user.click(screen.getByRole('button', { name: '保存' }));
      await waitFor(() => {
        expect(close).toHaveBeenCalledOnce();
      });
      expect(bodies).toHaveLength(1);
      expect(bodies[0]?.revision).toBe(0);
      expect(bodies[0]?.owner_id).toBe(2);
      expect(bodies[0]?.participant_ids).toEqual([3, 4]);
      expect(bodies[0]?.links.map((link) => link.url)).toEqual(
        record.links.map((link) => link.url),
      );
      expect(bodies[0]).not.toHaveProperty('owner');
    },
  );

  it.each(['student', 'cpc'] as const)(
    '%s 编辑可保留不再可选的参与人，也可移除或清空',
    async (category) => {
      const bodies: WorkRequest[] = [];
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string, options: RequestInit) => {
          if (path.endsWith('/owners'))
            return Promise.resolve(respond([{ value: 2, label: '虚构负责人(fictional.owner)' }]));
          const input = JSON.parse(
            typeof options.body === 'string' ? options.body : '',
          ) as WorkRequest;
          bodies.push(input);
          return Promise.resolve(respond({ ...record, ...input }));
        }),
      );
      render(
        <QueryClientProvider client={createClient()}>
          <WorkEditor
            workspaceID={first}
            category={category}
            record={record}
            actions={[workAction(category, 'list'), workAction(category, 'update')]}
            onClose={() => undefined}
          />
        </QueryClientProvider>,
      );
      const user = userEvent.setup();
      expect(screen.getByRole('combobox', { name: '参与人' })).toHaveTextContent(
        '虚构参与人甲(fictional.first)、虚构参与人乙(fictional.second)',
      );
      await user.click(screen.getByRole('button', { name: '保存' }));
      await waitFor(() => {
        expect(bodies).toHaveLength(1);
        expect(screen.getByRole('button', { name: '保存' })).toBeEnabled();
      });
      expect(bodies[0]?.participant_ids).toEqual([3, 4]);
      await user.click(screen.getByRole('combobox', { name: '参与人' }));
      await user.click(screen.getByRole('option', { name: '虚构参与人甲(fictional.first)' }));
      await user.keyboard('{Escape}');
      await user.click(screen.getByRole('button', { name: '保存' }));
      await waitFor(() => {
        expect(bodies).toHaveLength(2);
        expect(screen.getByRole('button', { name: '保存' })).toBeEnabled();
      });
      expect(bodies[1]?.participant_ids).toEqual([4]);
      await user.click(screen.getByRole('combobox', { name: '参与人' }));
      await user.click(screen.getByRole('option', { name: '未选择参与人' }));
      await user.keyboard('{Escape}');
      await user.click(screen.getByRole('button', { name: '保存' }));
      await waitFor(() => {
        expect(bodies).toHaveLength(3);
      });
      expect(bodies[2]?.participant_ids).toEqual([]);
    },
  );

  it.each([
    'javascript:alert(1)',
    'data:text/html,hello',
    '//example.com',
    'https://user:pass@example.com',
  ])('拒绝非 HTTP/HTTPS 或内嵌账号口令的链接 %s', (url) => {
    expect(isExternalURL(url)).toBe(false);
    expect(() => saveWork(first, record, { ...record, links: [{ name: '', url }] })).toThrow();
  });

  it('同空间搜索保留时间轴，切换工作空间立即隔离旧记录', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) => {
        if (path.includes(second) || path.includes('keyword=missing'))
          return new Promise<Response>(() => undefined);
        return Promise.resolve(listResponse(path));
      }),
    );
    const client = createClient();
    const props = {
      workspaceID: first,
      workspaceName: '虚构空间甲',
      actions: ['monthly_work.list'],
      refresh: () => Promise.resolve(),
    };
    const view = render(
      <QueryClientProvider client={client}>
        <MonthlyWorkPage {...props} />
      </QueryClientProvider>,
    );
    expect(await screen.findByText(record.work_item)).toBeInTheDocument();
    const user = userEvent.setup();
    await user.type(screen.getByRole('textbox', { name: '关键词' }), 'missing');
    await user.click(screen.getByRole('button', { name: '搜索' }));
    expect(screen.getByText(record.work_item)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '搜索' })).toHaveAttribute('aria-busy', 'true');
    view.rerender(
      <QueryClientProvider client={client}>
        <MonthlyWorkPage {...props} workspaceID={second} workspaceName="虚构空间乙" />
      </QueryClientProvider>,
    );
    expect(screen.queryByText(record.work_item)).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^学生工作\/月度工作$/,
    );
  });

  it('同一空间切换工作类别立即隔离旧记录，学生权限不能操作党建工作', async () => {
    const paths: string[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) => {
        paths.push(path);
        if (path.includes('/cpc-monthly-work')) return new Promise<Response>(() => undefined);
        return Promise.resolve(listResponse(path));
      }),
    );
    const client = createClient();
    const props = {
      workspaceID: first,
      workspaceName: '虚构空间',
      actions: [
        'monthly_work.list',
        'monthly_work.create',
        'monthly_work.update',
        'cpc.monthly_work.list',
      ],
      refresh: () => Promise.resolve(),
    };
    const view = render(
      <QueryClientProvider client={client}>
        <MonthlyWorkPage {...props} />
      </QueryClientProvider>,
    );
    await screen.findByText(record.work_item);
    expect(screen.getByRole('button', { name: '新增工作' })).toBeVisible();
    view.rerender(
      <QueryClientProvider client={client}>
        <MonthlyWorkPage {...props} category="cpc" />
      </QueryClientProvider>,
    );
    expect(screen.queryByText(record.work_item)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '新增工作' })).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^党建工作\/月度工作$/,
    );
    await waitFor(() => {
      expect(
        paths.some((path) => path.startsWith(`/v1/workspaces/${first}/cpc-monthly-work?`)),
      ).toBe(true);
    });
    view.rerender(
      <QueryClientProvider client={client}>
        <MonthlyWorkPage {...props} />
      </QueryClientProvider>,
    );
    expect(await screen.findByText(record.work_item)).toBeVisible();
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^学生工作\/月度工作$/,
    );
  });

  it.each(['student', 'cpc'] as const)(
    '%s 工作附件使用固定类别端点申请和确认直传',
    async (category) => {
      const calls: { path: string; options: RequestInit }[] = [];
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string, options: RequestInit) => {
          calls.push({ path, options });
          return Promise.resolve(
            respond(
              calls.length === 1
                ? preparedUpload()
                : {
                    id: 1,
                    monthly_work_id: record.id,
                    filename: '虚构材料.pdf',
                    category: category === 'cpc' ? 'cpc_monthly_work' : 'monthly_work',
                    state: 'ready',
                    size: 3,
                    revision: 2,
                    created_at: record.created_at,
                    updated_at: record.created_at,
                  },
            ),
          );
        }),
      );
      const resource = category === 'cpc' ? 'cpc-monthly-work' : 'monthly-work';
      await uploadWorkAttachment(first, record.id, new File(['abc'], '虚构材料.pdf'), category);
      expect(calls[0]?.path).toBe(`/v1/workspaces/${first}/${resource}/1/attachments`);
      expect(
        JSON.parse(typeof calls[0]?.options.body === 'string' ? calls[0].options.body : ''),
      ).toEqual({
        filename: '虚构材料.pdf',
        size: 3,
        content_type: 'application/pdf',
      });
      expect(calls[1]?.path).toBe(`/v1/workspaces/${first}/${resource}/1/attachments/complete`);
      expect(new Headers(calls[0]?.options.headers).get('X-CSRF-Protection')).toBe('1');
      expect(workAttachmentDownload(first, 1, category)).toBe(
        `/v1/workspaces/${first}/attachments/${resource}/1/download`,
      );
    },
  );
});
