import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Link, MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  attachmentDownload,
  type Participation,
  type ParticipationInput,
  type Project,
  saveParticipation,
  saveProject,
  uploadAttachment,
} from '@/features/project/api';
import { ParticipationEditor } from '@/features/project/participation-editor';
import { ProjectEditor } from '@/features/project/project-editor';
import { ProjectPage } from '@/features/project/project-page';
import { buildProjectSeries } from '@/features/project/series';
import { preparedUpload } from '@/test/mock-cos';

const first = 'w7m2k9p4s6n8c3x5';
const second = 'r4v8b2m6k9d3p5s7';
const record: Participation = {
  id: 1,
  project_id: 1,
  project_name: '虚构年度项目',
  year: 2025,
  student_id: 1,
  student_no: '00000001',
  student_name: '虚构学生',
  education_level: '硕士',
  major: '虚构专业',
  grade: '2026级',
  rank: 3,
  score: 90.123456,
  details: [{ name: '已有子项', description: '虚构详细说明', score: 5.123456 }],
  detail_count: 1,
  revision: 1,
  created_at: '2026-11-03T00:00:00Z',
  updated_at: '2026-11-03T00:00:00Z',
};
const client = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
function respond(data: unknown) {
  return new Response(
    JSON.stringify({ trace: '4bf92f3577b34da6a3ce929d0e0e4736', error_msg: '', data }),
  );
}
afterEach(() => {
  vi.unstubAllGlobals();
});

