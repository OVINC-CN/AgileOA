import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  emptyFilters,
  exportPeople,
  type Person,
  personFilterParams,
  personListOptions,
  personStatisticsOptions,
  savePerson,
  stages,
} from '@/features/cpc-person/api';
import { PersonChart } from '@/features/cpc-person/person-chart';
import { PersonEditor } from '@/features/cpc-person/person-editor';
import { PersonPage } from '@/features/cpc-person/person-page';
import type { TimelineFilterDraft } from '@/features/cpc-person/timeline-filter-model';
import {
  timelineViewListOptions,
  timelineViewStatisticsOptions,
} from '@/features/cpc-person/timeline-view-api';
import type { TimelineNode } from '@/features/cpc-timeline/model';
import { xlsxContentType } from '@/lib/spreadsheet';
import { PageNavigationHarness } from '@/test/page-navigation-harness';

const first = 'w7m2k9p4s6n8c3x5';
const second = 'x8n3m0q5t7p9d4y6';
const person: Person = {
  id: 1,
  student_id: 1,
  timeline_template_id: 1,
  timeline_template_name: '虚构时间线',
  timeline_status: 'normal',
  timeline_pending: 2,
  timeline_completed: 0,
  timeline_skipped: 0,
  stage: 'full_member',
  status: '在编',
  party_position: '自定义职务',
  student_no: '00000001',
  name: '虚构学生甲',
  gender: '女',
  ethnicity: '汉族',
  grade: '2026级',
  student_status: '在籍',
  education_level: '硕士',
  class_name: '虚构班级',
  major: '虚构专业',
  advisor: '虚构导师',
  phone: '',
  arrival_month: '2026-09',
  previous_school_or_employer: '虚构大学',
  previous_graduation_month: '2026-06',
  party_branch_id: 2,
  party_branch_name: '虚构支部全称',
  party_branch_short_name: '虚构支部',
  revision: 1,
  can_update: true,
  can_delete: false,
  created_at: '2026-11-06T00:00:00Z',
  updated_at: '2026-11-06T00:00:00Z',
};
const statistics = {
  total: 45,
  stages: stages.map((stage, index) => ({ ...stage, count: [11, 7, 2, 5, 20][index] ?? 0 })),
};
const respond = (data: unknown) =>
  new Response(
    JSON.stringify({
      trace: '4bf92f3577b34da6a3ce929d0e0e4736',
      error_msg: '',
      data,
    }),
  );

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('党建人员', () => {
  it('通知直接打开目标人员时间线，审核模式沿用独立审核接口及权限', async () => {
    const fetch = vi.fn(() =>
      Promise.resolve(
        respond({
          person_id: 42,
          person_name: '虚构审核人员',
          person_revision: 1,
          template_id: 1,
          template_name: '虚构待审核时间线',
          template_revision: 1,
          can_update: false,
          can_review_transfer: true,
          transfer_target_name: '虚构目标模板',
          status: 'transferring',
          nodes: [],
        }),
      ),
    );
    vi.stubGlobal('fetch', fetch);
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter
          initialEntries={[`/cpc/roster?workspace=${first}&person_id=42&timeline_mode=review`]}
        >
          <PersonPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={['cpc.person.update', 'cpc.person.timeline_transfer.review']}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    const drawer = await screen.findByRole('dialog', { name: '人员时间线' });
    expect(await within(drawer).findByRole('button', { name: '审核' })).toBeVisible();
    expect(drawer).toHaveAccessibleDescription('虚构审核人员 (42)');
    expect(fetch).toHaveBeenCalledExactlyOnceWith(
      `/v1/workspaces/${first}/cpc-people/42/timeline/review`,
      expect.anything(),
    );
    await user.click(within(drawer).getByRole('button', { name: '关闭抽屉' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: '人员时间线' })).not.toBeInTheDocument();
    });
  });

  it.each(['42', '0', 'invalid', '9007199254740992'])(
    '无授权时通知参数不会触发人员或时间线读取：%s',
    (personID) => {
      const fetch = vi.fn();
      vi.stubGlobal('fetch', fetch);
      render(
        <QueryClientProvider client={new QueryClient()}>
          <MemoryRouter initialEntries={[`/cpc/roster?workspace=${first}&person_id=${personID}`]}>
            <PersonPage
              workspaceID={first}
              workspaceName="虚构空间"
              actions={[]}
              refresh={() => Promise.resolve()}
            />
          </MemoryRouter>
        </QueryClientProvider>,
      );
      expect(fetch).not.toHaveBeenCalled();
      expect(screen.queryByRole('dialog', { name: '人员时间线' })).not.toBeInTheDocument();
    },
  );

  it('时间线范围、零和否通过名单、统计及两种导出发送相同条件，忽略空白草稿', async () => {
    const condition = (
      id: string,
      type: 'date' | 'number' | 'boolean',
      operator: string,
      value: string,
    ): TimelineFilterDraft => ({
      id,
      field: { node_id: 7, field_id: id, label: '虚构字段', type },
      operator,
      value,
    });
    const filters = {
      ...emptyFilters,
      timeline_filters: [
        condition('occurred_at', 'date', 'gte', '2026-11-01'),
        condition('occurred_at', 'date', 'lte', '2026-11-06'),
        condition('score', 'number', 'eq', '0'),
        condition('confirmed', 'boolean', 'eq', 'false'),
        condition('optional', 'date', 'empty', ''),
        condition('draft', 'number', 'eq', '   '),
      ],
    };
    const expected = personFilterParams(filters).getAll('timeline_filter');
    expect(expected.map((value) => JSON.parse(value) as unknown)).toEqual([
      { node_id: 7, field_id: 'occurred_at', operator: 'gte', value: '2026-11-01' },
      { node_id: 7, field_id: 'occurred_at', operator: 'lte', value: '2026-11-06' },
      { node_id: 7, field_id: 'score', operator: 'eq', value: '0' },
      { node_id: 7, field_id: 'confirmed', operator: 'eq', value: 'false' },
      { node_id: 7, field_id: 'optional', operator: 'empty' },
    ]);
    const calls: URL[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((input: string) => {
        const url = new URL(input, 'https://example.com');
        calls.push(url);
        const data = url.pathname.endsWith('/export')
          ? {
              filename:
                url.searchParams.get('format') === 'full'
                  ? 'cpc-people-full.xlsx'
                  : 'cpc-people.xlsx',
              content_type: xlsxContentType,
              content: '',
            }
          : url.pathname.endsWith('/statistics')
            ? url.pathname.includes('/timeline-view/')
              ? { total: 1, completed: 0, pending: 1, skipped: 0 }
              : statistics
            : {
                items: url.pathname.includes('/timeline-view/')
                  ? [{ person, values: {}, status: 'pending', status_label: '待处理' }]
                  : [person],
                total: 1,
                page: 1,
                page_size: 5,
                can_create: false,
                can_import: false,
                can_export: true,
              };
        return Promise.resolve(respond(data));
      }),
    );
    const client = new QueryClient();
    await client.query(personListOptions(first, filters, 1, 5));
    await client.query(personStatisticsOptions(first, filters));
    await client.query(timelineViewListOptions(first, 1, 7, filters, 1, 5));
    await client.query(timelineViewStatisticsOptions(first, 1, 7, filters));
    await exportPeople(first, filters, 'basic');
    await exportPeople(first, filters, 'full');
    expect(calls).toHaveLength(6);
    for (const url of calls) {
      expect(url.searchParams.getAll('timeline_filter')).toEqual(expected);
      expect(url.searchParams.has('timeline_filters')).toBe(false);
    }
  });

  it('完整导出发送已提交的筛选与完整数据类型，并校验对应文件名', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      respond({
        filename: 'cpc-people-full.xlsx',
        content_type: xlsxContentType,
        content: btoa('PK'),
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    const result = await exportPeople(
      first,
      {
        ...emptyFilters,
        status: ['转出'],
        timeline_status: ['abnormal'],
        node_status: ['pending', 'skipped'],
      },
      'full',
      1,
      7,
    );
    const url = new URL(String(fetchMock.mock.calls[0]?.[0]), 'https://example.com');
    expect(url.searchParams.get('format')).toBe('full');
    expect(url.searchParams.getAll('node_status')).toEqual(['pending', 'skipped']);
    expect(url.searchParams.has('timeline_status')).toBe(false);
    expect(url.searchParams.get('timeline_template_id')).toBe('1');
    expect(url.searchParams.get('timeline_node_id')).toBe('7');
    expect(url.searchParams.getAll('status')).toEqual(['转出']);
    expect(url.searchParams.has('student_status')).toBe(false);
    expect(result.filename).toBe('cpc-people-full.xlsx');
  });
  it('党员数量合并统计，备注分别展示中共预备党员和中共党员人数，顺序符合业务要求', () => {
    render(<PersonChart data={statistics} />);
    const cards = within(screen.getByLabelText('党建人员统计'));
    expect(cards.getByText('25')).toBeVisible();
    expect(cards.getByText('中共预备党员 5 人，中共党员 20 人')).toBeVisible();
    const labels = ['党员人数', '发展对象', '入党积极分子', '入党申请人'];
    for (let index = 1; index < labels.length; index += 1) {
      expect(
        cards
          .getByText(labels[index - 1] ?? '')
          .compareDocumentPosition(cards.getByText(labels[index] ?? '')),
      ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    }
  });

  it('编辑发送人员状态等四项字段及版本，不能更换时间线模板，不发送自动支部或学生基础信息', async () => {
    const fetchMock = vi.fn<(url: string, options: RequestInit) => Promise<Response>>();
    fetchMock.mockResolvedValue(respond(person));
    vi.stubGlobal('fetch', fetchMock);
    await savePerson(first, person, person);
    const body = fetchMock.mock.calls[0]?.[1].body;
    if (typeof body !== 'string') throw new Error('未提交人员编辑 JSON');
    expect(JSON.parse(body)).toEqual({
      student_id: 1,
      stage: 'full_member',
      status: '在编',
      party_position: '自定义职务',
      revision: 1,
    });
  });

  it('新增默认在编，编辑可新建状态并保存，之后可读取已有状态选项', async () => {
    let saved = person;
    const client = new QueryClient();
    const onClose = vi.fn();
    const fetchMock = vi.fn((input: string, options?: RequestInit) => {
      const url = new URL(input, 'https://example.com');
      if (options?.method === 'PUT') {
        if (typeof options.body !== 'string') throw new Error('未提交人员编辑 JSON');
        const values = JSON.parse(options.body) as { status: string };
        saved = { ...saved, status: values.status, revision: saved.revision + 1 };
        return Promise.resolve(respond(saved));
      }
      expect(url.searchParams.get('field')).toBe('status');
      return Promise.resolve(respond([{ value: saved.status, label: saved.status }]));
    });
    vi.stubGlobal('fetch', fetchMock);
    const create = render(
      <QueryClientProvider client={client}>
        <PersonEditor workspaceID={first} person={null} onClose={onClose} />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('combobox', { name: '状态' })).toHaveTextContent('在编');
    create.unmount();
    render(
      <QueryClientProvider client={client}>
        <PersonEditor workspaceID={first} person={person} onClose={onClose} />
      </QueryClientProvider>,
    );
    await userEvent.click(screen.getByRole('combobox', { name: '状态' }));
    const search = screen.getByRole('searchbox', { name: '搜索状态' });
    expect(search).toHaveAttribute('maxlength', '8');
    await screen.findByRole('option', { name: '在编' });
    expect(screen.getByRole('option', { name: '转出' })).toBeVisible();
    await userEvent.type(search, '待核实');
    await userEvent.click(screen.getByRole('option', { name: '新建“待核实”' }));
    expect(screen.getByRole('combobox', { name: '状态' })).toHaveTextContent('待核实');
    await userEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(onClose).toHaveBeenCalledOnce();
    });
    expect(saved.status).toBe('待核实');
    await userEvent.click(screen.getByRole('combobox', { name: '状态' }));
    expect(await screen.findByRole('option', { name: '待核实' })).toBeVisible();
    await waitFor(() => {
      expect(fetchMock.mock.calls.filter(([, options]) => options?.method === 'GET')).toHaveLength(
        2,
      );
    });
  });

  it('人员状态替换学生状态筛选，默认在编，清空与重置同步影响名单和统计', async () => {
    const calls: URL[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((input: string) => {
        const url = new URL(input, 'https://example.com');
        calls.push(url);
        const data = url.pathname.endsWith('/options')
          ? [
              { value: '在编', label: '在编' },
              { value: '转出', label: '转出' },
            ]
          : url.pathname.endsWith('/statistics')
            ? statistics
            : {
                items: [person],
                total: 1,
                page: 1,
                page_size: 5,
                can_create: false,
                can_import: false,
                can_export: false,
              };
        return Promise.resolve(respond(data));
      }),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <PersonPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={['cpc.person.list', 'cpc.person.statistics']}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByText(person.name);
    const latest = (path: string) =>
      calls.findLast((url) => url.pathname.endsWith(path))?.searchParams;
    expect(latest('/cpc-people')?.getAll('status')).toEqual(['在编']);
    expect(latest('/statistics')?.getAll('status')).toEqual(['在编']);
    expect(screen.getByRole('columnheader', { name: '状态' })).toBeVisible();
    expect(screen.queryByRole('columnheader', { name: '学生状态' })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: '更多筛选' }));
    const status = screen.getByRole('combobox', { name: '状态' });
    expect(status).toHaveTextContent('在编');
    expect(screen.queryByRole('combobox', { name: '学生状态' })).not.toBeInTheDocument();
    await userEvent.click(status);
    await userEvent.click(await screen.findByRole('option', { name: '全部状态' }));
    await waitFor(() => {
      expect(latest('/cpc-people')?.getAll('status')).toEqual([]);
    });
    expect(latest('/statistics')?.getAll('status')).toEqual([]);
    await userEvent.click(await screen.findByRole('option', { name: '转出' }));
    await waitFor(() => {
      expect(latest('/cpc-people')?.getAll('status')).toEqual(['转出']);
    });
    expect(latest('/statistics')?.getAll('status')).toEqual(['转出']);
    await userEvent.click(status);
    await userEvent.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() => {
      expect(latest('/cpc-people')?.getAll('status')).toEqual(['在编']);
    });
    expect(latest('/statistics')?.getAll('status')).toEqual(['在编']);
    expect(calls.some((url) => url.searchParams.has('student_status'))).toBe(false);
  });

  it('统计响应必须包含完整且唯一的五种阶段，合计必须与各阶段一致', async () => {
    const client = new QueryClient();
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(respond(statistics))),
    );
    expect(await client.query(personStatisticsOptions(first, emptyFilters))).toEqual(statistics);
    for (const invalid of [
      { ...statistics, total: 1 },
      { ...statistics, stages: statistics.stages.slice(1) },
      { ...statistics, stages: statistics.stages.map((item) => ({ ...item, value: 'applicant' })) },
    ]) {
      vi.stubGlobal(
        'fetch',
        vi.fn(() => Promise.resolve(respond(invalid))),
      );
      await expect(
        client.query({ ...personStatisticsOptions(first, emptyFilters), staleTime: 0 }),
      ).rejects.toThrow();
    }
  });

  it('切换空间立即隐藏旧名单，等待新空间结果，不复用旧人员', async () => {
    let resolveSecond: (response: Response) => void = () => {
      throw new Error('第二空间尚未请求');
    };
    const listing = (items: Person[]) => ({
      items,
      total: items.length,
      page: 1,
      page_size: 5,
      can_create: false,
      can_import: false,
      can_export: false,
    });
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) =>
        url.includes(first)
          ? Promise.resolve(respond(listing([person])))
          : new Promise<Response>((resolve) => {
              resolveSecond = resolve;
            }),
      ),
    );
    const client = new QueryClient();
    const view = (workspaceID: string) => (
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <PersonPage
            workspaceID={workspaceID}
            workspaceName="虚构空间"
            actions={['cpc.person.list']}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>
    );
    const rendered = render(view(first));
    await screen.findByText(person.name);
    rendered.rerender(view(second));
    expect(screen.queryByText(person.name)).not.toBeInTheDocument();
    resolveSecond(respond(listing([{ ...person, id: 2, name: '虚构学生乙' }])));
    expect(await screen.findByText('虚构学生乙')).toBeVisible();
  });

  it('两个视图共用筛选，节点列和操作限定当前节点，切换节点隔离旧数据', async () => {
    const calls: URL[] = [];
    const node: TimelineNode = {
      numeric_id: 7,
      id: 'node_first',
      name: '申请材料',
      description: '',
      group_name: '申请阶段',
      is_baseline: true,
      time_field_id: 'occurred_at',
      fields: [
        { id: 'occurred_at', name: '发生时间', type: 'date', required: true },
        { id: 'score', name: '节点分数', type: 'number', required: false },
        { id: 'approved', name: '已核对', type: 'boolean', required: false },
        { id: 'comment', name: '备注', type: 'textarea', required: false },
      ],
      rules: [],
    };
    const next = {
      ...node,
      numeric_id: 8,
      id: 'node_next',
      name: '后续节点',
      fields: [{ id: 'reviewed_at', name: '复核日期', type: 'date', required: true }],
      time_field_id: 'reviewed_at',
    };
    const list = (items: unknown[]) => ({
      items,
      total: 12,
      page: 1,
      page_size: 5,
      can_create: false,
      can_import: false,
      can_export: false,
    });
    let resolveNext: (response: Response) => void = () => {
      throw new Error('尚未请求后续节点');
    };
    vi.stubGlobal(
      'fetch',
      vi.fn((input: string) => {
        const url = new URL(input, 'https://example.com');
        calls.push(url);
        if (url.pathname.endsWith('/timeline-templates/list')) {
          return Promise.resolve(respond([{ value: '1', label: '虚构时间线' }]));
        }
        if (url.pathname.endsWith('/timeline-view/1')) {
          return Promise.resolve(
            respond({ template_id: 1, template_name: '虚构时间线', nodes: [node, next] }),
          );
        }
        if (url.pathname.endsWith('/1/timeline') || url.pathname.endsWith('/1/timeline/edit')) {
          return Promise.resolve(
            respond({
              person_id: person.id,
              person_revision: person.revision,
              template_id: 1,
              template_name: '虚构时间线',
              template_revision: 1,
              can_update: true,
              status: 'normal',
              nodes: [node, next].map((item) => ({
                ...item,
                values: { occurred_at: '2026-11-06', score: 0, approved: false },
                status: 'completed',
                status_label: '已完成',
                revision: 1,
                attachments: [],
                template_attachments: [],
                checks: [],
                issues: [],
              })),
            }),
          );
        }
        if (url.pathname.endsWith('/statistics')) {
          return Promise.resolve(
            respond(
              url.pathname.includes('/timeline-view/')
                ? { total: 12, completed: 8, pending: 3, skipped: 1 }
                : statistics,
            ),
          );
        }
        if (url.pathname.endsWith('/nodes/8')) {
          return new Promise<Response>((resolve) => {
            resolveNext = resolve;
          });
        }
        return Promise.resolve(
          respond(
            list(
              url.pathname.endsWith('/nodes/7')
                ? [
                    {
                      person,
                      values: {
                        occurred_at: '2026-11-06',
                        score: 0,
                        approved: false,
                        comment: ' ',
                      },
                      status: 'completed',
                      status_label: '已完成',
                    },
                  ]
                : [person],
            ),
          ),
        );
      }),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={[`/cpc/roster?workspace=${first}`]}>
          <PageNavigationHarness workspaceID={first} actions={['cpc.person.list']}>
            <PersonPage
              workspaceID={first}
              workspaceName="虚构空间"
              actions={[
                'cpc.person.list',
                'cpc.person.statistics',
                'cpc.person.detail',
                'cpc.person.update',
                'cpc.person.delete',
              ]}
              refresh={() => Promise.resolve()}
            />
          </PageNavigationHarness>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByText(person.name);
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: '人员视图', current: 'page' })).toBeVisible();
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^党建工作\/人员名单\/人员视图$/,
    );
    await userEvent.type(screen.getByRole('searchbox', { name: '学号或姓名' }), '虚构');
    await userEvent.click(screen.getByRole('button', { name: '搜索' }));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '搜索' })).toBeEnabled();
    });
    await userEvent.click(screen.getByRole('link', { name: '时间线视图' }));
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^党建工作\/人员名单\/时间线视图$/,
    );
    expect(await screen.findByText('2026-11-06')).toBeVisible();
    expect(screen.getByRole('combobox', { name: '时间线' })).toHaveTextContent('虚构时间线');
    const nodeSelector = screen.getByRole('combobox', { name: '节点' });
    expect(nodeSelector).toHaveTextContent('1. 申请材料');
    const filterForm = screen.getByRole('searchbox', { name: '学号或姓名' }).closest('form');
    if (!filterForm) throw new Error('缺少人员筛选表单');
    expect(
      Boolean(
        filterForm.compareDocumentPosition(screen.getByLabelText('时间线与节点选择')) &
        Node.DOCUMENT_POSITION_FOLLOWING,
      ),
    ).toBe(true);
    expect(screen.getByRole('button', { name: '申请材料' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(
      within(screen.getByRole('button', { name: '申请材料' })).getByRole('img', {
        name: '基准节点',
      }),
    ).toBeVisible();
    expect(screen.queryByText('基准节点', { exact: true })).not.toBeInTheDocument();
    const row = screen.getByText(person.name).closest('tr');
    if (!row) throw new Error('缺少节点人员行');
    expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual([
      '学号',
      '姓名',
      '节点状态',
      '发生时间',
      '节点分数',
      '已核对',
      '备注',
      '创建时间',
      '更新时间',
      '操作',
    ]);
    expect(
      within(row)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['时间线', '节点详情', '节点编辑']);
    expect(within(row).getByText('0', { exact: true })).toBeVisible();
    expect(within(row).getByText('否', { exact: true })).toBeVisible();
    expect(within(row).getByText('-', { exact: true })).toBeVisible();
    const chart = within(screen.getByLabelText('时间线节点统计'));
    expect(chart.getByText('12')).toBeVisible();
    expect(chart.getByText('8')).toBeVisible();
    expect(chart.getByText('3')).toBeVisible();
    expect(chart.getByText('1')).toBeVisible();
    expect(screen.getByRole('searchbox', { name: '学号或姓名' })).toHaveValue('虚构');
    const nodeCalls = calls.filter((url) => url.pathname.includes('/nodes/7'));
    expect(nodeCalls).toHaveLength(2);
    for (const url of nodeCalls) {
      expect(url.searchParams.get('keyword')).toBe('虚构');
      expect(url.searchParams.getAll('status')).toEqual(['在编']);
    }

    // 时间线视图使用节点状态多选，立即向列表和统计提交，人员视图不携带此条件。
    expect(screen.queryByRole('combobox', { name: '时间线状态' })).not.toBeInTheDocument();
    const statusFilter = screen.getByRole('combobox', { name: '节点状态' });
    await userEvent.click(statusFilter);
    await userEvent.click(screen.getByRole('option', { name: '已完成' }));
    await userEvent.click(screen.getByRole('option', { name: '已跳过' }));
    await waitFor(() => {
      for (const path of ['/nodes/7', '/nodes/7/statistics']) {
        const params = calls.findLast((url) => url.pathname.endsWith(path))?.searchParams;
        expect(params?.getAll('node_status')).toEqual(['completed', 'skipped']);
        expect(params?.has('timeline_status')).toBe(false);
      }
    });
    await userEvent.click(statusFilter);

    // 节点详情读取详情入口，节点编辑读取编辑入口并直接打开所选节点。
    await userEvent.click(within(row).getByRole('button', { name: '节点详情' }));
    const details = await screen.findByRole('dialog', { name: '时间线节点详情' });
    expect(within(details).getByText('0', { exact: true })).toBeVisible();
    expect(within(details).getByText('否', { exact: true })).toBeVisible();
    expect(within(details).queryByText('后续节点')).not.toBeInTheDocument();
    expect(calls.some((url) => url.pathname.endsWith('/1/timeline'))).toBe(true);
    await userEvent.click(within(details).getByRole('button', { name: '关闭抽屉' }));
    await userEvent.click(screen.getByRole('button', { name: '节点编辑' }));
    const editor = await screen.findByRole('dialog', { name: '编辑时间线节点' });
    expect(within(editor).getByLabelText('节点分数')).toHaveValue(0);
    expect(within(editor).queryByLabelText('复核日期 *')).not.toBeInTheDocument();
    expect(calls.some((url) => url.pathname.endsWith('/1/timeline/edit'))).toBe(true);
    await userEvent.click(within(editor).getByRole('button', { name: '取消' }));
    await waitFor(() => {
      expect(nodeSelector).toBeEnabled();
    });
    await userEvent.click(nodeSelector);
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      '1. 申请材料',
      '2. 后续节点',
    ]);
    await userEvent.click(screen.getByRole('option', { name: '2. 后续节点' }));
    await waitFor(() => {
      expect(calls.some((url) => url.pathname.endsWith('/nodes/8'))).toBe(true);
    });
    expect(screen.queryByText(person.name)).not.toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: '节点分数' })).not.toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: '复核日期' })).toBeVisible();
    resolveNext(
      respond(
        list([
          {
            person,
            values: { reviewed_at: '2026-11-07' },
            status: 'completed',
            status_label: '已完成',
          },
        ]),
      ),
    );
    expect(await screen.findByText('2026-11-07')).toBeVisible();
    await userEvent.type(screen.getByRole('searchbox', { name: '学号或姓名' }), '草稿');
    await userEvent.click(screen.getByRole('link', { name: '人员视图' }));
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^党建工作\/人员名单\/人员视图$/,
    );
    expect(screen.getByRole('searchbox', { name: '学号或姓名' })).toHaveValue('虚构草稿');
    expect(screen.queryByRole('combobox', { name: '节点状态' })).not.toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: '时间线状态' })).toBeVisible();
    expect(
      calls
        .findLast((url) => url.pathname.endsWith('/cpc-people'))
        ?.searchParams.has('node_status'),
    ).toBe(false);
    expect(calls.some((url) => url.searchParams.get('keyword') === '虚构草稿')).toBe(false);
    expect(calls.some((url) => url.pathname.includes('/cpc-timeline-templates'))).toBe(false);
  });

  it('无列表权限时隐藏视图入口，也不请求模板和节点数据', () => {
    const fetchMock = vi.fn(() => Promise.resolve(respond([])));
    vi.stubGlobal('fetch', fetchMock);
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/cpc/roster?tab=timeline']}>
          <PersonPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={[]}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(screen.getByText('暂无查看人员名单的权限')).toBeVisible();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