describe('评奖评优', () => {
  it('允许自行申报时缺少截止时间不能保存项目', () => {
    const fetch = vi.fn();
    vi.stubGlobal('fetch', fetch);
    expect(() =>
      saveProject(first, null, {
        name: '虚构项目',
        year: 2026,
        description: '',
        detail_names: [],
        allow_self_apply: true,
        self_apply_deadline: null,
        approval_steps: [],
      }),
    ).toThrow('允许学生自行申报时必须填写截止时间');
    expect(fetch).not.toHaveBeenCalled();
  });
  it.each([
    ['project.participation.list', '获奖学生'],
    ['project.list', '奖项管理'],
  ])('只有 %s 时进入对应视图，正文不再展示子 Tab', async (action, visible) => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) =>
        Promise.resolve(
          respond(
            path.includes('/filter-options')
              ? { names: [], years: [], education_levels: [], majors: [], grades: [] }
              : { items: [], total: 0, page: 1, page_size: 5 },
          ),
        ),
      ),
    );
    render(
      <QueryClientProvider client={client()}>
        <MemoryRouter initialEntries={[`/stu/projects?workspace=${first}&tab=invalid`]}>
          <ProjectPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={[action]}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      `学生工作/评奖评优/${visible}`,
    );
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
    });
  });

  it('没有任何 list 权限时隐藏全部子 Tab 且不请求列表', () => {
    const fetch = vi.fn();
    vi.stubGlobal('fetch', fetch);
    render(
      <QueryClientProvider client={client()}>
        <MemoryRouter>
          <ProjectPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={[]}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
  });

  it('通过链接直达奖项管理时只读取对应列表', async () => {
    const fetch = vi.fn((path: string) =>
      Promise.resolve(
        respond(
          path.includes('/filter-options')
            ? { names: [], years: [], education_levels: [], majors: [], grades: [] }
            : { items: [], total: 0, page: 1, page_size: 5 },
        ),
      ),
    );
    vi.stubGlobal('fetch', fetch);
    render(
      <QueryClientProvider client={client()}>
        <MemoryRouter initialEntries={[`/stu/projects?workspace=${first}&tab=projects`]}>
          <ProjectPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={['project.list', 'project.participation.list']}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^学生工作\/评奖评优\/奖项管理$/,
    );
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled();
    });
    expect(fetch.mock.calls.some(([path]) => path.includes('/projects?'))).toBe(true);
    expect(fetch.mock.calls.some(([path]) => path.includes('/project-participations?'))).toBe(
      false,
    );
  });
  it('按项目归属年份生成独立折线，同名项目共用系列并补零', () => {
    const series = buildProjectSeries([
      { name: '项目.甲[0]', year: 2025, count: 2 },
      { name: '项目.甲[0]', year: 2026, count: 3 },
      { name: '__proto__', year: 2026, count: 1 },
    ]);
    const index = series.names.indexOf('项目.甲[0]');
    expect(series.rows.map((row) => row[`project_${String(index)}`])).toEqual([2, 3]);
    expect(series.rows.map((row) => row['year'])).toEqual([2025, 2026]);
    expect(series.yearDomain).toEqual([2024, 2027]);
    const other = series.names.indexOf('__proto__');
    expect(series.rows[0]?.[`project_${String(other)}`]).toBe(0);
  });

  it('正文不展示子 Tab，仅奖项管理展示图表，操作与导出随 URL 切换', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) =>
        Promise.resolve(
          respond(
            path.includes('/filter-options')
              ? { names: [], years: [], education_levels: [], majors: [], grades: [] }
              : path.includes('/statistics?')
                ? []
                : { items: [], total: 0, page: 1, page_size: 5 },
          ),
        ),
      ),
    );
    render(
      <QueryClientProvider client={client()}>
        <MemoryRouter initialEntries={[`/stu/projects?workspace=${first}&tab=participations`]}>
          <Link to={`/stu/projects?workspace=${first}&tab=projects`}>切换至奖项管理</Link>
          <ProjectPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={[
              'project.list',
              'project.participation.list',
              'project.create',
              'project.participation.create',
              'project.import',
              'project.participation.import',
              'project.export',
              'project.participation.export',
              'project.attachment.read',
              'project.attachment.upload',
              'project.statistics',
            ]}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    const breadcrumb = screen.getByRole('navigation', { name: '面包屑' });
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('各奖项年度获奖人数')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '绑定学生' })).toBeVisible();
    expect(screen.queryByRole('button', { name: '新建项目' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '附件' })).not.toBeInTheDocument();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: '导出' }));
    expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual([
      '导出获奖学生',
      '导出获奖明细',
    ]);
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('link', { name: '切换至奖项管理' }));
    expect(breadcrumb).toHaveTextContent('学生工作/评奖评优/奖项管理');
    expect(screen.getByLabelText('各奖项年度获奖人数')).toBeVisible();
    expect(screen.queryByRole('button', { name: '提示字段' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '新建项目' })).toHaveAttribute(
      'data-variant',
      'default',
    );
    expect(screen.queryByRole('button', { name: '绑定学生' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '附件' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '导出' }));
    expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['导出奖项']);
  });

  it('项目详情以只读标签展示配置名称，说明及附件为空时显示短横线', async () => {
    const project: Project = {
      allow_self_apply: false,
      self_apply_deadline: null,
      approval_steps: [],
      id: 1,
      name: '虚构项目',
      year: 2026,
      description: '',
      detail_names: ['志愿服务', '社会实践'],
      attachment_ids: [],
      revision: 1,
      created_at: record.created_at,
      updated_at: record.updated_at,
    };
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) =>
        Promise.resolve(
          respond(
            path.includes('/filter-options')
              ? { names: [], years: [], education_levels: [], majors: [], grades: [] }
              : { items: [project], total: 1, page: 1, page_size: 5 },
          ),
        ),
      ),
    );
    render(
      <QueryClientProvider client={client()}>
        <MemoryRouter>
          <ProjectPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={['project.list']}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await userEvent.click(await screen.findByRole('button', { name: '详情' }));
    const tags = screen.getByRole('list', { name: '可选子项名称' });
    expect(tags.querySelectorAll('[data-slot="badge"]')).toHaveLength(2);
    expect(tags).toHaveTextContent('志愿服务社会实践');
    expect(screen.queryByRole('button', { name: '删除候选值 志愿服务' })).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '项目说明' }).nextElementSibling).toHaveTextContent(
      '-',
    );
    expect(screen.getByRole('heading', { name: '项目附件' }).nextElementSibling).toHaveTextContent(
      '-',
    );
  });

  it('总分使用数字且不累加子项，保留 null 与 0 的区别', async () => {
    let sent: ParticipationInput | null = null;
    vi.stubGlobal(
      'fetch',
      vi.fn((_url: string, options: RequestInit) => {
        sent = JSON.parse(
          typeof options.body === 'string' ? options.body : '',
        ) as ParticipationInput;
        return Promise.resolve(respond({ ...record, ...sent, revision: 2 }));
      }),
    );
    const input = {
      project_id: 1,
      student_id: 1,
      rank: null,
      score: 0,
      details: [{ name: '子项', description: '', score: null }],
    };
    await saveParticipation(first, record, input);
    expect(sent).toMatchObject({ score: 0, details: [{ score: null }] });
    await saveParticipation(first, record, { ...input, score: null });
    expect(sent).toMatchObject({ score: null });
  });

  it('新绑定的排名默认留空，提交为 null', async () => {
    const fetch = vi.fn((_url: string, options: RequestInit) => {
      const body = JSON.parse(
        typeof options.body === 'string' ? options.body : '',
      ) as ParticipationInput;

      // 新建响应使用服务端生成的初始版本，不回传请求中的零版本。
      return Promise.resolve(
        respond({ ...record, ...body, revision: 1, detail_count: body.details.length }),
      );
    });
    vi.stubGlobal('fetch', fetch);
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <QueryClientProvider client={client()}>
        <ParticipationEditor workspaceID={first} record={null} actions={[]} onClose={onClose} />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('spinbutton', { name: '排名（选填）' })).toHaveValue(null);
    await user.type(screen.getByRole('spinbutton', { name: '项目' }), '1');
    await user.type(screen.getByRole('spinbutton', { name: '学生' }), '1');
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalledOnce();
    });
    const options = fetch.mock.calls[0]?.[1];
    const body = JSON.parse(
      typeof options?.body === 'string' ? options.body : '',
    ) as ParticipationInput;
    expect(body).toMatchObject({ rank: null, revision: 0 });
  });

  it.each([0, -1, 1.5, 2_147_483_648])('拒绝无效排名 %s', (rank) => {
    const fetch = vi.fn();
    vi.stubGlobal('fetch', fetch);
    expect(() => saveParticipation(first, record, { ...record, rank })).toThrow('排名');
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each([true, false])(
    '子项仅选择项目配置，按列表权限查询：%s，保存保留独立总分',
    async (canList) => {
      const requests: { path: string; options: RequestInit }[] = [];
      vi.stubGlobal(
        'fetch',
        vi.fn((path: string, options: RequestInit) => {
          requests.push({ path, options });
          if (path.includes('/detail-names?'))
            return Promise.resolve(
              respond([
                { value: '已有子项', label: '已有子项' },
                { value: '配置子项', label: '配置子项' },
              ]),
            );
          if (path.includes('/students?'))
            return Promise.resolve(respond([{ value: 1, label: '虚构学生(00000001)' }]));
          if (path.includes('/options?'))
            return Promise.resolve(respond([{ value: 1, label: '2025 · 虚构年度项目' }]));
          return Promise.resolve(respond({ ...record, revision: 2 }));
        }),
      );
      const user = userEvent.setup();
      render(
        <QueryClientProvider client={client()}>
          <ParticipationEditor
            workspaceID={first}
            record={record}
            actions={canList ? ['project.participation.list'] : []}
            onClose={() => undefined}
          />
        </QueryClientProvider>,
      );
      const name = screen.getByRole('combobox', { name: '子项名称' });
      if (canList) {
        await user.click(name);
        const search = screen.getByRole('searchbox', { name: '搜索子项名称' });
        await user.type(search, '未提交');
        await user.clear(search);
        expect(search).toHaveValue('');
        expect(name).toHaveTextContent('已有子项');
        await user.type(search, '新建子项');
        expect(screen.queryByRole('option', { name: '新建“新建子项”' })).not.toBeInTheDocument();
        await user.clear(search);
        await user.click(await screen.findByRole('option', { name: '配置子项' }));
      } else {
        expect(name).toBeDisabled();
      }
      const rank = screen.getByRole('spinbutton', { name: '排名（选填）' });
      expect(rank).toHaveValue(3);
      await user.clear(rank);
      if (canList) await user.type(rank, '2');
      await user.click(screen.getByRole('button', { name: '保存' }));
      await waitFor(() => {
        expect(requests.some((item) => item.options.method === 'PUT')).toBe(true);
      });
      expect(requests.some((item) => item.path.includes('/detail-names?'))).toBe(canList);
      for (const { path } of requests.filter((item) => item.path.includes('/detail-names?'))) {
        expect(new URL(path, 'https://fixture.example').searchParams.get('project_id')).toBe('1');
      }
      const saved = requests.find((item) => item.options.method === 'PUT');
      const body = JSON.parse(
        typeof saved?.options.body === 'string' ? saved.options.body : '',
      ) as ParticipationInput;
      expect(body.score).toBe(90.123456);
      expect(body.rank).toBe(canList ? 2 : null);
      expect(body.details).toEqual([
        { name: canList ? '配置子项' : '已有子项', description: '虚构详细说明', score: 5.123456 },
      ]);
    },
  );

  it('项目编辑使用标签维护子项，支持添加、去重及移除', async () => {
    const project: Project = {
      allow_self_apply: false,
      self_apply_deadline: null,
      approval_steps: [],
      id: 1,
      name: '虚构年度项目',
      year: 2025,
      description: '',
      detail_names: ['已有子项'],
      attachment_ids: [],
      revision: 1,
      created_at: record.created_at,
      updated_at: record.updated_at,
    };
    const fetch = vi.fn((_path: string, options: RequestInit) => {
      const body: unknown = JSON.parse(typeof options.body === 'string' ? options.body : '');
      return Promise.resolve(respond({ ...project, ...(body as Project), revision: 2 }));
    });
    vi.stubGlobal('fetch', fetch);
    const onClose = vi.fn();
    render(
      <QueryClientProvider client={client()}>
        <ProjectEditor workspaceID={first} project={project} actions={[]} onClose={onClose} />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    const names = screen.getByRole('textbox', { name: '可选子项名称' });
    expect(screen.queryByRole('checkbox', { name: '申报后允许新增子项' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('tab', { name: '申报设置' }));
    expect(screen.queryByRole('combobox', { name: '自行申报截止时间' })).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: '申报后允许新增子项' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('checkbox', { name: '允许学生自行申报' }));
    expect(screen.queryByRole('checkbox', { name: '申报后允许新增子项' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: '自行申报截止时间' }));
    const dateDialog = await screen.findByRole('dialog', { name: '自行申报截止时间选择' });
    const dateInput = within(dateDialog).getByRole('textbox', { name: '输入日期' });
    await user.clear(dateInput);
    await user.type(dateInput, '2026-12-31');
    await user.click(within(dateDialog).getByRole('button', { name: '确定' }));
    expect(screen.getByRole('combobox', { name: '自行申报截止时间' })).toHaveTextContent(
      '2026年12月31日 23:59',
    );
    await user.click(screen.getByRole('button', { name: '添加审批级别' }));
    const allowEdit = screen.getByRole('checkbox', { name: '第 1 级允许编辑申报内容' });
    expect(allowEdit).toBeChecked();
    await user.click(allowEdit);
    await user.click(screen.getByRole('tab', { name: '主要信息' }));
    expect(screen.getByRole('button', { name: '删除候选值 已有子项' })).toBeVisible();
    await user.type(names, ' 已有子项 {Enter} 新配置子项 {Enter} 临时子项{Enter}');
    await user.click(screen.getByRole('button', { name: '删除候选值 临时子项' }));
    expect(screen.getAllByRole('button', { name: '删除候选值 已有子项' })).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(onClose).toHaveBeenCalledOnce();
    });
    const options = fetch.mock.calls[0]?.[1];
    const payload: unknown = JSON.parse(typeof options?.body === 'string' ? options.body : '');
    expect(payload).toMatchObject({
      detail_names: ['已有子项', '新配置子项'],
      approval_steps: [expect.objectContaining({ kind: 'applicant', allow_edit: false })],
      self_apply_deadline: '2026-12-31T15:59:00.000Z',
      revision: 1,
    });
    expect(payload).not.toHaveProperty('allow_add_details_after_submit');
  });

  it('切换项目清空子项名称并隔离候选项，保留已填说明和分数', async () => {
    const requests: { path: string; options: RequestInit }[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string, options: RequestInit) => {
        requests.push({ path, options });
        if (path.includes('/detail-names?')) {
          const secondProject =
            new URL(path, 'https://fixture.example').searchParams.get('project_id') === '2';
          const name = secondProject ? '另一项目子项' : '已有子项';
          return Promise.resolve(respond([{ value: name, label: name }]));
        }
        if (path.includes('/projects/options?')) {
          return Promise.resolve(
            respond([
              { value: 1, label: '2025 · 虚构年度项目' },
              { value: 2, label: '2026 · 另一项目' },
            ]),
          );
        }
        if (path.includes('/students?')) {
          return Promise.resolve(respond([{ value: 1, label: '虚构学生(00000001)' }]));
        }
        return Promise.resolve(
          respond({
            ...record,
            ...JSON.parse(typeof options.body === 'string' ? options.body : ''),
            revision: 2,
          }),
        );
      }),
    );
    render(
      <QueryClientProvider client={client()}>
        <ParticipationEditor
          workspaceID={first}
          record={record}
          actions={['project.list', 'project.participation.list']}
          onClose={() => undefined}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('combobox', { name: '项目' }));
    await user.click(await screen.findByRole('option', { name: '2026 · 另一项目' }));
    const name = screen.getByRole('combobox', { name: '子项名称' });
    expect(name).toHaveTextContent('选择子项名称');
    await user.click(name);
    expect(await screen.findByRole('option', { name: '另一项目子项' })).toBeVisible();
    expect(screen.queryByRole('option', { name: '已有子项' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('option', { name: '另一项目子项' }));
    await user.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => {
      expect(requests.some(({ options }) => options.method === 'PUT')).toBe(true);
    });
    const saved = requests.find(({ options }) => options.method === 'PUT');
    expect(
      JSON.parse(typeof saved?.options.body === 'string' ? saved.options.body : ''),
    ).toMatchObject({
      project_id: 2,
      score: record.score,
      details: [{ name: '另一项目子项', description: '虚构详细说明', score: 5.123456 }],
    });
  });

  it('附件申请和确认只发送元数据，文件通过 COS 直传', async () => {
    const calls: RequestInit[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((_path: string, options: RequestInit) => {
        calls.push(options);
        return Promise.resolve(
          respond(
            calls.length === 1
              ? preparedUpload()
              : {
                  id: 1,
                  project_id: 1,
                  filename: '虚构说明.txt',
                  category: 'project',
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
    await uploadAttachment(
      first,
      1,
      new File(['abc'], '虚构说明.pdf', { type: 'application/pdf' }),
    );
    expect(JSON.parse(typeof calls[0]?.body === 'string' ? calls[0].body : '')).toEqual({
      filename: '虚构说明.pdf',
      size: 3,
      content_type: 'application/pdf',
    });
    expect(JSON.parse(typeof calls[1]?.body === 'string' ? calls[1].body : '')).toEqual({
      upload_id: 1,
      revision: 1,
    });
    expect(new Headers(calls[0]?.headers).get('Content-Type')).toBe('application/json');
    expect(new Headers(calls[0]?.headers).get('X-CSRF-Protection')).toBe('1');
    expect(attachmentDownload(first, 1)).toBe(`/v1/workspaces/${first}/attachments/1/download`);
  });

  it('子项搜索仅在选择项目和年份后启用，清空范围或重置会清空关键词', async () => {
    const requests: string[] = [];
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) => {
        requests.push(path);
        if (path.includes('/filter-options'))
          return Promise.resolve(
            respond({
              names: [{ value: record.project_name, label: record.project_name }],
              years: [{ value: '2025', label: '2025 年' }],
              education_levels: [],
              majors: [],
              grades: [],
            }),
          );
        return Promise.resolve(respond({ items: [record], total: 1, page: 1, page_size: 5 }));
      }),
    );
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={client()}>
        <MemoryRouter>
          <ProjectPage
            workspaceID={first}
            workspaceName="虚构空间"
            actions={['project.participation.list']}
            refresh={() => Promise.resolve()}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await screen.findByText(record.student_name);
    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent);
    const rankIndex = headers.indexOf('排名');
    expect(rankIndex).toBeGreaterThan(-1);
    expect(headers[rankIndex + 1]).toBe('总分');
    expect(screen.getAllByRole('row')[1]?.children[rankIndex]).toHaveTextContent('3');
    const detail = screen.getByRole('textbox', { name: '子项名称或说明' });
    expect(detail).toBeDisabled();
    await user.click(screen.getByRole('combobox', { name: '项目名称' }));
    await user.click(await screen.findByRole('option', { name: record.project_name }));
    expect(detail).toBeDisabled();
    await user.click(screen.getByRole('combobox', { name: '归属年份' }));
    await user.click(screen.getByRole('option', { name: '2025 年' }));
    expect(detail).toBeEnabled();
    await waitFor(() => {
      const path = requests.filter((item) => item.includes('/project-participations?')).at(-1);
      const query = new URL(path ?? '', 'https://fixture.example').searchParams;
      expect(query.get('project_name')).toBe(record.project_name);
      expect(query.get('year')).toBe('2025');
      expect(query.get('page')).toBe('1');
    });
    await user.keyboard('{Escape}');
    await user.type(detail, '详细说明');
    await user.click(screen.getByRole('button', { name: '搜索' }));
    await waitFor(() => {
      expect(
        requests.some((path) => {
          const query = new URL(path, 'https://fixture.example').searchParams;
          return (
            query.get('project_name') === record.project_name &&
            query.get('year') === '2025' &&
            query.get('detail_keyword') === '详细说明'
          );
        }),
      ).toBe(true);
    });
    await user.click(screen.getByRole('combobox', { name: '归属年份' }));
    await user.click(screen.getByRole('option', { name: '全部年份' }));
    expect(detail).toBeDisabled();
    expect(detail).toHaveValue('');
    await waitFor(() => {
      const path = requests.filter((item) => item.includes('/project-participations?')).at(-1);
      const query = new URL(path ?? '', 'https://fixture.example').searchParams;
      expect(query.get('project_name')).toBe(record.project_name);
      expect(query.has('year')).toBe(false);
      expect(query.has('detail_keyword')).toBe(false);
    });
    await user.keyboard('{Escape}');
    const reset = screen.getByRole('button', { name: '重置' });
    await waitFor(() => {
      expect(reset).toBeEnabled();
    });
    await user.click(reset);
    expect(screen.getByRole('combobox', { name: '项目名称' })).toHaveTextContent('全部项目');
    await waitFor(() => {
      const path = requests.filter((item) => item.includes('/project-participations?')).at(-1);
      const query = new URL(path ?? '', 'https://fixture.example').searchParams;
      expect(query.get('page')).toBe('1');
      expect(query.has('project_name')).toBe(false);
      expect(query.has('year')).toBe(false);
      expect(query.has('detail_keyword')).toBe(false);
    });
  });

  it('同一空间搜索保留表格，切换空间不复用上一空间数据', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((path: string) => {
        if (path.includes('/filter-options'))
          return Promise.resolve(
            respond({
              names: [{ value: record.project_name, label: record.project_name }],
              years: [{ value: '2025', label: '2025 年' }],
              education_levels: [],
              majors: [],
              grades: [],
            }),
          );
        if (path.includes(second) || path.includes('student=missing'))
          return new Promise<Response>(() => undefined);
        return Promise.resolve(respond({ items: [record], total: 1, page: 1, page_size: 5 }));
      }),
    );
    const queryClient = client();
    const props = {
      workspaceID: first,
      workspaceName: '虚构空间甲',
      actions: ['project.participation.list'],
      refresh: () => Promise.resolve(),
    };
    const view = render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ProjectPage {...props} />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(await screen.findByText('虚构学生')).toBeInTheDocument();
    const user = userEvent.setup();
    await user.type(screen.getByRole('textbox', { name: '学号或姓名' }), 'missing');
    await user.click(screen.getByRole('button', { name: '搜索' }));
    expect(screen.getByText('虚构学生')).toBeInTheDocument();
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ProjectPage {...props} workspaceID={second} workspaceName="虚构空间乙" />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.queryByText('虚构学生')).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: '面包屑' })).toHaveTextContent(
      /^学生工作\/评奖评优\/获奖学生$/,
    );
  });
});
